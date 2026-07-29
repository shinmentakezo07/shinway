FROM golang:1.26-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-suggests --no-install-recommends build-essential git curl && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=1 GOOS=linux go build -buildvcs=false -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./shinway ./cmd/server/

FROM oven/bun:1.3.14 AS webbuilder

WORKDIR /app/web

COPY web/package.json web/bun.lock web/tsconfig.json web/tsconfig.node.json web/vite.config.ts web/index.html ./
COPY web/src ./src

RUN bun install --frozen-lockfile && bun run build

FROM debian:bookworm

RUN apt-get update && apt-get install -y --no-install-suggests --no-install-recommends tzdata ca-certificates curl && rm -rf /var/lib/apt/lists/*

RUN mkdir /shinway && mkdir /shinway/db

COPY --from=builder ./app/shinway /shinway/shinway
COPY --from=webbuilder ./app/web/dist/index.html /shinway/static/management.html

COPY config.example.yaml /shinway/config.example.yaml
COPY config.example.yaml /shinway/config.yaml

WORKDIR /shinway

EXPOSE 8317
EXPOSE 8085
EXPOSE 1455
EXPOSE 54545
EXPOSE 51121
EXPOSE 11451

ENV TZ=Asia/Shanghai
# Pre-built management panel asset; local web/ source is intentionally omitted.
ENV MANAGEMENT_STATIC_PATH=/shinway/static/management.html

RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

CMD ["./shinway"]
