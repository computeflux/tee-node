#!/usr/bin/env bash
set -e

# ext/actives/init.sh — 初始化开发环境
# 用法: ./init.sh

cd "$(dirname "$0")/.."

echo "[init] Creating data directory..."
mkdir -p data

echo "[init] Running gqlgen generate..."
go run github.com/99designs/gqlgen generate --config gql/gqlgen.yml

echo "[init] Done."
echo ""
echo "  Next steps:"
echo "    ./dev.sh              Start dev server on :9100"
echo "    ./dev.sh 9200         Start dev server on :9200"
echo "    ./init.sh -import=file.json   Import data then start"
