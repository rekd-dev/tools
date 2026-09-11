export type ViewNode = {
  id: string
  label: string
  layer: number
  archLayer?: string
  leaf: boolean
  abstract: boolean
  cycle: boolean
  path?: string
  fileRoot?: string
  kind?: string
  source?: string
  line?: number
  churnCommits?: number
  churnAuthors?: number
  churnLast?: string
}

export type ViewEdge = {
  from: string
  to: string
  kind: string
}

export type GraphEdge = {
  id?: string
  from: string
  to: string
  kind: string
  source?: string
  confidence?: number
  unresolved?: boolean
  file?: string
  line?: number
  detail?: string
}

export type EdgeEvidence = {
  source: string
  confidence?: number
  analyzer?: string
  file?: string
  line?: number
  snippet?: string
  detail?: string
}

export type EdgeDetail = GraphEdge & {
  fromId?: string
  toId?: string
  analyzer?: string
  evidence?: EdgeEvidence[]
}

export type RelationPick = {
  id?: string
  from: string
  to: string
  kind?: string
  file?: string
  line?: number
  unresolved?: boolean
  source?: string
  detail?: string
  confidence?: number
  evidence?: EdgeEvidence[]
}

export type View = {
  path: string
  fileRoot?: string
  nodes: ViewNode[]
  edges: ViewEdge[]
  cycles: string[]
  childPaths: string[]
}

export type GraphView = {
  lens: string
  path: string
  selection?: string
  nodes: ViewNode[]
  edges: GraphEdge[]
  cycles: string[]
  childPaths: string[]
  coverage?: Coverage[]
  truncated?: boolean
  reason?: string
}

export type Coverage = {
  analyzer: string
  status: string
  message?: string
}

export type Finding = {
  id: string
  kind: string
  severity: string
  message: string
  from?: string
  to?: string
  layer?: string
  cycle?: string
  relatedCount?: number
}

export type FitnessReport = {
  repo: string
  path: string
  gitSha?: string
  rulesSource: string
  summary: {
    modules: number
    deps: number
    errors: number
    warnings: number
    cycles: number
    layerViolations: number
    domainPurity: number
    unlayered: number
  }
  findings: Finding[]
  cycles: string[]
}

export type Meta = Record<string, string>

export type ChurnStat = {
  commits: number
  authors: number
  last?: string
}

export type ChurnReport = {
  since: string
  message?: string
  nodes: Record<string, ChurnStat>
}

export type SearchHit = {
  id: string
  kind: string
  name: string
  file?: string
  language?: string
  score: number
}

export type EntityDetail = {
  node: ViewNode & { name?: string; file?: string; layer?: string; extra?: Record<string, string> }
  incoming: GraphEdge[]
  outgoing: GraphEdge[]
  implementers?: { id: string; name?: string; label?: string; file?: string }[]
  bindings?: GraphEdge[]
  injectedInto?: GraphEdge[]
}

export type Lens = 'architecture' | 'focus' | 'flow'
