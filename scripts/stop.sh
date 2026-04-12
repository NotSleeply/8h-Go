#!/usr/bin/env sh
set -eu

echo "[stop] stopping services..."
docker compose down
echo "[stop] done."
