#!/usr/bin/env bash
set -euo pipefail

# ink! ABI → TypeScript 类型生成脚本
#
# 用法:
#   cd tools/ink-abi-ts-gen
#   ./gen.sh <abi.json> [out.ts]
#
# 示例:
#   ./gen.sh ../../tee-chain/abis/gov.json ./gov.ts
#   ./gen.sh ../../tee-chain/abis/gateway.json ./gateway.ts

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 编译 Go 生成器（如未编译或源码已修改）
if [[ ! -f ./ink-abi-ts-gen ]] || [[ ./main.go -nt ./ink-abi-ts-gen ]]; then
  echo "[build] 编译 Go 生成器..."
  go build -o ink-abi-ts-gen ./main.go
fi

# 解析参数
ABI_PATH="${1:-}"
OUT_PATH="${2:-}"

if [[ -z "$ABI_PATH" ]]; then
  echo "错误: 缺少 ABI 文件路径"
  echo ""
  echo "用法: $0 <abi.json> [out.ts]"
  echo ""
  echo "示例:"
  echo "  $0 ../../tee-chain/abis/gov.json     ./gov.ts"
  echo "  $0 ../../tee-chain/abis/gateway.json ./gateway.ts"
  exit 1
fi

if [[ -z "$OUT_PATH" ]]; then
  base=$(basename "$ABI_PATH" .json)
  OUT_PATH="./${base}.ts"
fi

echo "[gen] $ABI_PATH → $OUT_PATH"
./ink-abi-ts-gen "$ABI_PATH" "$OUT_PATH"
echo "[done] 生成完成: $OUT_PATH"
