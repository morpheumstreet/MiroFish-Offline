# Lake HTTP API

Lake exposes a JSON-first HTTP API aligned with the legacy Flask backend under `backend/app`, so clients can swap the host while keeping the same path prefixes.

**Default listen address:** `LAKE_HTTP_HOST` / `FLASK_HOST` (default `0.0.0.0`) and `LAKE_HTTP_PORT` / `FLASK_PORT` (default `5001`). See `internal/config` for full environment variables.

## Conventions

### Base paths

| Prefix | Purpose |
|--------|---------|
| `/health` | Liveness and optional Neo4j ping (not under `/api`) |
| `/api/graph/...` | Projects, tasks, ontology, graph build, Neo4j graph data, **entity reads** |
| `/api/simulation/...` | Simulation lifecycle, profiles, config, runtime telemetry |
| `/api/report/...` | Report generation, sections, logs, graph tools used by reporting |

If `LAKE_FRONTEND_DIST` points at a built SPA with `index.html`, the server also serves that app at `/` (static assets + SPA fallback).

### CORS

Responses include `Access-Control-Allow-Origin: *`, methods `GET, POST, PUT, PATCH, DELETE, OPTIONS`, and headers `Content-Type, Authorization`. `OPTIONS` returns `204 No Content`.

### JSON envelope (most routes)

Successful responses often use:

```json
{ "success": true, "data": { ... }, "count": 0 }
```

`count` is present on some list endpoints. Errors use:

```json
{ "success": false, "error": "message" }
```

**Exceptions:**

- **`POST /api/graph/ontology/generate`** — `multipart/form-data`, not JSON.
- **`GET /api/simulation/{id}/config/download`**, **`GET /api/simulation/download/script/{scriptName}`**, **`GET /api/report/{reportId}/download`** — file responses (`Content-Disposition`, raw body).
- **`POST /api/graph/build`** when a build is already in progress — may return `{ "success": false, "error": "...", "task_id": ... }` without the usual `data` wrapper.
- **`GET /api/report/by-simulation`** on missing report — `404` with `{ "success": false, "error": "...", "has_report": false }`.
- **`POST /api/simulation/interview/batch`** on failure — `400` with `{ "success": false, "error": "..." }`.
- **Project delete / graph delete / report delete** — success may use `{ "success": true, "message": "..." }` only.

### Lake-specific routing notes

- Neo4j **entity** listing and detail are implemented under **`/api/graph/entities/...`**. For Flask parity, the same handlers are also available at **`/api/simulation/entities/...`** (Lake dispatches that prefix before other `GET /api/simulation/...` routes so paths like `/api/simulation/entities/profiles/realtime` are unambiguous).
- **`GET /api/report/check`** and **`GET /api/report/by-simulation`** require **`?simulation_id=`** (query string), not path parameters, for the same reason.

---

## `GET /health`

Returns JSON including `status`, `service`, and when Neo4j is configured: `neo4j_ok` (boolean) and `neo4j_error` (string) if ping failed.

---

## Graph (`/api/graph`)

