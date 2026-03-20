import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import type { GraphData } from './GraphPanel'
import '../styles/Step3Simulation.css'

type LogLine = { time: string; msg: string }

type ActionRow = Record<string, unknown> & { _uniqueId?: string }

type Props = {
  simulationId?: string
  maxRounds?: number | null
  minutesPerRound?: number
  projectData: unknown
  graphData: GraphData | null
  systemLogs: LogLine[]
  onGoBack?: () => void
  onNextStep: () => void
  onAddLog: (msg: string) => void
  onUpdateStatus: (s: 'processing' | 'completed' | 'error') => void
}

export default function Step3Simulation({
  simulationId,
  maxRounds,
  minutesPerRound = 30,
  systemLogs,
  onAddLog,
  onUpdateStatus,
  onGoBack
}: Props) {
  const navigate = useNavigate()
  const [isGeneratingReport, setIsGeneratingReport] = useState(false)
  const [phase, setPhase] = useState(0)
  const [isStarting, setIsStarting] = useState(false)
  const [isStopping, setIsStopping] = useState(false)
  const [startError, setStartError] = useState<string | null>(null)
  const [runStatus, setRunStatus] = useState<Record<string, unknown>>({})
  const [allActions, setAllActions] = useState<ActionRow[]>([])
  const actionIdsRef = useRef<Set<string>>(new Set())
  const prevTwitterRound = useRef(0)
  const prevRedditRound = useRef(0)
  const statusTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const detailTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const logContentRef = useRef<HTMLDivElement>(null)

  const twitterActionsCount = useMemo(() => allActions.filter((a) => a.platform === 'twitter').length, [allActions])
  const redditActionsCount = useMemo(() => allActions.filter((a) => a.platform === 'reddit').length, [allActions])

  const formatElapsedTime = (currentRound: number) => {
    if (!currentRound || currentRound <= 0) return '0h 0m'
    const totalMinutes = currentRound * minutesPerRound
    const hours = Math.floor(totalMinutes / 60)
    const minutes = totalMinutes % 60
    return `${hours}h ${minutes}m`
  }

  const twitterElapsedTime = formatElapsedTime(Number(runStatus.twitter_current_round || 0))
  const redditElapsedTime = formatElapsedTime(Number(runStatus.reddit_current_round || 0))

  useEffect(() => {
    const el = logContentRef.current
    if (el) el.scrollTop = el.scrollHeight
  }, [systemLogs.length])

  const stopPolling = useCallback(() => {
    if (statusTimerRef.current) {
      clearInterval(statusTimerRef.current)
      statusTimerRef.current = null
    }
    if (detailTimerRef.current) {
      clearInterval(detailTimerRef.current)
      detailTimerRef.current = null
    }
  }, [])

  const checkPlatformsCompleted = (data: Record<string, unknown>) => {
    if (!data) return false
    const twitterCompleted = data.twitter_completed === true
    const redditCompleted = data.reddit_completed === true
    const twitterEnabled =
      Number(data.twitter_actions_count) > 0 || data.twitter_running === true || twitterCompleted
    const redditEnabled =
      Number(data.reddit_actions_count) > 0 || data.reddit_running === true || redditCompleted
    if (!twitterEnabled && !redditEnabled) return false
    if (twitterEnabled && !twitterCompleted) return false
    if (redditEnabled && !redditCompleted) return false
    return true
  }

  const fetchRunStatusDetail = useCallback(async () => {
    if (!simulationId) return
    try {
      const res = await api.simulation.getRunStatusDetail(simulationId)
      if (res.success && res.data) {
        const serverActions = (res.data.all_actions as ActionRow[]) || []
        setAllActions((prev) => {
          const next = [...prev]
          for (const action of serverActions) {
            const actionId =
              (action.id as string) ||
              `${action.timestamp}-${action.platform}-${action.agent_id}-${action.action_type}`
            if (!actionIdsRef.current.has(actionId)) {
              actionIdsRef.current.add(actionId)
              next.push({ ...action, _uniqueId: actionId })
            }
          }
          return next
        })
      }
    } catch {
      /* ignore */
    }
  }, [simulationId])

  const fetchRunStatus = useCallback(async () => {
    if (!simulationId) return
    try {
      const res = await api.simulation.getRunStatus(simulationId)
      if (res.success && res.data) {
        const data = res.data as Record<string, unknown>
        setRunStatus(data)

        const tw = Number(data.twitter_current_round || 0)
        if (tw > prevTwitterRound.current) {
          onAddLog(
            `[Info Plaza] R${tw}/${data.total_rounds} | T:${data.twitter_simulated_hours || 0}h | A:${data.twitter_actions_count}`
          )
          prevTwitterRound.current = tw
        }
        const rd = Number(data.reddit_current_round || 0)
        if (rd > prevRedditRound.current) {
          onAddLog(
            `[Topic Community] R${rd}/${data.total_rounds} | T:${data.reddit_simulated_hours || 0}h | A:${data.reddit_actions_count}`
          )
          prevRedditRound.current = rd
        }

        const isCompleted = data.runner_status === 'completed' || data.runner_status === 'stopped'
        const platformsCompleted = checkPlatformsCompleted(data)
        if (isCompleted || platformsCompleted) {
          if (platformsCompleted && !isCompleted) onAddLog('✓ Detected all platform simulations have ended')
          onAddLog('✓ Simulation completed')
          setPhase(2)
          stopPolling()
          onUpdateStatus('completed')
        }
      }
    } catch {
      /* ignore */
    }
  }, [simulationId, onAddLog, onUpdateStatus, stopPolling])

  const startStatusPolling = useCallback(() => {
    statusTimerRef.current = setInterval(() => void fetchRunStatus(), 2000)
  }, [fetchRunStatus])

  const startDetailPolling = useCallback(() => {
    detailTimerRef.current = setInterval(() => void fetchRunStatusDetail(), 3000)
  }, [fetchRunStatusDetail])

  const resetAllState = useCallback(() => {
    setPhase(0)
    setRunStatus({})
    setAllActions([])
    actionIdsRef.current = new Set()
    prevTwitterRound.current = 0
    prevRedditRound.current = 0
    setStartError(null)
    setIsStarting(false)
    setIsStopping(false)
    stopPolling()
  }, [stopPolling])

  const doStartSimulation = useCallback(async () => {
    if (!simulationId) {
      onAddLog('Error: Missing simulationId')
      return
    }
    resetAllState()
    setIsStarting(true)
    setStartError(null)
    onAddLog('Starting dual-platform parallel simulation...')
    onUpdateStatus('processing')
    try {
      const params: Record<string, unknown> = {
        simulation_id: simulationId,
        platform: 'parallel',
        force: true,
        enable_graph_memory_update: true
      }
      if (maxRounds) {
        params.max_rounds = maxRounds
        onAddLog(`Set max simulation rounds: ${maxRounds}`)
      }
      onAddLog('Dynamic graph update mode enabled')
      const res = await api.simulation.start(params)
      if (res.success && res.data) {
        if (res.data.force_restarted) onAddLog('✓ Cleaned old simulation logs and restarted simulation')
        onAddLog('✓ Simulation engine started successfully')
        onAddLog(`  ├─ PID: ${res.data.process_pid || '-'}`)
        setPhase(1)
        setRunStatus(res.data as Record<string, unknown>)
        startStatusPolling()
        startDetailPolling()
      } else {
        setStartError((res.error as string) || 'Start failed')
        onAddLog(`✗ Start failed: ${res.error || 'Unknown error'}`)
        onUpdateStatus('error')
      }
    } catch (err: unknown) {
      const m = err instanceof Error ? err.message : String(err)
      setStartError(m)
      onAddLog(`✗ Start exception: ${m}`)
      onUpdateStatus('error')
    } finally {
      setIsStarting(false)
    }
  }, [simulationId, maxRounds, onAddLog, onUpdateStatus, resetAllState, startStatusPolling, startDetailPolling])

  const handleStopSimulation = async () => {
    if (!simulationId) return
    setIsStopping(true)
    onAddLog('Stopping simulation...')
    try {
      const res = await api.simulation.stop({ simulation_id: simulationId })
      if (res.success) {
        onAddLog('✓ Simulation stopped')
        setPhase(2)
        stopPolling()
        onUpdateStatus('completed')
      } else {
        onAddLog(`Stop failed: ${res.error || 'Unknown error'}`)
      }
    } catch (err: unknown) {
      const m = err instanceof Error ? err.message : String(err)
      onAddLog(`Stop exception: ${m}`)
    } finally {
      setIsStopping(false)
    }
  }

  const handleNextStep = async () => {
    if (!simulationId) {
      onAddLog('Error: Missing simulationId')
      return
    }
    if (isGeneratingReport) {
      onAddLog('Report generation request sent, please wait...')
      return
    }
    setIsGeneratingReport(true)
    onAddLog('Starting report generation...')
    try {
      const res = await api.report.generate({ simulation_id: simulationId, force_regenerate: true })
      if (res.success && res.data) {
        const reportId = (res.data as { report_id: string }).report_id
        onAddLog(`✓ Report generation task started: ${reportId}`)
        navigate(`/report/${reportId}`)
      } else {
        onAddLog(`✗ Failed to start report generation: ${res.error || 'Unknown error'}`)
        setIsGeneratingReport(false)
      }
    } catch (err: unknown) {
      const m = err instanceof Error ? err.message : String(err)
      onAddLog(`✗ Report generation exception: ${m}`)
      setIsGeneratingReport(false)
    }
  }

  const doStartRef = useRef(doStartSimulation)
  doStartRef.current = doStartSimulation
  useEffect(() => {
    if (!simulationId) return
    onAddLog('Step3 Simulation initialization')
    void doStartRef.current()
    return () => stopPolling()
  }, [simulationId, onAddLog, stopPolling])

  const getActionTypeLabel = (type: string) => {
    const labels: Record<string, string> = {
      CREATE_POST: 'POST',
      REPOST: 'REPOST',
      LIKE_POST: 'LIKE',
      CREATE_COMMENT: 'COMMENT',
      LIKE_COMMENT: 'LIKE',
      DO_NOTHING: 'IDLE',
      FOLLOW: 'FOLLOW',
      SEARCH_POSTS: 'SEARCH',
      QUOTE_POST: 'QUOTE',
      UPVOTE_POST: 'UPVOTE',
      DOWNVOTE_POST: 'DOWNVOTE'
    }
    return labels[type] || type || 'UNKNOWN'
  }

  return (
    <div className="simulation-panel">
      <div className="control-bar">
        <div className="status-group">
          <div
            className={`platform-status twitter${runStatus.twitter_running ? ' active' : ''}${runStatus.twitter_completed ? ' completed' : ''}`}
          >
            <div className="platform-header">
              <span className="platform-name">Info Plaza</span>
            </div>
            <div className="platform-stats">
              <span className="stat">
                <span className="stat-label">ROUND</span>
                <span className="stat-value mono">
                  {Number(runStatus.twitter_current_round || 0)}
                  <span className="stat-total">/{String(runStatus.total_rounds || maxRounds || '-')}</span>
                </span>
              </span>
              <span className="stat">
                <span className="stat-label">Elapsed</span>
                <span className="stat-value mono">{twitterElapsedTime}</span>
              </span>
              <span className="stat">
                <span className="stat-label">ACTS</span>
                <span className="stat-value mono">{Number(runStatus.twitter_actions_count || 0)}</span>
              </span>
            </div>
          </div>
          <div
            className={`platform-status reddit${runStatus.reddit_running ? ' active' : ''}${runStatus.reddit_completed ? ' completed' : ''}`}
          >
            <div className="platform-header">
              <span className="platform-name">Topic Community</span>
            </div>
            <div className="platform-stats">
              <span className="stat">
                <span className="stat-label">ROUND</span>
                <span className="stat-value mono">
                  {Number(runStatus.reddit_current_round || 0)}
                  <span className="stat-total">/{String(runStatus.total_rounds || maxRounds || '-')}</span>
                </span>
              </span>
              <span className="stat">
                <span className="stat-label">Elapsed</span>
                <span className="stat-value mono">{redditElapsedTime}</span>
              </span>
              <span className="stat">
                <span className="stat-label">ACTS</span>
                <span className="stat-value mono">{Number(runStatus.reddit_actions_count || 0)}</span>
              </span>
            </div>
          </div>
        </div>
        <div className="action-controls">
          {onGoBack && (
            <button type="button" className="action-btn secondary" onClick={onGoBack}>
              ← Back
            </button>
          )}
          <button type="button" className="action-btn secondary" disabled={isStopping} onClick={handleStopSimulation}>
            {isStopping ? 'Stopping...' : 'Stop'}
          </button>
          <button type="button" className="action-btn primary" disabled={phase !== 2 || isGeneratingReport} onClick={handleNextStep}>
            {isGeneratingReport ? 'Starting...' : 'Start Generating Report'} →
          </button>
        </div>
      </div>

      {startError && <div className="error-banner">{startError}</div>}
      {isStarting && <div className="hint-banner">Starting engine…</div>}

      <div className="main-content-area">
        {allActions.length > 0 && (
          <div className="timeline-header">
            <div className="timeline-stats">
              <span className="total-count">
                TOTAL EVENTS: <span className="mono">{allActions.length}</span>
              </span>
              <span className="platform-breakdown">
                <span className="breakdown-item twitter">{twitterActionsCount}</span>
                <span className="breakdown-divider">/</span>
                <span className="breakdown-item reddit">{redditActionsCount}</span>
              </span>
            </div>
          </div>
        )}
        <div className="timeline-feed">
          {allActions.map((action) => (
            <div
              key={action._uniqueId || String(action.id)}
              className={`timeline-item ${String(action.platform || '')}`}
            >
              <div className="timeline-card">
                <div className="card-header">
                  <span className="agent-name">{String(action.agent_name || 'Agent')}</span>
                  <span className={`action-badge ${String(action.action_type || '')}`}>
                    {getActionTypeLabel(String(action.action_type || ''))}
                  </span>
                </div>
                <p className="card-content-text">
                  {String(action.content || action.summary || '').slice(0, 200)}
                  {String(action.content || '').length > 200 ? '…' : ''}
                </p>
              </div>
            </div>
          ))}
        </div>
      </div>

      <div className="system-logs">
        <div className="log-header">
          <span className="log-title">SYSTEM DASHBOARD</span>
          <span className="log-id">{simulationId || 'NO_SIMULATION'}</span>
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
