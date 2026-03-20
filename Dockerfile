FROM python:3.11

# Node (root tooling) + Bun (fishtank SPA build)
ENV BUN_INSTALL=/root/.bun
ENV PATH="${BUN_INSTALL}/bin:${PATH}"
RUN apt-get update \
  && apt-get install -y --no-install-recommends nodejs npm curl unzip \
  && rm -rf /var/lib/apt/lists/* \
  && curl -fsSL https://bun.sh/install | bash

# 从 uv 官方镜像复制 uv
COPY --from=ghcr.io/astral-sh/uv:0.9.26 /uv /uvx /bin/

# rustls + webpki 在部分网络下对 PyPI/Fastly 校验会误报；用系统 OpenSSL 信任库
ENV UV_NATIVE_TLS=1

WORKDIR /app

# 先复制依赖描述文件以利用缓存
COPY package.json ./
COPY fishtank/package.json ./fishtank/
COPY backend/pyproject.toml backend/uv.lock ./backend/

# 若构建环境对 PyPI 的 TLS 异常（代理/证书替换导致 CN 与 Fastly 默认证书不匹配），
# 请构建时传入: --build-arg UV_ALLOW_INSECURE_PYPI=1
ARG UV_ALLOW_INSECURE_PYPI=0

# 安装依赖（Node + Bun + Python）
RUN npm install \
  && cd fishtank && bun install \
  && cd ../backend \
  && if [ "$UV_ALLOW_INSECURE_PYPI" = "1" ]; then \
       uv sync --allow-insecure-host pypi.org --allow-insecure-host files.pythonhosted.org; \
     else \
       uv sync; \
     fi

# 复制项目源码
COPY . .

# 构建前端静态资源（Bun -> fishtank/dist）；生产环境 axios 使用同源 /api
RUN npm run build

EXPOSE 5001

# 仅启动 Flask；静态页面与 /api 同源
CMD ["bash", "-c", "cd backend && uv run python run.py"]
