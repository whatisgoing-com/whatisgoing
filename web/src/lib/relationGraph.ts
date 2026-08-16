import type { RelatedEntity } from '../api/types'

export interface RelationGraphNode {
  id: number
  name: string
  label: string
  type: string
  weight: number
  x: number
  y: number
  labelX: number
  labelY: number
  anchor: 'start' | 'middle' | 'end'
  radius: number
  edgeWidth: number
  colorClass: string
}

export interface RelationGraphLayout {
  size: number
  cx: number
  cy: number
  centerName: string
  centerInitial: string
  nodes: RelationGraphNode[]
}

// Keeps node labels short enough that even long topic phrases stay
// legible next to their node instead of overlapping a neighbor — SVG text
// doesn't wrap.
const LABEL_MAX_CHARS = 22

function truncateLabel(name: string): string {
  if (name.length <= LABEL_MAX_CHARS) return name
  return name.slice(0, LABEL_MAX_CHARS - 1).trimEnd() + '…'
}

function nodeColorClass(entityType: string): string {
  switch (entityType) {
    case 'PERSON':
      return 'fill-blue-500'
    case 'ORG':
      return 'fill-purple-500'
    case 'TOPIC':
      return 'fill-amber-500'
    default:
      return 'fill-gray-400'
  }
}

// Ported from cmd/ui/templates.go's buildRelationGraph (issue #46): lays
// related entities out on a circle around a center node, edges weighted
// by co-occurrence count, nodes colored by type.
export function buildRelationGraph(centerName: string, related: RelatedEntity[]): RelationGraphLayout {
  const size = 420
  const center = size / 2
  const ringRadius = 150
  const labelGap = 14

  const maxWeight = related.reduce((max, r) => Math.max(max, r.cooccurrence_count), 0)
  const n = related.length

  const nodes: RelationGraphNode[] = related.map((r, i) => {
    const angle = -Math.PI / 2 + i * ((2 * Math.PI) / n)
    const cos = Math.cos(angle)
    const sin = Math.sin(angle)
    const x = center + ringRadius * cos
    const y = center + ringRadius * sin

    let anchor: RelationGraphNode['anchor'] = 'middle'
    let labelX = x
    if (cos > 0.3) {
      anchor = 'start'
      labelX = x + labelGap
    } else if (cos < -0.3) {
      anchor = 'end'
      labelX = x - labelGap
    }

    let labelY = y
    if (sin < -0.5) {
      labelY = y - labelGap
    } else if (sin > 0.5) {
      labelY = y + labelGap + 4
    }

    const edgeWidth = maxWeight > 0 ? 1.5 + (5 * r.cooccurrence_count) / maxWeight : 1.5

    return {
      id: r.id,
      name: r.name,
      label: truncateLabel(r.name),
      type: r.type,
      weight: r.cooccurrence_count,
      x,
      y,
      labelX,
      labelY,
      anchor,
      radius: 8,
      edgeWidth,
      colorClass: nodeColorClass(r.type),
    }
  })

  return {
    size,
    cx: center,
    cy: center,
    centerName,
    centerInitial: centerName ? centerName[0].toUpperCase() : '?',
    nodes,
  }
}
