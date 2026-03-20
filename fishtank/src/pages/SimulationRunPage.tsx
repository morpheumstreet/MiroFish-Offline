import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../api'
import GraphPanel, { type GraphData } from '../components/GraphPanel'
import Step3Simulation from '../components/Step3Simulation'
import WorkflowLayout, { panelStyles } from '../components/WorkflowLayout'

type LogLine = { time: string; msg: string }

export default function SimulationRunPage() {
  const { simulationId } = useParams<{ simulationId: string }>()
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const maxRoundsParam = searchParams.get('maxRounds')
  const maxRounds = maxRoundsParam ? parseInt(maxRoundsParam, 10) : null

  const [viewMode, setViewMode] = useState<'graph' | 'split' | 'workbench'>('split')
  const [projectData, setProjectData] = useState<Record<string, unknown> | null>(null)
  const [graphData, setGraphData] = useState<GraphData | null>(null)
  const [graphLoading, setGraphLoading] = useState(false)
  const [systemLogs, setSystemLogs] = useState<LogLine[]>([])
  const [currentStatus, setCurrentStatus] = useState<'processing' | 'completed' | 'error'>('processing')
  const [minutesPerRound, setMinutesPerRound] = useState(30)

  const graphTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const layout = panelStyles(viewMode)
  const isSimulating = currentStatus === 'processing'

  const addLog = useCallback((msg: string) => {
    const time =
      new Date().toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }) +
      '.' +
      new Date().getMilliseconds().toString().padStart(3, '0')
    setSystemLogs((prev) => {
      const next = [...prev, { time, msg }]
      return next.length > 200 ? next.slice(-200) : next
    })
  }, [])

  const loadGraph = async (graphId: string) => {
    if (!isSimulating) setGraphLoading(true)
    try {
      const res = await api.graph.getGraphData(graphId)
      if (res.success) {
        setGraphData(res.data as GraphData)
        if (!isSimulating) addLog('Graph data loaded successfully')
      }
    } catch (e: unknown) {
      addLog(`Graph load failed: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setGraphLoading(false)
    }
  }

  const refreshGraph = () => {
    const gid = projectData?.graph_id as string | undefined
    if (gid) void loadGraph(gid)
  }

  const stopGraphRefresh = () => {
    if (graphTimerRef.current) {
      clearInterval(graphTimerRef.current)
      graphTimerRef.current = null
      addLog('Graph auto-refresh stopped')
    }
  }

  const startGraphRefresh = () => {
    if (graphTimerRef.current) return
    addLog('Graph auto-refresh started (30s)')
    graphTimerRef.current = setInterval(refreshGraph, 30000)
  }

  useEffect(() => {
    if (isSimulating) startGraphRefresh()
    else stopGraphRefresh()
    return () => stopGraphRefresh()
  }, [isSimulating])

  const loadSimulationData = useCallback(async () => {
    if (!simulationId) return
    addLog(`Loading simulation data: ${simulationId}`)
    try {
      const simRes = await api.simulation.get(simulationId)
      if (simRes.success && simRes.data) {
        const simData = simRes.data as { project_id?: string }
        try {
          const configRes = await api.simulation.getConfig(simulationId)
          const m = (configRes.data as { time_config?: { minutes_per_round?: number } })?.time_config?.minutes_per_round
          if (configRes.success && m) {
            setMinutesPerRound(m)
            addLog(`Time config: ${m} min/round`)
          }
        } catch {
          addLog(`Failed to get time config, using default: ${minutesPerRound} min/round`)
        }
        if (simData.project_id) {
          const projRes = await api.graph.getProject(simData.project_id)
          if (projRes.success && projRes.data) {
            setProjectData(projRes.data as Record<string, unknown>)
            addLog(`Project loaded: ${simData.project_id}`)
            const gid = (projRes.data as { graph_id?: string }).graph_id
            if (gid) await loadGraph(gid)
          }
        }
      }
    } catch (e: unknown) {
      addLog(`Load error: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [simulationId, addLog, minutesPerRound])

  useEffect(() => {
    addLog('SimulationRunView initialized')
    if (maxRounds) addLog(`Custom simulation rounds: ${maxRounds}`)
    void loadSimulationData()
  }, [simulationId])

  const handleGoBack = async () => {
    addLog('Returning to Step 2, closing simulation...')
    stopGraphRefresh()
    if (!simulationId) return
    try {
      const envStatusRes = await api.simulation.getEnvStatus({ simulation_id: simulationId })
      if (envStatusRes.success && (envStatusRes.data as { env_alive?: boolean })?.env_alive) {
        addLog('Closing simulation environment...')
        try {
          await api.simulation.closeEnv({ simulation_id: simulationId, timeout: 10 })
          addLog('✓ Simulation environment closed')
        } catch {
          await api.simulation.stop({ simulation_id: simulationId })
          addLog('✓ Simulation force stopped')
        }
      } else if (isSimulating) {
        await api.simulation.stop({ simulation_id: simulationId })
        addLog('✓ Simulation stopped')
      }
    } catch (e: unknown) {
      addLog(`Failed to check simulation status: ${e instanceof Error ? e.message : String(e)}`)
    }
    navigate(`/simulation/${simulationId}`)
  }

  const statusClass = currentStatus
  const statusText = useMemo(() => {
    if (currentStatus === 'error') return 'Error'
    if (currentStatus === 'completed') return 'Completed'
    return 'Running'
  }, [currentStatus])

  return (
    <WorkflowLayout
      stepFraction="3/5"
      stepName="Simulation"
      statusClass={statusClass}
      statusText={statusText}
      viewMode={viewMode}
      onViewModeChange={setViewMode}
      leftPanelStyle={layout.left}
      rightPanelStyle={layout.right}
      graphPanel={
        <GraphPanel
          graphData={graphData}
          loading={graphLoading}
          currentPhase={3}
          isSimulating={isSimulating}
          onRefresh={refreshGraph}
          onToggleMaximize={() => setViewMode((m) => (m === 'graph' ? 'split' : 'graph'))}
        />
      }
      workbench={
        <Step3Simulation
          simulationId={simulationId}
          maxRounds={maxRounds}
          minutesPerRound={minutesPerRound}
          projectData={projectData}
          graphData={graphData}
          systemLogs={systemLogs}
          onGoBack={() => void handleGoBack()}
          onNextStep={() => addLog('Entering Step 4: Report')}
          onAddLog={addLog}
          onUpdateStatus={setCurrentStatus}
        />
      }
    />
  )
}
