import { describe, expect, it } from 'vitest'
import { canOpenFolder, nodeMenuActions } from './boxActions'
import type { ViewNode } from './types'

const folder = (id: string): ViewNode => ({
  id,
  label: id,
  layer: 0,
  leaf: false,
  abstract: false,
  cycle: false,
})

const file = (id: string): ViewNode => ({
  ...folder(id),
  leaf: true,
  path: `apps/${id}/src/a.ts`,
})

describe('node menu', () => {
  it('opens folders in architecture and source on leaves', () => {
    expect(canOpenFolder(folder('schedule-web'), 'architecture')).toBe(true)
    expect(nodeMenuActions(folder('schedule-web'), 'architecture').map((a) => a.id)).toEqual(['open', 'focus', 'flow', 'copy'])
    expect(nodeMenuActions(folder('schedule-web'), 'architecture')[0]?.label).toBe('Open folder')
    expect(canOpenFolder(file('a.ts'), 'architecture')).toBe(false)
    expect(nodeMenuActions(file('a.ts'), 'architecture')[0]?.label).toBe('Open source')
  })

  it('opens source from focus instead of drilling a folder', () => {
    const typeNode: ViewNode = { ...folder('ClockInUseCase'), kind: 'type', path: 'uc.ts' }
    expect(canOpenFolder(typeNode, 'focus')).toBe(false)
    expect(nodeMenuActions(typeNode, 'focus').map((a) => a.id)).toEqual(['open', 'flow', 'copy'])
  })
})
