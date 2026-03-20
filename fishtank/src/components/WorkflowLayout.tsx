import { type ReactNode, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import '../styles/MainView.css'

type ViewMode = 'graph' | 'split' | 'workbench'

export type WorkflowLayoutProps = {
  stepFraction: string
  stepName: string
  statusClass: string
  statusText: string
  leftPanelStyle: React.CSSProperties
  rightPanelStyle: React.CSSProperties
  graphPanel: ReactNode
  workbench: ReactNode
  initialViewMode?: ViewMode
  viewMode?: ViewMode
  onViewModeChange?: (m: ViewMode) => void
}

export default function WorkflowLayout({
  stepFraction,
  stepName,
  statusClass,
  statusText,
  leftPanelStyle,
  rightPanelStyle,
  graphPanel,
  workbench,
  initialViewMode = 'split',
  viewMode: controlledViewMode,
  onViewModeChange
}: WorkflowLayoutProps) {
  const navigate = useNavigate()
  const [internalMode, setInternalMode] = useState<ViewMode>(initialViewMode)
  const viewMode = controlledViewMode ?? internalMode
  const setViewMode = onViewModeChange ?? setInternalMode

  return (
    <div className="main-view">
      <header className="app-header">
        <div className="header-left">
          <div className="brand" onClick={() => navigate('/')}>
            MIROFISH OFFLINE
          </div>
        </div>
        <div className="header-center">
          <div className="view-switcher">
            {(['graph', 'split', 'workbench'] as const).map((mode) => (
              <button
                key={mode}
                type="button"
                className={`switch-btn${viewMode === mode ? ' active' : ''}`}
                onClick={() => setViewMode(mode)}
              >
                {mode === 'graph' ? 'Graph' : mode === 'split' ? 'Split' : 'Workbench'}
              </button>
            ))}
          </div>
        </div>
        <div className="header-right">
          <div className="workflow-step">
            <span className="step-num">Step {stepFraction}</span>
            <span className="step-name">{stepName}</span>
          </div>
          <div className="step-divider" />
          <span className={`status-indicator ${statusClass}`}>
            <span className="dot" />
            {statusText}
          </span>
        </div>
      </header>
      <main className="content-area">
        <div className="panel-wrapper left" style={leftPanelStyle}>
          {graphPanel}
        </div>
        <div className="panel-wrapper right" style={rightPanelStyle}>
          {workbench}
        </div>
      </main>
    </div>
  )
}

export function panelStyles(viewMode: ViewMode): {
  left: React.CSSProperties
  right: React.CSSProperties
} {
  if (viewMode === 'graph') {
    return {
      left: { width: '100%', opacity: 1, transform: 'translateX(0)' },
      right: { width: '0%', opacity: 0, transform: 'translateX(20px)' }
    }
  }
  if (viewMode === 'workbench') {
    return {
      left: { width: '0%', opacity: 0, transform: 'translateX(-20px)' },
      right: { width: '100%', opacity: 1, transform: 'translateX(0)' }
    }
  }
  return {
    left: { width: '50%', opacity: 1, transform: 'translateX(0)' },
    right: { width: '50%', opacity: 1, transform: 'translateX(0)' }
  }
}
