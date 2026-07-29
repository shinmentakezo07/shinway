FROM golang:1.26-bookworm AS builder

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends build-essential git curl && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_DATE=unknown

RUN CGO_ENABLED=1 GOOS=linux go build -buildvcs=false -ldflags="-s -w -X 'main.Version=${VERSION}' -X 'main.Commit=${COMMIT}' -X 'main.BuildDate=${BUILD_DATE}'" -o ./shinway ./cmd/server/

FROM debian:bookworm

RUN apt-get update && apt-get install -y --no-install-recommends tzdata ca-certificates curl unzip && rm -rf /var/lib/apt/lists/*

# Install bun so the management panel web UI can be built on demand from web/
RUN curl -fsSL https://bun.sh/install | bash && ln -s /root/.bun/bin/bun /usr/local/bin/bun

RUN mkdir /shinway && mkdir /shinway/db

COPY --from=builder ./app/shinway /shinway/shinway
COPY web /shinway/web

COPY config.example.yaml /shinway/config.example.yaml

WORKDIR /shinway

EXPOSE 8317
EXPOSE 8085
EXPOSE 1455
EXPOSE 54545
EXPOSE 51121
EXPOSE 11451

ENV TZ=Asia/Shanghai
# Local web/ source is preferred for the management panel; built on demand to dist/index.html.
ENV MANAGEMENT_LOCAL_WEB_PATH=/shinway/web

RUN cp /usr/share/zoneinfo/${TZ} /etc/localtime && echo "${TZ}" > /etc/timezone

CMD ["./shinway"]
