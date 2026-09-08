import { useCallback, useEffect, useMemo, useState } from 'react'
import type { Edge, Node } from '@xyflow/react'
import { api } from './api'
import { emptyView, layerPhrase } from './Diagram'
import { GraphCanvas } from './GraphCanvas'
import { layoutArchitecture, layoutGraph, type ArchNodeData } from './layout'
import { overlayParam, parseHash, writeHash, type ViewerState } from './state'
import type { Coverage, EntityDetail, Finding, FitnessReport, GraphView, Meta, SearchHit, View, ViewNode } from './types'

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

  const update = useCallback((patch: Partial<ViewerState>) => {
    setState((s) => {
      const next = { ...s, ...patch, overlays: patch.overlays ?? s.overlays }
      writeHash(next)
      return next
    })
  }, [])

  useEffect(() => {
    const onHash = () => setState(parseHash())
    window.addEventListener('hashchange', onHash)
    return () => window.removeEventListener('hashchange', onHash)
  }, [])

  const load = useCallback(async (opts?: { full?: boolean }) => {
    setLoading(true)
    setError(null)
    try {
      if (opts?.full) {
        const [f, m, c] = await Promise.all([api.fitness(), api.meta(), api.coverage().catch(() => [])])
        setFitness(f)
        setMeta(m)
        setCoverage(c)
      }
      if (state.lens === 'architecture') {
        const v = await api.view(path)
        setView(v)
        setGraphView(null)
      } else {
        const gv = await api.graphView({
          lens: state.lens,
          path,
          sel: state.sel,
          overlays: overlayParam(state),
        })
        setGraphView(gv)
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [path, state.lens, state.sel, state.overlays.data, state.overlays.async])

  useEffect(() => {
    void load({ full: !fitness })
  }, [load])

  useEffect(() => {
    setFocus(null)
    setEntity(null)
  }, [path, state.lens])

  useEffect(() => {
    if (!state.sel) {
      setEntity(null)
      return
    }
    void api.entity(state.sel).then(setEntity).catch(() => setEntity(null))
  }, [state.sel])

  const crumbs = path ? path.split('/').filter(Boolean) : []

  const scopedFindings = useMemo(() => {
    const all = fitness?.findings ?? []
    const nodes = view.nodes ?? []
    if (focus && !showAllFindings) {
      return all.filter((f) => findingTouchesNode(f, focus, path))
    }
    if (!path || showAllFindings) return all
    return all.filter((f) => findingInScope(f, path, nodes))
  }, [fitness, path, showAllFindings, view.nodes, focus])

  const nodeIssues = useMemo(() => {
    const all = fitness?.findings ?? []
    const map: Record<string, 'error' | 'warning'> = {}
    for (const n of (state.lens === 'architecture' ? view.nodes : graphView?.nodes) ?? []) {
      const hitsF = all.filter((f) => findingTouchesNode(f, n, path))
      if (n.cycle || hitsF.some((h) => h.severity === 'error')) map[n.id] = 'error'
      else if (hitsF.some((h) => h.severity === 'warning')) map[n.id] = 'warning'
    }
    return map
  }, [fitness, view.nodes, graphView, path, state.lens])

  const rf = useMemo(() => {
    if (state.lens === 'architecture') return layoutArchitecture(view, nodeIssues)
    if (!graphView) return { nodes: [] as Node<ArchNodeData>[], edges: [] as Edge[] }
    return layoutGraph(graphView.nodes ?? [], graphView.edges ?? [], nodeIssues)
  }, [state.lens, view, graphView, nodeIssues])

  const onDrill = (node: ViewNode) => {
    if (node.leaf || node.kind === 'type' || node.kind === 'method') {
      if (node.path) void openSource(node.path, node.line)
      return
    }
    const next = path ? `${path}/${node.id}` : node.id
    update({ path: next, sel: '' })
    setSource(null)
  }

  const openSource = async (file: string, line?: number) => {
    try {
      const text = await api.source(file)
      setSource({ file, text, line })
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

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
    <div className="min-h-screen flex flex-col bg-zinc-950 text-zinc-100">
      <header className="border-b border-zinc-800 px-4 py-2 flex items-center gap-3 flex-wrap">
        <h1 className="text-sm font-semibold tracking-tight">Architecture Viewer</h1>
        <span className="text-zinc-500 text-xs">
          {meta.repo ?? '—'} {meta.gitSha ? `@ ${meta.gitSha}` : ''}
        </span>
        <label className="text-xs text-zinc-400">
          Search
          <input
            value={query}
            onChange={(e) => void onSearch(e.target.value)}
            className="ml-2 h-8 w-56 rounded border border-zinc-700 bg-zinc-900 px-2 text-sm text-zinc-100"
            placeholder="interface, type, endpoint…"
          />
        </label>
        <div className="flex-1" />
        <LensBtn active={state.lens === 'architecture'} onClick={() => update({ lens: 'architecture' })}>
          Architecture
        </LensBtn>
        <LensBtn active={state.lens === 'focus'} onClick={() => update({ lens: 'focus' })}>
          Focus
        </LensBtn>
        <LensBtn active={state.lens === 'flow'} onClick={() => update({ lens: 'flow' })}>
          Flow
        </LensBtn>
        {state.lens === 'flow' ? (
          <>
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
          </>
        ) : null}
        <button
          type="button"
          className="h-8 px-3 text-sm rounded border border-zinc-700 hover:bg-zinc-900"
          onClick={() => void load({ full: true })}
        >
          Reanalyze
        </button>
      </header>

      {hits.length ? (
        <div className="border-b border-zinc-800 px-4 py-2 text-sm max-h-40 overflow-auto">
          {hits.map((h) => (
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

      <nav className="border-b border-zinc-800 px-4 py-2 text-sm flex items-center gap-1 flex-wrap">
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

      <div className="grid grid-cols-12 gap-0 flex-1 min-h-0">
        <main className="col-span-12 lg:col-span-8 overflow-auto p-4 border-r border-zinc-800">
          <p className="text-sm text-zinc-300 mb-1">
            {state.lens === 'architecture'
              ? path
                ? `Inside ${path}: each node is a child. Outer rows depend on the rows below.`
                : 'Each node is a package or app. Double-click a folder to drill in.'
              : state.lens === 'focus'
                ? 'Focus shows implementations, bindings, and consumers around the selected type.'
                : 'Flow is a bounded static call path. Toggle data and async overlays without losing selection.'}
          </p>
          <p className="text-xs text-zinc-500 mb-4">
            Click to select. Double-click a folder to drill, or a file/type to open source. Unresolved edges are dashed.
          </p>
          {loading ? <p className="text-zinc-500 text-sm">Loading…</p> : null}
          {error ? <p className="text-red-400 text-sm mb-2">{error}</p> : null}
          {graphView?.reason ? <p className="text-xs text-amber-300 mb-2">{graphView.reason}</p> : null}
          {!loading && state.lens === 'architecture' && (view.nodes?.length ?? 0) > 0 ? (
            <GraphCanvas
              nodes={rf.nodes}
              edges={rf.edges}
              onSelect={(id) => {
                const n = view.nodes.find((x) => x.id === id)
                if (n) setFocus(n)
                update({ sel: id })
              }}
              onDrill={(id) => {
                const n = view.nodes.find((x) => x.id === id)
                if (n) onDrill(n)
              }}
            />
          ) : null}
          {!loading && state.lens !== 'architecture' && (graphView?.nodes?.length ?? 0) > 0 ? (
            <GraphCanvas
              nodes={rf.nodes}
              edges={rf.edges}
              onSelect={(id) => update({ sel: id })}
              onDrill={(id) => {
                const n = graphView?.nodes.find((x) => x.id === id)
                if (n?.path) void openSource(n.path, n.line)
              }}
            />
          ) : null}
          {!loading && state.lens === 'architecture' && (view.nodes?.length ?? 0) === 0 ? (
            <p className="text-zinc-500 text-sm">Nothing under this path. Run inventory + serve.</p>
          ) : null}
          {!loading && view.cycles?.length ? (
            <div className="mt-4 rounded-md border border-red-900 bg-red-950/40 p-3 text-sm text-red-200">
              <div className="font-semibold mb-1">Circular dependencies here</div>
              <ul className="space-y-1 font-mono text-xs">
                {view.cycles.map((c) => (
                  <li key={c}>{c.replace(/->/g, ' → ')}</li>
                ))}
              </ul>
            </div>
          ) : null}
        </main>

        <aside className="col-span-12 lg:col-span-4 overflow-auto p-4 space-y-3">
          {entity?.node?.id || focus ? (
            <section className="rounded-md border border-zinc-700 bg-zinc-900 p-3">
              <div className="text-xs font-medium uppercase tracking-wide text-zinc-500">Selected</div>
              <div className="text-base font-semibold mt-1 break-words">{entity?.node?.name || focus?.label}</div>
              <div className="text-xs text-zinc-400 mt-1 space-y-0.5">
                {focus?.archLayer ? <div>{layerPhrase(focus.archLayer)}</div> : null}
                {state.sel ? <div className="font-mono break-all text-zinc-500">{state.sel}</div> : null}
              </div>
              {entity?.implementers?.length ? (
                <div className="mt-2 text-xs">
                  <div className="text-zinc-500">Implementers</div>
                  {entity.implementers.map((i) => (
                    <button key={i.id} type="button" className="block text-sky-400" onClick={() => update({ sel: i.id, lens: 'focus' })}>
                      {i.name || i.label || i.id}
                    </button>
                  ))}
                </div>
              ) : null}
              {entity?.bindings?.length ? (
                <div className="mt-2 text-xs text-zinc-400">
                  Bindings: {entity.bindings.map((b) => b.detail || b.kind).join(', ')}
                </div>
              ) : null}
              {(entity?.outgoing ?? []).slice(0, 8).map((e) => (
                <button
                  key={e.id || e.from + e.to + e.kind}
                  type="button"
                  className="block mt-1 text-left text-xs text-zinc-400 hover:text-zinc-200"
                  onClick={() => {
                    if (e.file) void openSource(e.file, e.line)
                  }}
                >
                  {e.kind} → {shortName(e.to)} {e.source ? `(${e.source}${e.confidence != null ? ` ${e.confidence.toFixed(2)}` : ''})` : ''}
                  {e.unresolved ? ' unresolved' : ''}
                </button>
              ))}
            </section>
          ) : (
            <section className="rounded-md border border-zinc-800 p-3 text-sm text-zinc-400">
              Search or click a node. Compiler-resolved facts show higher confidence than heuristic ones.
            </section>
          )}

          {fitness ? (
            <section className="rounded-md border border-zinc-800 p-3">
              <div className="flex items-center justify-between gap-2 mb-2">
                <div className="text-sm font-semibold">Problems</div>
                {path ? (
                  <button type="button" className="text-xs text-zinc-400 hover:text-zinc-200" onClick={() => setShowAllFindings((v) => !v)}>
                    {showAllFindings ? 'This view only' : 'Show all'}
                  </button>
                ) : null}
              </div>
              <div className="grid grid-cols-3 gap-2 text-xs mb-3">
                <Stat label="Errors" value={countSev(scopedFindings, 'error')} bad />
                <Stat label="Warnings" value={countSev(scopedFindings, 'warning')} />
                <Stat label="Hidden" value={Math.max(0, (fitness.findings ?? []).length - scopedFindings.length)} />
              </div>
              {scopedFindings.length === 0 ? (
                <p className="text-xs text-zinc-500">No issues in this scope.</p>
              ) : (
                <ul className="space-y-2">
                  {scopedFindings.slice(0, 30).map((f) => {
                    const card = explainFinding(f)
                    return (
                      <li
                        key={f.id}
                        className={`rounded border px-2 py-1.5 text-xs ${
                          f.severity === 'error' ? 'border-red-900 bg-red-950/30' : 'border-amber-900/60 bg-amber-950/20'
                        }`}
                      >
                        <div className={f.severity === 'error' ? 'text-red-200 font-medium' : 'text-amber-200 font-medium'}>{card.title}</div>
                        <div className="text-zinc-400 mt-0.5 break-words leading-relaxed">{card.detail}</div>
                      </li>
                    )
                  })}
                </ul>
              )}
            </section>
          ) : null}

          {source ? (
            <section className="rounded-md border border-zinc-800 p-3">
              <div className="flex items-center justify-between mb-2 gap-2">
                <div className="text-xs font-semibold truncate">
                  {source.file}
                  {source.line ? `:${source.line}` : ''}
                </div>
                <button type="button" className="text-xs text-zinc-400 hover:text-zinc-200" onClick={() => setSource(null)}>
                  Close
                </button>
              </div>
              <pre className="text-[11px] leading-snug overflow-auto max-h-[28rem] text-zinc-300 whitespace-pre-wrap">
                {source.text}
              </pre>
            </section>
          ) : null}
        </aside>
      </div>
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

function findingInScope(f: Finding, path: string, nodes: ViewNode[]) {
  if (nodes.some((n) => findingTouchesNode(f, n, path))) return true
  const blob = `${f.from ?? ''} ${f.to ?? ''} ${f.message} ${f.cycle ?? ''}`.toLowerCase()
  return Boolean(path && blob.includes(path.toLowerCase()))
}

function findingTouchesNode(f: Finding, node: ViewNode, viewPath: string) {
  const blob = `${f.from ?? ''} ${f.to ?? ''} ${f.message} ${f.cycle ?? ''}`.replace(/\\/g, '/').toLowerCase()
  const segs = (viewPath ? `${viewPath}/${node.id}` : node.id)
    .split('/')
    .map((s) => s.toLowerCase())
    .filter(Boolean)
  if (segs.length === 0) return false
  const parts = blob.split(/[^a-z0-9._-]+/)
  return segs.every((s) => parts.includes(s))
}

function explainFinding(f: Finding): { title: string; detail: string } {
  const from = shortName(f.from)
  const to = shortName(f.to)
  switch (f.kind) {
    case 'cycle':
      return { title: 'Circular dependency', detail: (f.cycle ?? f.message).replace(/->/g, ' → ') }
    case 'layer_violation':
      return {
        title: 'Depends the wrong way',
        detail: from && to ? `${from} must not depend on ${to}. Inner layers cannot import outer ones.` : f.message,
      }
    case 'domain_purity':
      return { title: 'Framework code in an inner layer', detail: f.message }
    case 'unlayered':
      return { title: 'Not in a known architecture folder', detail: shortName(f.from) || f.message }
    default:
      return { title: f.kind, detail: f.message }
  }
}

function shortName(id?: string) {
  if (!id) return ''
  const slash = id.replace(/\\/g, '/').split('/')
  return slash[slash.length - 1] ?? id
}
