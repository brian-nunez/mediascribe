#!/usr/bin/env bash
set -euo pipefail

# Multi-arch Docker build script for MediaScribe.
# Builds for linux/amd64 and linux/arm64 using docker buildx.
#
# Usage examples:
#   ./scripts/docker_build_multiarch.sh
#   IMAGE=ghcr.io/brian-nunez/mediascribe TAG=v1.0.0 PUSH=1 ./scripts/docker_build_multiarch.sh
#   IMAGE=mediascribe TAG=dev LOAD=1 LOCAL_MULTIARCH=1 ./scripts/docker_build_multiarch.sh

IMAGE="${IMAGE:-mediascribe}"
TAG="${TAG:-latest}"
PLATFORMS="${PLATFORMS:-linux/amd64}"
BUILDER_NAME="${BUILDER_NAME:-mediascribe-multiarch}"
PUSH="${PUSH:-0}"
LOAD="${LOAD:-0}"
LOCAL_MULTIARCH="${LOCAL_MULTIARCH:-0}"
DOCKERFILE="${DOCKERFILE:-Dockerfile}"
CONTEXT_DIR="${CONTEXT_DIR:-.}"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required" >&2
  exit 1
fi

if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required" >&2
  exit 1
fi

if ! docker buildx inspect "$BUILDER_NAME" >/dev/null 2>&1; then
  docker buildx create --name "$BUILDER_NAME" --use
else
  docker buildx use "$BUILDER_NAME"
fi

# Ensure builder is initialized.
docker buildx inspect --bootstrap >/dev/null

has_multi_platforms=0
if [[ "$PLATFORMS" == *","* ]]; then
  has_multi_platforms=1
fi

# Default behavior:
# - Multi-platform builds default to --push (manifest list).
# - Single-platform builds default to --load.
OUTPUT_FLAG=""
if [[ "$PUSH" == "1" ]]; then
  OUTPUT_FLAG="--push"
elif [[ "$LOAD" == "1" ]]; then
  OUTPUT_FLAG="--load"
else
  if [[ "$has_multi_platforms" == "1" ]]; then
    OUTPUT_FLAG="--push"
  else
    OUTPUT_FLAG="--load"
  fi
fi

if [[ "$has_multi_platforms" == "1" && "$OUTPUT_FLAG" == "--load" ]]; then
  if [[ "$LOCAL_MULTIARCH" == "1" ]]; then
    echo "LOCAL_MULTIARCH=1: building each platform separately with --load."
    IFS=',' read -r -a plats <<< "$PLATFORMS"
    for p in "${plats[@]}"; do
      suffix="${p##*/}"
      set -x
      docker buildx build \
        --platform "$p" \
        -f "$DOCKERFILE" \
        -t "$IMAGE:$TAG-$suffix" \
        --load \
        "$CONTEXT_DIR"
      set +x
    done
    echo "Built local images:"
    for p in "${plats[@]}"; do
      suffix="${p##*/}"
      echo "  $IMAGE:$TAG-$suffix"
    done
    exit 0
  fi

  echo "ERROR: --load cannot export multi-arch manifest lists."
  echo "Use one of:"
  echo "  1) PUSH=1 (recommended): publish multi-arch image"
  echo "  2) LOCAL_MULTIARCH=1 LOAD=1: build per-arch local tags"
  exit 1
fi

set -x
docker buildx build \
  --platform "$PLATFORMS" \
  -f "$DOCKERFILE" \
  -t "$IMAGE:$TAG" \
  "$OUTPUT_FLAG" \
  "$CONTEXT_DIR"
set +x

echo "Built image: $IMAGE:$TAG"
echo "Platforms: $PLATFORMS"
