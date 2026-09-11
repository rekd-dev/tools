import type { Lens } from './types'

export type ViewerState = {
  lens: Lens
  path: string
  sel: string
  overlays: { data: boolean; async: boolean; deps: boolean; churn: boolean; unresolved: boolean }
  view: 'graph' | 'matrix'
}

export function parseHash(hash = window.location.hash): ViewerState {
  const raw = hash.startsWith('#') ? hash.slice(1) : hash
  const p = new URLSearchParams(raw)
  const lens = (p.get('lens') as Lens) || 'architecture'
  return {
    lens: lens === 'focus' || lens === 'flow' ? lens : 'architecture',
    path: p.get('path') ?? '',
    sel: p.get('sel') ?? '',
    overlays: {
      data: (p.get('overlays') ?? '').split(',').includes('data'),
      async: (p.get('overlays') ?? '').split(',').includes('async'),
      deps: (p.get('overlays') ?? '').split(',').includes('deps'),
      churn: (p.get('overlays') ?? '').split(',').includes('churn'),
      unresolved: (p.get('overlays') ?? '').split(',').includes('unresolved'),
    },
    view: p.get('view') === 'matrix' ? 'matrix' : 'graph',
  }
}

export function paneOverlays(s: ViewerState): string[] {
  if (s.lens === 'flow') {
    return [
      s.overlays.data ? 'data' : '',
      s.overlays.async ? 'async' : '',
      s.overlays.deps ? 'deps' : '',
      s.overlays.unresolved ? 'unresolved' : '',
    ].filter(Boolean)
  }
  if (s.lens === 'architecture' && s.view !== 'matrix' && s.overlays.churn) return ['churn']
  return []
}

export function hashFor(s: ViewerState): string {
  const p = new URLSearchParams()
  p.set('lens', s.lens)
  if (s.path) p.set('path', s.path)
  if (s.sel) p.set('sel', s.sel)
  const overlays = paneOverlays(s).join(',')
  if (overlays) p.set('overlays', overlays)
  if (s.lens === 'architecture' && s.view === 'matrix') p.set('view', 'matrix')
  return '#' + p.toString()
}

export function writeHash(s: ViewerState, push = false) {
  const next = hashFor(s)
  if (window.location.hash === next) return
  if (push) history.pushState(null, '', next)
  else history.replaceState(null, '', next)
}

export function overlayParam(s: ViewerState) {
  return [s.overlays.data ? 'data' : null, s.overlays.async ? 'async' : null, s.overlays.deps ? 'deps' : null, s.overlays.unresolved ? 'unresolved' : null]
    .filter(Boolean)
    .join(',')
}

export function parentPath(path: string) {
  const parts = path.split('/').filter(Boolean)
  parts.pop()
  return parts.join('/')
}
