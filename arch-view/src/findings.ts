import type { Finding, ViewNode } from './types'

export function findingFiles(f: Finding): string[] {
  const out: string[] = []
  const add = (raw?: string) => {
    if (!raw) return
    for (const part of raw.split(/->/)) {
      const t = part.trim().replace(/\\/g, '/')
      if (t) out.push(t)
    }
  }
  add(f.from)
  add(f.to)
  add(f.cycle)
  return out
}

export function underRoot(file: string, root: string) {
  const f = file.replace(/\\/g, '/').toLowerCase()
  const r = root.replace(/\\/g, '/').toLowerCase()
  if (!r) return false
  return f === r || f.startsWith(`${r}/`)
}

export function findingTouchesNode(f: Finding, node: ViewNode) {
  const root = node.fileRoot || node.path || ''
  if (!root) return false
  return findingFiles(f).some((file) => underRoot(file, root))
}

export function nodesTouchedByFinding(f: Finding, nodes: ViewNode[]) {
  return nodes.filter((n) => findingTouchesNode(f, n))
}

export function findingInView(f: Finding, fileRoot: string | undefined, nodes: ViewNode[]) {
  if (fileRoot && findingFiles(f).some((file) => underRoot(file, fileRoot))) return true
  return nodes.some((n) => findingTouchesNode(f, n))
}
