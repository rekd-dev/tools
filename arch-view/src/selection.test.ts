import { describe, expect, it } from 'vitest'
import { displayName, groupedRelations, kindPhrase, relationPhrase } from './selection'
import type { GraphEdge } from './types'

describe('selection copy', () => {
  it('uses the type name, not the raw id', () => {
    expect(displayName('type:typescript:apps/api/src/foo.ts:UserRepository')).toBe('UserRepository')
    expect(displayName('file:typescript:apps/web/src/pages/duties/duties-shared.ts')).toBe('duties-shared.ts')
  })

  it('groups contains as Defines', () => {
    const edges: GraphEdge[] = [
      { from: 'file:ts:a.ts', to: 'type:ts:a.ts:Foo', kind: 'contains', source: 'heuristic', confidence: 0.55 },
      { from: 'file:ts:a.ts', to: 'type:ts:a.ts:Bar', kind: 'contains', source: 'heuristic', confidence: 0.55 },
    ]
    const groups = groupedRelations(edges, 'out')
    expect(groups).toEqual([{ label: 'Defines', names: ['Bar', 'Foo'] }])
  })

  it('reads fromId/toId when from/to are missing', () => {
    const edges = [{ kind: 'imports', fromId: 'a', toId: 'file:ts:b.ts', source: 'heuristic' }] as unknown as GraphEdge[]
    const groups = groupedRelations(edges, 'out')
    expect(groups[0]?.names).toEqual(['b.ts'])
  })

  it('calls an interface an interface', () => {
    expect(kindPhrase('type', true)).toBe('Interface')
    expect(kindPhrase('file')).toBe('File')
  })

  it('names a relation in plain language', () => {
    expect(relationPhrase('direct')).toBe('Depends on')
    expect(relationPhrase('injects')).toBe('Injects into')
    expect(relationPhrase('calls', true)).toBe('Calls (unresolved)')
  })
})
