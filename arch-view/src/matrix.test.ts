import { describe, expect, it } from 'vitest'
import { buildDepMatrix, cellKey, cyclePairs } from './matrix'
import type { ViewNode } from './types'

const node = (id: string): ViewNode => ({
  id,
  label: id,
  layer: 0,
  leaf: false,
  abstract: false,
  cycle: false,
})

describe('buildDepMatrix', () => {
  it('counts directed deps and marks cycle cells', () => {
    const model = buildDepMatrix(
      [node('application'), node('infrastructure'), node('domain')],
      [
        { from: 'application', to: 'domain', kind: 'direct' },
        { from: 'infrastructure', to: 'application', kind: 'direct' },
        { from: 'application', to: 'infrastructure', kind: 'direct' },
      ],
      ['application -> infrastructure -> application'],
    )
    expect(model.nodes.map((n) => n.id)).toEqual(['application', 'domain', 'infrastructure'])
    expect(model.cells.get(cellKey('application', 'domain'))?.count).toBe(1)
    expect(model.cells.get(cellKey('application', 'infrastructure'))?.cyclic).toBe(true)
    expect(model.cells.get(cellKey('infrastructure', 'application'))?.cyclic).toBe(true)
    expect(model.cells.get(cellKey('application', 'domain'))?.cyclic).toBe(false)
  })
})

describe('cyclePairs', () => {
  it('parses arrows in either style', () => {
    const pairs = cyclePairs(['a → b -> a'], ['a', 'b'])
    expect(pairs.has(cellKey('a', 'b'))).toBe(true)
    expect(pairs.has(cellKey('b', 'a'))).toBe(true)
  })
})
