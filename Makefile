SHELL := /bin/sh
.DEFAULT_GOAL := help

GO_DIR := services/agent-service
GOLANGCI_VERSION := v2.13.2
GOLANGCI_LINT := $(CURDIR)/bin/golangci-lint
RUN_WITH_ENV := go -C $(GO_DIR) run ./cmd/dev-env "$(CURDIR)/.env"

.PHONY: help tools up down logs gen gen-go gen-ts gen-sqlc check-gen check-contract \
	migrate-up migrate-down migrate-new dev-api dev-web test test-go test-web \
	test-integration lint lint-go lint-web lint-api build build-go build-web e2e

help:
	@printf '%s\n' 'make up / down / logs: quản lý Postgres' \
	  'make gen / check-gen: sinh và kiểm tra mã từ OpenAPI' \
	  'make tools: cài golangci-lint được ghim phiên bản' \
	  'make lint test build: kiểm tra toàn bộ phần đã triển khai' \
	  'make dev-api: chạy API; make dev-web: chạy giao diện React' \
	  'make migrate-up / migrate-down / migrate-new name=identity: migration'

tools:
	@sh scripts/install-golangci-lint.sh $(GOLANGCI_VERSION)

up:
	$(RUN_WITH_ENV) docker compose up -d --wait --wait-timeout 30 postgres

down:
	$(RUN_WITH_ENV) docker compose down

logs:
	$(RUN_WITH_ENV) docker compose logs -f postgres

gen: gen-go gen-ts gen-sqlc

gen-go:
	cd $(GO_DIR) && go tool oapi-codegen -config oapi-codegen.yaml ../../api/openapi.yaml

gen-ts:
	pnpm gen:api

gen-sqlc:
	node scripts/generate-sqlc.mjs

check-gen:
	node scripts/check-generated.mjs

check-contract:
	sh services/agent-service/scripts/check_contract_coverage.sh

migrate-up:
	@$(RUN_WITH_ENV) sh -c 'cd services/agent-service && GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$DATABASE_URL" go tool goose -dir migrations up'

migrate-down:
	@$(RUN_WITH_ENV) sh -c 'cd services/agent-service && GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$DATABASE_URL" go tool goose -dir migrations down'

migrate-new: export MIGRATION_NAME = $(name)
migrate-new:
	@node scripts/create-migration.mjs

dev-api:
	$(RUN_WITH_ENV) go -C $(GO_DIR) run ./cmd/agent-service

dev-web:
	pnpm --filter @agent-platform/admin-web dev

test: test-go test-web

test-go:
	cd $(GO_DIR) && go test -race ./...

test-web:
	@printf '%s\n' 'Chạy kiểm thử component, API boundary và route guard bằng Vitest/MSW.'
	pnpm --filter @agent-platform/admin-web test

test-integration:
	$(RUN_WITH_ENV) go -C $(GO_DIR) test -race -count=1 -tags integration ./...

lint: lint-api lint-go lint-web

lint-api:
	pnpm lint:api

lint-go:
	@test -x "$(GOLANGCI_LINT)" || { printf '%s\n' 'Chạy make tools để cài golangci-lint.' >&2; exit 1; }
	cd $(GO_DIR) && "$(GOLANGCI_LINT)" run
	cd $(GO_DIR) && "$(GOLANGCI_LINT)" fmt --diff

lint-web:
	pnpm --filter @agent-platform/admin-web lint

build: build-go build-web

build-go:
	@mkdir -p bin
	cd $(GO_DIR) && go build -o ../../bin/agent-service ./cmd/agent-service

build-web:
	@printf '%s\n' 'Kiểm tra kiểu và tạo bundle production React.'
	pnpm --filter @agent-platform/admin-web build

e2e:
	@printf '%s\n' 'Chưa có luồng E2E. Kiểm thử này được bổ sung sau khi backend và giao diện hoàn tất.' >&2
	@exit 1
