import { describe, expect, it } from 'vitest'
import { parseHash } from './state'

describe('hash state', () => {
  it('restores lens, path, selection, and overlays', () => {
    const s = parseHash('#lens=focus&path=application&sel=type:csharp:a.cs:IRepo&overlays=data,async')
    expect(s.lens).toBe('focus')
    expect(s.path).toBe('application')
    expect(s.sel).toContain('IRepo')
    expect(s.overlays.data).toBe(true)
    expect(s.overlays.async).toBe(true)
  })
})
