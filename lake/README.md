# Lake (Go backend)

Structured port of the Python Flask app in `../backend/`. See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for layers, SOLID mapping, and incremental porting order.

## Run

From repo root (so `LLM_*` / `NEO4J_*` match `.env` if exported):

```bash
cd lake
export $(grep -v '^#' ../.env | xargs) 2>/dev/null || true
go run ./cmd/lake
```

- Defaults: `LAKE_HTTP_HOST` / `LAKE_HTTP_PORT` fall back to `FLASK_HOST` / `FLASK_PORT`, then `0.0.0.0:5001`.
- `GET /health` — if Neo4j driver initializes, includes `neo4j_ok`.
- **Implemented:** graph project CRUD/list/reset, task list/get, **`POST /api/graph/ontology/generate`**, **`POST /api/graph/build`** (async task + Neo4j: NER via LLM, embeddings via Ollama `/api/embed`, same graph model as Python), **`GET /api/graph/data/{graphId}`**, **`DELETE /api/graph/delete/{graphId}`**. Requires working Neo4j (`GraphReady`); `LLM_*` + `EMBEDDING_*` / `OLLAMA_NUM_CTX` / `LLM_TIMEOUT_SECONDS`.
- Other `/api/*` routes return **501** until ported.

## Layout

| Path | Purpose |
|------|---------|
| `cmd/lake` | `main` |
| `internal/config` | Environment loading / validation |
| `internal/domain` | Entities and domain errors |
| `internal/ports` | Interfaces (adapters implement these) |
| `internal/app` | Wires adapters into `Deps` |
| `internal/adapters` | Neo4j, filesystem, LLM, … |
| `internal/httpapi` | HTTP transport and route map |
| `internal/usecase` | Add packages here as logic is ported |
