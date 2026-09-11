import type { ViewNode } from './types'

export const MATRIX_MAX = 40

export type MatrixLink = {
  from: string
  to: string
  kind?: string
}

export type MatrixCell = {
  from: string
  to: string
  count: number
  kinds: string[]
  cyclic: boolean
}

export type DepMatrixModel = {
  nodes: ViewNode[]
  truncated: number
  cells: Map<string, MatrixCell>
}

export function cellKey(from: string, to: string) {
  return `${from}\0${to}`
}

export function buildDepMatrix(nodes: ViewNode[], edges: MatrixLink[], cycles: string[] = []): DepMatrixModel {
  const sorted = nodes
    .filter((n) => n.id && !n.id.startsWith('caption:'))
    .slice()
    .sort((a, b) => a.label.localeCompare(b.label))
  const truncated = Math.max(0, sorted.length - MATRIX_MAX)
  const shown = sorted.slice(0, MATRIX_MAX)
  const cyclic = cyclePairs(cycles, shown.map((n) => n.id))
  const cells = new Map<string, MatrixCell>()
  const ids = new Set(shown.map((n) => n.id))
  for (const e of edges) {
    if (!ids.has(e.from) || !ids.has(e.to) || e.from === e.to) continue
    const key = cellKey(e.from, e.to)
    const cur = cells.get(key) ?? { from: e.from, to: e.to, count: 0, kinds: [], cyclic: cyclic.has(key) }
    cur.count += 1
    if (e.kind && !cur.kinds.includes(e.kind)) cur.kinds.push(e.kind)
    if (cyclic.has(key)) cur.cyclic = true
    cells.set(key, cur)
  }
  return { nodes: shown, truncated, cells }
}

export function cyclePairs(cycles: string[], ids: string[]): Set<string> {
  const set = new Set<string>()
  const idSet = new Set(ids)
  for (const raw of cycles) {
    const parts = raw
      .split(/\s*(?:→|->)\s*/)
      .map((s) => s.trim())
      .filter(Boolean)
    for (let i = 0; i < parts.length - 1; i++) {
      const a = matchId(parts[i], idSet)
      const b = matchId(parts[i + 1], idSet)
      if (a && b) set.add(cellKey(a, b))
    }
  }
  return set
}

function matchId(part: string, ids: Set<string>): string | undefined {
  if (ids.has(part)) return part
  const leaf = part.replace(/\\/g, '/').split('/').pop() || part
  if (ids.has(leaf)) return leaf
  for (const id of ids) {
    if (id === leaf || id.endsWith(`/${leaf}`)) return id
  }
  return undefined
}
