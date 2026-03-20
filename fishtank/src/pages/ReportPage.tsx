import { useCallback, useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { api } from '../api'
import GraphPanel, { type GraphData } from '../components/GraphPanel'
import Step4Report from '../components/Step4Report'
import WorkflowLayout, { panelStyles } from '../components/WorkflowLayout'

type LogLine = { time: string; msg: string }

export default function ReportPage() {
  const { reportId } = useParams<{ reportId: string }>()
  const [viewMode, setViewMode] = useState<'graph' | 'split' | 'workbench'>('workbench')
  const [simulationId, setSimulationId] = useState<string | null>(null)
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
    setSystemLogs((prev) => [...prev, { time, msg }].slice(-100))
  }, [])

  useEffect(() => {
    if (!reportId) return
    void (async () => {
      try {
        const r = await api.report.get(reportId)
        if (r.success && r.data) {
          const sid = (r.data as { simulation_id?: string }).simulation_id
          if (sid) {
            setSimulationId(sid)
            const simRes = await api.simulation.get(sid)
            if (simRes.success && simRes.data) {
              const pid = (simRes.data as { project_id?: string }).project_id
              if (pid) {
                const projRes = await api.graph.getProject(pid)
                if (projRes.success && projRes.data) {
                  setProjectData(projRes.data as Record<string, unknown>)
                  const gid = (projRes.data as { graph_id?: string }).graph_id
                  if (gid) {
                    setGraphLoading(true)
                    const g = await api.graph.getGraphData(gid)
                    if (g.success) setGraphData(g.data as GraphData)
                    setGraphLoading(false)
                  }
                }
              }
            }
          }
        }
      } catch {
        addLog('Failed to resolve report context')
      }
    })()
  }, [reportId, addLog])

  const statusClass = currentStatus
  const statusText = currentStatus === 'error' ? 'Error' : currentStatus === 'completed' ? 'Ready' : 'Generating'

  return (
    <WorkflowLayout
      stepFraction="4/5"
      stepName="Report"
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
          currentPhase={4}
          onRefresh={() => {
            const gid = projectData?.graph_id as string | undefined
            if (gid)
              void api.graph.getGraphData(gid).then((res) => {
                if (res.success) setGraphData(res.data as GraphData)
              })
          }}
          onToggleMaximize={() => setViewMode((m) => (m === 'graph' ? 'workbench' : 'graph'))}
        />
      }
      workbench={
        <Step4Report
          reportId={reportId}
          simulationId={simulationId}
          systemLogs={systemLogs}
          onAddLog={addLog}
          onUpdateStatus={setCurrentStatus}
        />
      }
    />
  )
}
