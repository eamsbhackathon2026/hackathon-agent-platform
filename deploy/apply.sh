#!/bin/sh
# Áp dụng toàn bộ manifest lên cluster VKS đang được kubectl trỏ tới.
#
# Cách dùng: sh deploy/apply.sh <tag>
# Yêu cầu:   chạy deploy/create-secrets.sh trước để có Secret trong namespace.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
REGISTRY=${REGISTRY:-vcr.vngcloud.vn/114544-ea-hackathon}
NAMESPACE=agent-platform
TAG=${1:-}

if [ -z "$TAG" ]; then
	printf 'Thiếu tag. Ví dụ: sh deploy/apply.sh 2026-09-16-1\n' >&2
	exit 1
fi

API_IMAGE="$REGISTRY/agent-service:$TAG"
WEB_IMAGE="$REGISTRY/admin-web:$TAG"

render() {
	sed -e "s|IMAGE_AGENT_SERVICE|$API_IMAGE|g" -e "s|IMAGE_ADMIN_WEB|$WEB_IMAGE|g" "$1"
}

kubectl apply -f "$ROOT/deploy/k8s/00-namespace.yaml"

for secret in postgres-credentials agent-service-config; do
	if ! kubectl -n "$NAMESPACE" get secret "$secret" >/dev/null 2>&1; then
		printf 'Chưa có Secret %s. Chạy sh deploy/create-secrets.sh trước.\n' "$secret" >&2
		exit 1
	fi
done

kubectl apply -f "$ROOT/deploy/k8s/10-postgres.yaml"
kubectl -n "$NAMESPACE" rollout status statefulset/postgres --timeout=5m

# Job là bất biến nên phải xóa bản cũ trước khi áp dụng bản mới.
kubectl -n "$NAMESPACE" delete job agent-service-migrate --ignore-not-found
render "$ROOT/deploy/k8s/20-migrate-job.yaml" | kubectl apply -f -
kubectl -n "$NAMESPACE" wait --for=condition=complete job/agent-service-migrate --timeout=5m

render "$ROOT/deploy/k8s/30-agent-service.yaml" | kubectl apply -f -
render "$ROOT/deploy/k8s/40-admin-web.yaml" | kubectl apply -f -
kubectl apply -f "$ROOT/deploy/k8s/50-ingress.yaml"

kubectl -n "$NAMESPACE" rollout status deployment/agent-service --timeout=5m
kubectl -n "$NAMESPACE" rollout status deployment/admin-web --timeout=5m

printf '\nĐịa chỉ công khai (chờ vài phút nếu còn trống):\n'
kubectl -n "$NAMESPACE" get ingress agent-platform
