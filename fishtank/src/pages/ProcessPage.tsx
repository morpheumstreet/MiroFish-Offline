import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { api } from '../api'
import GraphPanel, { type GraphData } from '../components/GraphPanel'
import Step1GraphBuild from '../components/Step1GraphBuild'
import WorkflowLayout, { panelStyles } from '../components/WorkflowLayout'
import { clearPendingUpload, getPendingUpload } from '../store/pendingUpload'

type LogLine = { time: string; msg: string }

type ProjectRow = Record<string, unknown> & { project_id?: string; graph_id?: string; status?: string; graph_build_task_id?: string }

export default function ProcessPage() {
  const { projectId } = useParams<{ projectId: string }>()
  const navigate = useNavigate()
  const [viewMode, setViewMode] = useState<'graph' | 'split' | 'workbench'>('split')
  const [currentProjectId, setCurrentProjectId] = useState(projectId || '')
  const [loading, setLoading] = useState(false)
  const [graphLoading, setGraphLoading] = useState(false)
  const [error, setError] = useState('')
  const [projectData, setProjectData] = useState<ProjectRow | null>(null)
  const [graphData, setGraphData] = useState<GraphData | null>(null)
  const [currentPhase, setCurrentPhase] = useState(-1)
  const [ontologyProgress, setOntologyProgress] = useState<{ message?: string } | null>(null)
  const [buildProgress, setBuildProgress] = useState<{ progress?: number; message?: string } | null>(null)
  const [systemLogs, setSystemLogs] = useState<LogLine[]>([])

  const pollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const graphPollTimerRef = useRef<ReturnType<typeof setInterval> | null>(null)

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

  const stopPolling = useCallback(() => {
    if (pollTimerRef.current) {
      clearInterval(pollTimerRef.current)
      pollTimerRef.current = null
    }
  }, [])

  const stopGraphPolling = useCallback(() => {
    if (graphPollTimerRef.current) {
      clearInterval(graphPollTimerRef.current)
      graphPollTimerRef.current = null
    }
  }, [])

  const loadGraph = useCallback(
    async (graphId: string) => {
      setGraphLoading(true)
      addLog(`Loading full graph data: ${graphId}`)
      try {
        const res = await api.graph.getGraphData(graphId)
        if (res.success) {
          setGraphData(res.data as GraphData)
          addLog('Graph data loaded successfully.')
        } else {
          addLog(`Failed to load graph data: ${res.error}`)
        }
      } catch (e: unknown) {
        addLog(`Exception loading graph: ${e instanceof Error ? e.message : String(e)}`)
      } finally {
        setGraphLoading(false)
      }
    },
    [addLog]
  )

  const fetchGraphData = useCallback(async () => {
    try {
      const projRes = await api.graph.getProject(currentProjectId)
      if (projRes.success && projRes.data && (projRes.data as ProjectRow).graph_id) {
        const gRes = await api.graph.getGraphData((projRes.data as ProjectRow).graph_id!)
        if (gRes.success) {
          setGraphData(gRes.data as GraphData)
          const d = gRes.data as GraphData
          const nodeCount = d.node_count ?? d.nodes?.length ?? 0
          const edgeCount = d.edge_count ?? d.edges?.length ?? 0
          addLog(`Graph data refreshed. Nodes: ${nodeCount}, Edges: ${edgeCount}`)
        }
      }
    } catch {
      /* ignore */
    }
  }, [currentProjectId, addLog])

  const startGraphPolling = useCallback(() => {
    addLog('Started polling for graph data...')
    void fetchGraphData()
    graphPollTimerRef.current = setInterval(() => void fetchGraphData(), 10000)
  }, [addLog, fetchGraphData])

  const pollTaskStatus = useCallback(
    async (taskId: string) => {
      try {
        const res = await api.graph.getTaskStatus(taskId)
        if (res.success && res.data) {
          const task = res.data as { message?: string; progress?: number; status?: string; error?: string }
          setBuildProgress({ progress: task.progress || 0, message: task.message })
          if (task.message) addLog(task.message)
          if (task.status === 'completed') {
            addLog('Graph build task completed.')
            stopPolling()
            stopGraphPolling()
            setCurrentPhase(2)
            const projRes = await api.graph.getProject(currentProjectId)
            if (projRes.success && projRes.data && (projRes.data as ProjectRow).graph_id) {
              setProjectData(projRes.data as ProjectRow)
              await loadGraph((projRes.data as ProjectRow).graph_id!)
            }
          } else if (task.status === 'failed') {
            stopPolling()
            setError(task.error || 'failed')
            addLog(`Graph build task failed: ${task.error}`)
          }
        }
      } catch {
        /* ignore */
      }
    },
    [addLog, currentProjectId, loadGraph, stopGraphPolling, stopPolling]
  )

  const startPollingTask = useCallback(
    (taskId: string) => {
      void pollTaskStatus(taskId)
      pollTimerRef.current = setInterval(() => void pollTaskStatus(taskId), 2000)
    },
    [pollTaskStatus]
  )

  const startBuildGraph = useCallback(async () => {
    try {
      setCurrentPhase(1)
      setBuildProgress({ progress: 0, message: 'Starting build...' })
      addLog('Initiating graph build...')
      const res = await api.graph.buildGraph({ project_id: currentProjectId })
      if (res.success && res.data) {
        addLog(`Graph build task started. Task ID: ${(res.data as { task_id: string }).task_id}`)
        startGraphPolling()
        startPollingTask((res.data as { task_id: string }).task_id)
      } else {
        setError((res as { error?: string }).error || 'build failed')
        addLog(`Error starting build: ${(res as { error?: string }).error}`)
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
      addLog(`Exception in startBuildGraph: ${e instanceof Error ? e.message : String(e)}`)
    }
  }, [addLog, currentProjectId, startGraphPolling, startPollingTask])

  const handleNewProject = useCallback(async () => {
    const pending = getPendingUpload()
    if (!pending.isPending || pending.files.length === 0) {
      setError('No pending files found.')
      addLog('Error: No pending files found for new project.')
      return
    }
    try {
      setLoading(true)
      setCurrentPhase(0)
      setOntologyProgress({ message: 'Uploading and analyzing docs...' })
      addLog('Starting ontology generation: Uploading files...')
      const formData = new FormData()
      pending.files.forEach((f) => formData.append('files', f))
      formData.append('simulation_requirement', pending.simulationRequirement)
      const res = await api.graph.generateOntology(formData)
      if (res.success && res.data) {
        clearPendingUpload()
        const pid = (res.data as { project_id: string }).project_id
        setCurrentProjectId(pid)
        setProjectData(res.data as ProjectRow)
        navigate(`/process/${pid}`, { replace: true })
        setOntologyProgress(null)
        addLog(`Ontology generated successfully for project ${pid}`)
        await startBuildGraph()
      } else {
        setError((res as { error?: string }).error || 'Ontology generation failed')
        addLog(`Error generating ontology: ${(res as { error?: string }).error}`)
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
      addLog(`Exception in handleNewProject: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setLoading(false)
    }
  }, [addLog, navigate, startBuildGraph])

  const loadProject = useCallback(async () => {
    try {
      setLoading(true)
      addLog(`Loading project ${currentProjectId}...`)
      const res = await api.graph.getProject(currentProjectId)
      if (res.success && res.data) {
        const data = res.data as ProjectRow
        setProjectData(data)
        const st = data.status || ''
        if (st === 'created' || st === 'ontology_generated') setCurrentPhase(0)
        else if (st === 'graph_building') setCurrentPhase(1)
        else if (st === 'graph_completed') setCurrentPhase(2)
        else if (st === 'failed') setError('Project failed')
        addLog(`Project loaded. Status: ${st}`)
        if (st === 'ontology_generated' && !data.graph_id) {
          await startBuildGraph()
        } else if (st === 'graph_building' && data.graph_build_task_id) {
          setCurrentPhase(1)
          startPollingTask(data.graph_build_task_id)
          startGraphPolling()
        } else if (st === 'graph_completed' && data.graph_id) {
          setCurrentPhase(2)
          await loadGraph(data.graph_id)
        }
      } else {
        setError((res as { error?: string }).error || 'load failed')
        addLog(`Error loading project: ${(res as { error?: string }).error}`)
      }
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : String(e))
      addLog(`Exception in loadProject: ${e instanceof Error ? e.message : String(e)}`)
    } finally {
      setLoading(false)
    }
  }, [addLog, currentProjectId, loadGraph, startBuildGraph, startGraphPolling, startPollingTask])

  useEffect(() => {
    setCurrentProjectId(projectId || '')
  }, [projectId])

  useEffect(() => {
    addLog('Project view initialized.')
    if (currentProjectId === 'new') void handleNewProject()
    else if (currentProjectId) void loadProject()
    return () => {
      stopPolling()
      stopGraphPolling()
    }
  }, [currentProjectId])

  const statusClass = useMemo(() => {
    if (error) return 'error'
    if (currentPhase >= 2) return 'completed'
    return 'processing'
  }, [error, currentPhase])

  const statusText = useMemo(() => {
    if (error) return 'Error'
    if (currentPhase >= 2) return 'Ready'
    if (currentPhase === 1) return 'Building Graph'
    if (currentPhase === 0) return 'Generating Ontology'
    return 'Initializing'
  }, [error, currentPhase])

  const refreshGraph = () => {
    if (projectData?.graph_id) {
      addLog('Manual graph refresh triggered.')
      void loadGraph(projectData.graph_id)
    }
  }

  return (
    <WorkflowLayout
      stepFraction="1/5"
      stepName="Graph Build"
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
          currentPhase={currentPhase}
          onRefresh={refreshGraph}
          onToggleMaximize={() =>
            setViewMode((m) => (m === 'graph' ? 'split' : 'graph'))
          }
        />
      }
      workbench={
        <Step1GraphBuild
          currentPhase={currentPhase}
          projectData={projectData}
          ontologyProgress={ontologyProgress}
          buildProgress={buildProgress}
          graphData={graphData}
          systemLogs={systemLogs}
        />
      }
    />
  )
}
