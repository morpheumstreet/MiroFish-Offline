import { HttpClient } from './client/http'
import { withRetry } from './client/retry'
import { GraphEndpoints } from './endpoints/GraphEndpoints'
import { ReportEndpoints } from './endpoints/ReportEndpoints'
import { SimulationEndpoints } from './endpoints/SimulationEndpoints'

export type { ApiData, ApiEnvelope, CreateSimulationDto, OntologyGenerateDto, TaskStartedDto } from './schemas'
export { isApiFailure } from './schemas'
export type { HttpClientOptions } from './client/http'
export { HttpClient, defaultBaseURL } from './client/http'
export { withRetry } from './client/retry'
export { GraphEndpoints } from './endpoints/GraphEndpoints'
export { SimulationEndpoints } from './endpoints/SimulationEndpoints'
export { ReportEndpoints } from './endpoints/ReportEndpoints'

export type MiroFishApi = ReturnType<typeof createApi>

/** Wire endpoint classes to a shared `HttpClient` (use a mock client in tests). */
export function createApi(http: HttpClient) {
  return {
    graph: new GraphEndpoints(http),
    simulation: new SimulationEndpoints(http),
    report: new ReportEndpoints(http)
  } as const
}

const defaultHttp = new HttpClient()
export const api = createApi(defaultHttp)
