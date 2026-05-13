# Local Technical Video-to-Blog System

Local-first technical video analysis pipeline using Go, SQLite, local artifacts, and Ollama-hosted models.

## Features

- Single binary (`go run ./cmd/server`) for API + UI
- Public read-only landing page (`/`) for sections, transcripts, and multilingual blogs
- Authenticated admin page (`/admin`) for:
  - creating/editing/deleting sections
  - moving blogs between sections
  - deleting/restoring blogs
  - editing blog markdown per language
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

export ADMIN_SESSION_TTL=72h
export ADMIN_COOKIE_NAME=vtb_admin_session
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

4. Create the first admin user (one-time):

```bash
make admin-create USER=admin PASS='change-this-password'
```

5. Open:

- Public landing page: http://localhost:8080
- Admin page: http://localhost:8080/admin

## API

- Public:
  - `GET /api/public/catalog`
- Admin auth:
  - `POST /api/admin/login`
  - `POST /api/admin/logout`
  - `GET /api/admin/me`
- Admin content:
  - `GET /api/admin/catalog`
  - `GET /api/admin/sections`
  - `POST /api/admin/sections`
  - `PUT /api/admin/sections/{section_id}`
  - `DELETE /api/admin/sections/{section_id}`
  - `GET /api/admin/blogs/{blog_id}`
  - `PUT /api/admin/blogs/{blog_id}`
  - `DELETE /api/admin/blogs/{blog_id}`
  - `POST /api/admin/blogs/{blog_id}/restore`
  - `PUT /api/admin/blogs/{blog_id}/section`
  - `PUT /api/admin/blogs/{blog_id}/content`
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

- No self-registration endpoint is provided. Admin users can only be created via CLI.
- URL downloads use `yt-dlp` (`YTDLP_BIN`, default `yt-dlp`) when `source_type=url`.
- Transcription uses `whisper-cli`; if unavailable, fallback transcript file can be used via `TRANSCRIPT_FALLBACK_PATH`.
- Markdown translation preserves structure by prompt instruction, but model behavior depends on the translation model.
- If your shell has conflicting Go environment variables (for example from `gvm`), run with a clean toolchain path, e.g. `GOROOT=/usr/local/go GOPATH=$HOME/go go run ./cmd/server`.
