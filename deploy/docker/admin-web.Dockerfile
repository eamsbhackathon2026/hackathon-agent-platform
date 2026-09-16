# Build context is the repository root.
# syntax=docker/dockerfile:1

FROM node:22.19-alpine AS build
WORKDIR /src
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/admin-web/package.json ./apps/admin-web/
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile --filter @agent-platform/admin-web
COPY apps/admin-web ./apps/admin-web
RUN pnpm --filter @agent-platform/admin-web build

FROM nginxinc/nginx-unprivileged:1.29-alpine AS runtime
COPY deploy/docker/admin-web-nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=build /src/apps/admin-web/dist /usr/share/nginx/html
EXPOSE 8080
