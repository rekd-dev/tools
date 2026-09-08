import { Position, type Edge, type Node } from '@xyflow/react'
import type { GraphEdge, View, ViewNode } from './types'

export type ArchNodeData = {
  node: ViewNode
  incoming: string[]
  outgoing: string[]
  issue?: 'error' | 'warning'
}

export function layoutArchitecture(view: View, issues: Record<string, 'error' | 'warning'>): {
  nodes: Node<ArchNodeData>[]
  edges: Edge[]
} {
  const list = view.nodes ?? []
  const maxLayer = list.reduce((m, n) => Math.max(m, Number.isFinite(n.layer) ? n.layer : 0), 0)
  const byLayer = new Map<number, ViewNode[]>()
  for (const n of list) {
    const layer = Number.isFinite(n.layer) ? n.layer : 0
    const row = byLayer.get(layer) ?? []
    row.push(n)
    byLayer.set(layer, row)
  }
  const nodes: Node<ArchNodeData>[] = []
  for (let layer = maxLayer; layer >= 0; layer--) {
    const row = (byLayer.get(layer) ?? []).slice().sort((a, b) => a.label.localeCompare(b.label))
    row.forEach((n, i) => {
      nodes.push({
        id: n.id,
        position: { x: i * 240, y: (maxLayer - layer) * 150 },
        data: {
          node: n,
          incoming: (view.edges ?? []).filter((e) => e.to === n.id).map((e) => e.from),
          outgoing: (view.edges ?? []).filter((e) => e.from === n.id).map((e) => e.to),
          issue: issues[n.id],
        },
        sourcePosition: Position.Bottom,
        targetPosition: Position.Top,
        type: 'arch',
      })
    })
  }
  const edges: Edge[] = (view.edges ?? []).map((e, i) => ({
    id: e.from + '->' + e.to + ':' + i,
    source: e.from,
    target: e.to,
    label: e.kind,
    className: list.find((n) => n.id === e.from)?.cycle || list.find((n) => n.id === e.to)?.cycle ? 'cycle' : '',
  }))
  return { nodes, edges }
}

export function layoutGraph(nodesIn: ViewNode[], edgesIn: GraphEdge[], issues: Record<string, 'error' | 'warning'>): {
  nodes: Node<ArchNodeData>[]
  edges: Edge[]
} {
  const incoming: Record<string, string[]> = {}
  const outgoing: Record<string, string[]> = {}
  for (const e of edgesIn) {
    outgoing[e.from] = [...(outgoing[e.from] ?? []), e.to]
    incoming[e.to] = [...(incoming[e.to] ?? []), e.from]
  }
  const cols = Math.max(1, Math.ceil(Math.sqrt(nodesIn.length)))
  const nodes: Node<ArchNodeData>[] = nodesIn.map((n, i) => ({
    id: n.id,
    position: { x: (i % cols) * 260, y: Math.floor(i / cols) * 150 },
    data: { node: n, incoming: incoming[n.id] ?? [], outgoing: outgoing[n.id] ?? [], issue: issues[n.id] },
    sourcePosition: Position.Bottom,
    targetPosition: Position.Top,
    type: 'arch',
  }))
  const edges: Edge[] = edgesIn.map((e, i) => ({
    id: e.id || e.from + '->' + e.to + ':' + i,
    source: e.from,
    target: e.to,
    label: e.unresolved ? `${e.kind} ?` : e.kind,
    style: e.unresolved ? { strokeDasharray: '4 3' } : undefined,
    data: e,
  }))
  return { nodes, edges }
}
