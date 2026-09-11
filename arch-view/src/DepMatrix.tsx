import { useMemo } from 'react'
import { buildDepMatrix, cellKey, type MatrixLink } from './matrix'
import type { ViewNode } from './types'

type Props = {
  nodes: ViewNode[]
  edges: MatrixLink[]
  cycles?: string[]
  focusId?: string
  pickedFrom?: string
  pickedTo?: string
  onSelect: (id: string) => void
  onEdge: (from: string, to: string) => void
  onDrill?: (id: string) => void
  onMenu?: (id: string, x: number, y: number) => void
}

export function DepMatrix({ nodes, edges, cycles, focusId, pickedFrom, pickedTo, onSelect, onEdge, onDrill, onMenu }: Props) {
  const model = useMemo(() => buildDepMatrix(nodes, edges, cycles), [nodes, edges, cycles])
  const n = model.nodes.length
  if (!n) {
    return <p className="text-zinc-500 text-sm">Nothing under this path. Run inventory + serve.</p>
  }
  return (
    <div className="h-full min-h-0 w-full rounded-md border border-zinc-800 bg-zinc-950 overflow-auto">
      {model.truncated ? (
        <p className="px-3 pt-2 text-[11px] text-zinc-500">
          Showing {n} of {n + model.truncated}. Drill in for a smaller folder.
        </p>
      ) : null}
      <table className="border-collapse text-[11px] m-2">
        <thead>
          <tr>
            <th className="sticky left-0 top-0 z-20 bg-zinc-950 w-40" />
            {model.nodes.map((col, i) => (
              <th key={col.id} className="p-0 align-bottom">
                <button
                  type="button"
                  title={col.label}
                  className={`w-7 h-16 text-zinc-400 hover:text-zinc-100 ${focusId === col.id ? 'text-sky-300' : ''}`}
                  onClick={() => onSelect(col.id)}
                  onDoubleClick={() => onDrill?.(col.id)}
                  onContextMenu={(e) => {
                    e.preventDefault()
                    onSelect(col.id)
                    onMenu?.(col.id, e.clientX, e.clientY)
                  }}
                >
                  {i + 1}
                </button>
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {model.nodes.map((row, ri) => (
            <tr key={row.id}>
              <th className="sticky left-0 z-10 bg-zinc-950 text-left font-medium px-2 py-0.5 whitespace-nowrap">
                <button
                  type="button"
                  className={`max-w-[10rem] truncate ${focusId === row.id ? 'text-sky-300' : 'text-zinc-300 hover:text-zinc-100'}`}
                  onClick={() => onSelect(row.id)}
                  onDoubleClick={() => onDrill?.(row.id)}
                  onContextMenu={(e) => {
                    e.preventDefault()
                    onSelect(row.id)
                    onMenu?.(row.id, e.clientX, e.clientY)
                  }}
                  title={row.label}
                >
                  {ri + 1} {row.label}
                </button>
              </th>
              {model.nodes.map((col) => {
                const cell = model.cells.get(cellKey(row.id, col.id))
                const picked = pickedFrom === row.id && pickedTo === col.id
                const focused = focusId === row.id || focusId === col.id
                if (!cell) {
                  return (
                    <td
                      key={col.id}
                      className={`w-7 h-7 border border-zinc-900 ${focused ? 'bg-zinc-900/80' : 'bg-zinc-950'}`}
                    />
                  )
                }
                const kinds = cell.kinds.filter((k) => k && k !== 'direct').join(', ')
                return (
                  <td key={col.id} className="p-0 border border-zinc-800">
                    <button
                      type="button"
                      title={`${row.label} → ${col.label}${kinds ? ` (${kinds})` : ''}`}
                      className={`w-7 h-7 block text-center font-medium ${
                        cell.cyclic
                          ? 'bg-red-950 text-red-300'
                          : picked
                            ? 'bg-sky-950 text-sky-200'
                            : focused
                              ? 'bg-zinc-800 text-zinc-200'
                              : 'bg-zinc-900 text-zinc-300'
                      }`}
                      onClick={() => onEdge(row.id, col.id)}
                    >
                      {cell.count > 1 ? cell.count : '·'}
                    </button>
                  </td>
                )
              })}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
