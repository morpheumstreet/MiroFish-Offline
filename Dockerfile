# --- Stage 1: build fishtank SPA (Bun); output must include dist/index.html for Flask ---
FROM oven/bun:1.3.11 AS frontend-builder

WORKDIR /app
COPY fishtank/package.json fishtank/bun.lock ./fishtank/
RUN cd fishtank && bun install --frozen-lockfile

COPY fishtank/ ./fishtank/
RUN cd fishtank && bun run build \
  && test -f dist/index.html \
  || (echo "fishtank build incomplete: missing dist/index.html" >&2 && exit 1)

# --- Stage 2: Python runtime only; serve prebuilt SPA from fishtank/dist ---
FROM python:3.11-slim-bookworm

# 从 uv 官方镜像复制 uv
COPY --from=ghcr.io/astral-sh/uv:0.9.26 /uv /uvx /bin/

# rustls + webpki 在部分网络下对 PyPI/Fastly 校验会误报；用系统 OpenSSL 信任库
ENV UV_NATIVE_TLS=1

WORKDIR /app

COPY backend/ ./backend/

# Corporate proxies / SSL inspection often make PyPI/Fastly certs fail hostname check (e.g. cert CN
# is *.fastly.net). --allow-insecure-host applies only to these origins and only relaxes verification
# when it would otherwise fail; public CI (e.g. GitHub Actions) still gets normal TLS when certs match.
RUN cd backend \
  && uv sync \
    --allow-insecure-host pypi.org \
    --allow-insecure-host files.pythonhosted.org \
    --allow-insecure-host download.pytorch.org

COPY --from=frontend-builder /app/fishtank/dist ./fishtank/dist

EXPOSE 5001

# 仅启动 Flask；静态页面与 /api 同源
CMD ["bash", "-c", "cd backend && uv run python run.py"]
