import type { Coverage, EdgeDetail, EntityDetail, FitnessReport, GraphView, Meta, SearchHit, View, ChurnReport } from './types'

async function getJSON<T>(url: string): Promise<T> {
  const res = await fetch(url)
  if (!res.ok) {
    throw new Error(`${res.status} ${res.statusText} for ${url}`)
  }
  return res.json() as Promise<T>
}

export const api = {
  meta: () => getJSON<Meta>('/api/meta'),
  view: (path: string) => getJSON<View>(`/api/view?path=${encodeURIComponent(path)}`),
  fitness: () => getJSON<FitnessReport>('/api/fitness'),
  coverage: () => getJSON<Coverage[]>('/api/coverage'),
  search: async (q: string) => (await getJSON<SearchHit[] | null>(`/api/search?q=${encodeURIComponent(q)}`)) ?? [],
  entity: (id: string) => getJSON<EntityDetail>(`/api/entity?id=${encodeURIComponent(id)}`),
  edge: (id: string) => getJSON<EdgeDetail>(`/api/edge?id=${encodeURIComponent(id)}`),
  graphView: (opts: { lens: string; path: string; sel?: string; root?: string; overlays?: string }) => {
    const p = new URLSearchParams()
    p.set('lens', opts.lens)
    p.set('path', opts.path)
    if (opts.sel) p.set('sel', opts.sel)
    if (opts.root) p.set('root', opts.root)
    if (opts.overlays) p.set('overlays', opts.overlays)
    return getJSON<GraphView>(`/api/graph-view?${p}`)
  },
  churn: (path: string) => getJSON<ChurnReport>(`/api/churn?path=${encodeURIComponent(path)}`),
  source: async (file: string) => {
    const res = await fetch(`/api/source?file=${encodeURIComponent(file)}`)
    if (!res.ok) throw new Error('source not found')
    return res.text()
  },
}
