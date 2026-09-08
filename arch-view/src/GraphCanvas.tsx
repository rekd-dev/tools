import { memo } from 'react'
import {
  Background,
  Controls,
  Handle,
  MiniMap,
  Position,
  ReactFlow,
  ReactFlowProvider,
  type Edge,
  type Node,
  type NodeMouseHandler,
  type NodeProps,
} from '@xyflow/react'
import '@xyflow/react/dist/style.css'
import type { ArchNodeData } from './layout'
import { layerPhrase } from './Diagram'

const ArchNode = memo(function ArchNode({ data, selected }: NodeProps<Node<ArchNodeData>>) {
  const n = data.node
  const issue = data.issue ?? (n.cycle ? 'error' : undefined)
  return (
    <div
      className={[
        'min-w-[11rem] max-w-[16rem] rounded-md border px-3 py-2 text-left bg-zinc-900',
        selected ? 'ring-2 ring-sky-400' : '',
        issue === 'error' ? 'border-red-500' : issue === 'warning' ? 'border-amber-500' : 'border-zinc-700',
      ].join(' ')}
    >
      <Handle type="target" position={Position.Top} />
      <div className="text-sm font-semibold text-zinc-100 break-words leading-snug">{n.label}</div>
      <div className="mt-1 flex flex-wrap gap-1 text-[11px]">
        {n.kind ? <span className="rounded bg-zinc-800 px-1.5 py-0.5 text-zinc-400">{n.kind}</span> : null}
        {n.archLayer ? <span className="rounded bg-zinc-800 px-1.5 py-0.5 text-zinc-300">{layerPhrase(n.archLayer)}</span> : null}
        {n.abstract ? <span className="rounded bg-emerald-950 px-1.5 py-0.5 text-emerald-300">interface</span> : null}
        {n.cycle ? <span className="rounded bg-red-950 px-1.5 py-0.5 text-red-300">cycle</span> : null}
      </div>
      <Handle type="source" position={Position.Bottom} />
    </div>
  )
})

const nodeTypes = { arch: ArchNode }

type Props = {
  nodes: Node<ArchNodeData>[]
  edges: Edge[]
  onSelect: (id: string) => void
  onDrill: (id: string) => void
}

export function GraphCanvas({ nodes, edges, onSelect, onDrill }: Props) {
  const onNodeClick: NodeMouseHandler = (_e, node) => {
    onSelect(node.id)
  }
  const onNodeDoubleClick: NodeMouseHandler = (_e, node) => {
    onDrill(node.id)
  }
  return (
    <div className="h-[34rem] rounded-md border border-zinc-800 bg-zinc-950">
      <ReactFlowProvider>
        <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={nodeTypes}
        onNodeClick={onNodeClick}
        onNodeDoubleClick={onNodeDoubleClick}
        nodesDraggable={false}
        nodesConnectable={false}
        fitView
        minZoom={0.2}
        maxZoom={1.5}
        proOptions={{ hideAttribution: true }}
      >
        <Background color="#3f3f46" gap={18} />
        <Controls />
        <MiniMap pannable zoomable style={{ background: '#18181b' }} />
      </ReactFlow>
      </ReactFlowProvider>
    </div>
  )
}
