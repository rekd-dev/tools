import type { Lens, ViewNode } from './types'

export type NodeMenuAction = 'open' | 'focus' | 'flow' | 'copy'

export function canOpenFolder(node: ViewNode, lens: Lens) {
  if (lens !== 'architecture') return false
  return !node.leaf && node.kind !== 'type' && node.kind !== 'method'
}

export function nodeMenuActions(node: ViewNode, lens: Lens): { id: NodeMenuAction; label: string }[] {
  const items: { id: NodeMenuAction; label: string }[] = []
  if (canOpenFolder(node, lens)) items.push({ id: 'open', label: 'Open folder' })
  else if (node.path) items.push({ id: 'open', label: 'Open source' })
  if (lens !== 'focus') items.push({ id: 'focus', label: 'Focus' })
  if (lens !== 'flow') items.push({ id: 'flow', label: 'Flow' })
  items.push({ id: 'copy', label: 'Copy name' })
  return items
}
