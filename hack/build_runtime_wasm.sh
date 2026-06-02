#!/bin/bash
# build_runtime_wasm.sh - 构建运行时 WASM 文件
#
# 由于当前 Go 合约依赖 PebbleDB(CGO)，无法直接用 GOOS=wasip1 GOARCH=wasm 编译。
# 此脚本使用 tools/buildwasm 生成一个包含版本元数据的有效 WASM 模块：
#   - 导出函数 version() -> i32（可被 wasmtime 加载执行）
#   - custom section "runtime_meta"（源码 hash、ABI hash、描述等）
#
# 用法:
#   ./hack/build_runtime_wasm.sh [version] [output_path]
#   默认: version=0, output=./token_chain_data/runtime_v0.wasm

set -e

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR/.."

VERSION="${1:-0}"
OUTPUT="${2:-./token_chain_data/runtime_v${VERSION}.wasm}"
DESCRIPTION="runtime v${VERSION}"

echo "Building runtime WASM..."
echo "  Version:     $VERSION"
echo "  Contracts:   ./tee-chain/contracts"
echo "  ABIs:        ./tee-chain/abis"
echo "  Output:      $OUTPUT"

go run tools/buildwasm/main.go \
    -version "$VERSION" \
    -contracts ./tee-chain/contracts \
    -abis ./tee-chain/abis \
    -out "$OUTPUT" \
    -description "$DESCRIPTION"

echo "Done."
