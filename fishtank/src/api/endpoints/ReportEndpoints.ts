import type { HttpClient } from '../client/http'
import { withRetry } from '../client/retry'
import type { ApiEnvelope } from '../schemas/envelope'

export class ReportEndpoints {
  constructor(private readonly http: HttpClient) {}

  generate(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return withRetry(() => this.http.post('/api/report/generate', data), 3, 1000)
  }

  getGenerateStatus(reportId: string): Promise<ApiEnvelope> {
    return this.http.get('/api/report/generate/status', { report_id: reportId })
  }

  getAgentLog(reportId: string, fromLine = 0): Promise<ApiEnvelope> {
    return this.http.get(`/api/report/${encodeURIComponent(reportId)}/agent-log`, {
      from_line: fromLine
    })
  }

  getConsoleLog(reportId: string, fromLine = 0): Promise<ApiEnvelope> {
    return this.http.get(`/api/report/${encodeURIComponent(reportId)}/console-log`, {
      from_line: fromLine
    })
  }

  get(reportId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/report/${encodeURIComponent(reportId)}`)
  }

  chat(data: Record<string, unknown>): Promise<ApiEnvelope> {
    return withRetry(() => this.http.post('/api/report/chat', data), 3, 1000)
  }
}
