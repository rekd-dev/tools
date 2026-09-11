import { memo, useEffect, useRef } from 'react'
import {
  Background,
  Controls,
  Handle,
  Position,
  ReactFlow,
  ReactFlowProvider,
  useReactFlow,
  type Edge,
  type EdgeMouseHandler,
  type Node,
  type NodeMouseHandler,
  type NodeProps,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { ArchNodeData } from './layout'
import { layerPhrase } from './Diagram'

const ArchNode = memo(function ArchNode({ data, selected, sourcePosition, targetPosition }: NodeProps<Node<ArchNodeData>>) {
  const n = data.node
  const issue = data.issue ?? (n.cycle ? 'error' : undefined)
  return (
    <div
      className={[
        'min-w-[11rem] max-w-[16rem] rounded-md border px-3 py-2 text-left bg-zinc-900',
        selected ? 'ring-2 ring-sky-400' : '',
        issue === 'error' ? 'border-red-500' : issue === 'warning' ? 'border-amber-500' : n.churnCommits ? 'border-orange-500' : 'border-zinc-700',
      ].join(' ')}
    >
      <Handle type="target" position={targetPosition ?? Position.Top} />
      <div className="text-sm font-semibold text-zinc-100 break-words leading-snug">{n.label}</div>
      <div className="mt-1 flex flex-wrap gap-1 text-[11px]">
        {n.kind ? <span className="rounded bg-zinc-800 px-1.5 py-0.5 text-zinc-400">{n.kind}</span> : null}
        {n.archLayer ? <span className="rounded bg-zinc-800 px-1.5 py-0.5 text-zinc-300">{layerPhrase(n.archLayer)}</span> : null}
        {n.abstract ? <span className="rounded bg-emerald-950 px-1.5 py-0.5 text-emerald-300">interface</span> : null}
        {n.cycle ? <span className="rounded bg-red-950 px-1.5 py-0.5 text-red-300">cycle</span> : null}
        {n.churnCommits ? (
          <span className="rounded bg-orange-950 px-1.5 py-0.5 text-orange-300">{n.churnCommits} commits</span>
        ) : null}
      </div>
      <Handle type="source" position={sourcePosition ?? Position.Bottom} />
    </div>
  )
})

const CaptionNode = memo(function CaptionNode({ data }: NodeProps<Node<ArchNodeData>>) {
  return <div className="text-[11px] font-medium uppercase tracking-wide text-zinc-500 min-w-36">{data.label}</div>
})

const nodeTypes = { arch: ArchNode, caption: CaptionNode }

type Props = {
  nodes: Node<ArchNodeData>[]
  edges: Edge[]
  onSelect: (id: string) => void
  onDrill: (id: string) => void
  onUp?: () => void
  onEdge?: (id: string, from: string, to: string) => void
  onMenu?: (id: string, x: number, y: number) => void
  onPane?: () => void
}

function FitToBox() {
  const { fitView } = useReactFlow()
  const box = useRef<HTMLDivElement>(null)
  useEffect(() => {
    const el = box.current
    if (!el) return
    let t = 0
    const ro = new ResizeObserver(() => {
      window.clearTimeout(t)
      t = window.setTimeout(() => fitView({ padding: 0.12 }), 60)
    })
    ro.observe(el)
    return () => {
      window.clearTimeout(t)
      ro.disconnect()
    }
  }, [fitView])
  return <div ref={box} className="pointer-events-none absolute inset-0" aria-hidden />
}

export function GraphCanvas({ nodes, edges, onSelect, onDrill, onUp, onEdge, onMenu, onPane }: Props) {
  const onNodeClick: NodeMouseHandler = (_e, node) => {
    if (node.type === 'caption') return
    onSelect(node.id)
  }
  const onNodeDoubleClick: NodeMouseHandler = (e, node) => {
    if (node.type === 'caption') return
    e.preventDefault()
    onDrill(node.id)
  }
  const onNodeContextMenu: NodeMouseHandler = (e, node) => {
    if (node.type === 'caption') return
    e.preventDefault()
    e.stopPropagation()
    onSelect(node.id)
    onMenu?.(node.id, e.clientX, e.clientY)
  }
  const onEdgeClick: EdgeMouseHandler = (_e, edge) => {
    onEdge?.(edge.id, edge.source, edge.target)
  }
  return (
    <div className="h-full min-h-0 w-full rounded-md border border-zinc-800 bg-zinc-950">
      <ReactFlowProvider>
        <div className="relative h-full w-full">
          <FitToBox />
          <ReactFlow
            nodes={nodes}
            edges={edges}
            nodeTypes={nodeTypes}
            onNodeClick={onNodeClick}
            onNodeDoubleClick={onNodeDoubleClick}
            onNodeContextMenu={onNodeContextMenu}
            onEdgeClick={onEdgeClick}
            onPaneClick={(e) => {
              onPane?.()
              if (e.detail === 2) onUp?.()
            }}
            zoomOnDoubleClick={false}
            nodesDraggable={false}
            nodesConnectable={false}
            edgesReconnectable={false}
            defaultEdgeOptions={{ interactionWidth: 24 }}
            fitView
            fitViewOptions={{ padding: 0.12 }}
            minZoom={0.15}
            maxZoom={1.5}
            colorMode="dark"
            proOptions={{ hideAttribution: true }}
            style={{ width: '100%', height: '100%' }}
          >
            <Background color="#3f3f46" gap={18} />
            <Controls />
          </ReactFlow>
        </div>
      </ReactFlowProvider>
    </div>
  )
}
