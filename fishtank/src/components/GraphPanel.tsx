import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent
} from 'react'
import * as d3 from 'd3'
import '../styles/GraphPanel.css'

export type GraphNode = {
  uuid: string
  name?: string
  labels?: string[]
  attributes?: Record<string, unknown>
  summary?: string
  created_at?: string
}

export type GraphEdge = {
  uuid?: string
  source_node_uuid: string
  target_node_uuid: string
  name?: string
  fact_type?: string
  fact?: string
  episodes?: string[]
  created_at?: string
  valid_at?: string
}

export type GraphData = {
  nodes?: GraphNode[]
  edges?: GraphEdge[]
  node_count?: number
  edge_count?: number
}

type SimNode = d3.SimulationNodeDatum & {
  id: string
  name: string
  type: string
  rawData: GraphNode
}

type SimEdge = {
  source: string | SimNode
  target: string | SimNode
  type: string
  name: string
  curvature: number
  isSelfLoop: boolean
  pairIndex?: number
  pairTotal?: number
  rawData: Record<string, unknown>
}

export type SelectedGraphItem =
  | {
      type: 'node'
      data: GraphNode
      entityType: string
      color: string
    }
  | { type: 'edge'; data: Record<string, unknown> }

type Props = {
  graphData: GraphData | null
  loading?: boolean
  currentPhase?: number
  isSimulating?: boolean
  onRefresh?: () => void
  onToggleMaximize?: () => void
}

