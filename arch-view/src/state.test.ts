import { describe, expect, it } from 'vitest'
import { hashFor, parentPath, parseHash, type ViewerState } from './state'

const overlaysOff = { data: false, async: false, deps: false, churn: false, unresolved: false }

function viewer(partial: Partial<ViewerState>): ViewerState {
  return {
    lens: 'architecture',
    path: '',
    sel: '',
    overlays: overlaysOff,
    view: 'graph',
    ...partial,
  }
}

describe('hash state', () => {
  it('restores lens, path, selection, and overlays', () => {
    const s = parseHash('#lens=focus&path=application&sel=type:csharp:a.cs:IRepo&overlays=data,async')
    expect(s.lens).toBe('focus')
    expect(s.path).toBe('application')
    expect(s.sel).toContain('IRepo')
    expect(s.overlays.data).toBe(true)
    expect(s.overlays.async).toBe(true)
  })

  it('restores unresolved overlay', () => {
    expect(parseHash('#lens=flow&overlays=unresolved').overlays.unresolved).toBe(true)
  })

  it('restores deps and churn overlays', () => {
    const s = parseHash('#lens=flow&overlays=data,deps,churn')
    expect(s.overlays.deps).toBe(true)
    expect(s.overlays.churn).toBe(true)
  })

  it('restores the architecture matrix', () => {
    expect(parseHash('#lens=architecture&view=matrix').view).toBe('matrix')
    expect(parseHash('#lens=architecture').view).toBe('graph')
  })
})

describe('hashFor', () => {
  it('writes churn only on the architecture graph', () => {
    const withChurn = { ...overlaysOff, churn: true }
    expect(hashFor(viewer({ overlays: withChurn }))).toBe('#lens=architecture&overlays=churn')
    expect(hashFor(viewer({ overlays: withChurn, view: 'matrix' }))).toBe('#lens=architecture&view=matrix')
  })

  it('writes flow overlays and drops architecture-only flags', () => {
    const hash = hashFor(
      viewer({
        lens: 'flow',
        view: 'matrix',
        overlays: { data: true, async: false, deps: false, churn: true, unresolved: true },
      }),
    )
    const s = parseHash(hash)
    expect(s.lens).toBe('flow')
    expect(s.overlays.data).toBe(true)
    expect(s.overlays.unresolved).toBe(true)
    expect(s.overlays.churn).toBe(false)
    expect(s.view).toBe('graph')
    expect(hash).not.toMatch(/churn/)
    expect(hash).not.toMatch(/view=matrix/)
  })

  it('does not keep matrix view on Focus', () => {
    expect(hashFor(viewer({ lens: 'focus', view: 'matrix' }))).toBe('#lens=focus')
  })
})

describe('parentPath', () => {
  it('walks up one folder and stops at root', () => {
    expect(parentPath('schedule-api/application')).toBe('schedule-api')
    expect(parentPath('schedule-api')).toBe('')
    expect(parentPath('')).toBe('')
  })
})
