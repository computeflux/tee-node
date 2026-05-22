#!/usr/bin/env bash
set -euo pipefail

# 获取脚本所在目录
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GEN_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ABI_DIR="$(cd "$GEN_DIR/../../tee-chain/abis" && pwd)"

echo "=== ink! ABI TypeScript 生成器示例 ==="
echo ""

# 1. 编译生成器
echo "[1/3] 编译 Go 生成器..."
cd "$GEN_DIR"
go build -o ink-abi-ts-gen ./main.go

# 2. 生成 gov.ts bindings
echo "[2/3] 生成 gov.ts 类型文件..."
"$GEN_DIR/ink-abi-ts-gen" "$ABI_DIR/gov.json" "$SCRIPT_DIR/gov.ts"
echo "       → $SCRIPT_DIR/gov.ts"

# 3. 生成 gateway.ts bindings（备用）
echo "[3/3] 生成 gateway.ts 类型文件..."
"$GEN_DIR/ink-abi-ts-gen" "$ABI_DIR/gateway.json" "$SCRIPT_DIR/gateway.ts"
echo "       → $SCRIPT_DIR/gateway.ts"

echo ""
echo "=== 生成完成 ==="
echo ""
echo "查看调用示例:"
echo "  cat $SCRIPT_DIR/gov_example.ts"
echo ""
echo "运行示例（需要安装 @polkadot/api-contract）:"
echo "  cd $SCRIPT_DIR"
echo "  npm i @polkadot/api-contract @polkadot/types-codec"
echo "  npx ts-node gov_example.ts"
