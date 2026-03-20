import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import type { GraphData } from './GraphPanel'
import '../styles/Step1GraphBuild.css'

type OntologyItem = {
  name: string
  description?: string
  itemType: 'entity' | 'relation'
  attributes?: { name: string; type: string; description?: string }[]
  examples?: string[]
  source_targets?: { source: string; target: string }[]
}

type ProjectData = {
  project_id?: string
  graph_id?: string
  ontology?: {
    entity_types?: OntologyItem[]
    edge_types?: OntologyItem[]
  }
}

type LogLine = { time: string; msg: string }

type Props = {
  currentPhase: number
  projectData: ProjectData | null
  ontologyProgress: { message?: string } | null
  buildProgress: { progress?: number; message?: string } | null
  graphData: GraphData | null
  systemLogs: LogLine[]
}

export default function Step1GraphBuild({
  currentPhase,
  projectData,
  ontologyProgress,
  buildProgress,
  graphData,
  systemLogs
}: Props) {
  const navigate = useNavigate()
  const [selectedOntologyItem, setSelectedOntologyItem] = useState<OntologyItem | null>(null)
  const [creatingSimulation, setCreatingSimulation] = useState(false)
  const logContentRef = useRef<HTMLDivElement>(null)

  const graphStats = useMemo(() => {
    const nodes = graphData?.node_count ?? graphData?.nodes?.length ?? 0
    const edges = graphData?.edge_count ?? graphData?.edges?.length ?? 0
    const types = projectData?.ontology?.entity_types?.length ?? 0
    return { nodes, edges, types }
  }, [graphData, projectData])

  useEffect(() => {
    const el = logContentRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [systemLogs.length])

  const handleEnterEnvSetup = async () => {
    if (!projectData?.project_id || !projectData?.graph_id) {
      console.error('Missing project or graph information')
      return
    }
    setCreatingSimulation(true)
    try {
      const res = await api.simulation.create({
        project_id: projectData.project_id,
        graph_id: projectData.graph_id,
        enable_twitter: true,
        enable_reddit: true
      })
      if (res.success && res.data?.simulation_id) {
        navigate(`/simulation/${res.data.simulation_id}`)
      } else {
        console.error('Failed to create simulation:', res.error)
        alert('Failed to create simulation: ' + (res.error || 'Unknown error'))
      }
    } catch (err: unknown) {
      const m = err instanceof Error ? err.message : String(err)
      console.error('Simulation creation exception:', err)
      alert('Simulation creation exception: ' + m)
    } finally {
      setCreatingSimulation(false)
    }
  }

  const selectOntologyItem = (item: OntologyItem, itemType: 'entity' | 'relation') => {
    setSelectedOntologyItem({ ...item, itemType })
  }

  return (
    <div className="workbench-panel">
      <div className="scroll-container">
        <div className={`step-card${currentPhase === 0 ? ' active' : ''}${currentPhase > 0 ? ' completed' : ''}`}>
          <div className="card-header">
            <div className="step-info">
              <span className="step-num">01</span>
              <span className="step-title">Ontology Generation</span>
            </div>
            <div className="step-status">
              {currentPhase > 0 && <span className="badge success">Completed</span>}
              {currentPhase === 0 && <span className="badge processing">Generating</span>}
              {currentPhase < 0 && <span className="badge pending">Waiting</span>}
            </div>
          </div>
          <div className="card-content">
            <p className="api-note">POST /api/graph/ontology/generate</p>
            <p className="description">
              LLM analyzes document content and simulation requirements, extracts reality seeds, and automatically generates
              appropriate ontology structures
            </p>
            {currentPhase === 0 && ontologyProgress && (
              <div className="progress-section">
                <div className="spinner-sm" />
                <span>{ontologyProgress.message || 'Analyzing documents...'}</span>
              </div>
            )}
            {selectedOntologyItem && (
              <div className="ontology-detail-overlay">
                <div className="detail-header">
                  <div className="detail-title-group">
                    <span className="detail-type-badge">{selectedOntologyItem.itemType === 'entity' ? 'ENTITY' : 'RELATION'}</span>
                    <span className="detail-name">{selectedOntologyItem.name}</span>
                  </div>
                  <button type="button" className="close-btn" onClick={() => setSelectedOntologyItem(null)}>
                    ×
                  </button>
                </div>
                <div className="detail-body">
                  <div className="detail-desc">{selectedOntologyItem.description}</div>
                  {selectedOntologyItem.attributes && selectedOntologyItem.attributes.length > 0 && (
                    <div className="detail-section">
                      <span className="section-label">ATTRIBUTES</span>
                      <div className="attr-list">
                        {selectedOntologyItem.attributes.map((attr) => (
                          <div key={attr.name} className="attr-item">
                            <span className="attr-name">{attr.name}</span>
                            <span className="attr-type">({attr.type})</span>
                            <span className="attr-desc">{attr.description}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                  {selectedOntologyItem.examples && selectedOntologyItem.examples.length > 0 && (
                    <div className="detail-section">
                      <span className="section-label">EXAMPLES</span>
                      <div className="example-list">
                        {selectedOntologyItem.examples.map((ex) => (
                          <span key={ex} className="example-tag">
                            {ex}
                          </span>
                        ))}
                      </div>
                    </div>
                  )}
                  {selectedOntologyItem.source_targets && selectedOntologyItem.source_targets.length > 0 && (
                    <div className="detail-section">
                      <span className="section-label">CONNECTIONS</span>
                      <div className="conn-list">
                        {selectedOntologyItem.source_targets.map((conn, idx) => (
                          <div key={idx} className="conn-item">
                            <span className="conn-node">{conn.source}</span>
                            <span className="conn-arrow">→</span>
                            <span className="conn-node">{conn.target}</span>
                          </div>
                        ))}
                      </div>
                    </div>
                  )}
                </div>
              </div>
            )}
            {projectData?.ontology?.entity_types && (
              <div className={`tags-container${selectedOntologyItem ? ' dimmed' : ''}`}>
                <span className="tag-label">GENERATED ENTITY TYPES</span>
                <div className="tags-list">
                  {projectData.ontology.entity_types.map((entity) => (
                    <span
                      key={entity.name}
                      className="entity-tag clickable"
                      onClick={() => selectOntologyItem(entity as OntologyItem, 'entity')}
                      onKeyDown={(e) => e.key === 'Enter' && selectOntologyItem(entity as OntologyItem, 'entity')}
                      role="button"
                      tabIndex={0}
                    >
                      {entity.name}
                    </span>
                  ))}
                </div>
              </div>
            )}
            {projectData?.ontology?.edge_types && (
              <div className={`tags-container${selectedOntologyItem ? ' dimmed' : ''}`}>
                <span className="tag-label">GENERATED RELATION TYPES</span>
                <div className="tags-list">
                  {projectData.ontology.edge_types.map((rel) => (
                    <span
                      key={rel.name}
                      className="entity-tag clickable"
                      onClick={() => selectOntologyItem(rel as OntologyItem, 'relation')}
                      onKeyDown={(e) => e.key === 'Enter' && selectOntologyItem(rel as OntologyItem, 'relation')}
                      role="button"
                      tabIndex={0}
                    >
                      {rel.name}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        <div className={`step-card${currentPhase === 1 ? ' active' : ''}${currentPhase > 1 ? ' completed' : ''}`}>
          <div className="card-header">
            <div className="step-info">
              <span className="step-num">02</span>
              <span className="step-title">GraphRAG Build</span>
            </div>
            <div className="step-status">
              {currentPhase > 1 && <span className="badge success">Completed</span>}
              {currentPhase === 1 && <span className="badge processing">{buildProgress?.progress ?? 0}%</span>}
              {currentPhase < 1 && <span className="badge pending">Waiting</span>}
            </div>
          </div>
          <div className="card-content">
            <p className="api-note">POST /api/graph/build</p>
            <p className="description">
              Based on the generated ontology, automatically chunk documents and invoke Neo4j to build knowledge graphs, extract
              entities and relationships, and form temporal memory and community summaries
            </p>
            <div className="stats-grid">
              <div className="stat-card">
                <span className="stat-value">{graphStats.nodes}</span>
                <span className="stat-label">Entity Nodes</span>
              </div>
              <div className="stat-card">
                <span className="stat-value">{graphStats.edges}</span>
                <span className="stat-label">Relation Edges</span>
              </div>
              <div className="stat-card">
                <span className="stat-value">{graphStats.types}</span>
                <span className="stat-label">SCHEMA Types</span>
              </div>
            </div>
          </div>
        </div>

        <div className={`step-card${currentPhase === 2 ? ' active' : ''}${currentPhase >= 2 ? ' completed' : ''}`}>
          <div className="card-header">
            <div className="step-info">
              <span className="step-num">03</span>
              <span className="step-title">Build Complete</span>
            </div>
            <div className="step-status">
              {currentPhase >= 2 && <span className="badge accent">In Progress</span>}
            </div>
          </div>
          <div className="card-content">
            <p className="api-note">POST /api/simulation/create</p>
            <p className="description">Graph build is complete. Please proceed to the next step to set up the simulation environment</p>
            <button
              type="button"
              className="action-btn"
              disabled={currentPhase < 2 || creatingSimulation}
              onClick={handleEnterEnvSetup}
            >
              {creatingSimulation && <span className="spinner-sm" />}
              {creatingSimulation ? 'Creating...' : 'Enter Environment Setup ➝'}
            </button>
          </div>
        </div>
      </div>

      <div className="system-logs">
        <div className="log-header">
          <span className="log-title">SYSTEM DASHBOARD</span>
          <span className="log-id">{projectData?.project_id || 'NO_PROJECT'}</span>
        </div>
        <div className="log-content" ref={logContentRef}>
          {systemLogs.map((log, idx) => (
            <div key={idx} className="log-line">
              <span className="log-time">{log.time}</span>
              <span className="log-msg">{log.msg}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
