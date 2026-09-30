# syntax=docker/dockerfile:1
FROM golang:1.26.6-bookworm AS build
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN go build -tags duckdb -o /out/qllm ./cmd/qllm

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /config
COPY --from=build /out/qllm /usr/local/bin/qllm
# Product bake: edit deploy/prd/*.yaml then rebuild. Harness uses Dockerfile.dev.
# Override at runtime with -v …:/config if you do not want a rebuild.
COPY deploy/prd/qllm.preset.yaml deploy/prd/qllm.catalog.yaml /config/
COPY deploy/prd/qllm.config.yaml deploy/prd/qllm.env.yaml /config/
COPY deploy/prd/qllm.access.yaml /config/

EXPOSE 8088 8089
ENTRYPOINT ["qllm"]
CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]
