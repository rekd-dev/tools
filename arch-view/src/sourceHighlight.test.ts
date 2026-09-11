import { describe, expect, it } from 'vitest'
import { escapeHtml, highlightSource, plainSourceHtml } from './sourceHighlight'

describe('source highlighting', () => {
  it('marks the active line in the plaintext fallback', () => {
    const html = plainSourceHtml('one\ntwo\nthree', 2)
    expect(html).toContain('source-line-active')
    expect(html).toContain('two')
    expect(html).not.toContain('<script')
  })

  it('escapes html in source text', () => {
    expect(escapeHtml('<div class="x">')).toBe('&lt;div class="x"&gt;')
  })

  it('colorizes typescript with an IDE theme', async () => {
    const html = await highlightSource('const name = "ClockInUseCase"\n', 'apps/web/src/clock.ts', 1)
    expect(html).toContain('shiki')
    expect(html).toContain('source-line-active')
    expect(html).toMatch(/color\s*:/)
  })
})
