import { useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import '../styles/Step4Report.css'

type LogLine = { time: string; msg: string }

type AgentLog = Record<string, unknown> & { action?: string; timestamp?: string }

type Props = {
  reportId?: string
  simulationId?: string | null
  systemLogs: LogLine[]
  onAddLog: (msg: string) => void
  onUpdateStatus: (s: 'processing' | 'completed' | 'error') => void
}

export default function Step4Report({ reportId, onAddLog, onUpdateStatus }: Props) {
  const navigate = useNavigate()
  const [agentLogs, setAgentLogs] = useState<AgentLog[]>([])
  const [consoleLogs, setConsoleLogs] = useState<Record<string, unknown>[]>([])
  const agentLogLineRef = useRef(0)
  const consoleLogLineRef = useRef(0)
  const [isComplete, setIsComplete] = useState(false)
  const [outline, setOutline] = useState<unknown>(null)
  const [sections, setSections] = useState<Record<number, string>>({})
  const agentTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const consoleTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPolling = useCallback(() => {
    if (agentTimerRef.current) {
      clearInterval(agentTimerRef.current)
      agentTimerRef.current = null
    }
    if (consoleTimerRef.current) {
      clearInterval(consoleTimerRef.current)
      consoleTimerRef.current = null
    }
  }, [])

  const fetchAgentLog = useCallback(async () => {
    if (!reportId) return
    try {
      const res = await api.report.getAgentLog(reportId, agentLogLineRef.current)
      if (res.success && res.data) {
        const newLogs = (res.data.logs as AgentLog[]) || []
        if (newLogs.length > 0) {
          setAgentLogs((prev) => [...prev, ...newLogs])
          for (const log of newLogs) {
            if (log.action === 'planning_complete' && (log.details as { outline?: unknown })?.outline) {
              setOutline((log.details as { outline: unknown }).outline)
            }
            if (log.action === 'section_complete' && (log.details as { content?: string })?.content != null) {
              const si = log.section_index as number
              setSections((s) => ({ ...s, [si]: (log.details as { content: string }).content }))
            }
            if (log.action === 'report_complete') {
              setIsComplete(true)
              onUpdateStatus('completed')
              stopPolling()
            }
          }
          agentLogLineRef.current = (res.data as { from_line: number }).from_line + newLogs.length
        }
      }
    } catch {
      /* ignore */
    }
  }, [reportId, onUpdateStatus, stopPolling])

  const fetchConsoleLog = useCallback(async () => {
    if (!reportId) return
    try {
      const res = await api.report.getConsoleLog(reportId, consoleLogLineRef.current)
      if (res.success && res.data) {
        const newLogs = (res.data.logs as Record<string, unknown>[]) || []
        if (newLogs.length > 0) {
          setConsoleLogs((prev) => [...prev, ...newLogs])
          consoleLogLineRef.current = (res.data as { from_line: number }).from_line + newLogs.length
        }
      }
    } catch {
      /* ignore */
    }
  }, [reportId])

  const startPolling = useCallback(() => {
    if (agentTimerRef.current || consoleTimerRef.current) return
    void fetchAgentLog()
    void fetchConsoleLog()
    agentTimerRef.current = setInterval(() => void fetchAgentLog(), 2000)
    consoleTimerRef.current = setInterval(() => void fetchConsoleLog(), 1500)
  }, [fetchAgentLog, fetchConsoleLog])

  useEffect(() => {
    if (!reportId) return
    setAgentLogs([])
    setConsoleLogs([])
    agentLogLineRef.current = 0
    consoleLogLineRef.current = 0
    setIsComplete(false)
    setOutline(null)
    setSections({})
    stopPolling()
    onAddLog(`Report Agent initialized: ${reportId}`)
    startPolling()
    return () => stopPolling()
  }, [reportId, onAddLog, startPolling, stopPolling])

  return (
    <div className="report-panel">
      <div className="main-split-layout">
        <div className="left-panel report-style">
          <div className="panel-header">
            <span className="header-dot" />
            Report outline
          </div>
          <div className="panel-content">
            {outline != null && <pre className="outline-pre">{JSON.stringify(outline, null, 2)}</pre>}
            {Object.keys(sections).length > 0 && (
              <div className="sections-block">
                {Object.entries(sections).map(([k, v]) => (
                  <div key={k} className="section-chunk">
                    <h4>Section {k}</h4>
                    <pre className="section-pre">{v}</pre>
                  </div>
                ))}
              </div>
            )}
            {!outline && Object.keys(sections).length === 0 && <p className="muted">Waiting for planner output…</p>}
          </div>
        </div>
        <div className="right-panel">
          <div className="panel-header">
            <span className="header-dot" />
            Agent trace ({agentLogs.length})
          </div>
          <div className="panel-content agent-trace">
            {agentLogs.map((log, i) => (
              <div key={i} className="trace-line">
                <span className="trace-action">{String(log.action)}</span>
                <span className="trace-time">{String(log.timestamp || '')}</span>
                <pre className="trace-json">{JSON.stringify(log, null, 2)}</pre>
              </div>
            ))}
          </div>
        </div>
      </div>
      <div className="console-strip">
        <span className="console-label">Console</span>
        <div className="console-lines">
          {consoleLogs.map((l, i) => (
            <div key={i} className="console-line">
              {JSON.stringify(l)}
            </div>
          ))}
        </div>
      </div>
      <div className="report-actions">
        <button
          type="button"
          className="action-btn primary"
          disabled={!isComplete}
          onClick={() => reportId && navigate(`/interaction/${reportId}`)}
        >
          Continue to Interaction →
        </button>
      </div>
    </div>
  )
}
