# Local Technical Video-to-Blog System

Local-first technical video analysis pipeline using Go, SQLite, local artifacts, and Ollama-hosted models.

## Features

- Single binary (`go run ./cmd/server`) for API + UI
- SQLite metadata + chunk embeddings storage
- Local artifact persistence per job
- Independently configurable model hosts for:
  - main reasoning/writing
  - embeddings
  - translation
- Background job pipeline with stage/status tracking

## Quick Start

1. Configure environment variables (optional):

```bash
export SQLITE_PATH=./data/app.db
export ARTIFACT_ROOT=./artifacts/jobs

export MAIN_MODEL=gpt-oss
export MAIN_MODEL_BASE_URL=http://localhost:11434
export MAIN_MODEL_TIMEOUT=1800s
export MAIN_MODEL_MAX_RETRIES=2

export EMBEDDING_MODEL=embeddinggemma
export EMBEDDING_MODEL_BASE_URL=http://localhost:11435
export EMBEDDING_MODEL_TIMEOUT=300s
export EMBEDDING_MODEL_MAX_RETRIES=2

export TRANSLATE_MODEL=translategemma
export TRANSLATE_MODEL_BASE_URL=http://localhost:11436
export TRANSLATE_MODEL_TIMEOUT=1800s
export TRANSLATE_MODEL_MAX_RETRIES=2

export MODEL_RETRY_BACKOFF=10s

export FFMPEG_BIN=ffmpeg
export WHISPER_CPP_BIN=./deps/whisper.cpp/build/bin/whisper-cli
export WHISPER_MODEL_PATH=./deps/whisper.cpp/models/ggml-base.bin
```

2. Run:

```bash
go run ./cmd/server
```

Or use the Makefile defaults:

```bash
make deps-whisper
make run
```

3. Open:

- http://localhost:8080

## API

- `POST /api/jobs`
- `GET /api/jobs`
- `GET /api/jobs/{job_id}`
- `GET /api/jobs/{job_id}/transcript`
- `GET /api/jobs/{job_id}/blog`
- `GET /api/jobs/{job_id}/blog?lang=es`
- `GET /api/jobs/{job_id}/translations`
- `POST /api/jobs/{job_id}/translate`
- `POST /api/jobs/{job_id}/retry`
- `GET /api/search?q=...&limit=10`

## Notes

- URL downloads use `yt-dlp` (`YTDLP_BIN`, default `yt-dlp`) when `source_type=url`.
- Transcription uses `whisper-cli`; if unavailable, fallback transcript file can be used via `TRANSCRIPT_FALLBACK_PATH`.
- Markdown translation preserves structure by prompt instruction, but model behavior depends on the translation model.
- If your shell has conflicting Go environment variables (for example from `gvm`), run with a clean toolchain path, e.g. `GOROOT=/usr/local/go GOPATH=$HOME/go go run ./cmd/server`.
