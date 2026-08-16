import { Link } from 'react-router-dom'
import type { RelatedEntity } from '../api/types'
import { buildRelationGraph } from '../lib/relationGraph'

export function RelationGraph({ centerName, related }: { centerName: string; related: RelatedEntity[] }) {
  if (related.length === 0) {
    return <p className="py-4 text-center text-sm text-gray-500">No related entities yet.</p>
  }

  const graph = buildRelationGraph(centerName, related)

  return (
    <svg viewBox={`0 0 ${graph.size} ${graph.size}`} className="mx-auto block w-full max-w-xl overflow-visible">
      {graph.nodes.map((node) => (
        <line key={`edge-${node.id}`} x1={graph.cx} y1={graph.cy} x2={node.x} y2={node.y} stroke="#e5e7eb" strokeWidth={node.edgeWidth}>
          <title>
            {graph.centerName} &harr; {node.name}: {node.weight} shared article{node.weight !== 1 ? 's' : ''}
          </title>
        </line>
      ))}
      {graph.nodes.map((node) => (
        <Link key={`node-${node.id}`} to={`/entities/${node.id}`}>
          <circle cx={node.x} cy={node.y} r={node.radius} className={node.colorClass} stroke="white" strokeWidth={1.5}>
            <title>
              {node.name} ({node.type})
            </title>
          </circle>
          <text x={node.labelX} y={node.labelY} textAnchor={node.anchor} className="fill-gray-700 text-[10px]">
            {node.label}
          </text>
        </Link>
      ))}
      <circle cx={graph.cx} cy={graph.cy} r={18} className="fill-[#1C2430]" stroke="white" strokeWidth={2}>
        <title>{graph.centerName} (this entity)</title>
      </circle>
      <text x={graph.cx} y={graph.cy} textAnchor="middle" dominantBaseline="central" className="fill-white text-[11px] font-semibold">
        {graph.centerInitial}
      </text>
    </svg>
  )
}
