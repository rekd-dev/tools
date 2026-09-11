import { createHighlighterCore } from 'shiki/core'
import { createJavaScriptRegexEngine } from 'shiki/engine/javascript'
import { languageFromPath } from './sourceLang'

type Highlighter = Awaited<ReturnType<typeof createHighlighterCore>>

let highlighterPromise: Promise<Highlighter> | null = null

function loadHighlighter() {
  if (!highlighterPromise) {
    highlighterPromise = createHighlighterCore({
      themes: [import('@shikijs/themes/dark-plus')],
      langs: [
        import('@shikijs/langs/typescript'),
        import('@shikijs/langs/tsx'),
        import('@shikijs/langs/javascript'),
        import('@shikijs/langs/jsx'),
        import('@shikijs/langs/go'),
        import('@shikijs/langs/csharp'),
        import('@shikijs/langs/json'),
        import('@shikijs/langs/css'),
        import('@shikijs/langs/scss'),
        import('@shikijs/langs/html'),
        import('@shikijs/langs/markdown'),
        import('@shikijs/langs/python'),
        import('@shikijs/langs/sql'),
        import('@shikijs/langs/yaml'),
        import('@shikijs/langs/bash'),
        import('@shikijs/langs/xml'),
      ],
      engine: createJavaScriptRegexEngine(),
    })
  }
  return highlighterPromise
}

export function escapeHtml(text: string) {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}

export function plainSourceHtml(text: string, line?: number) {
  const lines = text.split('\n')
  const body = lines
    .map((ln, i) => {
      const n = i + 1
      const cls = line === n ? 'line source-line-active' : 'line'
      return `<span class="${cls}">${escapeHtml(ln)}</span>`
    })
    .join('\n')
  return `<pre class="shiki source-plain"><code>${body}</code></pre>`
}

export async function highlightSource(text: string, file: string, line?: number): Promise<string> {
  const lang = languageFromPath(file)
  try {
    const highlighter = await loadHighlighter()
    const loaded = highlighter.getLoadedLanguages()
    const useLang = loaded.includes(lang) ? lang : 'plaintext'
    return highlighter.codeToHtml(text, {
      lang: useLang,
      theme: 'dark-plus',
      transformers: [
        {
          name: 'active-line',
          line(node, ln) {
            if (line && ln === line) this.addClassToHast(node, 'source-line-active')
          },
        },
      ],
    })
  } catch {
    return plainSourceHtml(text, line)
  }
}
