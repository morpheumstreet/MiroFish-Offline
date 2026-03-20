import { useCallback, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import HistoryDatabase from '../components/HistoryDatabase'
import { setPendingUpload } from '../store/pendingUpload'
import heroIcon from '../../public/icon.png'

const steps = [
  { num: '01', title: 'Graph Build', desc: 'Extract reality seeds from your document, build knowledge graph with Neo4j + GraphRAG' },
  { num: '02', title: 'Env Setup', desc: 'Generate agent personas, configure simulation parameters via local Ollama LLM' },
  { num: '03', title: 'Simulation', desc: 'Run multi-agent simulation locally with dynamic memory updates and emergent behavior' },
  { num: '04', title: 'Report', desc: 'ReportAgent analyzes the simulation results and generates a detailed prediction report' },
  { num: '05', title: 'Interaction', desc: 'Chat with any agent from the simulated world or discuss findings with ReportAgent' }
]

export default function HomePage() {
  const navigate = useNavigate()
  const [simulationRequirement, setSimulationRequirement] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const [loading] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const canSubmit = simulationRequirement.trim() !== '' && files.length > 0

  const addFiles = (newFiles: File[]) => {
    const allowed = ['.pdf', '.md', '.txt']
    const valid = newFiles.filter((f) => allowed.some((ext) => f.name.toLowerCase().endsWith(ext)))
    setFiles((prev) => [...prev, ...valid])
  }

  const startSimulation = () => {
    if (!canSubmit || loading) return
    setPendingUpload(files, simulationRequirement)
    navigate('/process/new')
  }

  const scrollToBottom = () => window.scrollTo({ top: document.body.scrollHeight, behavior: 'smooth' })

  const onDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    addFiles(Array.from(e.dataTransfer.files))
  }, [])

  return (
    <div className="home-container">
      <nav className="navbar">
        <div className="nav-brand">MIROFISH OFFLINE</div>
        <div className="nav-links">
          <a href="https://github.com/nikmcfly/MiroFish-Offline" target="_blank" rel="noreferrer" className="github-link">
            Visit our Github <span>↗</span>
          </a>
        </div>
      </nav>

      <div className="main-content">
        <section className="hero-section">
          <div className="hero-left">
            <div className="tag-row">
              <span className="orange-tag">Offline Multi-Agent Simulation Engine</span>
              <span className="version-text">/ v0.1-preview</span>
            </div>
            <h1 className="main-title">
              Upload Any Document
              <br />
              <span className="gradient-text">Predict What Happens Next</span>
            </h1>
            <div className="hero-desc">
              <p>
                From a single document, <span className="highlight-bold">MiroFish Offline</span> extracts reality seeds and builds a
                parallel world of <span className="highlight-orange">autonomous AI agents</span> — running entirely on your machine.
                Inject variables, observe emergent behavior, and find <span className="highlight-code">&quot;local optima&quot;</span> in
                complex social dynamics.
              </p>
              <p className="slogan-text">
                Your data never leaves your machine. The future is simulated locally<span className="blinking-cursor">_</span>
              </p>
            </div>
            <div className="decoration-square" />
          </div>
          <div className="hero-right">
            <div className="logo-container">
              <img src={heroIcon} alt="MiroFish" className="hero-logo" />
            </div>
            <button type="button" className="scroll-down-btn" onClick={scrollToBottom}>
              ↓
            </button>
          </div>
        </section>

        <section className="dashboard-section">
          <div className="left-panel">
            <div className="panel-header">
              <span className="status-dot">■</span> System Status
            </div>
            <h2 className="section-title">Ready</h2>
            <p className="section-desc">Local prediction engine on standby. Upload unstructured data to initialize a simulation.</p>
            <div className="metrics-row">
              <div className="metric-card">
                <div className="metric-value">Free</div>
                <div className="metric-label">Runs on your hardware</div>
              </div>
              <div className="metric-card">
                <div className="metric-value">Private</div>
                <div className="metric-label">100% offline, no cloud</div>
              </div>
            </div>
            <div className="steps-container">
              <div className="steps-header">
                <span className="diamond-icon">◇</span> Workflow Sequence
              </div>
              <div className="workflow-list">
                {steps.map((step) => (
                  <div key={step.num} className="workflow-item">
                    <span className="step-num">{step.num}</span>
                    <div className="step-info">
                      <div className="step-title">{step.title}</div>
                      <div className="step-desc">{step.desc}</div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div className="right-panel">
            <div className="console-box">
              <div className="console-section">
                <div className="console-header">
                  <span>01 / Reality Seeds</span>
                  <span>Supported: PDF, MD, TXT</span>
                </div>
                <div
                  className={`upload-zone${files.length ? ' has-files' : ''}`}
                  onDragOver={(e) => e.preventDefault()}
                  onDrop={onDrop}
                  onClick={() => !loading && fileInputRef.current?.click()}
                  role="presentation"
                >
                  <input
                    ref={fileInputRef}
                    type="file"
                    multiple
                    accept=".pdf,.md,.txt"
                    style={{ display: 'none' }}
                    disabled={loading}
                    onChange={(e) => addFiles(Array.from(e.target.files || []))}
                  />
                  {files.length === 0 ? (
                    <div className="upload-placeholder">
                      <div className="upload-icon">↑</div>
                      <div className="upload-title">Drag & drop files here</div>
                      <div className="upload-hint">or click to browse</div>
                    </div>
                  ) : (
                    <div className="file-list">
                      {files.map((file, index) => (
                        <div key={`${file.name}-${index}`} className="file-item">
                          <span>📄</span>
                          <span className="file-name">{file.name}</span>
                          <button type="button" className="remove-btn" onClick={(e) => { e.stopPropagation(); setFiles((f) => f.filter((_, i) => i !== index)) }}>
                            ×
                          </button>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </div>

              <div className="console-divider">
                <span>Parameters</span>
              </div>

              <div className="console-section">
                <div className="console-header">
                  <span>&gt;_ 02 / Simulation Prompt</span>
                </div>
                <div className="input-wrapper">
                  <textarea
                    className="code-input"
                    placeholder="// Describe your simulation or prediction goal in natural language"
                    rows={6}
                    disabled={loading}
                    value={simulationRequirement}
                    onChange={(e) => setSimulationRequirement(e.target.value)}
                  />
                  <div className="model-badge">Engine: Ollama + Neo4j (local)</div>
                </div>
              </div>

              <div className="btn-section">
                <button type="button" className="start-engine-btn" disabled={!canSubmit || loading} onClick={startSimulation}>
                  <span>{loading ? 'Initializing...' : 'Start Engine'}</span>
                  <span>→</span>
                </button>
              </div>
            </div>
          </div>
        </section>

        <HistoryDatabase />
      </div>
    </div>
  )
}
