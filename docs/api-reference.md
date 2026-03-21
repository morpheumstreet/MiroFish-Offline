# MiroFish Backend — API path reference

Flask app from `backend/run.py`. Default bind: `FLASK_HOST` (default `0.0.0.0`) and `FLASK_PORT` (default `5001`). CORS is enabled for `/api/*` with `origins: *`.

Unless noted, JSON responses wrap payloads as `{ "success": boolean, "data": ... }` or `{ "success": false, "error": string, ... }`.

**Source of truth:** `backend/app/__init__.py` (blueprint prefixes), `backend/app/api/graph.py`, `simulation.py`, `report.py`.

---

## Health & static

| Method | Path | Notes |
|--------|------|--------|
| GET | `/health` | `{ "status": "ok", "service": "MiroFish-Offline Backend" }` |
| GET | `/`, `/<path:path>` | SPA from `fishtank/dist` when built; paths under `api/` return 404 |

---

## `/api/graph` — projects, ontology, build, graph data

Prefix: `graph_bp` → `/api/graph`.

### Projects

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/graph/project/<project_id>` | Project detail |
| GET | `/api/graph/project/list` | List projects (`limit` query, default 50) |
| DELETE | `/api/graph/project/<project_id>` | Delete project |
| POST | `/api/graph/project/<project_id>/reset` | Reset project for rebuild |

### Ontology & build

| Method | Path | Notes |
|--------|------|--------|
| POST | `/api/graph/ontology/generate` | `multipart/form-data`: `files`, required `simulation_requirement`; optional `project_name`, `additional_context` |
| POST | `/api/graph/build` | JSON: `project_id` (required); optional `graph_name`, `chunk_size`, `chunk_overlap`, `force` |

### Tasks (graph build)

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/graph/task/<task_id>` | Task status |
| GET | `/api/graph/tasks` | List tasks |

### Graph CRUD / data

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/graph/data/<graph_id>` | Nodes and edges |
| DELETE | `/api/graph/delete/<graph_id>` | Delete graph |

---

## `/api/simulation` — entities, lifecycle, run, interview

Prefix: `simulation_bp` → `/api/simulation`.

### Knowledge graph entities (Neo4j)

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/simulation/entities/<graph_id>` | Query: `entity_types` (comma-separated), `enrich` (default `true`) — **Flask** |
| GET | `/api/simulation/entities/<graph_id>/<entity_uuid>` | Single entity + context — **Flask** |
| GET | `/api/simulation/entities/<graph_id>/by-type/<entity_type>` | Query: `enrich` — **Flask** |

**Lake (Go):** same semantics, but paths live under the **graph** blueprint so they do not clash with `GET /api/simulation/<simulation_id>/...`:

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/graph/entities/<graph_id>` | Query: `entity_types`, `enrich` |
| GET | `/api/graph/entities/<graph_id>/<entity_uuid>` | Single entity + context |
| GET | `/api/graph/entities/<graph_id>/by-type/<entity_type>` | Query: `enrich` |

### Simulation CRUD & prepare

| Method | Path | Notes |
|--------|------|--------|
| POST | `/api/simulation/create` | JSON: `project_id`; optional `graph_id`, `enable_twitter`, `enable_reddit` |
| POST | `/api/simulation/prepare` | JSON: `simulation_id`; optional `entity_types`, `use_llm_for_profiles`, `parallel_profile_count`, `force_regenerate` |
| POST | `/api/simulation/prepare/status` | JSON: `task_id` and/or `simulation_id` |
| GET | `/api/simulation/<simulation_id>` | Simulation state; `run_instructions` when ready |
| GET | `/api/simulation/list` | Query: optional `project_id` |
| GET | `/api/simulation/history` | Query: `limit` (default 20) |

### Profiles & config

| Method | Path | Notes |
|--------|------|--------|
| POST | `/api/simulation/generate-profiles` | JSON: `graph_id`; optional `entity_types`, `use_llm`, `platform` (`reddit` / `twitter`) |
| GET | `/api/simulation/<simulation_id>/profiles` | Query: `platform` (`reddit` default) |
| GET | `/api/simulation/<simulation_id>/profiles/realtime` | Live file read during generation |
| GET | `/api/simulation/<simulation_id>/config` | After prepare |
| GET | `/api/simulation/<simulation_id>/config/realtime` | Partial config + generation metadata |
| GET | `/api/simulation/<simulation_id>/config/download` | Attachment `simulation_config.json` |
| GET | `/api/simulation/script/<script_name>/download` | Allowed: `run_twitter_simulation.py`, `run_reddit_simulation.py`, `run_parallel_simulation.py`, `action_logger.py` — **Flask** |

**Lake (Go):** same handler, path `GET /api/simulation/download/script/<script_name>` (avoids `http.ServeMux` overlap with `/<simulation_id>/config/download`).

### Run control & status

| Method | Path | Notes |
|--------|------|--------|
| POST | `/api/simulation/start` | JSON: `simulation_id`; optional `platform` (`twitter` / `reddit` / `parallel`), `max_rounds`, `enable_graph_memory_update`, `force` |
| POST | `/api/simulation/stop` | JSON: `simulation_id` |
| GET | `/api/simulation/<simulation_id>/run-status` | Polling-friendly summary |
| GET | `/api/simulation/<simulation_id>/run-status/detail` | Query: optional `platform`; includes action lists |
| GET | `/api/simulation/<simulation_id>/actions` | Query: `limit`, `offset`, `platform`, `agent_id`, `round_num` |
| GET | `/api/simulation/<simulation_id>/timeline` | Query: `start_round`, `end_round` |
| GET | `/api/simulation/<simulation_id>/agent-stats` | Per-agent stats |

### SQLite-backed social data

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/simulation/<simulation_id>/posts` | Query: `platform` (`reddit` default), `limit`, `offset` |
| GET | `/api/simulation/<simulation_id>/comments` | Reddit; query: `post_id`, `limit`, `offset` |

