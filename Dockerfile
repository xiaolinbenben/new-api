# 阶段1：构建前端
FROM oven/bun:1-alpine AS web-builder

WORKDIR /build
COPY web/package.json web/bun.lock ./
RUN bun install
COPY web/ ./
ARG VERSION=dev
RUN DISABLE_ESLINT_PLUGIN='true' VITE_REACT_APP_VERSION=${VERSION} bun run build

# 阶段2：构建 Go 后端
FROM golang:1.23-alpine AS go-builder
ENV GO111MODULE=on CGO_ENABLED=0

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

ENV GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH:-amd64}
ENV GOEXPERIMENT=greenteagc

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
# 复制前端构建产物到 web/dist 目录，Go embed 会将其打包进二进制
COPY --from=web-builder /build/dist ./web/dist
RUN go build -ldflags "-s -w -X 'github.com/QuantumNous/new-api/common.Version=${VERSION}'" -o terln-api

# 阶段3：运行环境
FROM debian:bookworm-slim@sha256:f06537653ac770703bc45b4b113475bd402f451e85223f0f2837acbf89ab020a

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates tzdata wget \
    && rm -rf /var/lib/apt/lists/* \
    && update-ca-certificates

COPY --from=go-builder /build/terln-api /
EXPOSE 3000
WORKDIR /data
ENTRYPOINT ["/terln-api"]
