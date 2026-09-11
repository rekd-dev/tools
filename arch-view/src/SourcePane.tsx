import { useEffect, useRef, useState } from 'react'
import { highlightSource, plainSourceHtml } from './sourceHighlight'

const MIN_H = 160

function defaultHeight() {
  if (typeof window === 'undefined') return 320
  return Math.round(window.innerHeight * 0.38)
}

function readHeight() {
  try {
    const n = Number(localStorage.getItem('arch-view.sourceH'))
    if (Number.isFinite(n) && n >= MIN_H) return n
  } catch {
    /* ignore */
  }
  return defaultHeight()
}

function persistHeight(px: number) {
  try {
    localStorage.setItem('arch-view.sourceH', String(px))
  } catch {
    /* ignore */
  }
}

export function SourcePane({
  file,
  text,
  line,
  onClose,
}: {
  file: string
  text: string
  line?: number
  onClose: () => void
}) {
  const [html, setHtml] = useState(() => plainSourceHtml(text, line))
  const [height, setHeight] = useState(readHeight)
  const scroller = useRef<HTMLDivElement>(null)
  const heightRef = useRef(height)
  heightRef.current = height

  useEffect(() => {
    setHtml(plainSourceHtml(text, line))
    let cancelled = false
    void highlightSource(text, file, line).then((next) => {
      if (!cancelled) setHtml(next)
    })
    return () => {
      cancelled = true
    }
  }, [file, text, line])

  useEffect(() => {
    const el = scroller.current?.querySelector('.source-line-active')
    el?.scrollIntoView({ block: 'center' })
  }, [html, line])

  const onDrag = (e: React.MouseEvent) => {
    e.preventDefault()
    const startY = e.clientY
    const startH = heightRef.current
    const onMove = (ev: MouseEvent) => {
      const max = Math.round(window.innerHeight * 0.75)
      const next = Math.min(Math.max(startH + (startY - ev.clientY), MIN_H), max)
      heightRef.current = next
      setHeight(next)
    }
    const onUp = () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
      persistHeight(heightRef.current)
    }
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
  }

  return (
    <section className="source-pane shrink-0 flex flex-col border-t border-zinc-800 bg-[#1e1e1e]" style={{ height }} aria-label="Source">
      <div
        role="separator"
        aria-orientation="horizontal"
        aria-label="Resize source"
        className="h-1.5 shrink-0 cursor-ns-resize bg-zinc-800 hover:bg-sky-700"
        onMouseDown={onDrag}
      />
      <header className="shrink-0 flex items-center gap-2 px-3 py-1.5 border-b border-zinc-800">
        <div className="text-xs font-semibold truncate font-mono text-zinc-200">
          {file}
          {line ? `:${line}` : ''}
        </div>
        <button type="button" className="ml-auto text-xs text-zinc-400 hover:text-zinc-200" onClick={onClose}>
          Close
        </button>
      </header>
      <div ref={scroller} className="flex-1 min-h-0 overflow-auto">
        <div dangerouslySetInnerHTML={{ __html: html }} />
      </div>
    </section>
  )
}
