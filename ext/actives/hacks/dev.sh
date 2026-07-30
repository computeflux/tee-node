#!/usr/bin/env bash
set -e

# ext/actives/dev.sh — 本地开发启动
# 用法: ./dev.sh [port]
#   ./dev.sh              默认 :9100
#   ./dev.sh 9200         指定端口

cd "$(dirname "$0")/.."

DB_PATH="${DB_PATH:-./data}"

PORT="${1:-9100}"

if [ ! -f "models_gen.go" ] || [ ! -f "generated.go" ]; then
    echo "[dev] Running gqlgen generate first..."
    go run github.com/99designs/gqlgen generate --config gql/gqlgen.yml
fi

echo "[dev] Starting server on :$PORT ..."
echo "[dev] CHECKIN_TEST_MODE=true (test mode enabled)"

export CHECKIN_TEST_MODE=true
go run ./cmd/server -port="$PORT" -db="$DB_PATH"
