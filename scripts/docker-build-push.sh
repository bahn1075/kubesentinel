#!/usr/bin/env bash
#
# KubeSentinel AI 이미지를 빌드하고 현재 로그인된 Docker Hub로 push 한다.
# 태그: latest + 연월일시분(YYYYMMDDHHMM)
#
# 사용법:
#   ./scripts/docker-build-push.sh <dockerhub-username>
#   DOCKERHUB_USER=<id> ./scripts/docker-build-push.sh
#
# 프론트엔드 이미지를 올리려면 (별도 repo):
#   DOCKERFILE=frontend/Dockerfile CONTEXT=frontend REPO=kubesentinel-ai-frontend \
#     ./scripts/docker-build-push.sh <dockerhub-username>
#
set -euo pipefail

# ── 설정 (환경변수로 override 가능) ──────────────────────────────
# NOTE: 요청에는 'kubensentinel-ai'로 적혀 있었으나 프로젝트명은 'kubesentinel-ai'라
#       오타로 보고 기본값을 kubesentinel-ai로 둔다. 다른 이름을 원하면 REPO=... 로 지정.
REPO="${REPO:-kubesentinel-ai}"        # Docker Hub 저장소 이름
DOCKERFILE="${DOCKERFILE:-Dockerfile}" # 빌드할 Dockerfile
CONTEXT="${CONTEXT:-.}"                 # 빌드 컨텍스트
PLATFORMS="${PLATFORMS:-linux/amd64,linux/arm64}" # 멀티아치 대상 (단일아치는 예: linux/arm64)
BUILDER="${BUILDER:-kubesentinel-builder}"        # buildx 빌더 이름

# ── 프로젝트 루트로 이동 (스크립트 위치 기준) ──────────────────
cd "$(dirname "$0")/.."

# ── Docker Hub 사용자명 결정 (1st arg > env > docker info 자동감지) ──
DOCKERHUB_USER="${1:-${DOCKERHUB_USER:-}}"
if [ -z "$DOCKERHUB_USER" ]; then
  DOCKERHUB_USER="$(docker info 2>/dev/null | awk -F': ' '/Username:/{print $2; exit}')"
fi
if [ -z "$DOCKERHUB_USER" ]; then
  echo "ERROR: Docker Hub 사용자명을 알 수 없습니다." >&2
  echo "       사용법: $0 <dockerhub-username>   또는   DOCKERHUB_USER=<id> $0" >&2
  exit 1
fi

IMAGE="docker.io/${DOCKERHUB_USER}/${REPO}"
TIMESTAMP="$(date +%Y%m%d%H%M)"   # 연월일시분

echo "▶ 이미지    : ${IMAGE}"
echo "▶ 태그      : latest, ${TIMESTAMP}"
echo "▶ 플랫폼    : ${PLATFORMS}"
echo "▶ Dockerfile: ${DOCKERFILE}  (context: ${CONTEXT})"
echo

# ── buildx 빌더 준비 (멀티아치는 docker-container 드라이버 필요) ──
if ! docker buildx inspect "$BUILDER" >/dev/null 2>&1; then
  echo "▶ buildx 빌더 생성: ${BUILDER}"
  docker buildx create --name "$BUILDER" --driver docker-container --use >/dev/null
else
  docker buildx use "$BUILDER"
fi

# ── 멀티아치 빌드 + push (buildx는 빌드와 push를 한 번에 수행) ────
docker buildx build \
  --platform "$PLATFORMS" \
  -f "$DOCKERFILE" \
  -t "${IMAGE}:latest" \
  -t "${IMAGE}:${TIMESTAMP}" \
  --push \
  "$CONTEXT"

echo
echo "✓ 완료 (${PLATFORMS})"
echo "  ${IMAGE}:latest"
echo "  ${IMAGE}:${TIMESTAMP}"
