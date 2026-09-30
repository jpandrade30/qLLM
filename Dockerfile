# syntax=docker/dockerfile:1
FROM golang:1.26-bookworm AS build
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
# Bake harness/demo YAML only (deploy/image/config). This is not fleet-ops and not
# a customer project. Override at runtime: docker -v …:/config or K8s ConfigMap
# on /config (deploy/prd). Rebuild after editing image/config; mounts hide these files.
COPY deploy/image/config/qllm.preset.yaml deploy/image/config/qllm.catalog.yaml /config/
COPY deploy/image/config/qllm.config.yaml deploy/image/config/qllm.env.yaml /config/

EXPOSE 8088 8089
ENTRYPOINT ["qllm"]
CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]
