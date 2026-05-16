# syntax=docker/dockerfile:1

FROM golang:1.25-bookworm AS builder
WORKDIR /src

ARG TARGETOS
ARG TARGETARCH

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
     git \
     cmake \
     make \
     g++ \
     curl \
  && rm -rf /var/lib/apt/lists/*

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/server ./cmd
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/admin ./cmd/admin
RUN CGO_ENABLED=1 GOOS=${TARGETOS:-linux} GOARCH=${TARGETARCH} go build -o /out/rebuild-embeddings ./cmd/rebuild-embeddings

# Build whisper.cpp and download bundled base model for in-image transcription support.
RUN git clone --depth=1 https://github.com/ggml-org/whisper.cpp.git /tmp/whisper.cpp \
  && cmake -S /tmp/whisper.cpp -B /tmp/whisper.cpp/build \
       -DWHISPER_BUILD_TESTS=OFF \
       -DGGML_WERROR=OFF \
       -DCMAKE_C_FLAGS="-Wno-error=stringop-overflow -Wno-stringop-overflow" \
       -DCMAKE_CXX_FLAGS="-Wno-error=stringop-overflow -Wno-stringop-overflow" \
  && cmake --build /tmp/whisper.cpp/build -j \
  && bash /tmp/whisper.cpp/models/download-ggml-model.sh base

FROM debian:bookworm-slim
WORKDIR /app

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
     ca-certificates \
     ffmpeg \
     curl \
     python3 \
     sqlite3 \
  && rm -rf /var/lib/apt/lists/* \
  && curl -L https://github.com/yt-dlp/yt-dlp/releases/latest/download/yt-dlp -o /usr/local/bin/yt-dlp \
  && chmod +x /usr/local/bin/yt-dlp \
  && command -v ffmpeg >/dev/null \
  && command -v yt-dlp >/dev/null

COPY --from=builder /out/server /usr/local/bin/server
COPY --from=builder /out/admin /usr/local/bin/admin
COPY --from=builder /out/rebuild-embeddings /usr/local/bin/rebuild-embeddings
COPY --from=builder /tmp/whisper.cpp/build/bin/whisper-cli /opt/whisper/bin/whisper-cli
COPY --from=builder /tmp/whisper.cpp/models/ggml-base.bin /opt/whisper/models/ggml-base.bin
COPY --from=builder /tmp/whisper.cpp/build/src/libwhisper.so* /opt/whisper/lib/
COPY --from=builder /tmp/whisper.cpp/build/ggml/src/libggml*.so* /opt/whisper/lib/

# Runtime assets
COPY assets ./assets
COPY internal/db/migrations ./internal/db/migrations
COPY migrations ./migrations

# Default runtime folders (should be mounted in production)
RUN mkdir -p /app/data /app/artifacts/jobs /app/logs
RUN /usr/bin/env sh -c 'ffmpeg -version >/dev/null && yt-dlp --version >/dev/null && LD_LIBRARY_PATH=/opt/whisper/lib /opt/whisper/bin/whisper-cli --help >/dev/null'

ENV HTTP_ADDR=:8080 \
    SQLITE_PATH=/app/data/app.db \
    ARTIFACT_ROOT=/app/artifacts/jobs \
    FFMPEG_BIN=ffmpeg \
    WHISPER_CPP_BIN=/opt/whisper/bin/whisper-cli \
    WHISPER_MODEL_PATH=/opt/whisper/models/ggml-base.bin \
    LD_LIBRARY_PATH=/opt/whisper/lib \
    YTDLP_BIN=yt-dlp \
    OTEL_ENABLED=false \
    ENABLE_TRANSLATION=false

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/server"]
