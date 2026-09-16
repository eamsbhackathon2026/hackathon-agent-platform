# Build context is the repository root.
# syntax=docker/dockerfile:1

FROM golang:1.27-alpine AS build
WORKDIR /src
COPY services/agent-service/go.mod services/agent-service/go.sum ./services/agent-service/
RUN --mount=type=cache,target=/go/pkg/mod \
    cd services/agent-service && go mod download
COPY services/agent-service ./services/agent-service
# CGO stays off so the binaries run on the distroless-style scratch base below.
ENV CGO_ENABLED=0
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd services/agent-service && \
    go build -trimpath -ldflags="-s -w" -o /out/agent-service ./cmd/agent-service && \
    go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM gcr.io/distroless/static-debian12:nonroot AS runtime
COPY --from=build /out/agent-service /usr/local/bin/agent-service
COPY --from=build /out/migrate /usr/local/bin/migrate
USER nonroot:nonroot
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/agent-service"]
