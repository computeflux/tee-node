#!/usr/bin/env bash
set -e

# ext/actives/dev.sh — 本地开发启动
# 用法: ./dev.sh [port] [data_file]
#   ./dev.sh              默认 :9100
#   ./dev.sh 9200         指定端口
#   ./dev.sh 9100 data_files/data.json  导入数据后启动

cd "$(dirname "$0")/.."

PORT="${1:-9100}"
DATA_FILE="${2:-}"

if [ ! -f "models_gen.go" ] || [ ! -f "generated.go" ]; then
    echo "[dev] Running gqlgen generate first..."
    go run github.com/99designs/gqlgen generate --config gql/gqlgen.yml
fi

echo "[dev] Starting server on :$PORT ..."
echo "[dev] CHECKIN_TEST_MODE=true (test mode enabled)"

export CHECKIN_TEST_MODE=true

if [ -n "$DATA_FILE" ]; then
    go run ./cmd/server -port="$PORT" -data="$DATA_FILE" -db="./data"
else
    go run ./cmd/server -port="$PORT" -db="./data"
fi
