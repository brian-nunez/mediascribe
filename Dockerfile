# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS builder
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/server ./cmd/server
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/admin ./cmd/admin
RUN CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -o /out/rebuild-embeddings ./cmd/rebuild-embeddings

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
     ca-certificates \
     ffmpeg \
     yt-dlp \
  && rm -rf /var/lib/apt/lists/*

COPY --from=builder /out/server /usr/local/bin/server
COPY --from=builder /out/admin /usr/local/bin/admin
COPY --from=builder /out/rebuild-embeddings /usr/local/bin/rebuild-embeddings

# Runtime assets
COPY ui ./ui
COPY internal/db/migrations ./internal/db/migrations
COPY migrations ./migrations

# Default runtime folders (should be mounted in production)
RUN mkdir -p /app/data /app/artifacts/jobs /app/logs

ENV HTTP_ADDR=:8080 \
    SQLITE_PATH=/app/data/app.db \
    ARTIFACT_ROOT=/app/artifacts/jobs \
    FFMPEG_BIN=ffmpeg \
    YTDLP_BIN=yt-dlp \
    ENABLE_TRANSLATION=false

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
