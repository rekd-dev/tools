import { layerPhrase } from './Diagram'
import type { EntityDetail, GraphEdge, ViewNode } from './types'

export type RelGroup = {
  label: string
  names: string[]
}

const OUT_LABEL: Record<string, string> = {
  imports: 'Imports',
  contains: 'Defines',
  calls: 'Calls',
  implements: 'Implements',
  extends: 'Extends',
  binds: 'Binds',
  injects: 'Injects into',
  publishes: 'Publishes',
  subscribes: 'Subscribes to',
  awaits: 'Awaits',
  handles: 'Handles',
  reads: 'Reads',
  writes: 'Writes',
  carries_data: 'Carries data to',
}

const IN_LABEL: Record<string, string> = {
  imports: 'Imported by',
  contains: 'Defined in',
  calls: 'Called by',
  implements: 'Implemented by',
  extends: 'Extended by',
  binds: 'Bound by',
  injects: 'Injected into this from',
  publishes: 'Published by',
  subscribes: 'Subscribed by',
  handles: 'Handled by',
  reads: 'Read by',
  writes: 'Written by',
}

export function edgeTarget(e: GraphEdge, direction: 'in' | 'out'): string {
  const extra = e as GraphEdge & { fromId?: string; toId?: string }
  if (direction === 'out') return e.to || extra.toId || ''
  return e.from || extra.fromId || ''
}

export function displayName(id?: string): string {
  if (!id) return ''
  const parts = id.split(':')
  const kind = parts[0]
  if (kind === 'type' || kind === 'method' || kind === 'endpoint' || kind === 'event') {
    return parts[parts.length - 1] || id
  }
  const path = kind === 'file' && parts.length >= 3 ? parts.slice(2).join(':') : id
  const slash = path.replace(/\\/g, '/').split('/')
  return slash[slash.length - 1] || id
}

export function kindPhrase(kind?: string, abstract?: boolean): string {
  switch (kind) {
    case 'file':
      return 'File'
    case 'type':
      return abstract ? 'Interface' : 'Type'
    case 'method':
      return 'Function'
    case 'endpoint':
      return 'HTTP endpoint'
    case 'module':
      return 'Folder'
    default:
      return kind ? kind[0].toUpperCase() + kind.slice(1) : ''
  }
}

export function groupedRelations(edges: GraphEdge[] | undefined, direction: 'in' | 'out'): RelGroup[] {
  const labels = direction === 'out' ? OUT_LABEL : IN_LABEL
  const map = new Map<string, string[]>()
  for (const e of edges ?? []) {
    const other = edgeTarget(e, direction)
    if (!other) continue
    const label = labels[e.kind] ?? (direction === 'out' ? `Goes to` : `Comes from`)
    const list = map.get(label) ?? []
    const name = displayName(other)
    if (name && !list.includes(name)) list.push(name)
    map.set(label, list)
  }
  return [...map.entries()].map(([label, names]) => ({
    label,
    names: names.sort((a, b) => a.localeCompare(b)),
  }))
}

export function evidenceNote(entity: EntityDetail | null): string | null {
  const edges = [...(entity?.outgoing ?? []), ...(entity?.incoming ?? [])]
  if (!edges.length) return null
  const compiler = edges.some((e) => e.source === 'typescript' || e.source === 'roslyn')
  if (compiler) return 'Names here are mostly compiler-resolved.'
  if (edges.some((e) => e.source === 'heuristic')) return 'Guessed from source text, not a compiler.'
  return null
}

export function selectionTitle(entity: EntityDetail | null, focus: ViewNode | null): string {
  return entity?.node?.name || focus?.label || 'Selection'
}

export function selectionKind(entity: EntityDetail | null, focus: ViewNode | null): string {
  const kind = entity?.node?.kind || focus?.kind
  const abstract = entity?.node?.abstract || focus?.abstract
  const layer = entity?.node?.archLayer || entity?.node?.layer || focus?.archLayer
  const bits = [kindPhrase(kind, abstract)]
  if (layer) bits.push(layerPhrase(layer))
  return bits.filter(Boolean).join(' · ')
}

export function relationPhrase(kind?: string, unresolved?: boolean): string {
  if (!kind || kind === 'direct') return unresolved ? 'Depends on (unresolved)' : 'Depends on'
  const label = OUT_LABEL[kind] ?? kind.replace(/_/g, ' ')
  return unresolved ? `${label} (unresolved)` : label
}
