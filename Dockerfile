# --- Stage 1: build fishtank SPA (Bun); output must include dist/index.html ---
FROM oven/bun:1.3.11 AS frontend-builder

WORKDIR /app
COPY fishtank/package.json fishtank/bun.lock ./fishtank/
RUN cd fishtank && bun install --frozen-lockfile

COPY fishtank/ ./fishtank/
RUN cd fishtank && bun run build \
  && test -f dist/index.html \
  || (echo "fishtank build incomplete: missing dist/index.html" >&2 && exit 1)

# --- Stage 2: build Lake (Go API + static SPA at runtime via LAKE_FRONTEND_DIST) ---
FROM golang:1.24-bookworm AS lake-builder

WORKDIR /src
COPY lake/go.mod lake/go.sum ./
RUN go mod download

COPY lake/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /lake -trimpath -ldflags="-s -w" ./cmd/lake

# --- Stage 3: minimal runtime; Python simulation scripts from backend/scripts (volume or image) ---
FROM debian:bookworm-slim

RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates bash \
  && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=lake-builder /lake /usr/local/bin/lake
COPY --from=frontend-builder /app/fishtank/dist ./fishtank/dist

# OASIS / parallel sim still invoke Python under backend/scripts (see LAKE_BACKEND_ROOT).
COPY backend/scripts ./backend/scripts

# Default layout matches compose volume ./backend/uploads -> /app/backend/uploads
RUN mkdir -p /app/backend/uploads

ENV LAKE_FRONTEND_DIST=/app/fishtank/dist
ENV LAKE_UPLOAD_FOLDER=/app/backend/uploads
ENV LAKE_BACKEND_ROOT=/app/backend
# Ollama ignores the key; Lake requires non-empty (see config.Validate).
ENV LLM_API_KEY=ollama
ENV NEO4J_PASSWORD=mirofish
# Compose: override in .env for service hostnames (ollama, neo4j).
ENV LLM_BASE_URL=http://ollama:11434/v1
ENV EMBEDDING_BASE_URL=http://ollama:11434
ENV NEO4J_URI=bolt://neo4j:7687

EXPOSE 5001

CMD ["lake"]
