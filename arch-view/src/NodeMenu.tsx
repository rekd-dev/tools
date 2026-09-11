import { useEffect, useRef } from 'react'

export type MenuItem = { id: string; label: string }

type Props = {
  x: number
  y: number
  items: MenuItem[]
  onPick: (id: string) => void
  onClose: () => void
}

export function NodeMenu({ x, y, items, onPick, onClose }: Props) {
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const onDoc = (e: MouseEvent) => {
      if (box.current?.contains(e.target as Node)) return
      onClose()
    }
    const onKey = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      e.preventDefault()
      e.stopImmediatePropagation()
      onClose()
    }
    document.addEventListener('mousedown', onDoc)
    document.addEventListener('keydown', onKey, true)
    return () => {
      document.removeEventListener('mousedown', onDoc)
      document.removeEventListener('keydown', onKey, true)
    }
  }, [onClose])
  const left = Math.min(x, window.innerWidth - 176)
  const top = Math.min(y, window.innerHeight - items.length * 36 - 16)
  return (
    <div
      ref={box}
      role="menu"
      className="fixed z-50 min-w-40 rounded-md border border-zinc-700 bg-zinc-900 py-1 shadow-xl"
      style={{ left, top }}
      onContextMenu={(e) => e.preventDefault()}
    >
      {items.map((item) => (
        <button
          key={item.id}
          type="button"
          role="menuitem"
          className="block w-full px-3 py-1.5 text-left text-sm text-zinc-200 hover:bg-zinc-800"
          onClick={() => onPick(item.id)}
        >
          {item.label}
        </button>
      ))}
    </div>
  )
}
