import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import GraphPanel, { type GraphData } from '../components/GraphPanel'
import Step2EnvSetup from '../components/Step2EnvSetup'
import WorkflowLayout, { panelStyles } from '../components/WorkflowLayout'

type LogLine = { time: string; msg: string }

export default function SimulationPage() {
  const { simulationId } = useParams<{ simulationId: string }>()
  const navigate = useNavigate()
  const [viewMode, setViewMode] = useState<'graph' | 'split' | 'workbench'>('split')
  const [projectData, setProjectData] = useState<Record<string, unknown> | null>(null)
  const [graphData, setGraphData] = useState<GraphData | null>(null)
  const [graphLoading, setGraphLoading] = useState(false)
  const [systemLogs, setSystemLogs] = useState<LogLine[]>([])
  const [currentStatus, setCurrentStatus] = useState<'processing' | 'completed' | 'error'>('processing')

  const layout = panelStyles(viewMode)

  const addLog = useCallback((msg: string) => {
    const time =
      new Date().toLocaleTimeString('en-US', { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' }) +
      '.' +
      new Date().getMilliseconds().toString().padStart(3, '0')
    setSystemLogs((prev) => {
      const next = [...prev, { time, msg }]
      return next.length > 100 ? next.slice(-100) : next
    })
  }, [])

  const loadGraph = async (graphId: string) => {
    setGraphLoading(true)
    try {
      const res = await api.graph.getGraphData(graphId)
      if (res.success) setGraphData(res.data as GraphData)
    } catch {
      /* ignore */
    } finally {
      setGraphLoading(false)
    }
  }

  const loadSimulationData = useCallback(async () => {
    if (!simulationId) return
    addLog(`Loading simulation data: ${simulationId}`)
    try {
      const simRes = await api.simulation.get(simulationId)
      if (simRes.success && simRes.data) {
        const simData = simRes.data as { project_id?: string }
        if (simData.project_id) {
          const projRes = await api.graph.getProject(simData.project_id)
          if (projRes.success && projRes.data) {
            setProjectData(projRes.data as Record<string, unknown>)
            addLog(`Project loaded: ${simData.project_id}`)
            const gid = (projRes.data as { graph_id?: string }).graph_id
            if (gid) await loadGraph(gid)
          }
        }
      } else {
        addLog(`Failed to load simulation: ${(simRes as { error?: string }).error}`)
      }
    } catch (e: unknown) {
      addLog(`Load error: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [simulationId, addLog])

  useEffect(() => {
    void loadSimulationData()
  }, [loadSimulationData])

  const checkAndStop = async () => {
    if (!simulationId) return
    try {
      const envStatusRes = await api.simulation.getEnvStatus({ simulation_id: simulationId })
      if (envStatusRes.success && (envStatusRes.data as { env_alive?: boolean })?.env_alive) {
        addLog('Simulation environment running, shutting down...')
        try {
          await api.simulation.closeEnv({ simulation_id: simulationId, timeout: 10 })
          addLog('✓ Simulation environment closed')
        } catch {
          await api.simulation.stop({ simulation_id: simulationId })
          addLog('✓ Simulation force stopped')
        }
      }
    } catch {
      /* ignore */
    }
  }

  const handleGoBack = async () => {
    await checkAndStop()
    const pid = projectData?.project_id as string | undefined
    if (pid) navigate(`/process/${pid}`)
    else navigate('/')
  }

  const handleNextStep = (params: { maxRounds?: number }) => {
    if (!simulationId) return
    if (params.maxRounds) {
      navigate(`/simulation/${simulationId}/start?maxRounds=${params.maxRounds}`)
    } else {
      navigate(`/simulation/${simulationId}/start`)
    }
  }

  const statusClass = currentStatus
  const statusText = currentStatus === 'error' ? 'Error' : currentStatus === 'completed' ? 'Ready' : 'Preparing'

  return (
    <WorkflowLayout
      stepFraction="2/5"
      stepName="Env Setup"
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
          currentPhase={2}
          onRefresh={() => {
            const gid = projectData?.graph_id as string | undefined
            if (gid) void loadGraph(gid)
          }}
          onToggleMaximize={() => setViewMode((m) => (m === 'graph' ? 'split' : 'graph'))}
        />
      }
      workbench={
        <Step2EnvSetup
          simulationId={simulationId}
          projectData={projectData}
          graphData={graphData}
          systemLogs={systemLogs}
          onGoBack={handleGoBack}
          onNextStep={handleNextStep}
          onAddLog={addLog}
          onUpdateStatus={setCurrentStatus}
        />
      }
    />
  )
}
