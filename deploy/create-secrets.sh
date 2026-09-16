#!/bin/sh
# Tạo hoặc cập nhật Secret cho namespace agent-platform trên VKS.
#
# Lần chạy đầu sinh mật khẩu Postgres và ba khóa 32 byte rồi lưu vào
# deploy/.env.production (đã được .gitignore bỏ qua). Các lần sau đọc lại
# file đó, nên ba khóa giữ nguyên giữa các lần rollout — đổi khóa sẽ làm
# mất hiệu lực toàn bộ token và dữ liệu đã mã hóa.
set -eu

ROOT=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
ENV_FILE="$ROOT/deploy/.env.production"
NAMESPACE=agent-platform

if [ ! -f "$ENV_FILE" ]; then
	printf 'Sinh giá trị mới cho %s\n' "$ENV_FILE"
	umask 077
	cat > "$ENV_FILE" <<INNER
# Cấu hình triển khai VKS. Không commit file này.
# APP_ORIGIN là origin người dùng gõ trên trình duyệt, ví dụ http://123.45.67.89
# hoặc https://agent.example.com. Bắt buộc phải điền, nếu bỏ trống thì mọi
# yêu cầu POST/PUT/DELETE từ trình duyệt sẽ bị chặn 403.
APP_ORIGIN=
APP_ENV=dev
LOG_LEVEL=info
POSTGRES_USER=agent_platform
POSTGRES_DB=agent_platform
POSTGRES_PASSWORD=$(openssl rand -hex 24)
JWT_SIGNING_KEY=$(openssl rand -base64 32)
APP_ENCRYPTION_KEY=$(openssl rand -base64 32)
API_KEY_PEPPER=$(openssl rand -base64 32)
JWT_ISSUER=agent-platform
JWT_AUDIENCE=agent-platform-admin
EGRESS_ALLOWLIST=
WORKER_ENABLED=true
WORKER_CONCURRENCY=4
INNER
	printf 'Hãy điền APP_ORIGIN trong %s rồi chạy lại lệnh này.\n' "$ENV_FILE" >&2
	exit 1
fi

# shellcheck disable=SC1090
. "$ENV_FILE"

if [ -z "${APP_ORIGIN:-}" ]; then
	printf 'APP_ORIGIN đang trống trong %s.\n' "$ENV_FILE" >&2
	exit 1
fi

DATABASE_URL="postgres://${POSTGRES_USER}:${POSTGRES_PASSWORD}@postgres.${NAMESPACE}.svc.cluster.local:5432/${POSTGRES_DB}?sslmode=disable"

kubectl -n "$NAMESPACE" create secret generic postgres-credentials \
	--from-literal=POSTGRES_USER="$POSTGRES_USER" \
	--from-literal=POSTGRES_PASSWORD="$POSTGRES_PASSWORD" \
	--from-literal=POSTGRES_DB="$POSTGRES_DB" \
	--dry-run=client -o yaml | kubectl apply -f -

kubectl -n "$NAMESPACE" create secret generic agent-service-config \
	--from-literal=DATABASE_URL="$DATABASE_URL" \
	--from-literal=APP_ENV="$APP_ENV" \
	--from-literal=LOG_LEVEL="$LOG_LEVEL" \
	--from-literal=JWT_SIGNING_KEY="$JWT_SIGNING_KEY" \
	--from-literal=APP_ENCRYPTION_KEY="$APP_ENCRYPTION_KEY" \
	--from-literal=API_KEY_PEPPER="$API_KEY_PEPPER" \
	--from-literal=JWT_ISSUER="$JWT_ISSUER" \
	--from-literal=JWT_AUDIENCE="$JWT_AUDIENCE" \
	--from-literal=CORS_ORIGINS="$APP_ORIGIN" \
	--from-literal=EGRESS_ALLOWLIST="$EGRESS_ALLOWLIST" \
	--from-literal=WORKER_ENABLED="$WORKER_ENABLED" \
	--from-literal=WORKER_CONCURRENCY="$WORKER_CONCURRENCY" \
	--dry-run=client -o yaml | kubectl apply -f -

printf 'Đã cập nhật Secret trong namespace %s với origin %s\n' "$NAMESPACE" "$APP_ORIGIN"
