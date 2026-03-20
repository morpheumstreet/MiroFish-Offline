import type { HttpClient } from '../client/http'
import { withRetry } from '../client/retry'
import type { ApiEnvelope } from '../schemas/envelope'
import type { OntologyGenerateDto, TaskStartedDto } from '../schemas/dtos'

export class GraphEndpoints {
  constructor(private readonly http: HttpClient) {}

  generateOntology(formData: FormData): Promise<ApiEnvelope<OntologyGenerateDto>> {
    return withRetry(() => this.http.postForm<OntologyGenerateDto>('/api/graph/ontology/generate', formData))
  }

  buildGraph(data: Record<string, unknown>): Promise<ApiEnvelope<TaskStartedDto>> {
    return withRetry(() => this.http.post<TaskStartedDto>('/api/graph/build', data))
  }

  getTaskStatus(taskId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/graph/task/${encodeURIComponent(taskId)}`)
  }

  getGraphData(graphId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/graph/data/${encodeURIComponent(graphId)}`)
  }

  getProject(projectId: string): Promise<ApiEnvelope> {
    return this.http.get(`/api/graph/project/${encodeURIComponent(projectId)}`)
  }
}
