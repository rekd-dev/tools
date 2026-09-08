import type { View, ViewEdge, ViewNode } from './types'

type Props = {
  nodes: ViewNode[]
  edges: ViewEdge[]
  focusId: string | null
  nodeIssues?: Record<string, 'error' | 'warning'>
  onFocus: (node: ViewNode) => void
  onDrill: (node: ViewNode) => void
  onOpenSource: (node: ViewNode) => void
}

export function Diagram({ nodes, edges, focusId, nodeIssues, onFocus, onDrill, onOpenSource }: Props) {
  const list = nodes ?? []
  const edgeList = edges ?? []
  const maxLayer = list.reduce((m, n) => Math.max(m, Number.isFinite(n.layer) ? n.layer : 0), 0)

  const rows: { layer: number; label: string; nodes: ViewNode[] }[] = []
  for (let layer = maxLayer; layer >= 0; layer--) {
    const rowNodes = list.filter((n) => (Number.isFinite(n.layer) ? n.layer : 0) === layer)
    if (rowNodes.length === 0) continue
    rowNodes.sort((a, b) => a.label.localeCompare(b.label))
    rows.push({
      layer,
      label: rowCaption(layer, maxLayer),
      nodes: rowNodes,
    })
  }

  const incoming = (id: string) => edgeList.filter((e) => e.to === id).map((e) => e.from)
  const outgoing = (id: string) => edgeList.filter((e) => e.from === id).map((e) => e.to)

  return (
    <div className="space-y-5">
      {rows.map((row, i) => (
        <div key={row.layer}>
          <div className="text-[11px] font-medium uppercase tracking-wide text-zinc-500 mb-2">
            {row.label}
          </div>
          <div className="flex flex-wrap gap-2">
            {row.nodes.map((n) => {
              const ins = incoming(n.id)
              const outs = outgoing(n.id)
              const focused = focusId === n.id
              const issue = nodeIssues?.[n.id] ?? (n.cycle ? 'error' : undefined)
              return (
                <button
                  key={n.id}
                  type="button"
                  onClick={() => {
                    onFocus(n)
                    if (n.leaf) onOpenSource(n)
                    else onDrill(n)
                  }}
                  onMouseEnter={() => onFocus(n)}
                  className={[
                    'min-w-[11rem] max-w-[18rem] text-left rounded-md border px-3 py-2 transition-colors',
                    focused ? 'bg-zinc-800 ring-2 ring-sky-400' : 'bg-zinc-900 hover:border-zinc-500',
                    issue === 'error' ? 'border-red-500' : issue === 'warning' ? 'border-amber-500' : 'border-zinc-700',
                  ].join(' ')}
                >
                  <div className="text-sm font-semibold text-zinc-100 break-words leading-snug">{n.label}</div>
                  <div className="mt-1 flex flex-wrap gap-1">
                    {n.archLayer ? <Pill>{layerPhrase(n.archLayer)}</Pill> : n.leaf ? <Pill muted>no layer</Pill> : <Pill muted>package</Pill>}
                    <Pill muted>{n.leaf ? 'file' : 'folder'}</Pill>
                    {n.abstract ? <Pill>has interfaces</Pill> : null}
                    {n.cycle ? <Pill danger>in a cycle</Pill> : null}
                    {issue === 'error' && !n.cycle ? <Pill danger>has errors</Pill> : null}
                    {issue === 'warning' ? <Pill warn>has warnings</Pill> : null}
                  </div>
                  <div className="mt-2 text-xs text-zinc-400 leading-relaxed">
                    {outs.length ? <div>depends on {outs.join(', ')}</div> : <div>depends on nothing here</div>}
                    {ins.length ? <div>used by {ins.join(', ')}</div> : <div>used by nothing here</div>}
                  </div>
                </button>
              )
            })}
          </div>
          {i < rows.length - 1 ? (
            <div className="mt-3 text-[11px] text-zinc-600" aria-hidden>
              depends on ↓
            </div>
          ) : null}
        </div>
      ))}
    </div>
  )
}

function rowCaption(layer: number, max: number) {
  if (max === 0) return 'Modules'
  if (layer === max) return 'Outer — these depend on the rows below'
  if (layer === 0) return 'Inner — depended on by the rows above'
  return `Middle (level ${layer})`
}

export function layerPhrase(layer: string) {
  switch (layer) {
    case 'domain':
      return 'domain (core rules)'
    case 'application':
      return 'application (use cases)'
    case 'infrastructure':
      return 'infrastructure (DB, IO)'
    case 'interface':
      return 'interface (HTTP/API)'
    case 'composition':
      return 'composition (wiring)'
    case 'web':
      return 'web UI'
    default:
      return layer
  }
}

function Pill({
  children,
  muted,
  danger,
  warn,
}: {
  children: string
  muted?: boolean
  danger?: boolean
  warn?: boolean
}) {
  const color = danger
    ? 'bg-red-950 text-red-300'
    : warn
      ? 'bg-amber-950 text-amber-200'
      : muted
        ? 'bg-zinc-800 text-zinc-400'
        : 'bg-zinc-800 text-zinc-300'
  return <span className={`inline-block rounded px-1.5 py-0.5 text-[11px] ${color}`}>{children}</span>
}

export function emptyView(): View {
  return { path: '', nodes: [], edges: [], cycles: [], childPaths: [] }
}
