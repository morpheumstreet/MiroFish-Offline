#!/usr/bin/env bash
# Build and push the MiroFish-Offline app image (Dockerfile at repo root — app only; not neo4j/ollama).
#
# External inputs (nothing is read from stdin except the token via docker login):
#
#   Positional (optional):
#     $1  Image tag to build and push (default: git describe --tags --always, or "latest" if not a git repo)
#
#   Environment — required to push (build still needs Docker daemon only):
#     DOCKER_SPACE_SORA   Registry namespace: Docker Hub username, or "ghcr.io/owner" style prefix
#     DOCKER_TOKEN_SORA   Password or access token for docker login
#
#   Environment — optional:
#     IMAGE_NAME          Local/remote image name (repository name), default: mirofish-offline
#     DOCKER_LOGIN_USER   Only when DOCKER_SPACE_SORA contains "/" (e.g. ghcr.io/myorg): login user
#                         if it differs from the path after the host (e.g. push to org as your GitHub user)
#     PUSH_LATEST         Set to 0 to skip also tagging/pushing :latest when your tag is not "latest"
#
# Examples:
#   DOCKER_SPACE_SORA=myuser DOCKER_TOKEN_SORA=xxx bash scripts/docker-publish.sh
#   DOCKER_SPACE_SORA=ghcr.io/nikmcfly DOCKER_TOKEN_SORA=ghp_xxx bash scripts/docker-publish.sh v0.2.0
#   DOCKER_SPACE_SORA=ghcr.io/myorg DOCKER_LOGIN_USER=mygithubuser DOCKER_TOKEN_SORA=ghp_xxx bash scripts/docker-publish.sh

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

usage() {
  awk 'NR==1 {next} /^set -euo pipefail$/ {exit} { sub(/^# ?/, ""); print }' "$0"
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

cd "${REPO_ROOT}"

if [[ -n "${1:-}" ]]; then
  TAG="${1}"
else
  TAG="$(git describe --tags --always 2>/dev/null || echo 'latest')"
fi

IMAGE_NAME="${IMAGE_NAME:-mirofish-offline}"

if [[ -z "${DOCKER_SPACE_SORA:-}" || -z "${DOCKER_TOKEN_SORA:-}" ]]; then
  echo "Error: pushing requires both environment variables:" >&2
  echo "  DOCKER_SPACE_SORA  (registry prefix, e.g. myuser or ghcr.io/myorg)" >&2
  echo "  DOCKER_TOKEN_SORA (token or password for docker login)" >&2
  echo "" >&2
  echo "Optional positional: tag (default: ${TAG})" >&2
  echo "Run: bash scripts/docker-publish.sh --help" >&2
  exit 1
fi

echo "Building ${IMAGE_NAME}:${TAG}..."
docker build -t "${IMAGE_NAME}:${TAG}" .

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
