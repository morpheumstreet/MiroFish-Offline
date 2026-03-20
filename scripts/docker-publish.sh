#!/usr/bin/env bash
# Build and push the MiroFish-Offline app image (Dockerfile at repo root — app only; not neo4j/ollama).
#
# Usage: bash scripts/docker-publish.sh [tag]
#   If no tag: uses git describe (e.g. v0.1.0 or v0.1.0-1-gabc1234), else "latest".
#
# Env:
#   DOCKER_SPACE_SORA   Docker Hub username, or registry prefix like ghcr.io/owner (required)
#   DOCKER_TOKEN_SORA   Registry password or API token (required)
#   DOCKER_LOGIN_USER   Optional. User for `docker login -u` when it differs from the namespace
#                       (e.g. GHCR org image ghcr.io/myorg/... — login as your GitHub username).
#   PUSH_LATEST         Set to 0 to skip tagging/pushing :latest (only push the requested tag).
#   UV_ALLOW_INSECURE_PYPI  Set to 1 if docker build fails on uv sync with PyPI TLS / cert name
#                         mismatch (e.g. corporate SSL inspection). Weakens verification for PyPI only.
#
# Examples:
#   DOCKER_SPACE_SORA=myuser DOCKER_TOKEN_SORA=xxx bash scripts/docker-publish.sh
#   DOCKER_SPACE_SORA=ghcr.io/nikmcfly DOCKER_TOKEN_SORA=ghp_xxx bash scripts/docker-publish.sh v0.2.0
#   DOCKER_SPACE_SORA=ghcr.io/myorg DOCKER_LOGIN_USER=mygithubuser DOCKER_TOKEN_SORA=ghp_xxx bash scripts/docker-publish.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${REPO_ROOT}"

if [[ -n "${1:-}" ]]; then
  TAG="${1}"
else
  TAG="$(git describe --tags --always 2>/dev/null || echo 'latest')"
fi

IMAGE_NAME="mirofish-offline"

if [[ -z "${DOCKER_SPACE_SORA:-}" || -z "${DOCKER_TOKEN_SORA:-}" ]]; then
  echo "Error: DOCKER_SPACE_SORA and DOCKER_TOKEN_SORA must be set."
  echo "  DOCKER_SPACE_SORA = Docker Hub user or ghcr.io/owner (same value you use in docker tag prefix)"
  echo "  DOCKER_TOKEN_SORA = registry password or access token"
  exit 1
fi

BUILD_ARGS=()
if [[ "${UV_ALLOW_INSECURE_PYPI:-}" == "1" ]]; then
  BUILD_ARGS+=(--build-arg UV_ALLOW_INSECURE_PYPI=1)
fi

echo "Building ${IMAGE_NAME}:${TAG}..."
docker build "${BUILD_ARGS[@]}" -t "${IMAGE_NAME}:${TAG}" .

REMOTE_IMAGE="${DOCKER_SPACE_SORA}/${IMAGE_NAME}:${TAG}"
echo "Logging in and pushing ${REMOTE_IMAGE}..."

# Docker Hub: login with username. GHCR/other: registry host is first path segment; login user may
# differ from org namespace — use DOCKER_LOGIN_USER when pushing to ghcr.io/myorg/... as a person.
if [[ "${DOCKER_SPACE_SORA}" == *"/"* ]]; then
  REGISTRY="${DOCKER_SPACE_SORA%%/*}"
  if [[ -n "${DOCKER_LOGIN_USER:-}" ]]; then
    LOGIN_USER="${DOCKER_LOGIN_USER}"
  else
    LOGIN_USER="${DOCKER_SPACE_SORA#*/}"
  fi
  echo "${DOCKER_TOKEN_SORA}" | docker login "${REGISTRY}" -u "${LOGIN_USER}" --password-stdin
else
  echo "${DOCKER_TOKEN_SORA}" | docker login -u "${DOCKER_SPACE_SORA}" --password-stdin
fi

docker tag "${IMAGE_NAME}:${TAG}" "${REMOTE_IMAGE}"
docker push "${REMOTE_IMAGE}"

if [[ "${PUSH_LATEST:-1}" == "1" && "${TAG}" != "latest" ]]; then
  REMOTE_LATEST="${DOCKER_SPACE_SORA}/${IMAGE_NAME}:latest"
  docker tag "${IMAGE_NAME}:${TAG}" "${REMOTE_LATEST}"
  docker push "${REMOTE_LATEST}"
  echo "Published ${REMOTE_IMAGE} and ${REMOTE_LATEST}"
else
  echo "Published ${REMOTE_IMAGE}"
fi
