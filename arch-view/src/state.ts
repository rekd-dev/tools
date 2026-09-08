import type { Lens } from './types'

export type ViewerState = {
  lens: Lens
  path: string
  sel: string
  overlays: { data: boolean; async: boolean }
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
    },
  }
}

export function writeHash(s: ViewerState) {
  const p = new URLSearchParams()
  p.set('lens', s.lens)
  if (s.path) p.set('path', s.path)
  if (s.sel) p.set('sel', s.sel)
  const overlays = [s.overlays.data ? 'data' : '', s.overlays.async ? 'async' : ''].filter(Boolean).join(',')
  if (overlays) p.set('overlays', overlays)
  const next = '#' + p.toString()
  if (window.location.hash !== next) {
    history.replaceState(null, '', next)
  }
}

export function overlayParam(s: ViewerState) {
  return [s.overlays.data ? 'data' : null, s.overlays.async ? 'async' : null].filter(Boolean).join(',')
}
