import { MarkerType, Position, type Edge, type Node } from '@xyflow/react'
import { layerPhrase } from './Diagram'
import type { GraphEdge, View, ViewNode } from './types'

export type ArchNodeData = {
  node: ViewNode
  incoming: string[]
  outgoing: string[]
  issue?: 'error' | 'warning'
  label?: string
}

const COL_W = 230
const ROW_H = 170
const NODE_W = 200
const NODE_H = 80

const ARCH_RANK: Record<string, number> = {
  domain: 0,
  application: 1,
  infrastructure: 2,
  interface: 3,
  web: 3,
  composition: 4,
}
const OTHER_RANK = 5

export function layoutArchitecture(view: View, issues: Record<string, 'error' | 'warning'>): {
  nodes: Node<ArchNodeData>[]
  edges: Edge[]
} {
  const list = view.nodes ?? []
  const edgeList = view.edges ?? []
  if (!view.path && shouldUseRadial(list) && hasSharedFamilies(list)) {
    return layoutFamilies(list, edgeList, issues)
  }
  if (shouldUseRadial(list)) {
    return layoutRadial(list, edgeList, issues)
  }
  return layoutLayered(list, edgeList, issues)
}

export function layoutGraph(
  nodesIn: ViewNode[],
  edgesIn: GraphEdge[],
  issues: Record<string, 'error' | 'warning'>,
  selection?: string,
  mode: 'radial' | 'flow' = 'radial',
): {
  nodes: Node<ArchNodeData>[]
  edges: Edge[]
} {
  if (mode === 'flow') return layoutFlow(nodesIn, edgesIn, issues, selection)
  return layoutRadial(nodesIn, edgesIn, issues, selection)
}

export async function layoutGraphAsync(
  nodesIn: ViewNode[],
  edgesIn: GraphEdge[],
  issues: Record<string, 'error' | 'warning'>,
  selection?: string,
  mode: 'radial' | 'flow' = 'radial',
): Promise<{ nodes: Node<ArchNodeData>[]; edges: Edge[] }> {
  if (mode !== 'flow') return layoutGraph(nodesIn, edgesIn, issues, selection, mode)
  try {
    return await layoutFlowElk(nodesIn, edgesIn, issues, selection)
  } catch {
    return layoutFlow(nodesIn, edgesIn, issues, selection)
  }
}

export function shouldUseRadial(nodes: ViewNode[]): boolean {
  const ranks = new Set(
    nodes.map((n) => n.archLayer).filter((l): l is string => Boolean(l && l in ARCH_RANK)),
  )
  return ranks.size < 2
}

export function familyKey(id: string, allIds: string[]): string {
  const name = leafName(id)
  const tokenCount = new Map<string, number>()
  for (const x of allIds) {
    const token = leafName(x).split('-')[0]
    if (!token) continue
    tokenCount.set(token, (tokenCount.get(token) ?? 0) + 1)
  }
  const families = new Set([...tokenCount.entries()].filter(([, n]) => n >= 2).map(([t]) => t))
  const parts = name.split('-').filter(Boolean)
  const first = parts[0]
  if (first && families.has(first)) return first
  for (const part of parts.slice(1)) {
    if (families.has(part)) return part
  }
  return name
}

export function hasSharedFamilies(nodes: ViewNode[]): boolean {
  const ids = nodes.map((n) => n.id)
  const counts = new Map<string, number>()
  for (const id of ids) {
    const key = familyKey(id, ids)
    counts.set(key, (counts.get(key) ?? 0) + 1)
  }
  return [...counts.values()].some((n) => n >= 2)
}

function leafName(id: string) {
  const parts = id.replace(/\\/g, '/').split('/')
  return parts[parts.length - 1] || id
}

export function archRank(n: ViewNode): number {
  if (n.archLayer && n.archLayer in ARCH_RANK) return ARCH_RANK[n.archLayer]
  return OTHER_RANK
}