### Projects and tasks

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/graph/project/{id}` | Single project (see project fields below). |
| `GET` | `/api/graph/project/list?limit=` | List projects (`limit` optional, default 50). Response: `data` array + `count`. |
| `DELETE` | `/api/graph/project/{id}` | Delete project. |
| `POST` | `/api/graph/project/{id}/reset` | Clear graph linkage and errors; adjust status from ontology state. |
| `GET` | `/api/graph/task/{id}` | Async task status/result. |
| `GET` | `/api/graph/tasks` | List all tasks. |

**Project object (representative keys):** `project_id`, `name`, `status`, `created_at`, `updated_at`, `files`, `total_text_length`, `ontology`, `analysis_summary`, `graph_id`, `graph_build_task_id`, `simulation_requirement`, `chunk_size`, `chunk_overlap`, `error` (nullable when empty).

**Task object (representative keys):** `task_id`, `task_type`, `status`, `created_at`, `updated_at`, `progress`, `message`, `progress_detail`, `result`, `error`, `metadata`.

### Ontology and graph build

| Method | Path | Body / input | Description |
|--------|------|----------------|-------------|
| `POST` | `/api/graph/ontology/generate` | **Multipart form** | Creates a **new** project, stores uploads, runs LLM ontology generation. |
| `POST` | `/api/graph/build` | JSON | Starts async graph build; poll **`/api/graph/task/{task_id}`**. |

**`POST /api/graph/ontology/generate` (multipart fields)**

| Field | Required | Notes |
|-------|----------|--------|
| `simulation_requirement` | Yes | Simulation / scenario description. |
| `project_name` | No | Default `Unnamed Project`. |
| `additional_context` | No | Extra context for the LLM. |
| `files` | Yes (≥1) | Document parts; extensions **pdf**, **md**, **txt**, **markdown** (case-insensitive). |

**`POST /api/graph/build` (JSON)**

| Field | Type | Required | Notes |
|-------|------|----------|--------|
| `project_id` | string | Yes | |
| `graph_name` | string | No | Defaults to project name or `"MiroFish Graph"`. |
| `chunk_size` | int | No | Defaults from project or 500. |
| `chunk_overlap` | int | No | Defaults from project or 50. |
| `force` | bool | No | If true, can reset stuck/failed/completed build state and restart. |

Requires Neo4j (`503` if graph storage unavailable). Project must have ontology generated first (`400` if still `created`).

### Graph data and entities (Neo4j)

| Method | Path | Query | Description |
|--------|------|-------|-------------|
| `GET` | `/api/graph/data/{graphId}` | — | Full graph payload from storage (shape defined by `GraphStore.GetGraphData`). |
| `DELETE` | `/api/graph/delete/{graphId}` | — | Delete graph. |
| `GET` | `/api/graph/entities/{graphId}` | `enrich` (default true; use `enrich=false` to disable), `entity_types` (comma-separated) | Filtered entity map. |
| `GET` | `/api/graph/entities/{graphId}/{entityUUID}` | — | Single entity with context. |
| `GET` | `/api/graph/entities/{graphId}/by-type/{entityType}` | `enrich` | `{ entity_type, count, entities }`. |

---

## Simulation (`/api/simulation`)

### Lifecycle

| Method | Path | JSON body | Description |
|--------|------|-----------|-------------|
| `POST` | `/api/simulation/create` | `project_id`, `graph_id`, `enable_twitter`?, `enable_reddit`? (default true) | Create simulation state. |
| `POST` | `/api/simulation/prepare` | `simulation_id`, `entity_types`, `use_llm_for_profiles`?, `parallel_profile_count`? (default 5), `force_regenerate` | Prepare profiles + config. |
| `POST` | `/api/simulation/prepare/status` | `task_id`, `simulation_id` (both accepted; empty fields ignored) | Prepare job status. |
| `POST` | `/api/simulation/start` | `simulation_id`, `platform`? (default `parallel`), `max_rounds`?, `enable_graph_memory_update`, `force` | Start runtime. |
| `POST` | `/api/simulation/stop` | `simulation_id` | Stop runtime. |
| `POST` | `/api/simulation/env-status` | `simulation_id` | Environment status. |
| `POST` | `/api/simulation/close-env` | `simulation_id`, `timeout`? | Close environment. |
| `POST` | `/api/simulation/generate-profiles` | `graph_id`, `entity_types`, `use_llm`?, `platform`? (default `reddit`) | Standalone profile generation (requires Neo4j). |
| `POST` | `/api/simulation/interview/batch` | `simulation_id`, `interviews` (array of maps, e.g. `prompt`), `platform`?, `timeout`? | Batch interviews; prompts may be prefixed server-side. |

### Listing and detail

| Method | Path | Query | Description |
|--------|------|-------|-------------|
| `GET` | `/api/simulation/list` | `project_id` optional | Simulations; envelope includes `count`. |
| `GET` | `/api/simulation/history` | `limit` optional | History list + `count`. |
| `GET` | `/api/simulation/{simulationId}` | — | Simulation metadata map. |

### Profiles and config

| Method | Path | Query | Description |
|--------|------|-------|-------------|
| `GET` | `/api/simulation/{simulationId}/profiles` | `platform` (default `reddit`) | JSON array in `profiles` (Reddit) or CSV rows as objects (Twitter). |
| `GET` | `/api/simulation/{simulationId}/profiles/realtime` | `platform` | Profiles plus `file_exists`, `file_modified_at`, `total_expected`, `is_generating`. |
| `GET` | `/api/simulation/{simulationId}/config` | — | Parsed `simulation_config.json` or 404 if missing. |
| `GET` | `/api/simulation/{simulationId}/config/realtime` | — | Config + generation stage summary fields. |
| `GET` | `/api/simulation/{simulationId}/config/download` | — | **File** download of `simulation_config.json`. |

### Scripts

| Method | Path | Description |
|--------|------|-------------|
| `GET` | `/api/simulation/download/script/{scriptName}` | Allowed names: `run_twitter_simulation.py`, `run_reddit_simulation.py`, `run_parallel_simulation.py`, `action_logger.py`. |

### Runtime reads

| Method | Path | Query | Description |
|--------|------|-------|-------------|
| `GET` | `/api/simulation/{simulationId}/run-status` | — | Run state summary. |
| `GET` | `/api/simulation/{simulationId}/run-status/detail` | `platform` | Detailed run state. |
| `GET` | `/api/simulation/{simulationId}/actions` | `limit` (default 100), `offset`, `platform`, `agent_id`, `round_num` | Filtered actions. |
| `GET` | `/api/simulation/{simulationId}/timeline` | `start_round`, `end_round` | Timeline data. |
| `GET` | `/api/simulation/{simulationId}/agent-stats` | — | Per-agent stats. |
| `GET` | `/api/simulation/{simulationId}/posts` | `platform` (default `reddit`), `limit`, `offset` | Posts feed. |
| `GET` | `/api/simulation/{simulationId}/comments` | `platform` (default `reddit`), `post_id`?, `limit` (default 50), `offset` (default 0) | Rows from SQLite `{platform}_simulation.db` table `comment` (same as Flask **OnlyReddit** path); empty if DB missing or table error. |

---

## Report (`/api/report`)

Report routes require the report service (and graph tools where noted). Neo4j-backed tools return **`503`** if the graph stack is unavailable.

### Generation and lookup

| Method | Path | Input | Description |
|--------|------|-------|-------------|
| `POST` | `/api/report/generate` | JSON: `simulation_id`, `force_regenerate`? | Start or resume generation. |
| `GET` | `/api/report/generate/status` | Query: **`report_id`** (required) | Poll generation status. |
| `POST` | `/api/report/generate/status` | JSON: `task_id`, `simulation_id` | Status by task/simulation. |
| `GET` | `/api/report/check` | Query: **`simulation_id`** (required) | Whether a report exists for the simulation. |
| `GET` | `/api/report/by-simulation` | Query: **`simulation_id`** (required) | Report payload or 404 with `has_report: false`. |
| `GET` | `/api/report/list` | `simulation_id`?, `limit`? (default 50) | List reports + `count`. |
| `GET` | `/api/report/{reportId}` | — | Report metadata. |
| `DELETE` | `/api/report/{reportId}` | — | Delete report artifacts. |

### Content, progress, sections

| Method | Path | Query | Description |
|--------|------|-------|-------------|
| `GET` | `/api/report/{reportId}/progress` | — | Generation progress. |
| `GET` | `/api/report/{reportId}/sections` | — | Section index / list. |
| `GET` | `/api/report/{reportId}/section/{sectionIndex}` | — | One section; `sectionIndex` is **1-based**. |
| `GET` | `/api/report/{reportId}/download` | — | **Markdown** attachment (`full_report` content). |

### Logs

| Method | Path | Query | Description |
|--------|------|-------|-------------|
| `GET` | `/api/report/{reportId}/agent-log` | `from_line` (default 0) | Agent log snapshot. |
| `GET` | `/api/report/{reportId}/console-log` | `from_line` | Console log snapshot. |
| `GET` | `/api/report/{reportId}/agent-log/stream` | — | Recent agent log lines + count. |
| `GET` | `/api/report/{reportId}/console-log/stream` | — | Recent console log lines + count. |

### Graph tools and chat

| Method | Path | JSON body | Description |
|--------|------|-----------|-------------|
| `POST` | `/api/report/tools/search` | `graph_id`, `query`, `limit`? (default 10) | Keyword search over graph data. |
| `POST` | `/api/report/tools/statistics` | `graph_id` | Aggregate statistics. |
| `POST` | `/api/report/chat` | `simulation_id`, `message`, `chat_history` (array of `{role, content}`-style maps) | Chat turn; response includes `response` and `simulation_id`. |

---

## Typical status codes

| Code | When |
|------|------|
| `200` | Success (including some “empty but OK” payloads). |
| `400` | Bad JSON, missing required fields, invalid parameters. |
| `404` | Missing project, task, simulation, report, entity, script, or empty content where applicable. |
| `405` | Wrong method (e.g. non-POST on ontology/build). |
| `503` | Neo4j / graph storage not initialized or entity endpoints unavailable. |
| `500` | Unexpected server or persistence errors. |

For authoritative behavior and response shapes as the codebase evolves, refer to `internal/httpapi/*.go`.
