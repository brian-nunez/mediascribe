SHELL := /bin/zsh

GO ?= go
TEMPL ?= go run github.com/a-h/templ/cmd/templ@v0.3.924
TAILWIND ?= tailwindcss

APP_ENV_VARS := HTTP_ADDR SQLITE_PATH ARTIFACT_ROOT \
	MAIN_MODEL MAIN_MODEL_BASE_URL MAIN_MODEL_TIMEOUT MAIN_MODEL_MAX_RETRIES \
	EMBEDDING_MODEL EMBEDDING_MODEL_BASE_URL EMBEDDING_MODEL_TIMEOUT EMBEDDING_MODEL_MAX_RETRIES \
	TRANSLATE_MODEL TRANSLATE_MODEL_BASE_URL TRANSLATE_MODEL_TIMEOUT TRANSLATE_MODEL_MAX_RETRIES \
	MODEL_RETRY_BACKOFF \
	FFMPEG_BIN WHISPER_CPP_BIN WHISPER_MODEL_PATH YTDLP_BIN \
	TRANSCRIPT_FALLBACK_PATH ENABLE_TRANSLATION \
	ADMIN_SESSION_TTL ADMIN_COOKIE_NAME \
	OTEL_ENABLED OTEL_SERVICE_NAME OTEL_SERVICE_VERSION OTEL_ENVIRONMENT \
	OTEL_EXPORTER_OTLP_ENDPOINT OTEL_EXPORTER_OTLP_INSECURE OTEL_FAIL_FAST

UNSET_FLAGS := $(foreach v,$(APP_ENV_VARS),-u $(v))

# App runtime configuration
export HTTP_ADDR ?= :8080
export SQLITE_PATH ?= ./data/app.db
export ARTIFACT_ROOT ?= ./artifacts/jobs

# OpenTelemetry defaults from the Go/Echo/templ template
export OTEL_ENABLED ?= false
export OTEL_SERVICE_NAME ?= video-to-blog-page
export OTEL_SERVICE_VERSION ?=
export OTEL_ENVIRONMENT ?= development
export OTEL_EXPORTER_OTLP_ENDPOINT ?= localhost:4317
export OTEL_EXPORTER_OTLP_INSECURE ?= true
export OTEL_FAIL_FAST ?= false

# Main reasoning/writing model
export MAIN_MODEL ?= gpt-oss:20b
export MAIN_MODEL_BASE_URL ?= http://10.0.0.119:11434
export MAIN_MODEL_TIMEOUT ?= 1800s
export MAIN_MODEL_MAX_RETRIES ?= 2

# Embedding model
export EMBEDDING_MODEL ?= embeddinggemma:300m
export EMBEDDING_MODEL_BASE_URL ?= http://localhost:11434
export EMBEDDING_MODEL_TIMEOUT ?= 300s
export EMBEDDING_MODEL_MAX_RETRIES ?= 2

# Translation model
export TRANSLATE_MODEL ?= translategemma:4b
export TRANSLATE_MODEL_BASE_URL ?= http://localhost:11434
export TRANSLATE_MODEL_TIMEOUT ?= 1800s
export TRANSLATE_MODEL_MAX_RETRIES ?= 2

export MODEL_RETRY_BACKOFF ?= 10s

# Local tools
export FFMPEG_BIN ?= ffmpeg
export WHISPER_CPP_BIN := ./deps/whisper.cpp/build/bin/whisper-cli
export WHISPER_MODEL_PATH := ./deps/whisper.cpp/models/ggml-base.bin
export YTDLP_BIN ?= yt-dlp

# Optional development fallback transcript file
export TRANSCRIPT_FALLBACK_PATH ?=

# Default translation toggle for new jobs
export ENABLE_TRANSLATION ?= false
export ADMIN_SESSION_TTL ?= 72h
export ADMIN_COOKIE_NAME ?= vtb_admin_session

DOCKER_IMAGE ?= mediascribe
DOCKER_TAG ?= latest