function layoutLayered(
  list: ViewNode[],
  edgeList: View['edges'] | GraphEdge[],
  issues: Record<string, 'error' | 'warning'>,
): { nodes: Node<ArchNodeData>[]; edges: Edge[] } {
  const links = normalizeLinks(edgeList)
  const byRank = new Map<number, ViewNode[]>()
  for (const n of list) {
    const rank = archRank(n)
    const row = byRank.get(rank) ?? []
    row.push(n)
    byRank.set(rank, row)
  }
  for (const [rank, row] of byRank) {
    byRank.set(
      rank,
      row.slice().sort((a, b) => a.label.localeCompare(b.label)),
    )
  }

  const neighbors = adjacency(links)
  for (let i = 0; i < 6; i++) {
    const index = new Map<string, number>()
    for (const row of byRank.values()) {
      row.forEach((n, j) => index.set(n.id, j))
    }
    for (const [rank, row] of byRank) {
      byRank.set(
        rank,
        row.slice().sort((a, b) => {
          const da = barycenter(a.id, neighbors, index)
          const db = barycenter(b.id, neighbors, index)
          if (da !== db) return da - db
          return a.label.localeCompare(b.label)
        }),
      )
    }
  }

  const ranks = [...byRank.keys()].sort((a, b) => b - a)
  const maxCols = Math.max(1, ...ranks.map((r) => byRank.get(r)?.length ?? 0))
  const canvasW = maxCols * COL_W
  const nodes: Node<ArchNodeData>[] = []

  ranks.forEach((rank, rowI) => {
    const row = byRank.get(rank) ?? []
    const rowW = row.length * COL_W
    const x0 = (canvasW - rowW) / 2
    const y = rowI * ROW_H
    nodes.push({
      id: `caption:${rank}`,
      type: 'caption',
      position: { x: Math.max(-160, x0 - 170), y: y + 18 },
      data: { node: captionNode(rank), incoming: [], outgoing: [], label: captionForRank(rank, row) },
      selectable: false,
      draggable: false,
    })
    row.forEach((n, i) => {
      nodes.push(archBox(n, { x: x0 + i * COL_W, y }, links, issues, Position.Bottom, Position.Top))
    })
  })

  return { nodes, edges: architectureEdges(links, list, 'smoothstep') }
}

function layoutFamilies(
  list: ViewNode[],
  edgeList: View['edges'] | GraphEdge[],
  issues: Record<string, 'error' | 'warning'>,
): { nodes: Node<ArchNodeData>[]; edges: Edge[] } {
  const links = normalizeLinks(edgeList)
  const ids = list.map((n) => n.id)
  const groups = new Map<string, ViewNode[]>()
  for (const n of list) {
    const key = familyKey(n.id, ids)
    const row = groups.get(key) ?? []
    row.push(n)
    groups.set(key, row)
  }
  for (const [key, row] of groups) {
    groups.set(
      key,
      row.slice().sort((a, b) => a.label.localeCompare(b.label)),
    )
  }
  const keys = [...groups.keys()].sort((a, b) => {
    const na = groups.get(a)?.length ?? 0
    const nb = groups.get(b)?.length ?? 0
    if (na !== nb) return nb - na
    return a.localeCompare(b)
  })

  const innerCols = (members: ViewNode[]) => Math.min(3, Math.max(members.length, 1))
  const famW = (members: ViewNode[]) => innerCols(members) * COL_W
  const famH = (members: ViewNode[]) => {
    const cols = innerCols(members)
    return 28 + Math.ceil(members.length / cols) * ROW_H
  }

  const packCols = Math.max(2, Math.ceil(Math.sqrt(keys.length)))
  const colW = Array.from({ length: packCols }, () => 0)
  const rowH: number[] = []
  keys.forEach((key, i) => {
    const members = groups.get(key) ?? []
    const c = i % packCols
    const r = Math.floor(i / packCols)
    colW[c] = Math.max(colW[c], famW(members))
    rowH[r] = Math.max(rowH[r] ?? 0, famH(members))
  })
  const colX: number[] = []
  let x = 0
  for (let c = 0; c < packCols; c++) {
    colX[c] = x
    x += colW[c] + 48
  }
  const rowY: number[] = []
  let y = 0
  const nrows = Math.ceil(keys.length / packCols)
  for (let r = 0; r < nrows; r++) {
    rowY[r] = y
    y += (rowH[r] ?? 0) + 56
  }

  const nodes: Node<ArchNodeData>[] = []
  keys.forEach((key, i) => {
    const members = groups.get(key) ?? []
    const c = i % packCols
    const r = Math.floor(i / packCols)
    const ox = colX[c] ?? 0
    const oy = rowY[r] ?? 0
    const cols = innerCols(members)
    nodes.push({
      id: `caption:fam:${key}`,
      type: 'caption',
      position: { x: ox, y: oy },
      data: { node: captionNode(-1), incoming: [], outgoing: [], label: key },
      selectable: false,
      draggable: false,
    })
    members.forEach((n, j) => {
      nodes.push(
        archBox(
          n,
          { x: ox + (j % cols) * COL_W, y: oy + 28 + Math.floor(j / cols) * ROW_H },
          links,
          issues,
          Position.Bottom,
          Position.Top,
        ),
      )
    })
  })

  return { nodes, edges: architectureEdges(links, list, 'default') }
}

