import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useLocation, useNavigate } from 'react-router-dom'
import { api } from '../api'
import '../styles/HistoryDatabase.css'

type ProjectRow = {
  simulation_id: string
  project_id?: string
  report_id?: string
  simulation_requirement?: string
  created_at?: string
  current_round?: number
  total_rounds?: number
  files?: { filename: string }[]
}

const CARDS_PER_ROW = 4
const CARD_WIDTH = 280
const CARD_HEIGHT = 280
const CARD_GAP = 24

export default function HistoryDatabase() {
  const navigate = useNavigate()
  const { pathname } = useLocation()
  const [projects, setProjects] = useState<ProjectRow[]>([])
  const [loading, setLoading] = useState(true)
  const [isExpanded, setIsExpanded] = useState(false)
  const [hoveringCard, setHoveringCard] = useState<number | null>(null)
  const historyContainerRef = useRef<HTMLDivElement>(null)
  const [selectedProject, setSelectedProject] = useState<ProjectRow | null>(null)
  const observerRef = useRef<IntersectionObserver | null>(null)
  const isAnimatingRef = useRef(false)
  const expandDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const pendingStateRef = useRef<boolean | null>(null)

  const containerStyle = useMemo(() => {
    if (!isExpanded) return { minHeight: '420px' }
    const total = projects.length
    if (total === 0) return { minHeight: '280px' }
    const rows = Math.ceil(total / CARDS_PER_ROW)
    const expandedHeight = rows * CARD_HEIGHT + (rows - 1) * CARD_GAP + 10
    return { minHeight: `${expandedHeight}px` }
  }, [isExpanded, projects.length])

  const getCardStyle = (index: number) => {
    const total = projects.length
    const transition =
      'transform 700ms cubic-bezier(0.23, 1, 0.32, 1), opacity 700ms cubic-bezier(0.23, 1, 0.32, 1), box-shadow 0.3s ease, border-color 0.3s ease'
    if (isExpanded) {
      const col = index % CARDS_PER_ROW
      const row = Math.floor(index / CARDS_PER_ROW)
      const currentRowStart = row * CARDS_PER_ROW
      const currentRowCards = Math.min(CARDS_PER_ROW, total - currentRowStart)
      const rowWidth = currentRowCards * CARD_WIDTH + (currentRowCards - 1) * CARD_GAP
      const startX = -(rowWidth / 2) + CARD_WIDTH / 2
      const colInRow = index % CARDS_PER_ROW
      const x = startX + colInRow * (CARD_WIDTH + CARD_GAP)
      const y = 20 + row * (CARD_HEIGHT + CARD_GAP)
      return {
        transform: `translate(${x}px, ${y}px) rotate(0deg) scale(1)`,
        zIndex: 100 + index,
        opacity: 1,
        transition
      }
    }
    const centerIndex = (total - 1) / 2
    const offset = index - centerIndex
    const x = offset * 35
    const y = 25 + Math.abs(offset) * 8
    const r = offset * 3
    const s = 0.95 - Math.abs(offset) * 0.05
    return {
      transform: `translate(${x}px, ${y}px) rotate(${r}deg) scale(${s})`,
      zIndex: 10 + index,
      opacity: 1,
      transition
    }
  }

  const getProgressClass = (simulation: ProjectRow) => {
    const current = simulation.current_round || 0
    const total = simulation.total_rounds || 0
    if (total === 0 || current === 0) return 'not-started'
    if (current >= total) return 'completed'
    return 'in-progress'
  }

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return ''
    try {
      return new Date(dateStr).toISOString().slice(0, 10)
    } catch {
      return dateStr.slice(0, 10) || ''
    }
  }

  const formatTime = (dateStr?: string) => {
    if (!dateStr) return ''
    try {
      const date = new Date(dateStr)
      return `${date.getHours().toString().padStart(2, '0')}:${date.getMinutes().toString().padStart(2, '0')}`
    } catch {
      return ''
    }
  }

  const truncateText = (text: string, maxLength: number) => {
    if (!text) return ''
    return text.length > maxLength ? text.slice(0, maxLength) + '...' : text
  }

  const getSimulationTitle = (requirement?: string) => {
    if (!requirement) return 'Unnamed Simulation'
    const title = requirement.slice(0, 20)
    return requirement.length > 20 ? title + '...' : title
  }

  const formatSimulationId = (simulationId: string) => {
    if (!simulationId) return 'SIM_UNKNOWN'
    const prefix = simulationId.replace('sim_', '').slice(0, 6)
    return `SIM_${prefix.toUpperCase()}`
  }

  const formatRounds = (simulation: ProjectRow) => {
    const current = simulation.current_round || 0
    const total = simulation.total_rounds || 0
    if (total === 0) return 'Not Started'
    return `${current}/${total} rounds`
  }

  const getFileType = (filename: string) => {
    const ext = filename.split('.').pop()?.toLowerCase()
    const typeMap: Record<string, string> = {
      pdf: 'pdf',
      doc: 'doc',
      docx: 'doc',
      txt: 'txt',
      md: 'txt',
      json: 'code',
      png: 'img',
      jpg: 'img',
      jpeg: 'img'
    }
    return typeMap[ext || ''] || 'other'
  }

  const getFileTypeLabel = (filename: string) => {
    if (!filename) return 'FILE'
    return filename.split('.').pop()?.toUpperCase() || 'FILE'
  }

  const truncateFilename = (filename: string, maxLength: number) => {
    if (!filename) return 'Unknown File'
    if (filename.length <= maxLength) return filename
    const ext = filename.includes('.') ? '.' + filename.split('.').pop() : ''
    const nameWithoutExt = filename.slice(0, filename.length - ext.length)
    return nameWithoutExt.slice(0, maxLength - ext.length - 3) + '...' + ext
  }

  const loadHistory = useCallback(async () => {
    try {
      setLoading(true)
      const response = await api.simulation.history(20)
      if (response.success) setProjects((response.data as ProjectRow[]) || [])
    } catch {
      setProjects([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadHistory()
  }, [pathname, loadHistory])

  useEffect(() => {
    const el = historyContainerRef.current
    if (!el) return
    if (observerRef.current) observerRef.current.disconnect()
    observerRef.current = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          const shouldExpand = entry.isIntersecting
          pendingStateRef.current = shouldExpand
          if (expandDebounceRef.current) {
            clearTimeout(expandDebounceRef.current)
            expandDebounceRef.current = null
          }
          if (isAnimatingRef.current) return
          if (shouldExpand === isExpanded) {
            pendingStateRef.current = null
            return
          }
          const delay = shouldExpand ? 50 : 200
          expandDebounceRef.current = setTimeout(() => {
            if (isAnimatingRef.current) return
            const pending = pendingStateRef.current
            if (pending === null || pending === isExpanded) return
            isAnimatingRef.current = true
            setIsExpanded(pending)
            pendingStateRef.current = null
            setTimeout(() => {
              isAnimatingRef.current = false
              const p = pendingStateRef.current
              if (p !== null && p !== isExpanded) {
                expandDebounceRef.current = setTimeout(() => {
                  const p2 = pendingStateRef.current
                  if (p2 !== null && p2 !== isExpanded) {
                    isAnimatingRef.current = true
                    setIsExpanded(p2)
                    pendingStateRef.current = null
                    setTimeout(() => {
                      isAnimatingRef.current = false
                    }, 750)
                  }
                }, 100)
              }
            }, 750)
          }, delay)
        })
      },
      { threshold: [0.4, 0.6, 0.8], rootMargin: '0px 0px -150px 0px' }
    )
    observerRef.current.observe(el)
    return () => {
      observerRef.current?.disconnect()
      if (expandDebounceRef.current) clearTimeout(expandDebounceRef.current)
    }
  }, [isExpanded, projects.length])

  const closeModal = () => setSelectedProject(null)

  const modal =
    selectedProject &&
    createPortal(
      <div className="modal-overlay" onClick={closeModal} role="presentation">
        <div className="modal-content" onClick={(e) => e.stopPropagation()} role="dialog">
          <div className="modal-header">
            <div className="modal-title-section">
              <span className="modal-id">{formatSimulationId(selectedProject.simulation_id)}</span>
              <span className={`modal-progress ${getProgressClass(selectedProject)}`}>
                <span className="status-dot">●</span> {formatRounds(selectedProject)}
              </span>
              <span className="modal-create-time">
                {formatDate(selectedProject.created_at)} {formatTime(selectedProject.created_at)}
              </span>
            </div>
            <button type="button" className="modal-close" onClick={closeModal}>
              ×
            </button>
          </div>
          <div className="modal-body">
            <div className="modal-section">
              <div className="modal-label">Simulation Requirement</div>
              <div className="modal-requirement">{selectedProject.simulation_requirement || 'None'}</div>
            </div>
            <div className="modal-section">
              <div className="modal-label">Associated Files</div>
              {selectedProject.files && selectedProject.files.length > 0 ? (
                <div className="modal-files">
                  {selectedProject.files.map((file, index) => (
                    <div key={index} className="modal-file-item">
                      <span className={`file-tag ${getFileType(file.filename)}`}>{getFileTypeLabel(file.filename)}</span>
                      <span className="modal-file-name">{file.filename}</span>
                    </div>
                  ))}
                </div>
              ) : (
                <div className="modal-empty">No Associated Files</div>
              )}
            </div>
          </div>
          <div className="modal-divider">
            <span className="divider-line" />
            <span className="divider-text">Simulation Playback</span>
            <span className="divider-line" />
          </div>
          <div className="modal-actions">
            <button
              type="button"
              className="modal-btn btn-project"
              disabled={!selectedProject.project_id}
              onClick={() => {
                if (selectedProject.project_id) {
                  navigate(`/process/${selectedProject.project_id}`)
                  closeModal()
                }
              }}
            >
              <span className="btn-step">Step1</span>
              <span className="btn-icon">◇</span>
              <span className="btn-text">Graph Construction</span>
            </button>
            <button
              type="button"
              className="modal-btn btn-simulation"
              onClick={() => {
                navigate(`/simulation/${selectedProject.simulation_id}`)
                closeModal()
              }}
            >
              <span className="btn-step">Step2</span>
              <span className="btn-icon">◈</span>
              <span className="btn-text">Environment Setup</span>
            </button>
            <button
              type="button"
              className="modal-btn btn-report"
              disabled={!selectedProject.report_id}
              onClick={() => {
                if (selectedProject.report_id) {
                  navigate(`/report/${selectedProject.report_id}`)
                  closeModal()
                }
              }}
            >
              <span className="btn-step">Step4</span>
              <span className="btn-icon">◆</span>
              <span className="btn-text">Analysis Report</span>
            </button>
          </div>
          <div className="modal-playback-hint">
            <span className="hint-text">
              Step3 &quot;Start Simulation&quot; and Step5 &quot;Deep Interaction&quot; must be launched during execution and do not support
              history playback
            </span>
          </div>
        </div>
      </div>,
      document.body
    )

  return (
    <>
      <div
        ref={historyContainerRef}
        className={`history-database${projects.length === 0 && !loading ? ' no-projects' : ''}`}
      >
        {(projects.length > 0 || loading) && (
          <div className="tech-grid-bg">
            <div className="grid-pattern" />
            <div className="gradient-overlay" />
          </div>
        )}
        <div className="section-header">
          <div className="section-line" />
          <span className="section-title">Simulation Records</span>
          <div className="section-line" />
        </div>
        {projects.length > 0 && (
          <div className={`cards-container${isExpanded ? ' expanded' : ''}`} style={containerStyle}>
            {projects.map((project, index) => (
              <div
                key={project.simulation_id}
                className={`project-card${isExpanded ? ' expanded' : ''}${hoveringCard === index ? ' hovering' : ''}`}
                style={getCardStyle(index)}
                onMouseEnter={() => setHoveringCard(index)}
                onMouseLeave={() => setHoveringCard(null)}
                onClick={() => setSelectedProject(project)}
                role="presentation"
              >
                <div className="card-header">
                  <span className="card-id">{formatSimulationId(project.simulation_id)}</span>
                  <div className="card-status-icons">
                    <span className={`status-icon${project.project_id ? ' available' : ' unavailable'}`} title="Graph Construction">
                      ◇
                    </span>
                    <span className="status-icon available" title="Environment Setup">
                      ◈
                    </span>
                    <span className={`status-icon${project.report_id ? ' available' : ' unavailable'}`} title="Analysis Report">
                      ◆
                    </span>
                  </div>
                </div>
                <div className="card-files-wrapper">
                  <div className="corner-mark top-left-only" />
                  {project.files && project.files.length > 0 ? (
                    <div className="files-list">
                      {project.files.slice(0, 3).map((file, fileIndex) => (
                        <div key={fileIndex} className="file-item">
                          <span className={`file-tag ${getFileType(file.filename)}`}>{getFileTypeLabel(file.filename)}</span>
                          <span className="file-name">{truncateFilename(file.filename, 20)}</span>
                        </div>
                      ))}
                      {project.files.length > 3 && <div className="files-more">+{project.files.length - 3} files</div>}
                    </div>
                  ) : (
                    <div className="files-empty">
                      <span className="empty-file-icon">◇</span>
                      <span className="empty-file-text">No Files</span>
                    </div>
                  )}
                </div>
                <h3 className="card-title">{getSimulationTitle(project.simulation_requirement)}</h3>
                <p className="card-desc">{truncateText(project.simulation_requirement || '', 55)}</p>
                <div className="card-footer">
                  <div className="card-datetime">
                    <span className="card-date">{formatDate(project.created_at)}</span>
                    <span className="card-time">{formatTime(project.created_at)}</span>
                  </div>
                  <span className={`card-progress ${getProgressClass(project)}`}>
                    <span className="status-dot">●</span> {formatRounds(project)}
                  </span>
                </div>
                <div className="card-bottom-line" />
              </div>
            ))}
          </div>
        )}
        {loading && (
          <div className="loading-state">
            <span className="loading-spinner" />
            <span className="loading-text">Loading...</span>
          </div>
        )}
      </div>
      {modal}
    </>
  )
}
