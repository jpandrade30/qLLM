# syntax=docker/dockerfile:1
FROM golang:1.25-bookworm AS build
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN go build -tags duckdb -o /out/qllm ./cmd/qllm

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/qllm /usr/local/bin/qllm
COPY deploy/image/config /config

EXPOSE 8088 8089
ENTRYPOINT ["qllm"]
CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]
