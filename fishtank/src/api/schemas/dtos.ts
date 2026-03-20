/**
 * Narrow response shapes where the UI depends on specific keys.
 * Extend here and use as `ApiEnvelope<MyDto>` / `HttpClient.post<MyDto>` for stricter call sites.
 */

export type TaskStartedDto = { task_id: string }

export type OntologyGenerateDto = { project_id: string } & Record<string, unknown>

export type CreateSimulationDto = { simulation_id: string }
