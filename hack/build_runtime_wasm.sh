#!/bin/bash
# build_runtime_wasm.sh - 使用 TinyGo 构建运行时 WASM 文件
#
#   - 使用 -buildmode=c-shared (reactor 模式)
#   - 导出 execute(), get_buffer(), ping()
#   - 无 _start 依赖，可被引擎多次实例化
#
# 需要安装 tinygo: https://tinygo.org/getting-started/install/
#
# 用法:
#   ./hack/build_runtime_wasm.sh [output_path]
#   默认: output=./tee-chain/contracts/wasm/runtime/runtime.wasm

set -e

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR/.."

OUTPUT="${1:-./tee-chain/contracts/wasm/runtime/runtime.wasm}"

echo "Building runtime WASM with TinyGo..."
echo "  Source: ./tee-chain/contracts/wasm/runtime/"
echo "  Output: $OUTPUT"

tinygo build -buildmode=c-shared -target=wasip1 -o "$OUTPUT" ./tee-chain/contracts/wasm/runtime/

echo "Done: $(wc -c < "$OUTPUT") bytes"
