#!/usr/bin/env bash
set -e

# ext/actives/init.sh — 初始化开发环境
# 用法: ./init.sh

cd "$(dirname "$0")/.."

DB_PATH="${DB_PATH:-./data}"

echo "[init] Creating data directory: $DB_PATH ..."
mkdir -p "$DB_PATH"

echo "[init] Running gqlgen generate..."
gqlgen generate --config gql/gqlgen.yml

echo "[init] Done."
echo ""
echo "  Hacks:"
echo "    ./hacks/dev.sh               Start dev server on :9100"
echo "    ./hacks/dev.sh 9200          Start dev server on :9200"
echo "    ./hacks/import.sh data.json  Import historical data"
echo ""
echo "  Go run:"
echo "    go run ./cmd/mint -to=0x... -amount=1000   Mint FLUX_TEST tokens"
