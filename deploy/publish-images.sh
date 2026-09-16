#!/bin/sh
# Đóng gói và đẩy hai image lên vContainer Registry.
#
# Cách dùng: sh deploy/publish-images.sh <tag>
# Ví dụ:     sh deploy/publish-images.sh 2026-09-16-1
#
# Trước khi chạy cần đăng nhập registry một lần bằng tài khoản Repository User
# tạo trên https://vcr.console.greennode.ai:
#   docker login vcr.vngcloud.vn
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
REGISTRY=${REGISTRY:-vcr.vngcloud.vn/114544-ea-hackathon}
TAG=${1:-}

if [ -z "$TAG" ]; then
	printf 'Thiếu tag. Ví dụ: sh deploy/publish-images.sh 2026-09-16-1\n' >&2
	exit 1
fi

# Node của cluster chạy linux/amd64; máy Mac Apple Silicon mặc định tạo arm64.
PLATFORM=${PLATFORM:-linux/amd64}

set -x
docker buildx build --platform "$PLATFORM" \
	-f "$ROOT/deploy/docker/agent-service.Dockerfile" \
	-t "$REGISTRY/agent-service:$TAG" \
	--push "$ROOT"

docker buildx build --platform "$PLATFORM" \
	-f "$ROOT/deploy/docker/admin-web.Dockerfile" \
	-t "$REGISTRY/admin-web:$TAG" \
	--push "$ROOT"
set +x

printf '\nĐã đẩy:\n  %s/agent-service:%s\n  %s/admin-web:%s\n' \
	"$REGISTRY" "$TAG" "$REGISTRY" "$TAG"
