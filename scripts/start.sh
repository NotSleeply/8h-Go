#!/usr/bin/env sh
set -eu

echo "[start] starting services..."
docker compose up -d
echo "[start] done. check with: docker compose ps"