### Interview & environment

| Method | Path | Notes |
|--------|------|--------|
| POST | `/api/simulation/interview` | JSON: `simulation_id`, `agent_id`, `prompt`; optional `platform`, `timeout` |
| POST | `/api/simulation/interview/batch` | JSON: `simulation_id`, `interviews[]`; optional `platform`, `timeout` |
| POST | `/api/simulation/interview/all` | JSON: `simulation_id`, `prompt`; optional `platform`, `timeout` |
| POST | `/api/simulation/interview/history` | JSON: `simulation_id`; optional `platform`, `agent_id`, `limit` |
| POST | `/api/simulation/env-status` | JSON: `simulation_id` |
| POST | `/api/simulation/close-env` | JSON: `simulation_id`; optional `timeout` |

---

## `/api/report` — generation, retrieval, chat, logs, tools

Prefix: `report_bp` → `/api/report`.

### Generation

| Method | Path | Notes |
|--------|------|--------|
| POST | `/api/report/generate` | JSON: `simulation_id`; optional `force_regenerate` |
| POST | `/api/report/generate/status` | JSON: `task_id` and/or `simulation_id` |

### Reports

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/report/<report_id>` | Full report payload |
| GET | `/api/report/by-simulation/<simulation_id>` | Latest report for simulation — **Flask** path param |
| GET | `/api/report/by-simulation?simulation_id=` | **Lake:** query param (avoids mux clash with `/{report_id}/...`) |
| GET | `/api/report/list` | Query: `simulation_id`, `limit` (default 50) |
| GET | `/api/report/<report_id>/download` | Markdown attachment |
| DELETE | `/api/report/<report_id>` | Delete report |

### Chat & streaming sections

| Method | Path | Notes |
|--------|------|--------|
| POST | `/api/report/chat` | JSON: `simulation_id`, `message`; optional `chat_history` |
| GET | `/api/report/<report_id>/progress` | Generation progress |
| GET | `/api/report/<report_id>/sections` | Section list + `is_complete` |
| GET | `/api/report/<report_id>/section/<section_index>` | Single section markdown (`section_index` integer) |

### Status & logs

| Method | Path | Notes |
|--------|------|--------|
| GET | `/api/report/check/<simulation_id>` | `has_report`, `report_status`, `interview_unlocked` — **Flask** |
| GET | `/api/report/check?simulation_id=` | **Lake:** same JSON shape |
| GET | `/api/report/<report_id>/agent-log` | Query: `from_line` |
| GET | `/api/report/<report_id>/agent-log/stream` | Batch log lines |
| GET | `/api/report/<report_id>/console-log` | Query: `from_line` |
| GET | `/api/report/<report_id>/console-log/stream` | Batch console lines |

### Debug / graph tools (report namespace)

| Method | Path | Notes |
|--------|------|--------|
| POST | `/api/report/tools/search` | JSON: `graph_id`, `query`; optional `limit` |
| POST | `/api/report/tools/statistics` | JSON: `graph_id` |

---

## Dependencies

Many routes require Neo4j (`GraphStorage` / `neo4j_storage`). If initialization fails, graph- and some simulation/report endpoints return errors (often 500 with a message about GraphStorage).