function layoutRadial(
  list: ViewNode[],
  edgeList: View['edges'] | GraphEdge[],
  issues: Record<string, 'error' | 'warning'>,
  selection?: string,
): { nodes: Node<ArchNodeData>[]; edges: Edge[] } {
  if (!list.length) return { nodes: [], edges: [] }
  const links = normalizeLinks(edgeList)
  const neighbors = adjacency(links)
  const centerId = pickCenter(list, neighbors, selection)
  const rings = bfsRings(list, neighbors, centerId)
  const cx = 420
  const cy = 280
  const nodes: Node<ArchNodeData>[] = []

  rings.forEach((ring, r) => {
    if (r === 0) {
      const n = ring[0]
      if (n) nodes.push(archBox(n, { x: cx - NODE_W / 2, y: cy - NODE_H / 2 }, links, issues, Position.Bottom, Position.Top))
      return
    }
    const radius = 70 + r * 210
    ring.forEach((n, i) => {
      const angle = -Math.PI / 2 + (2 * Math.PI * i) / Math.max(ring.length, 1)
      nodes.push(
        archBox(
          n,
          { x: cx + radius * Math.cos(angle) - NODE_W / 2, y: cy + radius * Math.sin(angle) - NODE_H / 2 },
          links,
          issues,
          Position.Bottom,
          Position.Top,
        ),
      )
    })
  })

  return { nodes, edges: architectureEdges(links, list, 'default') }
}

function layoutFlow(
  list: ViewNode[],
  edgeList: GraphEdge[],
  issues: Record<string, 'error' | 'warning'>,
  selection?: string,
): { nodes: Node<ArchNodeData>[]; edges: Edge[] } {
  if (!list.length) return { nodes: [], edges: [] }
  const links = normalizeLinks(edgeList)
  const neighbors = adjacency(links)
  const start = pickCenter(list, neighbors, selection)
  const dist = new Map<string, number>()
  const q = [start]
  dist.set(start, 0)
  while (q.length) {
    const id = q.shift()!
    for (const n of neighbors.get(id) ?? []) {
      if (!dist.has(n)) {
        dist.set(n, (dist.get(id) ?? 0) + 1)
        q.push(n)
      }
    }
  }
  for (const n of list) {
    if (!dist.has(n.id)) dist.set(n.id, 99)
  }
  const cols = new Map<number, ViewNode[]>()
  for (const n of list) {
    const d = dist.get(n.id) ?? 99
    const col = cols.get(d) ?? []
    col.push(n)
    cols.set(d, col)
  }
  const nodes: Node<ArchNodeData>[] = []
  const depths = [...cols.keys()].sort((a, b) => a - b)
  depths.forEach((d) => {
    const col = (cols.get(d) ?? []).slice().sort((a, b) => a.label.localeCompare(b.label))
    const colH = col.length * ROW_H
    col.forEach((n, i) => {
      nodes.push(
        archBox(
          n,
          { x: d * 260, y: (i * ROW_H) - colH / 2 + 200 },
          links,
          issues,
          Position.Right,
          Position.Left,
        ),
      )
    })
  })
  return { nodes, edges: architectureEdges(links, list, 'smoothstep') }
}

async function layoutFlowElk(
  list: ViewNode[],
  edgeList: GraphEdge[],
  issues: Record<string, 'error' | 'warning'>,
  selection?: string,
): Promise<{ nodes: Node<ArchNodeData>[]; edges: Edge[] }> {
  if (!list.length) return { nodes: [], edges: [] }
  const links = normalizeLinks(edgeList)
  const elkMod = await import('elkjs/lib/elk.bundled.js')
  const ELK = elkMod.default
  const elk = new ELK()
  const ids = new Set(list.map((n) => n.id))
  const laid = await elk.layout({
    id: 'root',
    layoutOptions: {
      'elk.algorithm': 'layered',
      'elk.direction': 'RIGHT',
      'elk.edgeRouting': 'ORTHOGONAL',
      'elk.layered.spacing.nodeNodeBetweenLayers': '80',
      'elk.spacing.nodeNode': '28',
    },
    children: list.map((n) => ({
      id: n.id,
      width: NODE_W,
      height: NODE_H,
      layoutOptions: n.id === selection ? { 'elk.layered.layering.layerConstraint': 'FIRST' } : undefined,
    })),
    edges: links
      .filter((e) => ids.has(e.from) && ids.has(e.to))
      .map((e, i) => ({
        id: e.id || `${e.from}->${e.to}:${i}`,
        sources: [e.from],
        targets: [e.to],
      })),
  })
  const pos = new Map((laid.children ?? []).map((c) => [c.id, { x: c.x ?? 0, y: c.y ?? 0 }]))
  const nodes = list.map((n) =>
    archBox(n, pos.get(n.id) ?? { x: 0, y: 0 }, links, issues, Position.Right, Position.Left),
  )
  return { nodes, edges: architectureEdges(links, list, 'smoothstep') }
}

