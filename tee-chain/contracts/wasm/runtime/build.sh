#!/bin/bash
set -e
cd "$(dirname "$0")"

echo ">>> 编译 wasm (GOOS=wasip1 GOARCH=wasm) ..."
GOOS=wasip1 GOARCH=wasm go build -ldflags="-s -w" -o runtime.wasm ./

# echo ">>> wasm-opt 优化 ..."
# wasm-opt -Oz --enable-bulk-memory --enable-nontrapping-float-to-int \
#     runtime.wasm -o runtime.wasm.tmp 2>&1
# mv runtime.wasm.tmp runtime.wasm

echo ">>> 完成 $(ls -lh runtime.wasm | awk '{print $5}')"
