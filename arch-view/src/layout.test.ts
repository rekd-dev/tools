import { describe, expect, it } from 'vitest'
import { archRank, familyKey, hasSharedFamilies, layoutArchitecture, layoutGraph, layoutGraphAsync, pickCenter, shouldUseRadial } from './layout'
import type { ViewNode } from './types'

const node = (id: string, extra: Partial<ViewNode> = {}): ViewNode => ({
  id,
  label: id,
  layer: 0,
  leaf: false,
  abstract: false,
  cycle: false,
  ...extra,
})

describe('architecture layout', () => {
  it('uses layered ranks when Clean Architecture folders are present', () => {
    const view = {
      path: 'schedule-api',
      nodes: [
        node('domain', { archLayer: 'domain' }),
        node('application', { archLayer: 'application' }),
        node('http', { archLayer: 'interface' }),
      ],
      edges: [
        { from: 'http', to: 'application', kind: 'direct' },
        { from: 'application', to: 'domain', kind: 'direct' },
      ],
      cycles: [],
      childPaths: [],
    }
    expect(shouldUseRadial(view.nodes)).toBe(false)
    const { nodes } = layoutArchitecture(view, {})
    const y = Object.fromEntries(nodes.filter((n) => n.type === 'arch').map((n) => [n.id, n.position.y]))
    expect(y.http).toBeLessThan(y.application)
    expect(y.application).toBeLessThan(y.domain)
  })

  it('uses a hub layout when apps do not share architecture folders', () => {
    const nodes = [node('auth'), node('schedule-web', { archLayer: 'web' }), node('playwright')]
    expect(shouldUseRadial(nodes)).toBe(true)
    const view = {
      path: '',
      nodes,
      edges: [
        { from: 'schedule-web', to: 'auth', kind: 'direct' },
        { from: 'playwright', to: 'auth', kind: 'direct' },
      ],
      cycles: [],
      childPaths: [],
    }
    const placed = layoutArchitecture(view, {}).nodes.filter((n) => n.type === 'arch')
    const auth = placed.find((n) => n.id === 'auth')!
    const others = placed.filter((n) => n.id !== 'auth')
    const dist = (n: (typeof placed)[0]) =>
      Math.hypot(n.position.x - auth.position.x, n.position.y - auth.position.y)
    expect(others.every((n) => dist(n) > 80)).toBe(true)
  })

  it('clusters sibling apps that share a product name', () => {
    const nodes = [
      node('auth'),
      node('schedule-api'),
      node('schedule-web'),
      node('reporting-api'),
      node('reporting-web'),
      node('ouac-schedule-import'),
    ]
    expect(hasSharedFamilies(nodes)).toBe(true)
    expect(familyKey('schedule-web', nodes.map((n) => n.id))).toBe('schedule')
    expect(familyKey('ouac-schedule-import', nodes.map((n) => n.id))).toBe('schedule')
    const placed = layoutArchitecture(
      {
        path: '',
        nodes,
        edges: [{ from: 'schedule-web', to: 'auth', kind: 'direct' }],
        cycles: [],
        childPaths: [],
      },
      {},
    ).nodes.filter((n) => n.type === 'arch')
    const pos = (id: string) => placed.find((n) => n.id === id)!.position
    const dist = (a: string, b: string) => Math.hypot(pos(a).x - pos(b).x, pos(a).y - pos(b).y)
    expect(dist('schedule-api', 'schedule-web')).toBeLessThan(dist('schedule-api', 'reporting-api'))
    expect(dist('reporting-api', 'reporting-web')).toBeLessThan(dist('reporting-api', 'auth'))
  })

  it('puts the selected focus node in the center', () => {
    const nodes = [node('a'), node('b'), node('sel')]
    const { nodes: placed } = layoutGraph(nodes, [{ from: 'a', to: 'b', kind: 'calls' }], {}, 'sel', 'radial')
    const sel = placed.find((n) => n.id === 'sel')!
    const a = placed.find((n) => n.id === 'a')!
    const cx = sel.position.x
    const cy = sel.position.y
    expect(Math.hypot(a.position.x - cx, a.position.y - cy)).toBeGreaterThan(80)
  })
})

describe('pickCenter', () => {
  it('prefers the most connected node', () => {
    const nodes = [node('auth'), node('web'), node('tools')]
    const neighbors = new Map([
      ['auth', ['web', 'tools']],
      ['web', ['auth']],
      ['tools', ['auth']],
    ])
    expect(pickCenter(nodes, neighbors)).toBe('auth')
  })
})

describe('archRank', () => {
  it('puts domain below application', () => {
    expect(archRank(node('d', { archLayer: 'domain' }))).toBeLessThan(archRank(node('a', { archLayer: 'application' })))
  })
})

describe('flow layout', () => {
  it('places callers left of callees', async () => {
    const { nodes: placed } = await layoutGraphAsync(
      [node('ClockInUseCase'), node('TimeSessionRepository')],
      [{ from: 'ClockInUseCase', to: 'TimeSessionRepository', kind: 'injects' }],
      {},
      'ClockInUseCase',
      'flow',
    )
    const from = placed.find((n) => n.id === 'ClockInUseCase')!
    const to = placed.find((n) => n.id === 'TimeSessionRepository')!
    expect(to.position.x).toBeGreaterThan(from.position.x)
  })
})