export function pickCenter(list: ViewNode[], neighbors: Map<string, string[]>, selection?: string): string {
  if (selection && list.some((n) => n.id === selection)) return selection
  let best = list[0]?.id ?? ''
  let bestDeg = -1
  for (const n of list) {
    const deg = neighbors.get(n.id)?.length ?? 0
    if (deg > bestDeg || (deg === bestDeg && n.label < (list.find((x) => x.id === best)?.label ?? ''))) {
      best = n.id
      bestDeg = deg
    }
  }
  return best
}

function bfsRings(list: ViewNode[], neighbors: Map<string, string[]>, centerId: string): ViewNode[][] {
  const byId = new Map(list.map((n) => [n.id, n]))
  const seen = new Set<string>()
  const rings: ViewNode[][] = []
  let frontier = [centerId]
  seen.add(centerId)
  while (frontier.length) {
    const ring = frontier.map((id) => byId.get(id)).filter((n): n is ViewNode => Boolean(n))
    ring.sort((a, b) => a.label.localeCompare(b.label))
    rings.push(ring)
    const next: string[] = []
    for (const id of frontier) {
      for (const n of neighbors.get(id) ?? []) {
        if (!seen.has(n) && byId.has(n)) {
          seen.add(n)
          next.push(n)
        }
      }
    }
    frontier = next
  }
  const leftover = list.filter((n) => !seen.has(n.id)).sort((a, b) => a.label.localeCompare(b.label))
  if (leftover.length) rings.push(leftover)
  return rings
}

function barycenter(id: string, neighbors: Map<string, string[]>, index: Map<string, number>): number {
  const ns = neighbors.get(id) ?? []
  const vals = ns.map((n) => index.get(n)).filter((v): v is number => v != null)
  if (!vals.length) return 50
  return vals.reduce((a, b) => a + b, 0) / vals.length
}

function adjacency(links: { from: string; to: string }[]): Map<string, string[]> {
  const m = new Map<string, Set<string>>()
  const add = (a: string, b: string) => {
    if (a === b) return
    const s = m.get(a) ?? new Set()
    s.add(b)
    m.set(a, s)
  }
  for (const e of links) {
    add(e.from, e.to)
    add(e.to, e.from)
  }
  return new Map([...m].map(([k, v]) => [k, [...v]]))
}

function normalizeLinks(edgeList: View['edges'] | GraphEdge[]): { from: string; to: string; kind?: string; unresolved?: boolean; id?: string }[] {
  return (edgeList ?? []).map((e) => ({
    id: 'id' in e ? e.id : undefined,
    from: e.from,
    to: e.to,
    kind: e.kind,
    unresolved: 'unresolved' in e ? e.unresolved : false,
  }))
}

function architectureEdges(
  links: { from: string; to: string; kind?: string; unresolved?: boolean; id?: string }[],
  list: ViewNode[],
  type: Edge['type'],
): Edge[] {
  const cyclic = new Set(list.filter((n) => n.cycle).map((n) => n.id))
  return links.map((e, i) => ({
    id: e.id || `${e.from}->${e.to}:${i}`,
    source: e.from,
    target: e.to,
    type,
    label: e.kind && e.kind !== 'direct' ? (e.unresolved ? `${e.kind} ?` : e.kind) : undefined,
    className: cyclic.has(e.from) || cyclic.has(e.to) ? 'cycle' : '',
    style: e.unresolved ? { strokeDasharray: '4 3' } : undefined,
    markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14, color: '#71717a' },
    data: e,
  }))
}

function archBox(
  n: ViewNode,
  position: { x: number; y: number },
  links: { from: string; to: string }[],
  issues: Record<string, 'error' | 'warning'>,
  sourcePosition: Position,
  targetPosition: Position,
): Node<ArchNodeData> {
  return {
    id: n.id,
    position,
    data: {
      node: n,
      incoming: links.filter((e) => e.to === n.id).map((e) => e.from),
      outgoing: links.filter((e) => e.from === n.id).map((e) => e.to),
      issue: issues[n.id],
    },
    sourcePosition,
    targetPosition,
    type: 'arch',
  }
}

function captionForRank(rank: number, row: ViewNode[]): string {
  const layers = [...new Set(row.map((n) => n.archLayer).filter(Boolean))] as string[]
  if (layers.length === 1 && layers[0]) return layerPhrase(layers[0])
  if (rank === OTHER_RANK) return 'Other'
  if (rank === 0) return 'Inner'
  return `Layer ${rank}`
}

function captionNode(rank: number): ViewNode {
  return { id: `caption:${rank}`, label: '', layer: rank, leaf: true, abstract: false, cycle: false }
}
