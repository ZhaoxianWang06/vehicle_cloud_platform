# 构建上下文是仓库根目录：docker build -f apps/telemetry-api/Dockerfile .
ARG GO_BASE=golang:1.22-alpine
ARG RUNTIME_BASE=alpine:3.20
FROM ${GO_BASE} AS build
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org
ENV GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB}
WORKDIR /src
COPY pkg/platform ./pkg/platform
COPY apps/telemetry-api/go.mod apps/telemetry-api/go.sum ./apps/telemetry-api/
WORKDIR /src/apps/telemetry-api
RUN go mod download
COPY apps/telemetry-api ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/telemetry-api ./cmd/server

FROM ${RUNTIME_BASE}
RUN apk add --no-cache ca-certificates curl \
    && adduser -D -H -u 65532 app
COPY --from=build /out/telemetry-api /usr/local/bin/telemetry-api
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/telemetry-api"]
