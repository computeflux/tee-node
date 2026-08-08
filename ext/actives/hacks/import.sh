#!/usr/bin/env bash
set -e

# ext/actives/hacks/import.sh — 导入历史数据
# 用法: ./import.sh <data_file> [db_path]
#   ./import.sh data_files/data.json
#   ./import.sh data_files/data.json ./data

cd "$(dirname "$0")/.."

DATA_FILE="${1:-}"
DB_PATH="${DB_PATH:-./data}"

if [ -z "$DATA_FILE" ]; then
    echo "Usage: ./hacks/import.sh <data_file>"
    echo "  ./hacks/import.sh data_files/data.json"
    echo "  DB_PATH=./mydata ./hacks/import.sh data.json"
    exit 1
fi

echo "[import] Importing: $DATA_FILE (db: $DB_PATH)"
if command -v go >/dev/null 2>&1; then
    go run ./cmd/import_data -data="$DATA_FILE" -db="$DB_PATH"
else
    echo "[import] go not found, using prebuilt binary: build/import_data"
    ./build/import_data -data="$DATA_FILE" -db="$DB_PATH"
fi
