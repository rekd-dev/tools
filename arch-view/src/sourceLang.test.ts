import { describe, expect, it } from 'vitest'
import { languageFromPath } from './sourceLang'

describe('languageFromPath', () => {
  it('maps common source extensions to highlighter ids', () => {
    expect(languageFromPath('apps/web/src/useClock.ts')).toBe('typescript')
    expect(languageFromPath('apps/web/src/App.tsx')).toBe('tsx')
    expect(languageFromPath('scripts/seed.mjs')).toBe('javascript')
    expect(languageFromPath('internal/serve/graph.go')).toBe('go')
    expect(languageFromPath('src/Program.cs')).toBe('csharp')
    expect(languageFromPath('package.json')).toBe('json')
    expect(languageFromPath('docs/notes.md')).toBe('markdown')
    expect(languageFromPath('compose.yml')).toBe('yaml')
  })

  it('falls back to plaintext for unknown files', () => {
    expect(languageFromPath('README')).toBe('plaintext')
    expect(languageFromPath('bin/app.exe')).toBe('plaintext')
  })
})
