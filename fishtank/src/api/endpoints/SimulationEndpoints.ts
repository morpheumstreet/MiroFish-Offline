import type { HttpClient } from '../client/http'
import { withRetry } from '../client/retry'
import type { ApiEnvelope } from '../schemas/envelope'
import type { CreateSimulationDto } from '../schemas/dtos'

export class SimulationEndpoints {
  constructor(private readonly http: HttpClient) {}

  create(data: Record<string, unknown>): Promise<ApiEnvelope<CreateSimulationDto>> {
    return withRetry(() => this.http.post<CreateSimulationDto>('/api/simulation/create', data), 3, 1000)
  }

  prepare(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return withRetry(() => this.http.post('/api/simulation/prepare', data), 3, 1000)
  }

  getPrepareStatus(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return this.http.post('/api/simulation/prepare/status', data)
  }

  get(simulationId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}`)
  }

  getProfiles(simulationId: string, platform = 'reddit'): Promise<ApiEnvelope> {
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/profiles`, {
      platform
    })
  }

  getProfilesRealtime(simulationId: string, platform = 'reddit'): Promise<ApiEnvelope> {
    return this.http.get(
      `/api/simulation/${encodeURIComponent(simulationId)}/profiles/realtime`,
      { platform }
    )
  }

  getConfig(simulationId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/config`)
  }

  getConfigRealtime(simulationId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/config/realtime`)
  }

  list(projectId?: string): Promise<ApiEnvelope> {
    return this.http.get('/api/simulation/list', projectId ? { project_id: projectId } : {})
  }

  start(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return withRetry(() => this.http.post('/api/simulation/start', data), 3, 1000)
  }

  stop(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return this.http.post('/api/simulation/stop', data)
  }

  getRunStatus(simulationId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/run-status`)
  }

  getRunStatusDetail(simulationId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/run-status/detail`)
  }

  getPosts(
    simulationId: string,
    platform = 'reddit',
    limit = 50,
    offset = 0
  ): Promise<ApiEnvelope> {
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/posts`, {
      platform,
      limit,
      offset
    })
  }

  getTimeline(
    simulationId: string,
    startRound = 0,
    endRound: number | null = null
  ): Promise<ApiEnvelope> {
    const q: Record<string, number> = { start_round: startRound }
    if (endRound !== null) q.end_round = endRound
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/timeline`, q)
  }

  getAgentStats(simulationId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/agent-stats`)
  }

  getActions(simulationId: string, params: Record<string, unknown> = {}): Promise<ApiEnvelope> {
    const flat: Record<string, string | number | boolean> = {}
    for (const [k, v] of Object.entries(params)) {
      if (v === undefined || v === null) continue
      if (typeof v === 'string' || typeof v === 'number' || typeof v === 'boolean') {
        flat[k] = v
      } else {
        flat[k] = JSON.stringify(v)
      }
    }
    return this.http.get(`/api/simulation/${encodeURIComponent(simulationId)}/actions`, flat)
  }

  closeEnv(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return this.http.post('/api/simulation/close-env', data)
  }

  getEnvStatus(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return this.http.post('/api/simulation/env-status', data)
  }

  interviewBatch(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return withRetry(() => this.http.post('/api/simulation/interview/batch', data), 3, 1000)
  }

  history(limit = 20): Promise<ApiEnvelope> {
    return this.http.get('/api/simulation/history', { limit })
  }
}
