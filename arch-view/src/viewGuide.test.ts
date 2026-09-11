import { describe, expect, it } from 'vitest'
import { guideKind, viewGuide, type GuideContext } from './viewGuide'

const overlaysOff = { data: false, async: false, deps: false, churn: false, unresolved: false }

function ctx(partial: Partial<GuideContext>): GuideContext {
  return {
    lens: 'architecture',
    view: 'graph',
    path: '',
    sel: '',
    overlays: overlaysOff,
    empty: false,
    ...partial,
  }
}

describe('viewGuide', () => {
  it('explains architecture at the repo root', () => {
    const g = viewGuide(ctx({}))
    expect(g.lookingAt).toBe('Architecture · all modules')
    expect(g.meaning).toMatch(/apps that share a name/i)
    expect(g.lookFor).toMatch(/red borders/i)
    expect(g.lookFor).toMatch(/Double-click/i)
  })

  it('explains a drilled architecture folder', () => {
    const g = viewGuide(ctx({ path: 'schedule-api/application' }))
    expect(g.lookingAt).toBe('Architecture · schedule-api/application')
    expect(g.meaning).toMatch(/inside this folder/i)
    expect(g.lookFor).toMatch(/interfaces/i)
  })

  it('explains the DSM and calls out cycles above the diagonal', () => {
    const g = viewGuide(ctx({ view: 'matrix', path: 'schedule-api' }))
    expect(g.lookingAt).toBe('Architecture · matrix · schedule-api')
    expect(g.meaning).toMatch(/row depends on the columns/i)
    expect(g.lookFor).toMatch(/above/i)
    expect(g.lookFor).toMatch(/cycles/i)
  })

  it('adds churn as activity, not a verdict', () => {
    const g = viewGuide(ctx({ overlays: { ...overlaysOff, churn: true } }))
    expect(g.lookFor).toMatch(/git churn/i)
    expect(g.lookFor).toMatch(/not a verdict/i)
  })

  it('does not mention churn on the matrix', () => {
    const g = viewGuide(ctx({ view: 'matrix', overlays: { ...overlaysOff, churn: true } }))
    expect(g.lookFor).not.toMatch(/churn/i)
  })

  it('tells Focus with no selection to search', () => {
    const g = viewGuide(ctx({ lens: 'focus', empty: true }))
    expect(g.lookingAt).toBe('Focus')
    expect(g.meaning).toMatch(/implements/i)
    expect(g.lookFor).toMatch(/Search/i)
    expect(g.lookFor).not.toMatch(/Deps/i)
  })

  it('treats a focused endpoint as a route neighborhood, not a type', () => {
    const g = viewGuide(
      ctx({
        lens: 'focus',
        sel: 'endpoint:ts:a.ts:GET /api/attendance/day',
        selectedKind: 'endpoint',
        selectedLabel: 'GET /api/attendance/day',
      }),
    )
    expect(g.meaning).toMatch(/route/i)
    expect(g.lookFor).toMatch(/handles/i)
    expect(g.lookFor).toMatch(/Flow/i)
  })

  it('names the focused type and prefers implementers on an interface', () => {
    const g = viewGuide(
      ctx({
        lens: 'focus',
        sel: 'type:ts:ports.ts:TimeSessionRepository',
        selectedKind: 'interface',
        selectedLabel: 'TimeSessionRepository',
      }),
    )
    expect(g.lookingAt).toBe('Focus · TimeSessionRepository')
    expect(g.meaning).toMatch(/neighbors/i)
    expect(g.lookFor).toMatch(/Implementers/i)
    expect(g.lookFor).toMatch(/move the center/i)
  })

  it('tells empty Flow to pick a route', () => {
    const g = viewGuide(ctx({ lens: 'flow', empty: true }))
    expect(g.lookFor).toMatch(/HTTP route/i)
  })

  it('explains the Flow spine for an endpoint', () => {
    const g = viewGuide(
      ctx({
        lens: 'flow',
        sel: 'endpoint:ts:me-time.ts:POST /api/me/time/clock-in',
        selectedKind: 'endpoint',
        selectedLabel: 'POST /api/me/time/clock-in',
      }),
    )
    expect(g.lookingAt).toContain('clock-in')
    expect(g.meaning).toMatch(/Left to right/i)
    expect(g.lookFor).toMatch(/use case/i)
    expect(g.lookFor).toMatch(/missing resolution/i)
  })

  it('annotates Flow overlays without replacing the spine advice', () => {
    const g = viewGuide(
      ctx({
        lens: 'flow',
        selectedKind: 'endpoint',
        selectedLabel: 'clock-in',
        overlays: { ...overlaysOff, data: true, unresolved: true },
      }),
    )
    expect(g.lookFor).toMatch(/adapter write sink|persistence/i)
    expect(g.lookFor).toMatch(/Unresolved/i)
    expect(g.lookFor).toMatch(/use case|ports|adapters/i)
  })

  it('warns when Flow is truncated', () => {
    const g = viewGuide(ctx({ lens: 'flow', truncated: true, selectedLabel: 'clock-in' }))
    expect(g.lookFor).toMatch(/node cap/i)
  })

  it('notes a picked relation', () => {
    const g = viewGuide(ctx({ picked: true, selectedLabel: 'schedule-api' }))
    expect(g.lookFor).toMatch(/inspector has the selected line/i)
  })
})

describe('guideKind', () => {
  it('reads kind from the selection id', () => {
    expect(guideKind('endpoint:ts:a.ts:GET /x')).toBe('endpoint')
    expect(guideKind('method:ts:a.ts:execute')).toBe('method')
    expect(guideKind('type:ts:a.ts:Clock', { abstract: true })).toBe('interface')
    expect(guideKind('file:ts:a.ts')).toBe('file')
  })

  it('falls back to the focused node', () => {
    expect(guideKind('', { kind: 'type', abstract: false })).toBe('type')
    expect(guideKind('', { leaf: true })).toBe('file')
    expect(guideKind('', { kind: 'module', leaf: false })).toBe('folder')
  })
})
