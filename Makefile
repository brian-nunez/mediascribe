SHELL := /bin/zsh

GO ?= go

APP_ENV_VARS := HTTP_ADDR SQLITE_PATH ARTIFACT_ROOT \
	MAIN_MODEL MAIN_MODEL_BASE_URL MAIN_MODEL_TIMEOUT MAIN_MODEL_MAX_RETRIES \
	EMBEDDING_MODEL EMBEDDING_MODEL_BASE_URL EMBEDDING_MODEL_TIMEOUT EMBEDDING_MODEL_MAX_RETRIES \
	TRANSLATE_MODEL TRANSLATE_MODEL_BASE_URL TRANSLATE_MODEL_TIMEOUT TRANSLATE_MODEL_MAX_RETRIES \
	MODEL_RETRY_BACKOFF \
	FFMPEG_BIN WHISPER_CPP_BIN WHISPER_MODEL_PATH YTDLP_BIN \
	TRANSCRIPT_FALLBACK_PATH ENABLE_TRANSLATION

UNSET_FLAGS := $(foreach v,$(APP_ENV_VARS),-u $(v))

# App runtime configuration
export HTTP_ADDR ?= :8080
export SQLITE_PATH ?= ./data/app.db
export ARTIFACT_ROOT ?= ./artifacts/jobs

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

.PHONY: run run-fresh build tidy clean env env-fresh unset-env deps-whisper

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

run:
	@test -x "$(WHISPER_CPP_BIN)" || (echo "Missing local whisper binary: $(WHISPER_CPP_BIN). Run: make deps-whisper" && exit 1)
	@test -f "$(WHISPER_MODEL_PATH)" || (echo "Missing local whisper model: $(WHISPER_MODEL_PATH). Run: make deps-whisper" && exit 1)
	$(GO) run ./cmd/server

run-fresh:
	@env $(UNSET_FLAGS) PATH="$$PATH" HOME="$$HOME" $(MAKE) --no-print-directory run

build:
	$(GO) build ./...

tidy:
	$(GO) mod tidy

clean:
	$(GO) clean -cache -testcache -fuzzcache

env:
	@echo "HTTP_ADDR=$(HTTP_ADDR)"
	@echo "SQLITE_PATH=$(SQLITE_PATH)"
	@echo "ARTIFACT_ROOT=$(ARTIFACT_ROOT)"
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

env-fresh:
	@env $(UNSET_FLAGS) PATH="$$PATH" HOME="$$HOME" $(MAKE) --no-print-directory env

unset-env:
	@echo "unset $(APP_ENV_VARS)"
