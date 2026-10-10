# 构建上下文是仓库根目录：docker build -f apps/control-api/Dockerfile .
ARG GO_BASE=golang:1.22-alpine
ARG RUNTIME_BASE=alpine:3.20
FROM ${GO_BASE} AS build
ARG GOPROXY=https://proxy.golang.org,direct
ARG GOSUMDB=sum.golang.org
ENV GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB}
WORKDIR /src
COPY pkg/platform ./pkg/platform
COPY apps/control-api/go.mod apps/control-api/go.sum ./apps/control-api/
WORKDIR /src/apps/control-api
RUN go mod download
COPY apps/control-api ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/control-api ./cmd/server

FROM ${RUNTIME_BASE}
RUN apk add --no-cache ca-certificates curl \
    && adduser -D -H -u 65532 app
COPY --from=build /out/control-api /usr/local/bin/control-api
USER app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/control-api"]
