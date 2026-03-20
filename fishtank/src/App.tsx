import { Navigate, Route, Routes } from 'react-router-dom'
import HomePage from './pages/HomePage'
import InteractionPage from './pages/InteractionPage'
import ProcessPage from './pages/ProcessPage'
import ReportPage from './pages/ReportPage'
import SimulationPage from './pages/SimulationPage'
import SimulationRunPage from './pages/SimulationRunPage'

export default function App() {
  return (
    <Routes>
      <Route path="/" element={<HomePage />} />
      <Route path="/process/:projectId" element={<ProcessPage />} />
      <Route path="/simulation/:simulationId" element={<SimulationPage />} />
      <Route path="/simulation/:simulationId/start" element={<SimulationRunPage />} />
      <Route path="/report/:reportId" element={<ReportPage />} />
      <Route path="/interaction/:reportId" element={<InteractionPage />} />
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  )
}
