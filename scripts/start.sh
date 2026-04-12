#!/usr/bin/env sh
set -eu

if [ ! -f ".env" ]; then
  cp .env.example .env
  echo "[start] .env not found, created from .env.example"
fi

echo "[start] starting services..."
docker compose up -d --build
echo "[start] done. check with: docker compose ps"