export default function GraphPanel({
  graphData,
  loading,
  currentPhase = -1,
  isSimulating,
  onRefresh,
  onToggleMaximize
}: Props) {
  const graphContainerRef = useRef<HTMLDivElement>(null)
  const graphSvgRef = useRef<SVGSVGElement>(null)
  const [selectedItem, setSelectedItem] = useState<SelectedGraphItem | null>(null)
  const [showEdgeLabels, setShowEdgeLabels] = useState(true)
  const [expandedSelfLoops, setExpandedSelfLoops] = useState<Set<string | number>>(new Set())
  const [showSimulationFinishedHint, setShowSimulationFinishedHint] = useState(false)
  const wasSimulatingRef = useRef(false)
  const selectedNodeUuidRef = useRef<string | null>(null)

  useEffect(() => {
    selectedNodeUuidRef.current = selectedItem?.type === 'node' ? selectedItem.data.uuid : null
  }, [selectedItem])

  const linkLabelsRef = useRef<d3.Selection<SVGTextElement, SimEdge, SVGGElement, unknown> | null>(null)
  const linkLabelBgRef = useRef<d3.Selection<SVGRectElement, SimEdge, SVGGElement, unknown> | null>(null)
  const currentSimulationRef = useRef<d3.Simulation<SimNode, undefined> | null>(null)

  useEffect(() => {
    if (wasSimulatingRef.current && !isSimulating) {
      setShowSimulationFinishedHint(true)
    }
    wasSimulatingRef.current = !!isSimulating
  }, [isSimulating])

  const entityTypes = useMemo(() => {
    if (!graphData?.nodes) return []
    const typeMap: Record<string, { name: string; count: number; color: string }> = {}
    const colors = [
      '#FF6B35',
      '#004E89',
      '#7B2D8E',
      '#1A936F',
      '#C5283D',
      '#E9724C',
      '#3498db',
      '#9b59b6',
      '#27ae60',
      '#f39c12'
    ]
    graphData.nodes.forEach((node) => {
      const type = node.labels?.find((l) => l !== 'Entity') || 'Entity'
      if (!typeMap[type]) {
        typeMap[type] = {
          name: type,
          count: 0,
          color: colors[Object.keys(typeMap).length % colors.length]
        }
      }
      typeMap[type].count++
    })
    return Object.values(typeMap)
  }, [graphData])

  const formatDateTime = (dateStr?: string) => {
    if (!dateStr) return ''
    try {
      const date = new Date(dateStr)
      return date.toLocaleString('en-US', {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
        hour12: true
      })
    } catch {
      return dateStr
    }
  }

  const toggleSelfLoop = (id: string | number) => {
    setExpandedSelfLoops((prev) => {
      const next = new Set(prev)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const closeDetailPanel = () => {
    setSelectedItem(null)
    setExpandedSelfLoops(new Set())
  }

  const renderGraph = useCallback(() => {
    const graphSvg = graphSvgRef.current
    const container = graphContainerRef.current
    if (!graphSvg || !graphData) return

    if (currentSimulationRef.current) {
      currentSimulationRef.current.stop()
    }

    const width = container?.clientWidth ?? 0
    const height = container?.clientHeight ?? 0

    const svg = d3
      .select(graphSvg)
      .attr('width', width)
      .attr('height', height)
      .attr('viewBox', `0 0 ${width} ${height}`)

    svg.selectAll('*').remove()

    const nodesData = graphData.nodes || []
    const edgesData = graphData.edges || []
    if (nodesData.length === 0) return

    const nodeMap: Record<string, GraphNode> = {}
    nodesData.forEach((n) => {
      nodeMap[n.uuid] = n
    })

    const nodes: SimNode[] = nodesData.map((n) => ({
      id: n.uuid,
      name: n.name || 'Unnamed',
      type: n.labels?.find((l) => l !== 'Entity') || 'Entity',
      rawData: n
    }))

    const nodeIds = new Set(nodes.map((n) => n.id))
    const edgePairCount: Record<string, number> = {}
    const selfLoopEdges: Record<string, GraphEdge[]> = {}
    const tempEdges = edgesData.filter(
      (e) => nodeIds.has(e.source_node_uuid) && nodeIds.has(e.target_node_uuid)
    )

    tempEdges.forEach((e) => {
      if (e.source_node_uuid === e.target_node_uuid) {
        if (!selfLoopEdges[e.source_node_uuid]) selfLoopEdges[e.source_node_uuid] = []
        selfLoopEdges[e.source_node_uuid].push({
          ...e,
          source_name: nodeMap[e.source_node_uuid]?.name,
          target_name: nodeMap[e.target_node_uuid]?.name
        } as GraphEdge)
      } else {
        const pairKey = [e.source_node_uuid, e.target_node_uuid].sort().join('_')
        edgePairCount[pairKey] = (edgePairCount[pairKey] || 0) + 1
      }
    })

    const edgePairIndex: Record<string, number> = {}
    const processedSelfLoopNodes = new Set<string>()

    const edges: SimEdge[] = []

    tempEdges.forEach((e) => {
      const isSelfLoop = e.source_node_uuid === e.target_node_uuid
      if (isSelfLoop) {
        if (processedSelfLoopNodes.has(e.source_node_uuid)) return
        processedSelfLoopNodes.add(e.source_node_uuid)
        const allSelfLoops = selfLoopEdges[e.source_node_uuid]
        const nodeName = nodeMap[e.source_node_uuid]?.name || 'Unknown'
        edges.push({
          source: e.source_node_uuid,
          target: e.target_node_uuid,
          type: 'SELF_LOOP',
          name: `Self Relations (${allSelfLoops.length})`,
          curvature: 0,
          isSelfLoop: true,
          rawData: {
            isSelfLoopGroup: true,
            source_name: nodeName,
            target_name: nodeName,
            selfLoopCount: allSelfLoops.length,
            selfLoopEdges: allSelfLoops
          }
        })
        return
      }

      const pairKey = [e.source_node_uuid, e.target_node_uuid].sort().join('_')
      const totalCount = edgePairCount[pairKey]
      const currentIndex = edgePairIndex[pairKey] || 0
      edgePairIndex[pairKey] = currentIndex + 1
      const isReversed = e.source_node_uuid > e.target_node_uuid
      let curvature = 0
      if (totalCount > 1) {
        const curvatureRange = Math.min(1.2, 0.6 + totalCount * 0.15)
        curvature = ((currentIndex / (totalCount - 1)) - 0.5) * curvatureRange * 2
        if (isReversed) curvature = -curvature
      }

      edges.push({
        source: e.source_node_uuid,
        target: e.target_node_uuid,
        type: e.fact_type || e.name || 'RELATED',
        name: e.name || e.fact_type || 'RELATED',
        curvature,
        isSelfLoop: false,
        pairIndex: currentIndex,
        pairTotal: totalCount,
        rawData: {
          ...e,
          source_name: nodeMap[e.source_node_uuid]?.name,
          target_name: nodeMap[e.target_node_uuid]?.name
        }
      })
    })

    const colorMap: Record<string, string> = {}
    entityTypes.forEach((t) => {
      colorMap[t.name] = t.color
    })
    const getColor = (type: string) => colorMap[type] || '#999'

    const simulation = d3
      .forceSimulation<SimNode>(nodes)
      .force(
        'link',
        d3
          .forceLink<SimNode, SimEdge>(edges)
          .id((d) => d.id)
          .distance((d) => {
            const link = d as SimEdge & { pairTotal?: number }
            const baseDistance = 150
            const edgeCount = link.pairTotal || 1
            return baseDistance + (edgeCount - 1) * 50
          })
      )
      .force('charge', d3.forceManyBody().strength(-400))
      .force('center', d3.forceCenter(width / 2, height / 2))
      .force('collide', d3.forceCollide(50))
      .force('x', d3.forceX(width / 2).strength(0.04))
      .force('y', d3.forceY(height / 2).strength(0.04))

    currentSimulationRef.current = simulation

    const g = svg.append('g')

    svg
      .call(
        d3
          .zoom<SVGSVGElement, unknown>()
          .extent([
            [0, 0],
            [width, height]
          ])
          .scaleExtent([0.1, 4])
          .on('zoom', (event) => {
            g.attr('transform', event.transform)
          })
      )

    const linkGroup = g.append('g').attr('class', 'links')

    const getLinkPath = (d: SimEdge & { source: { x: number; y: number }; target: { x: number; y: number } }) => {
      const sx = d.source.x
      const sy = d.source.y
      const tx = d.target.x
      const ty = d.target.y
      if (d.isSelfLoop) {
        const loopRadius = 30
        const x1 = sx + 8
        const y1 = sy - 4
        const x2 = sx + 8
        const y2 = sy + 4
        return `M${x1},${y1} A${loopRadius},${loopRadius} 0 1,1 ${x2},${y2}`
      }
      if (d.curvature === 0) {
        return `M${sx},${sy} L${tx},${ty}`
      }
      const dx = tx - sx
      const dy = ty - sy
      const dist = Math.sqrt(dx * dx + dy * dy)
      const pairTotal = d.pairTotal || 1
      const offsetRatio = 0.25 + pairTotal * 0.05
      const baseOffset = Math.max(35, dist * offsetRatio)
      const offsetX = (-dy / dist) * d.curvature * baseOffset
      const offsetY = (dx / dist) * d.curvature * baseOffset
      const cx = (sx + tx) / 2 + offsetX
      const cy = (sy + ty) / 2 + offsetY
      return `M${sx},${sy} Q${cx},${cy} ${tx},${ty}`
    }

    const getLinkMidpoint = (d: SimEdge & { source: { x: number; y: number }; target: { x: number; y: number } }) => {
      const sx = d.source.x
      const sy = d.source.y
      const tx = d.target.x
      const ty = d.target.y
      if (d.isSelfLoop) {
        return { x: sx + 70, y: sy }
      }
      if (d.curvature === 0) {
        return { x: (sx + tx) / 2, y: (sy + ty) / 2 }
      }
      const dx = tx - sx
      const dy = ty - sy
      const dist = Math.sqrt(dx * dx + dy * dy)
      const pairTotal = d.pairTotal || 1
      const offsetRatio = 0.25 + pairTotal * 0.05
      const baseOffset = Math.max(35, dist * offsetRatio)
      const offsetX = (-dy / dist) * d.curvature * baseOffset
      const offsetY = (dx / dist) * d.curvature * baseOffset
      const cx = (sx + tx) / 2 + offsetX
      const cy = (sy + ty) / 2 + offsetY
      const midX = 0.25 * sx + 0.5 * cx + 0.25 * tx
      const midY = 0.25 * sy + 0.5 * cy + 0.25 * ty
      return { x: midX, y: midY }
    }

    const link = linkGroup
      .selectAll<SVGPathElement, SimEdge>('path')
      .data(edges)
      .enter()
      .append('path')
      .attr('stroke', '#C0C0C0')
      .attr('stroke-width', 1.5)
      .attr('fill', 'none')
      .style('cursor', 'pointer')
      .on('click', (event: MouseEvent, d: SimEdge) => {
        event.stopPropagation()
        linkGroup.selectAll('path').attr('stroke', '#C0C0C0').attr('stroke-width', 1.5)
        linkLabelBg.attr('fill', 'rgba(255,255,255,0.95)')
        linkLabels.attr('fill', '#666')
        d3.select(event.currentTarget).attr('stroke', '#3498db').attr('stroke-width', 3)
        setSelectedItem({ type: 'edge', data: d.rawData })
      })

    const linkLabelBg = linkGroup
      .selectAll<SVGRectElement, SimEdge>('rect')
      .data(edges)
      .enter()
      .append('rect')
      .attr('fill', 'rgba(255,255,255,0.95)')
      .attr('rx', 3)
      .attr('ry', 3)
      .style('cursor', 'pointer')
      .style('pointer-events', 'all')
      .style('display', showEdgeLabels ? 'block' : 'none')
      .on('click', (event: MouseEvent, d: SimEdge) => {
        event.stopPropagation()
        linkGroup.selectAll('path').attr('stroke', '#C0C0C0').attr('stroke-width', 1.5)
        linkLabelBg.attr('fill', 'rgba(255,255,255,0.95)')
        linkLabels.attr('fill', '#666')
        link
          .filter((l) => l === d)
          .attr('stroke', '#3498db')
          .attr('stroke-width', 3)
        d3.select(event.currentTarget).attr('fill', 'rgba(52, 152, 219, 0.1)')
        setSelectedItem({ type: 'edge', data: d.rawData })
      })

    const linkLabels = linkGroup
      .selectAll<SVGTextElement, SimEdge>('text')
      .data(edges)
      .enter()
      .append('text')
      .text((d) => d.name)
      .attr('font-size', '9px')
      .attr('fill', '#666')
      .attr('text-anchor', 'middle')
      .attr('dominant-baseline', 'middle')
      .style('cursor', 'pointer')
      .style('pointer-events', 'all')
      .style('font-family', 'system-ui, sans-serif')
      .style('display', showEdgeLabels ? 'block' : 'none')
      .on('click', (event: MouseEvent, d: SimEdge) => {
        event.stopPropagation()
        linkGroup.selectAll('path').attr('stroke', '#C0C0C0').attr('stroke-width', 1.5)
        linkLabelBg.attr('fill', 'rgba(255,255,255,0.95)')
        linkLabels.attr('fill', '#666')
        link
          .filter((l) => l === d)
          .attr('stroke', '#3498db')
          .attr('stroke-width', 3)
        d3.select(event.currentTarget).attr('fill', '#3498db')
        setSelectedItem({ type: 'edge', data: d.rawData })
      })

    linkLabelsRef.current = linkLabels
    linkLabelBgRef.current = linkLabelBg

    const nodeGroup = g.append('g').attr('class', 'nodes')

    const node = nodeGroup
      .selectAll<SVGCircleElement, SimNode>('circle')
      .data(nodes)
      .enter()
      .append('circle')
      .attr('r', 10)
      .attr('fill', (d) => getColor(d.type))
      .attr('stroke', '#fff')
      .attr('stroke-width', 2.5)
      .style('cursor', 'pointer')
      .call(
        d3
          .drag<SVGCircleElement, SimNode>()
          .on('start', (event, d) => {
            d.fx = d.x
            d.fy = d.y
            ;(d as { _dragStartX?: number })._dragStartX = event.x
            ;(d as { _dragStartY?: number })._dragStartY = event.y
            ;(d as { _isDragging?: boolean })._isDragging = false
          })
          .on('drag', (event, d) => {
            const dx = event.x - ((d as { _dragStartX?: number })._dragStartX ?? 0)
            const dy = event.y - ((d as { _dragStartY?: number })._dragStartY ?? 0)
            const distance = Math.sqrt(dx * dx + dy * dy)
            if (!(d as { _isDragging?: boolean })._isDragging && distance > 3) {
              ;(d as { _isDragging?: boolean })._isDragging = true
              simulation.alphaTarget(0.3).restart()
            }
            if ((d as { _isDragging?: boolean })._isDragging) {
              d.fx = event.x
              d.fy = event.y
            }
          })
          .on('end', (_event, d) => {
            if ((d as { _isDragging?: boolean })._isDragging) {
              simulation.alphaTarget(0)
            }
            d.fx = null
            d.fy = null
            ;(d as { _isDragging?: boolean })._isDragging = false
          })
      )
      .on('click', (event: MouseEvent, d: SimNode) => {
        event.stopPropagation()
        node.attr('stroke', '#fff').attr('stroke-width', 2.5)
        linkGroup.selectAll('path').attr('stroke', '#C0C0C0').attr('stroke-width', 1.5)
        d3.select(event.currentTarget).attr('stroke', '#E91E63').attr('stroke-width', 4)
        link
          .filter((l) => {
            const s = l.source as { id?: string }
            const t = l.target as { id?: string }
            return s.id === d.id || t.id === d.id
          })
          .attr('stroke', '#E91E63')
          .attr('stroke-width', 2.5)
        setSelectedItem({
          type: 'node',
          data: d.rawData,
          entityType: d.type,
          color: getColor(d.type)
        })
      })
      .on('mouseenter', (event: MouseEvent, d: SimNode) => {
        if (selectedNodeUuidRef.current !== d.rawData.uuid) {
          d3.select(event.currentTarget).attr('stroke', '#333').attr('stroke-width', 3)
        }
      })
      .on('mouseleave', (event: MouseEvent, d: SimNode) => {
        if (selectedNodeUuidRef.current !== d.rawData.uuid) {
          d3.select(event.currentTarget).attr('stroke', '#fff').attr('stroke-width', 2.5)
        }
      })

    const nodeLabels = nodeGroup
      .selectAll<SVGTextElement, SimNode>('text')
      .data(nodes)
      .enter()
      .append('text')
      .text((d) => (d.name.length > 8 ? d.name.substring(0, 8) + '…' : d.name))
      .attr('font-size', '11px')
      .attr('fill', '#333')
      .attr('font-weight', '500')
      .attr('dx', 14)
      .attr('dy', 4)
      .style('pointer-events', 'none')
      .style('font-family', 'system-ui, sans-serif')

    simulation.on('tick', () => {
      link.attr('d', (d) => getLinkPath(d as SimEdge & { source: { x: number; y: number }; target: { x: number; y: number } }))

      linkLabels.each(function (d: SimEdge) {
        const dd = d as SimEdge & { source: { x: number; y: number }; target: { x: number; y: number } }
        const mid = getLinkMidpoint(dd)
        d3.select(this).attr('x', mid.x).attr('y', mid.y).attr('transform', '')
      })

      linkLabelBg.each(function (d: SimEdge, i: number) {
        const dd = d as SimEdge & { source: { x: number; y: number }; target: { x: number; y: number } }
        const mid = getLinkMidpoint(dd)
        const textEl = linkLabels.nodes()[i] as SVGTextElement
        const bbox = textEl.getBBox()
        d3.select(this)
          .attr('x', mid.x - bbox.width / 2 - 4)
          .attr('y', mid.y - bbox.height / 2 - 2)
          .attr('width', bbox.width + 8)
          .attr('height', bbox.height + 4)
          .attr('transform', '')
      })

      node.attr('cx', (d) => d.x!).attr('cy', (d) => d.y!)
      nodeLabels.attr('x', (d) => d.x!).attr('y', (d) => d.y!)
    })

    svg.on('click', () => {
      setSelectedItem(null)
      node.attr('stroke', '#fff').attr('stroke-width', 2.5)
      linkGroup.selectAll('path').attr('stroke', '#C0C0C0').attr('stroke-width', 1.5)
      linkLabelBg.attr('fill', 'rgba(255,255,255,0.95)')
      linkLabels.attr('fill', '#666')
    })
  }, [graphData, entityTypes, showEdgeLabels])

  useEffect(() => {
    requestAnimationFrame(() => renderGraph())
  }, [graphData, renderGraph])

  useEffect(() => {
    if (linkLabelsRef.current) {
      linkLabelsRef.current.style('display', showEdgeLabels ? 'block' : 'none')
    }
    if (linkLabelBgRef.current) {
      linkLabelBgRef.current.style('display', showEdgeLabels ? 'block' : 'none')
    }
  }, [showEdgeLabels])

  useEffect(() => {
    const handleResize = () => {
      requestAnimationFrame(() => renderGraph())
    }
    window.addEventListener('resize', handleResize)
    return () => {
      window.removeEventListener('resize', handleResize)
      if (currentSimulationRef.current) {
        currentSimulationRef.current.stop()
      }
    }
  }, [renderGraph])

  const edgeData = selectedItem?.type === 'edge' ? selectedItem.data : null
  const isSelfLoopGroup = edgeData && (edgeData as { isSelfLoopGroup?: boolean }).isSelfLoopGroup

  return (
    <div className="graph-panel">
      <div className="panel-header">
        <span className="panel-title">Graph Relationship Visualization</span>
        <div className="header-tools">
          <button type="button" className="tool-btn" onClick={() => onRefresh?.()} disabled={loading} title="Refresh graph">
            <span className={`icon-refresh${loading ? ' spinning' : ''}`}>↻</span>
            <span className="btn-text">Refresh</span>
          </button>
          <button type="button" className="tool-btn" onClick={() => onToggleMaximize?.()} title="Maximize/Restore">
            <span className="icon-maximize">⛶</span>
          </button>
        </div>
      </div>

      <div className="graph-container" ref={graphContainerRef}>
        {graphData ? (
          <div className="graph-view">
            <svg ref={graphSvgRef} className="graph-svg" />

            {(currentPhase === 1 || isSimulating) && (
              <div className="graph-building-hint">
                <div className="memory-icon-wrapper">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="memory-icon">
                    <path d="M9.5 2A2.5 2.5 0 0 1 12 4.5v15a2.5 2.5 0 0 1-4.96.44 2.5 2.5 0 0 1-2.96-3.08 3 3 0 0 1-.34-5.58 2.5 2.5 0 0 1 1.32-4.24 2.5 2.5 0 0 1 4.44-4.04z" />
                    <path d="M14.5 2A2.5 2.5 0 0 0 12 4.5v15a2.5 2.5 0 0 0 4.96.44 2.5 2.5 0 0 0 2.96-3.08 3 3 0 0 0 .34-5.58 2.5 2.5 0 0 0-1.32-4.24 2.5 2.5 0 0 0-4.44-4.04z" />
                  </svg>
                </div>
                {isSimulating
                  ? 'GraphRAG short-term/long-term memory updating in real-time'
                  : 'Updating in real-time...'}
              </div>
            )}

            {showSimulationFinishedHint && (
              <div className="graph-building-hint finished-hint">
                <div className="hint-icon-wrapper">
                  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="hint-icon">
                    <circle cx="12" cy="12" r="10" />
                    <line x1="12" y1="16" x2="12" y2="12" />
                    <line x1="12" y1="8" x2="12.01" y2="8" />
                  </svg>
                </div>
                <span className="hint-text">
                  Some content is still being processed. It is recommended to manually refresh the graph later
                </span>
                <button type="button" className="hint-close-btn" onClick={() => setShowSimulationFinishedHint(false)} title="Close hint">
                  <svg viewBox="0 0 24 24" width="14" height="14" fill="none" stroke="currentColor" strokeWidth="2">
                    <line x1="18" y1="6" x2="6" y2="18" />
                    <line x1="6" y1="6" x2="18" y2="18" />
                  </svg>
                </button>
              </div>
            )}

            {selectedItem && (
              <div className="detail-panel">
                <div className="detail-panel-header">
                  <span className="detail-title">{selectedItem.type === 'node' ? 'Node Details' : 'Relationship'}</span>
                  {selectedItem.type === 'node' && (
                    <span className="detail-type-badge" style={{ background: selectedItem.color, color: '#fff' }}>
                      {selectedItem.entityType}
                    </span>
                  )}
                  <button type="button" className="detail-close" onClick={closeDetailPanel}>
                    ×
                  </button>
                </div>

                {selectedItem.type === 'node' && (
                  <div className="detail-content">
                    <div className="detail-row">
                      <span className="detail-label">Name:</span>
                      <span className="detail-value">{selectedItem.data.name}</span>
                    </div>
                    <div className="detail-row">
                      <span className="detail-label">UUID:</span>
                      <span className="detail-value uuid-text">{selectedItem.data.uuid}</span>
                    </div>
                    {selectedItem.data.created_at && (
                      <div className="detail-row">
                        <span className="detail-label">Created:</span>
                        <span className="detail-value">{formatDateTime(selectedItem.data.created_at)}</span>
                      </div>
                    )}
                    {selectedItem.data.attributes && Object.keys(selectedItem.data.attributes).length > 0 && (
                      <div className="detail-section">
                        <div className="section-title">Properties:</div>
                        <div className="properties-list">
                          {Object.entries(selectedItem.data.attributes).map(([key, value]) => (
                            <div key={key} className="property-item">
                              <span className="property-key">{key}:</span>
                              <span className="property-value">{String(value ?? 'None')}</span>
                            </div>
                          ))}
                        </div>
                      </div>
                    )}
                    {selectedItem.data.summary && (
                      <div className="detail-section">
                        <div className="section-title">Summary:</div>
                        <div className="summary-text">{selectedItem.data.summary}</div>
                      </div>
                    )}
                    {selectedItem.data.labels && selectedItem.data.labels.length > 0 && (
                      <div className="detail-section">
                        <div className="section-title">Labels:</div>
                        <div className="labels-list">
                          {selectedItem.data.labels.map((label) => (
                            <span key={label} className="label-tag">
                              {label}
                            </span>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                )}

                {selectedItem.type === 'edge' && isSelfLoopGroup && (
                  <div className="detail-content">
                    <div className="edge-relation-header self-loop-header">
                      {(edgeData as { source_name?: string }).source_name} - Self Relations
                      <span className="self-loop-count">{(edgeData as { selfLoopCount?: number }).selfLoopCount} items</span>
                    </div>
                    <div className="self-loop-list">
                      {((edgeData as { selfLoopEdges?: GraphEdge[] }).selfLoopEdges || []).map((loop, idx) => {
                        const lid = loop.uuid ?? idx
                        const exp = expandedSelfLoops.has(lid)
                        return (
                          <div key={lid} className={`self-loop-item${exp ? ' expanded' : ''}`}>
                            <div
                              className="self-loop-item-header"
                              onClick={() => toggleSelfLoop(lid)}
                              onKeyDown={(e) => e.key === 'Enter' && toggleSelfLoop(lid)}
                              role="button"
                              tabIndex={0}
                            >
                              <span className="self-loop-index">#{idx + 1}</span>
                              <span className="self-loop-name">{loop.name || loop.fact_type || 'RELATED'}</span>
                              <span className="self-loop-toggle">{exp ? '−' : '+'}</span>
                            </div>
                            {exp && (
                              <div className="self-loop-item-content">
                                {loop.uuid && (
                                  <div className="detail-row">
                                    <span className="detail-label">UUID:</span>
                                    <span className="detail-value uuid-text">{loop.uuid}</span>
                                  </div>
                                )}
                                {loop.fact && (
                                  <div className="detail-row">
                                    <span className="detail-label">Fact:</span>
                                    <span className="detail-value fact-text">{loop.fact}</span>
                                  </div>
                                )}
                                {loop.fact_type && (
                                  <div className="detail-row">
                                    <span className="detail-label">Type:</span>
                                    <span className="detail-value">{loop.fact_type}</span>
                                  </div>
                                )}
                                {loop.created_at && (
                                  <div className="detail-row">
                                    <span className="detail-label">Created:</span>
                                    <span className="detail-value">{formatDateTime(loop.created_at)}</span>
                                  </div>
                                )}
                                {loop.episodes && loop.episodes.length > 0 && (
                                  <div className="self-loop-episodes">
                                    <span className="detail-label">Episodes:</span>
                                    <div className="episodes-list compact">
                                      {loop.episodes.map((ep) => (
                                        <span key={ep} className="episode-tag small">
                                          {ep}
                                        </span>
                                      ))}
                                    </div>
                                  </div>
                                )}
                              </div>
                            )}
                          </div>
                        )
                      })}
                    </div>
                  </div>
                )}

                {selectedItem.type === 'edge' && edgeData && !isSelfLoopGroup && (
                  <div className="detail-content">
                    <div className="edge-relation-header">
                      {(edgeData as { source_name?: string }).source_name} →{' '}
                      {(edgeData as { name?: string }).name || 'RELATED_TO'} →{' '}
                      {(edgeData as { target_name?: string }).target_name}
                    </div>
                    {(edgeData as { uuid?: string }).uuid && (
                      <div className="detail-row">
                        <span className="detail-label">UUID:</span>
                        <span className="detail-value uuid-text">{(edgeData as { uuid?: string }).uuid}</span>
                      </div>
                    )}
                    <div className="detail-row">
                      <span className="detail-label">Label:</span>
                      <span className="detail-value">{(edgeData as { name?: string }).name || 'RELATED_TO'}</span>
                    </div>
                    <div className="detail-row">
                      <span className="detail-label">Type:</span>
                      <span className="detail-value">{(edgeData as { fact_type?: string }).fact_type || 'Unknown'}</span>
                    </div>
                    {(edgeData as { fact?: string }).fact && (
                      <div className="detail-row">
                        <span className="detail-label">Fact:</span>
                        <span className="detail-value fact-text">{(edgeData as { fact?: string }).fact}</span>
                      </div>
                    )}
                    {(edgeData as { episodes?: string[] }).episodes &&
                      (edgeData as { episodes?: string[] }).episodes!.length > 0 && (
                        <div className="detail-section">
                          <div className="section-title">Episodes:</div>
                          <div className="episodes-list">
                            {(edgeData as { episodes?: string[] }).episodes!.map((ep) => (
                              <span key={ep} className="episode-tag">
                                {ep}
                              </span>
                            ))}
                          </div>
                        </div>
                      )}
                    {(edgeData as { created_at?: string }).created_at && (
                      <div className="detail-row">
                        <span className="detail-label">Created:</span>
                        <span className="detail-value">{formatDateTime((edgeData as { created_at?: string }).created_at)}</span>
                      </div>
                    )}
                    {(edgeData as { valid_at?: string }).valid_at && (
                      <div className="detail-row">
                        <span className="detail-label">Valid From:</span>
                        <span className="detail-value">{formatDateTime((edgeData as { valid_at?: string }).valid_at)}</span>
                      </div>
                    )}
                  </div>
                )}
              </div>
            )}
          </div>
        ) : loading ? (
          <div className="graph-state">
            <div className="loading-spinner" />
            <p>Loading graph data...</p>
          </div>
        ) : (
          <div className="graph-state">
            <div className="empty-icon">❖</div>
            <p className="empty-text">Waiting for ontology generation...</p>
          </div>
        )}
      </div>

      {graphData && entityTypes.length > 0 && (
        <div className="graph-legend">
          <span className="legend-title">Entity Types</span>
          <div className="legend-items">
            {entityTypes.map((type) => (
              <div key={type.name} className="legend-item">
                <span className="legend-dot" style={{ background: type.color }} />
                <span className="legend-label">{type.name}</span>
              </div>
            ))}
          </div>
        </div>
      )}

      {graphData && (
        <div className="edge-labels-toggle">
          <label className="toggle-switch">
            <input type="checkbox" checked={showEdgeLabels} onChange={(e) => setShowEdgeLabels(e.target.checked)} />
            <span className="slider" />
          </label>
          <span className="toggle-label">Show Edge Labels</span>
        </div>
      )}
    </div>
  )
}