.PHONY: templ templ-generate tailwind tailwind-build server dev run run-fresh build tidy clean env env-fresh unset-env deps-whisper admin-create \
	docker-build docker-build-multiarch docker-build-amd64 docker-push-amd64 docker-up docker-down docker-logs docker-rebuild-embeddings docker-verify-deps

templ:
	$(TEMPL) generate --watch --proxy="http://localhost:8080" --open-browser=false

templ-generate:
	$(TEMPL) generate

tailwind:
	$(TAILWIND) -i ./assets/css/input.css -o ./assets/css/output.css --watch

tailwind-build:
	$(TAILWIND) -i ./assets/css/input.css -o ./assets/css/output.css --minify

server:
	air \
		--build.cmd "go build -o tmp/bin/main ./cmd/main.go" \
		--build.bin "tmp/bin/main" \
		--build.delay "100" \
		--build.exclude_dir "node_modules,tmp,deps,data,artifacts" \
		--build.include_ext "go,templ" \
		--build.stop_on_error "false" \
		--misc.clean_on_exit true

dev:
	make -j3 templ tailwind server

deps-whisper:
	@set -euo pipefail; \
	mkdir -p deps deps/tools; \
	if [ ! -d deps/whisper.cpp/.git ]; then \
		git clone https://github.com/ggml-org/whisper.cpp.git deps/whisper.cpp; \
	else \
		git -C deps/whisper.cpp pull --ff-only; \
	fi; \
	CMAKE_VER=3.30.5; \
	CMAKE_DIR=deps/tools/cmake-$$CMAKE_VER-macos-universal; \
	CMAKE_TGZ=cmake-$$CMAKE_VER-macos-universal.tar.gz; \
	CMAKE_URL=https://github.com/Kitware/CMake/releases/download/v$$CMAKE_VER/$$CMAKE_TGZ; \
	if [ ! -f deps/tools/$$CMAKE_TGZ ]; then \
		curl -L $$CMAKE_URL -o deps/tools/$$CMAKE_TGZ; \
	fi; \
	if [ ! -d $$CMAKE_DIR ]; then \
		tar -xzf deps/tools/$$CMAKE_TGZ -C deps/tools; \
	fi; \
	CMAKE_BIN=$$(pwd)/$$CMAKE_DIR/CMake.app/Contents/bin/cmake; \
	$$CMAKE_BIN -S deps/whisper.cpp -B deps/whisper.cpp/build -DWHISPER_BUILD_TESTS=OFF; \
	$$CMAKE_BIN --build deps/whisper.cpp/build -j; \
	bash deps/whisper.cpp/models/download-ggml-model.sh base

run: templ-generate tailwind-build
	@test -x "$(WHISPER_CPP_BIN)" || (echo "Missing local whisper binary: $(WHISPER_CPP_BIN). Run: make deps-whisper" && exit 1)
	@test -f "$(WHISPER_MODEL_PATH)" || (echo "Missing local whisper model: $(WHISPER_MODEL_PATH). Run: make deps-whisper" && exit 1)
	$(GO) run ./cmd

admin-create:
	@test -n "$(USER)" || (echo "Usage: make admin-create USER=admin PASS='strong-password'" && exit 1)
	@test -n "$(PASS)" || (echo "Usage: make admin-create USER=admin PASS='strong-password'" && exit 1)
	$(GO) run ./cmd/admin create-user --username "$(USER)" --password "$(PASS)"

run-fresh:
	@env $(UNSET_FLAGS) PATH="$$PATH" HOME="$$HOME" $(MAKE) --no-print-directory run

build: templ-generate tailwind-build
	$(GO) build ./...

tidy:
	$(GO) mod tidy

clean:
	$(GO) clean -cache -testcache -fuzzcache

