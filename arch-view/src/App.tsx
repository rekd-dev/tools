import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import type { Edge, Node } from '@xyflow/react'
import { api } from './api'
import { emptyView } from './Diagram'
import { DepMatrix } from './DepMatrix'
import { GraphCanvas } from './GraphCanvas'
import { layoutArchitecture, layoutGraph, layoutGraphAsync, type ArchNodeData } from './layout'
import { findingFiles, findingInView, findingTouchesNode, nodesTouchedByFinding } from './findings'
import { displayName, evidenceNote, groupedRelations, relationPhrase, selectionKind, selectionTitle } from './selection'
import { NodeMenu } from './NodeMenu'
import { SourcePane } from './SourcePane'
import { nodeMenuActions } from './boxActions'
import { overlayParam, parentPath, parseHash, writeHash, type ViewerState } from './state'
import { guideKind, viewGuide } from './viewGuide'
import type { ChurnReport, Coverage, EntityDetail, Finding, FitnessReport, GraphView, Meta, RelationPick, SearchHit, View, ViewNode } from './types'

export function App() {
  const [state, setState] = useState<ViewerState>(() => parseHash())
  const path = state.path
  const [view, setView] = useState<View>(emptyView())
  const [graphView, setGraphView] = useState<GraphView | null>(null)
  const [fitness, setFitness] = useState<FitnessReport | null>(null)
  const [coverage, setCoverage] = useState<Coverage[]>([])
  const [meta, setMeta] = useState<Meta>({})
  const [error, setError] = useState<string | null>(null)
  const [source, setSource] = useState<{ file: string; text: string; line?: number } | null>(null)
  const [loading, setLoading] = useState(true)
  const [focus, setFocus] = useState<ViewNode | null>(null)
  const [entity, setEntity] = useState<EntityDetail | null>(null)
  const [showAllFindings, setShowAllFindings] = useState(false)
  const [query, setQuery] = useState('')
  const [hits, setHits] = useState<SearchHit[]>([])
  const [inspector, setInspector] = useState(() => {
    try {
      return localStorage.getItem('arch-view.inspector') !== '0'
    } catch {
      return true
    }
  })
  const [picked, setPicked] = useState<RelationPick | null>(null)
  const [churn, setChurn] = useState<ChurnReport | null>(null)
  const [elkRf, setElkRf] = useState<{ nodes: Node<ArchNodeData>[]; edges: Edge[] } | null>(null)
  const [menu, setMenu] = useState<{ id: string; x: number; y: number } | null>(null)
  const loadGen = useRef(0)
  const searchRef = useRef<HTMLInputElement>(null)

  const update = useCallback((patch: Partial<ViewerState>) => {
    setState((s) => {
      const next = { ...s, ...patch, overlays: patch.overlays ?? s.overlays }
      writeHash(next, patch.path !== undefined && patch.path !== s.path)
      return next
    })
  }, [])

  useEffect(() => {
    const onHash = () => setState(parseHash())
    window.addEventListener('hashchange', onHash)
    window.addEventListener('popstate', onHash)
    return () => {
      window.removeEventListener('hashchange', onHash)
      window.removeEventListener('popstate', onHash)
    }
  }, [])

  const load = useCallback(async (opts?: { full?: boolean }) => {
    const gen = ++loadGen.current
    setLoading(true)
    setError(null)
    try {
      if (opts?.full) {
        const [f, m, c] = await Promise.all([api.fitness(), api.meta(), api.coverage().catch(() => [])])
        if (gen !== loadGen.current) return
        setFitness(f)
        setMeta(m)
        setCoverage(c)
      }
      if (state.lens === 'architecture') {
        const v = await api.view(path)
        if (gen !== loadGen.current) return
        setView(v)
        setGraphView(null)
      } else {
        const gv = await api.graphView({
          lens: state.lens,
          path,
          sel: entitySel(state.sel),
          root: focus?.fileRoot || view.fileRoot || '',
          overlays: overlayParam(state),
        })
        if (gen !== loadGen.current) return
        setGraphView(gv)
      }
    } catch (e) {
      if (gen !== loadGen.current) return
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      if (gen === loadGen.current) setLoading(false)
    }
  }, [path, state.lens, state.overlays.data, state.overlays.async, state.overlays.deps, state.overlays.unresolved, state.lens === 'architecture' ? '' : state.sel, state.lens === 'architecture' ? '' : focus?.fileRoot || view.fileRoot || ''])

  useEffect(() => {
    void load({ full: !fitness })
  }, [load])

  useEffect(() => {
    setPicked(null)
    setMenu(null)
  }, [path, state.lens])

  useEffect(() => {
    setFocus(null)
  }, [path])

  useEffect(() => {
    if (state.lens === 'architecture') return
    if (entitySel(state.sel)) return
    if (graphView?.selection) update({ sel: graphView.selection })
  }, [state.lens, state.sel, graphView?.selection, update])

  useEffect(() => {
    if (state.lens === 'focus' && !loading && (graphView?.nodes?.length ?? 0) === 0) {
      searchRef.current?.focus()
    }
  }, [state.lens, loading, graphView])

  useEffect(() => {
    if (!state.sel || !state.sel.includes(':')) {
      setEntity(null)
      return
    }
    void api.entity(state.sel).then(setEntity).catch(() => setEntity(null))
  }, [state.sel])

  useEffect(() => {
    if (state.lens !== 'architecture' || state.view === 'matrix' || !state.overlays.churn) {
      setChurn(null)
      return
    }
    let cancelled = false
    void api
      .churn(path)
      .then((r) => {
        if (!cancelled) setChurn(r)
      })
      .catch(() => {
        if (!cancelled) setChurn(null)
      })
    return () => {
      cancelled = true
    }
  }, [state.lens, state.view, state.overlays.churn, path])

  const viewForLayout = useMemo(() => {
    if (!churn?.nodes) return view
    return {
      ...view,
      nodes: (view.nodes ?? []).map((n) => {
        const s = churn.nodes[n.id]
        if (!s) return n
        return { ...n, churnCommits: s.commits, churnAuthors: s.authors, churnLast: s.last }
      }),
    }
  }, [view, churn])

  const crumbs = path ? path.split('/').filter(Boolean) : []

  const viewFindings = useMemo(() => {
    const all = fitness?.findings ?? []
    const nodes = view.nodes ?? []
    if (!path) return all
    return all.filter((f) => findingInView(f, view.fileRoot, nodes))
  }, [fitness, path, view.fileRoot, view.nodes])

  const scopedFindings = useMemo(() => {
    if (showAllFindings) return fitness?.findings ?? []
    if (focus) return viewFindings.filter((f) => findingTouchesNode(f, focus))
    return viewFindings
  }, [fitness, showAllFindings, focus, viewFindings])

  const nodeIssues = useMemo(() => {
    const all = fitness?.findings ?? []
    const map: Record<string, 'error' | 'warning'> = {}
    for (const n of (state.lens === 'architecture' ? view.nodes : graphView?.nodes) ?? []) {
      const hitsF = all.filter((f) => findingTouchesNode(f, n))
      if (n.cycle || hitsF.some((h) => h.severity === 'error')) map[n.id] = 'error'
      else if (hitsF.some((h) => h.severity === 'warning')) map[n.id] = 'warning'
    }
    return map
  }, [fitness, view.nodes, graphView, path, state.lens])

  const rf = useMemo(() => {
    if (state.lens === 'architecture') return layoutArchitecture(viewForLayout, nodeIssues)
    if (!graphView) return { nodes: [] as Node<ArchNodeData>[], edges: [] as Edge[] }
    if (state.lens === 'flow' && elkRf) return elkRf
    return layoutGraph(
      graphView.nodes ?? [],
      graphView.edges ?? [],
      nodeIssues,
      graphView.selection || state.sel,
      state.lens === 'flow' ? 'flow' : 'radial',
    )
  }, [state.lens, state.sel, viewForLayout, graphView, nodeIssues, elkRf])

  useEffect(() => {
    if (state.lens !== 'flow' || !graphView) {
      setElkRf(null)
      return
    }
    let cancel = false
    void layoutGraphAsync(
      graphView.nodes ?? [],
      graphView.edges ?? [],
      nodeIssues,
      graphView.selection || state.sel,
      'flow',
    ).then((r) => {
      if (!cancel) setElkRf(r)
    })
    return () => {
      cancel = true
    }
  }, [state.lens, graphView, nodeIssues, state.sel])

  const painted = useMemo(
    () => ({
      nodes: rf.nodes.map((n) => ({ ...n, selected: n.id === focus?.id })),
      edges: rf.edges.map((e) => ({
        ...e,
        selected: Boolean(picked && e.source === picked.from && e.target === picked.to),
      })),
    }),
    [rf, focus?.id, picked],
  )

  const currentNodes = state.lens === 'architecture' ? viewForLayout.nodes : graphView?.nodes ?? []
  const emptyCanvas =
    !loading &&
    (state.lens === 'architecture' ? (view.nodes?.length ?? 0) === 0 : (graphView?.nodes?.length ?? 0) === 0)
  const selectedLabel = entity?.node?.name || focus?.label || (state.sel.includes(':') ? displayName(state.sel) : '')
  const guide = viewGuide({
    lens: state.lens,
    view: state.view,
    path,
    sel: state.sel,
    overlays: state.overlays,
    empty: emptyCanvas,
    truncated: graphView?.truncated,
    selectedKind: guideKind(state.sel, focus),
    selectedLabel,
    picked: Boolean(picked),
  })

  const persistInspector = (next: boolean) => {
    setInspector(next)
    try {
      localStorage.setItem('arch-view.inspector', next ? '1' : '0')
    } catch {
      /* ignore */
    }
  }

  const showInspector = () => {
    if (!inspector) persistInspector(true)
  }

  const nodeLabel = (id: string) => currentNodes.find((n) => n.id === id)?.label || displayName(id) || id

  const onPickEdge = (id: string, from: string, to: string) => {
    const ge = (graphView?.edges ?? []).find((e) => e.id === id || (e.from === from && e.to === to))
    const ve = (view.edges ?? []).find((e) => e.from === from && e.to === to)
    const storeId = ge?.id
    const next: RelationPick = {
      id: storeId,
      from,
      to,
      kind: ge?.kind || ve?.kind,
      file: ge?.file,
      line: ge?.line,
      unresolved: ge?.unresolved,
      source: ge?.source,
      detail: ge?.detail,
      confidence: ge?.confidence,
    }
    setPicked(next)
    const src = currentNodes.find((n) => n.id === from)
    if (src) setFocus(src)
    showInspector()
    if (storeId && !storeId.includes('->')) {
      void api
        .edge(storeId)
        .then((d) => {
          setPicked((cur) =>
            cur && cur.id === storeId
              ? {
                  ...cur,
                  kind: d.kind || cur.kind,
                  file: d.file || cur.file,
                  line: d.line || cur.line,
                  unresolved: d.unresolved ?? cur.unresolved,
                  source: d.source || cur.source,
                  detail: d.detail || cur.detail,
                  confidence: d.confidence ?? cur.confidence,
                  evidence: d.evidence,
                }
              : cur,
          )
        })
        .catch(() => {
          /* inventory edges without graph ids stay as the projection */
        })
    }
  }

  const onSelectNode = (id: string) => {
    setPicked(null)
    const n = currentNodes.find((x) => x.id === id)
    if (n) setFocus(n)
    if (state.lens === 'architecture') update({ sel: entitySel(id) })
    else update({ sel: id })
  }

  const onFinding = (f: Finding) => {
    const touched = nodesTouchedByFinding(f, currentNodes)
    if (touched[0]) setFocus(touched[0])
    if (touched.length >= 2) {
      const from = touched[0].id
      const to = touched[1].id
      const hit =
        (state.lens === 'architecture' ? view.edges : graphView?.edges ?? []).find(
          (e) => (e.from === from && e.to === to) || (e.from === to && e.to === from),
        ) ?? null
      if (hit) {
        const extra = hit as { id?: string }
        onPickEdge(typeof extra.id === 'string' && extra.id ? extra.id : `${hit.from}->${hit.to}`, hit.from, hit.to)
      } else setPicked({ from, to, kind: f.kind, file: f.from, detail: f.message })
    } else {
      setPicked(null)
    }
    showInspector()
  }

  const onDrill = (node: ViewNode) => {
    if (node.leaf || node.kind === 'type' || node.kind === 'method') {
      if (node.path) void openSource(node.path, node.line)
      return
    }
    const next = path ? `${path}/${node.id}` : node.id
    update({ path: next, sel: '' })
    setSource(null)
  }

  const openNodeMenu = (id: string, x: number, y: number) => {
    onSelectNode(id)
    setMenu({ id, x, y })
  }

  const runMenu = (action: string) => {
    const n = currentNodes.find((x) => x.id === menu?.id)
    setMenu(null)
    if (!n) return
    if (action === 'open') onDrill(n)
    if (action === 'focus') update({ lens: 'focus', sel: entitySel(n.id) })
    if (action === 'flow') update({ lens: 'flow', sel: entitySel(n.id) })
    if (action === 'copy') {
      void navigator.clipboard.writeText(n.label || n.id).catch(() => {
        /* ignore */
      })
    }
  }

  const toggleInspector = () => persistInspector(!inspector)

  const openSource = async (file: string, line?: number) => {
    try {
      const text = await api.source(file)
      setSource({ file, text, line })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  const goUp = useCallback(() => {
    if (!path) return
    update({ path: parentPath(path), sel: '' })
    setSource(null)
  }, [path, update])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      if (e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement) return
      if (menu) {
        setMenu(null)
        return
      }
      if (source) {
        setSource(null)
        return
      }
      goUp()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [goUp, source, menu])

  const goCrumb = (index: number) => {
    update({ path: crumbs.slice(0, index + 1).join('/'), sel: '' })
    setSource(null)
  }

  const onSearch = async (value: string) => {
    setQuery(value)
    if (value.trim().length < 2) {
      setHits([])
      return
    }
    try {
      setHits(await api.search(value))
    } catch {
      setHits([])
    }
  }

  return (
    <div className="h-full overflow-hidden flex flex-col bg-zinc-950 text-zinc-100">
      <header className="shrink-0 border-b border-zinc-800 px-4 py-2 flex items-center gap-3 flex-wrap">
        <h1 className="text-sm font-semibold tracking-tight">Architecture Viewer</h1>
        <span className="text-zinc-500 text-xs">
          {meta.repo ?? '—'} {meta.gitSha ? `@ ${meta.gitSha}` : ''}
        </span>
        <label className="text-xs text-zinc-400">
          Search
          <input
            value={query}
            ref={searchRef}
            onChange={(e) => void onSearch(e.target.value)}
            className="ml-2 h-8 w-56 rounded border border-zinc-700 bg-zinc-900 px-2 text-sm text-zinc-100"
            placeholder="interface, type, endpoint…"
          />
        </label>
        <div className="flex-1" />
        <LensBtn active={state.lens === 'architecture'} onClick={() => update({ lens: 'architecture' })}>
          Architecture
        </LensBtn>
        <LensBtn active={state.lens === 'focus'} onClick={() => update({ lens: 'focus', sel: entitySel(state.sel) })}>
          Focus
        </LensBtn>
        <LensBtn active={state.lens === 'flow'} onClick={() => update({ lens: 'flow', sel: entitySel(state.sel) })}>
          Flow
        </LensBtn>
        {state.lens === 'architecture' ? (
          <div className="flex items-center gap-2 pl-3 border-l border-zinc-700">
            <LensBtn active={state.view !== 'matrix'} onClick={() => update({ view: 'graph' })}>
              Graph
            </LensBtn>
            <LensBtn active={state.view === 'matrix'} onClick={() => update({ view: 'matrix' })}>
              Matrix
            </LensBtn>
            {state.view !== 'matrix' ? (
              <label className="text-xs text-zinc-400">
                <input
                  type="checkbox"
                  className="mr-1"
                  checked={state.overlays.churn}
                  onChange={(e) => update({ overlays: { ...state.overlays, churn: e.target.checked } })}
                />
                Churn
              </label>
            ) : null}
          </div>
        ) : null}
        {state.lens === 'flow' ? (
          <div className="flex items-center gap-3 pl-3 border-l border-zinc-700">
            <label className="text-xs text-zinc-400">
              <input
                type="checkbox"
                className="mr-1"
                checked={state.overlays.data}
                onChange={(e) => update({ overlays: { ...state.overlays, data: e.target.checked } })}
              />
              Data
            </label>
            <label className="text-xs text-zinc-400">
              <input
                type="checkbox"
                className="mr-1"
                checked={state.overlays.async}
                onChange={(e) => update({ overlays: { ...state.overlays, async: e.target.checked } })}
              />
              Async
            </label>
            <label className="text-xs text-zinc-400">
              <input
                type="checkbox"
                className="mr-1"
                checked={state.overlays.deps}
                onChange={(e) => update({ overlays: { ...state.overlays, deps: e.target.checked } })}
              />
              Deps
            </label>
            <label className="text-xs text-zinc-400">
              <input
                type="checkbox"
                className="mr-1"
                checked={state.overlays.unresolved}
                onChange={(e) => update({ overlays: { ...state.overlays, unresolved: e.target.checked } })}
              />
              Unresolved
            </label>
          </div>
        ) : null}
        <button
          type="button"
          className="h-8 px-3 text-sm rounded border border-zinc-700 hover:bg-zinc-900"
          onClick={() => void load({ full: true })}
        >
          Reanalyze
        </button>
        <button
          type="button"
          className={`h-8 px-3 text-sm rounded border ${inspector ? 'border-sky-500 bg-zinc-900' : 'border-zinc-700 hover:bg-zinc-900'}`}
          onClick={toggleInspector}
        >
          Inspector
        </button>
      </header>

      {(hits ?? []).length ? (
        <div className="shrink-0 border-b border-zinc-800 px-4 py-2 text-sm max-h-32 overflow-auto">
          {(hits ?? []).map((h) => (
            <button
              key={h.id}
              type="button"
              className="block w-full text-left py-1 hover:bg-zinc-900"
              onClick={() => {
                update({ lens: h.kind === 'endpoint' ? 'flow' : 'focus', sel: h.id })
                setHits([])
                setQuery(h.name)
              }}
            >
              <span className="text-zinc-400 mr-2">{h.kind}</span>
              {h.name}
              <span className="text-zinc-600 ml-2 text-xs">{h.file}</span>
            </button>
          ))}
        </div>
      ) : null}

      <nav className="shrink-0 border-b border-zinc-800 px-4 py-2 text-sm flex items-center gap-2 flex-wrap">
        <button
          type="button"
          className="h-7 px-2 text-xs rounded border border-zinc-700 hover:bg-zinc-900 disabled:opacity-40 disabled:hover:bg-transparent"
          disabled={!path}
          onClick={goUp}
        >
          ‹ Up
        </button>
        <Crumb active={crumbs.length === 0} onClick={() => update({ path: '', sel: '' })}>
          All modules
        </Crumb>
        {crumbs.map((c, i) => (
          <span key={`${c}-${i}`} className="flex items-center gap-1">
            <span className="text-zinc-600">/</span>
            <Crumb active={i === crumbs.length - 1} onClick={() => goCrumb(i)}>
              {c}
            </Crumb>
          </span>
        ))}
        <span className="ml-auto flex gap-2 text-[11px] text-zinc-500">
          {coverage.map((c) => (
            <span key={c.analyzer} title={c.message}>
              {c.analyzer}: {c.status}
            </span>
          ))}
        </span>
      </nav>

      <div className="flex-1 min-h-0 flex flex-col">
        <div className="flex-1 min-h-0 flex flex-col lg:flex-row">
        <main className="flex-1 min-w-0 min-h-0 flex flex-col px-3 py-2">
          <div className="shrink-0 mb-2 rounded border border-zinc-800 bg-zinc-900/50 px-3 py-2" role="status" aria-live="polite">
            <div className="text-[11px] font-medium uppercase tracking-wide text-zinc-500">{guide.lookingAt}</div>
            <p className="text-xs text-zinc-200 mt-0.5 leading-relaxed">{guide.meaning}</p>
            <p className="text-xs text-zinc-500 mt-0.5 leading-relaxed">{guide.lookFor}</p>
          </div>
          {loading ? <p className="shrink-0 text-zinc-500 text-sm mb-2">Loading…</p> : null}
          {error ? <p className="shrink-0 text-red-400 text-sm mb-2">{error}</p> : null}
          {graphView?.reason && state.lens !== 'architecture' ? (
            <p className="shrink-0 text-xs text-amber-300 mb-2">{graphView.reason}</p>
          ) : null}
          {churn?.message && state.overlays.churn && state.lens === 'architecture' && state.view !== 'matrix' ? (
            <p className="shrink-0 text-xs text-amber-300 mb-2">{churn.message}</p>
          ) : null}
          {!loading && state.lens === 'architecture' && (view.nodes?.length ?? 0) > 0 ? (
            <div className="shrink-0 flex flex-wrap gap-1 mb-2 max-h-16 overflow-auto">
              {view.nodes
                .slice()
                .sort((a, b) => a.label.localeCompare(b.label))
                .map((n) => (
                  <span key={n.id} className="inline-flex">
                    <button
                      type="button"
                      className={`h-7 px-2 text-xs rounded-l border ${
                        focus?.id === n.id ? 'border-sky-500 bg-zinc-900 text-zinc-100' : 'border-zinc-700 text-zinc-300 hover:bg-zinc-900'
                      }`}
                      onClick={() => onSelectNode(n.id)}
                      onDoubleClick={() => onDrill(n)}
                      onContextMenu={(e) => {
                        e.preventDefault()
                        openNodeMenu(n.id, e.clientX, e.clientY)
                      }}
                    >
                      {n.label}
                    </button>
                    <button
                      type="button"
                      className="h-7 px-1.5 text-xs rounded-r border border-l-0 border-zinc-700 text-zinc-500 hover:bg-zinc-900 hover:text-zinc-200"
                      aria-label={n.leaf ? `Open ${n.label}` : `Open ${n.label} folder`}
                      onClick={() => onDrill(n)}
                    >
                      {n.leaf ? '↗' : '›'}
                    </button>
                  </span>
                ))}
            </div>
          ) : null}
          <div className="flex-1 min-h-0 overflow-hidden">
            {!loading && state.lens === 'architecture' && (view.nodes?.length ?? 0) > 0 && state.view === 'matrix' ? (
              <DepMatrix
                nodes={view.nodes}
                edges={view.edges}
                cycles={view.cycles}
                focusId={focus?.id}
                pickedFrom={picked?.from}
                pickedTo={picked?.to}
                onSelect={onSelectNode}
                onEdge={(from, to) => onPickEdge(`${from}->${to}`, from, to)}
                onDrill={(id) => {
                  const n = view.nodes.find((x) => x.id === id)
                  if (n) onDrill(n)
                }}
                onMenu={openNodeMenu}
              />
            ) : null}
            {!loading && state.lens === 'architecture' && (view.nodes?.length ?? 0) > 0 && state.view !== 'matrix' ? (
              <GraphCanvas
                key={`arch:${path}`}
                nodes={painted.nodes}
                edges={painted.edges}
                onSelect={onSelectNode}
                onDrill={(id) => {
                  const n = view.nodes.find((x) => x.id === id)
                  if (n) onDrill(n)
                }}
                onUp={goUp}
                onEdge={onPickEdge}
                onMenu={openNodeMenu}
                onPane={() => setMenu(null)}
              />
            ) : null}
            {!loading && state.lens !== 'architecture' && (graphView?.nodes?.length ?? 0) > 0 ? (
              <GraphCanvas
                key={`${state.lens}:${state.sel}`}
                nodes={painted.nodes}
                edges={painted.edges}
                onSelect={onSelectNode}
                onDrill={(id) => {
                  const n = graphView?.nodes.find((x) => x.id === id)
                  if (n?.path) void openSource(n.path, n.line)
                }}
                onEdge={onPickEdge}
                onMenu={openNodeMenu}
                onPane={() => setMenu(null)}
              />
            ) : null}
            {!loading && state.lens === 'architecture' && (view.nodes?.length ?? 0) === 0 ? (
              <p className="text-zinc-500 text-sm">Nothing under this path. Run inventory + serve.</p>
            ) : null}
            {!loading && state.lens !== 'architecture' && (graphView?.nodes?.length ?? 0) === 0 && graphView?.reason ? (
              <p className="text-zinc-500 text-sm">{graphView.reason}</p>
            ) : null}
          </div>
        </main>

        <aside className={`${inspector ? 'lg:w-[24rem] max-h-[40vh] lg:max-h-none' : 'hidden'} lg:shrink-0 min-h-0 overflow-auto p-3 space-y-3 border-t lg:border-t-0 lg:border-l border-zinc-800`}>
          {picked ? (
            <section className="rounded-md border border-sky-900 bg-zinc-900 p-3">
              <div className="text-xs font-medium uppercase tracking-wide text-zinc-500">Relation</div>
              <div className="text-base font-semibold mt-1 break-words">
                {nodeLabel(picked.from)} → {nodeLabel(picked.to)}
              </div>
              <div className="text-xs text-zinc-400 mt-1">{relationPhrase(picked.kind, picked.unresolved)}</div>
              {picked.source ? (
                <div className="text-[11px] text-zinc-500 mt-1">
                  {picked.source}
                  {picked.confidence != null ? ` · ${Math.round(picked.confidence * 100)}%` : ''}
                </div>
              ) : null}
              {picked.detail ? <p className="text-xs text-zinc-400 mt-2 break-words">{picked.detail}</p> : null}
              {picked.file ? (
                <button
                  type="button"
                  className="mt-2 text-xs text-sky-400 hover:text-sky-300 break-all text-left"
                  onClick={() => void openSource(picked.file!, picked.line)}
                >
                  {picked.file}
                  {picked.line ? `:${picked.line}` : ''}
                </button>
              ) : null}
              {(picked.evidence ?? []).slice(0, 4).map((ev, i) => (
                <div key={`${ev.file ?? ''}:${ev.line ?? i}`} className="mt-2 text-[11px] text-zinc-400">
                  {ev.file ? (
                    <button
                      type="button"
                      className="text-sky-400 hover:text-sky-300 break-all text-left"
                      onClick={() => ev.file && void openSource(ev.file, ev.line)}
                    >
                      {ev.file}
                      {ev.line ? `:${ev.line}` : ''}
                    </button>
                  ) : null}
                  {ev.snippet ? <pre className="mt-1 whitespace-pre-wrap text-zinc-300">{ev.snippet}</pre> : null}
                  {ev.detail && ev.detail !== picked.detail ? <p className="mt-1">{ev.detail}</p> : null}
                </div>
              ))}
            </section>
          ) : null}
          {entity?.node?.id || focus ? (
            <section className="rounded-md border border-zinc-700 bg-zinc-900 p-3">
              <div className="text-xs font-medium uppercase tracking-wide text-zinc-500">Selected</div>
              <div className="text-base font-semibold mt-1 break-words">{selectionTitle(entity, focus)}</div>
              <div className="text-xs text-zinc-400 mt-1">{selectionKind(entity, focus) || (focus?.leaf ? 'File' : 'Folder')}</div>
              {focus?.churnCommits ? (
                <div className="text-xs text-orange-300 mt-2">
                  {focus.churnCommits} commits in 90 days
                  {focus.churnAuthors ? ` · ${focus.churnAuthors} authors` : ''}
                  {focus.churnLast ? ` · last ${focus.churnLast}` : ''}
                </div>
              ) : null}
              {entity?.node?.file || focus?.path ? (
                <button
                  type="button"
                  className="mt-1 text-xs text-sky-400 hover:text-sky-300 break-all text-left"
                  onClick={() => {
                    const file = entity?.node?.file || focus?.path
                    if (file) void openSource(file)
                  }}
                >
                  {entity?.node?.file || focus?.path}
                </button>
              ) : null}
              {entity?.implementers?.length ? (
                <div className="mt-3 text-xs">
                  <div className="text-zinc-500 mb-0.5">Implemented by</div>
                  {entity.implementers.map((i) => (
                    <button key={i.id} type="button" className="block text-sky-400" onClick={() => update({ sel: i.id, lens: 'focus' })}>
                      {i.name || i.label || i.id}
                    </button>
                  ))}
                </div>
              ) : null}
              {groupedRelations(entity?.outgoing, 'out').map((g) => (
                <RelList key={'out-' + g.label} group={g} />
              ))}
              {groupedRelations(entity?.incoming, 'in')
                .filter((g) => g.label !== 'Defined in')
                .slice(0, 4)
                .map((g) => (
                  <RelList key={'in-' + g.label} group={g} />
                ))}
              {(() => {
                const note = evidenceNote(entity)
                return note ? <p className="mt-3 text-[11px] text-zinc-500">{note}</p> : null
              })()}
              {!entity?.node?.id && focus ? (
                <p className="mt-3 text-xs text-zinc-500">Double-click or › opens this folder. Right-click for Focus or Flow.</p>
              ) : null}
            </section>
          ) : picked ? null : (
            <section className="rounded-md border border-zinc-800 p-3 text-sm text-zinc-400">
              Click a box to see what it is and what it connects to. Search for a type to open Focus. Click a line for the relation.
            </section>
          )}

          {!loading && state.lens === 'architecture' && view.cycles?.length ? (
            <section className="rounded-md border border-red-900 bg-red-950/40 p-3 text-sm text-red-200">
              <div className="font-semibold mb-1">Circular dependencies here</div>
              <ul className="space-y-1 font-mono text-xs">
                {view.cycles.map((c) => (
                  <li key={c}>{c.replace(/->/g, ' → ')}</li>
                ))}
              </ul>
            </section>
          ) : null}

          {fitness ? (
            <section className="rounded-md border border-zinc-800 p-3">
              <div className="flex items-center justify-between gap-2 mb-2">
                <div className="text-sm font-semibold">
                  Problems
                  <span className="ml-1 font-normal text-zinc-500">
                    {showAllFindings ? 'in repo' : focus ? `in ${focus.label}` : path ? `in ${path}` : 'in repo'}
                  </span>
                </div>
                {path || focus ? (
                  <button
                    type="button"
                    className="text-xs text-zinc-400 hover:text-zinc-200"
                    onClick={() => {
                      if (focus && !showAllFindings) {
                        setFocus(null)
                        setPicked(null)
                      } else setShowAllFindings((v) => !v)
                    }}
                  >
                    {showAllFindings ? 'This folder' : focus ? (path ? `All of ${path}` : 'All of repo') : 'Show all'}
                  </button>
                ) : null}
              </div>
              <div className="grid grid-cols-3 gap-2 text-xs mb-3">
                <Stat label="Errors" value={countSev(scopedFindings, 'error')} bad />
                <Stat label="Warnings" value={countSev(scopedFindings, 'warning')} />
                <Stat
                  label={focus && !showAllFindings ? 'Other here' : 'Rest of repo'}
                  value={Math.max(0, (focus && !showAllFindings ? viewFindings : fitness.findings ?? []).length - scopedFindings.length)}
                />
              </div>
              {scopedFindings.length === 0 ? (
                <p className="text-xs text-zinc-500">
                  {focus
                    ? `No issues in ${focus.label}.${viewFindings.length ? ` ${viewFindings.length} in this folder belong to other boxes.` : ''}`
                    : 'No issues in this folder.'}
                </p>
              ) : (
                <ul className="space-y-2">
                  {scopedFindings.slice(0, 30).map((f) => {
                    const card = explainFinding(f)
                    const files = findingFiles(f).filter((p, i, all) => all.indexOf(p) === i)
                    return (
                      <li key={f.id}>
                        <div
                          className={`w-full rounded border px-2 py-1.5 text-left text-xs ${
                            f.severity === 'error'
                              ? 'border-red-900 bg-red-950/30'
                              : f.severity === 'info'
                                ? 'border-zinc-700 bg-zinc-900/60'
                                : 'border-amber-900/60 bg-amber-950/20'
                          }`}
                        >
                          <button type="button" className="w-full text-left" onClick={() => onFinding(f)}>
                            <div
                              className={
                                f.severity === 'error'
                                  ? 'text-red-200 font-medium'
                                  : f.severity === 'info'
                                    ? 'text-zinc-300 font-medium'
                                    : 'text-amber-200 font-medium'
                              }
                            >
                              {card.title}
                            </div>
                            <div className="text-zinc-400 mt-0.5 break-words leading-relaxed">{card.detail}</div>
                          </button>
                          {files.length ? (
                            <div className="mt-1.5 flex flex-wrap gap-1">
                              {files.map((file) => (
                                <button
                                  key={file}
                                  type="button"
                                  className="text-[11px] text-sky-400 hover:text-sky-300 truncate max-w-full"
                                  onClick={() => void openSource(file)}
                                >
                                  {shortName(file)}
                                </button>
                              ))}
                            </div>
                          ) : null}
                        </div>
                      </li>
                    )
                  })}
                </ul>
              )}
            </section>
          ) : null}

        </aside>
        </div>
        {source ? <SourcePane file={source.file} text={source.text} line={source.line} onClose={() => setSource(null)} /> : null}
      </div>
      {menu ? (
        <NodeMenu
          x={menu.x}
          y={menu.y}
          items={nodeMenuActions(
            currentNodes.find((n) => n.id === menu.id) ?? {
              id: menu.id,
              label: menu.id,
              layer: 0,
              leaf: false,
              abstract: false,
              cycle: false,
            },
            state.lens,
          )}
          onPick={runMenu}
          onClose={() => setMenu(null)}
        />
      ) : null}
    </div>
  )
}

function RelList({ group }: { group: { label: string; names: string[] } }) {
  const shown = group.names.slice(0, 8)
  const extra = group.names.length - shown.length
  return (
    <div className="mt-3 text-xs">
      <div className="text-zinc-500 mb-0.5">{group.label}</div>
      <div className="text-zinc-200 leading-relaxed break-words">{shown.join(', ')}</div>
      {extra > 0 ? <div className="text-zinc-500">+{extra} more</div> : null}
    </div>
  )
}

function LensBtn({ active, onClick, children }: { active: boolean; onClick: () => void; children: string }) {
  return (
    <button
      type="button"
      className={`h-8 px-3 text-sm rounded border ${active ? 'border-sky-500 bg-zinc-900' : 'border-zinc-700 hover:bg-zinc-900'}`}
      onClick={onClick}
    >
      {children}
    </button>
  )
}

function Crumb({ children, active, onClick }: { children: string; active: boolean; onClick: () => void }) {
  if (active) return <span className="text-zinc-100 font-medium">{children}</span>
  return (
    <button type="button" className="text-sky-400 hover:text-sky-300" onClick={onClick}>
      {children}
    </button>
  )
}

function Stat({ label, value, bad }: { label: string; value: number; bad?: boolean }) {
  return (
    <div className="rounded bg-zinc-900 px-2 py-1.5">
      <div className="text-zinc-500">{label}</div>
      <div className={bad && value > 0 ? 'text-red-300 font-semibold' : 'text-zinc-100 font-semibold'}>{value}</div>
    </div>
  )
}

function countSev(findings: Finding[], sev: string) {
  return findings.filter((f) => f.severity === sev).length
}

function explainFinding(f: Finding): { title: string; detail: string } {
  const from = shortName(f.from)
  const to = shortName(f.to)
  switch (f.kind) {
    case 'cycle':
      return { title: 'Circular dependency', detail: (f.cycle ?? f.message).replace(/->/g, ' → ') }
    case 'layer_violation':
      return {
        title: f.relatedCount && f.relatedCount > 1 ? `Depends the wrong way · ${f.relatedCount} imports` : 'Depends the wrong way',
        detail: from && to ? `${from} must not depend on ${to}. Inner layers cannot import outer ones.` : f.message,
      }
    case 'layer_violation_type':
      return {
        title: f.relatedCount && f.relatedCount > 1 ? `Type-only import · ${f.relatedCount}` : 'Type-only import',
        detail: from && to ? `${from} imports a type from ${to}. Erased at compile time; not a runtime dependency.` : f.message,
      }
    case 'domain_purity':
      return { title: 'Framework code in an inner layer', detail: f.message }
    case 'unlayered':
      return { title: 'Not in a known architecture folder', detail: shortName(f.from) || f.message }
    default:
      return { title: f.kind, detail: f.message }
  }
}

function entitySel(id: string) {
  return id.includes(':') ? id : ''
}

function shortName(id?: string) {
  if (!id) return ''
  const slash = id.replace(/\\/g, '/').split('/')
  return slash[slash.length - 1] ?? id
}
