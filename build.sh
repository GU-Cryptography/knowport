#!/usr/bin/env bash
set -euo pipefail

# F4: One-shot local build for production compose stack
# Injects BUILD_TIME and GIT_COMMIT into docker build args (cache busting + future metadata)

BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo unknown)
export BUILD_TIME GIT_COMMIT

echo "================================================"
echo " AI LocalBase - Production Image Build"
echo "================================================"
echo " BUILD_TIME : ${BUILD_TIME}"
echo " GIT_COMMIT : ${GIT_COMMIT}"
echo "================================================"

docker compose build "$@"

echo ""
echo "Build complete. Images:"
docker images --format "  {{.Repository}}:{{.Tag}}  ({{.Size}})" | grep -E "(knowport|qdrant)" || true
echo ""
echo "Next: docker compose up -d"