env:
	@echo "HTTP_ADDR=$(HTTP_ADDR)"
	@echo "SQLITE_PATH=$(SQLITE_PATH)"
	@echo "ARTIFACT_ROOT=$(ARTIFACT_ROOT)"
	@echo "OTEL_ENABLED=$(OTEL_ENABLED)"
	@echo "OTEL_SERVICE_NAME=$(OTEL_SERVICE_NAME)"
	@echo "OTEL_SERVICE_VERSION=$(OTEL_SERVICE_VERSION)"
	@echo "OTEL_ENVIRONMENT=$(OTEL_ENVIRONMENT)"
	@echo "OTEL_EXPORTER_OTLP_ENDPOINT=$(OTEL_EXPORTER_OTLP_ENDPOINT)"
	@echo "OTEL_EXPORTER_OTLP_INSECURE=$(OTEL_EXPORTER_OTLP_INSECURE)"
	@echo "OTEL_FAIL_FAST=$(OTEL_FAIL_FAST)"
	@echo "MAIN_MODEL=$(MAIN_MODEL)"
	@echo "MAIN_MODEL_BASE_URL=$(MAIN_MODEL_BASE_URL)"
	@echo "MAIN_MODEL_TIMEOUT=$(MAIN_MODEL_TIMEOUT)"
	@echo "MAIN_MODEL_MAX_RETRIES=$(MAIN_MODEL_MAX_RETRIES)"
	@echo "EMBEDDING_MODEL=$(EMBEDDING_MODEL)"
	@echo "EMBEDDING_MODEL_BASE_URL=$(EMBEDDING_MODEL_BASE_URL)"
	@echo "EMBEDDING_MODEL_TIMEOUT=$(EMBEDDING_MODEL_TIMEOUT)"
	@echo "EMBEDDING_MODEL_MAX_RETRIES=$(EMBEDDING_MODEL_MAX_RETRIES)"
	@echo "TRANSLATE_MODEL=$(TRANSLATE_MODEL)"
	@echo "TRANSLATE_MODEL_BASE_URL=$(TRANSLATE_MODEL_BASE_URL)"
	@echo "TRANSLATE_MODEL_TIMEOUT=$(TRANSLATE_MODEL_TIMEOUT)"
	@echo "TRANSLATE_MODEL_MAX_RETRIES=$(TRANSLATE_MODEL_MAX_RETRIES)"
	@echo "MODEL_RETRY_BACKOFF=$(MODEL_RETRY_BACKOFF)"
	@echo "FFMPEG_BIN=$(FFMPEG_BIN)"
	@echo "WHISPER_CPP_BIN=$(WHISPER_CPP_BIN)"
	@echo "WHISPER_MODEL_PATH=$(WHISPER_MODEL_PATH)"
	@echo "YTDLP_BIN=$(YTDLP_BIN)"
	@echo "TRANSCRIPT_FALLBACK_PATH=$(TRANSCRIPT_FALLBACK_PATH)"
	@echo "ENABLE_TRANSLATION=$(ENABLE_TRANSLATION)"
	@echo "ADMIN_SESSION_TTL=$(ADMIN_SESSION_TTL)"
	@echo "ADMIN_COOKIE_NAME=$(ADMIN_COOKIE_NAME)"

env-fresh:
	@env $(UNSET_FLAGS) PATH="$$PATH" HOME="$$HOME" $(MAKE) --no-print-directory env

unset-env:
	@echo "unset $(APP_ENV_VARS)"

docker-build:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-build-multiarch:
	./scripts/docker_build_multiarch.sh IMAGE=$(DOCKER_IMAGE) TAG=$(DOCKER_TAG)

docker-build-amd64:
	./scripts/docker_build_multiarch.sh IMAGE=$(DOCKER_IMAGE) TAG=$(DOCKER_TAG) PLATFORMS=linux/amd64 LOAD=1

docker-push-amd64:
	./scripts/docker_build_multiarch.sh IMAGE=$(DOCKER_IMAGE) TAG=$(DOCKER_TAG) PLATFORMS=linux/amd64 PUSH=1

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f mediascribe

docker-rebuild-embeddings:
	docker compose exec mediascribe rebuild-embeddings

docker-verify-deps:
	docker compose exec mediascribe sh -lc 'echo ffmpeg=$$(command -v ffmpeg); ffmpeg -version | head -n1; echo yt-dlp=$$(command -v yt-dlp); yt-dlp --version; echo whisper=$$(command -v /opt/whisper/bin/whisper-cli); /opt/whisper/bin/whisper-cli --help >/dev/null && echo whisper-cli=ok'
