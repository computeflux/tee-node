#!/bin/bash
# generate_codec.sh - 为所有合约模块生成 SCALE codec
#
# 用法:
#   ./hack/generate_codec.sh          # 生成所有模块的 codec
#   ./hack/generate_codec.sh store    # 仅生成 store 包的 codec

set -e

DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$DIR/.."

GEN="go run ./tools/scale-codec-gen"

# 需要生成 codec 的目录列表
DIRS=(
    "./tee-chain/contracts/store"
    "./tee-chain/contracts/module/gov"
    "./tee-chain/contracts/module/auth"
    "./tee-chain/contracts/module/gateway"
    "./tee-chain/contracts/module/batch"
)

if [ $# -gt 0 ]; then
    # 指定目录
    case "$1" in
        store)   DIRS=("./tee-chain/contracts/store") ;;
        gov)     DIRS=("./tee-chain/contracts/module/gov") ;;
        auth)    DIRS=("./tee-chain/contracts/module/auth") ;;
        gateway) DIRS=("./tee-chain/contracts/module/gateway") ;;
        batch)   DIRS=("./tee-chain/contracts/module/batch") ;;
        all)     ;; # 使用默认全部
        *)       echo "unknown: $1 (use: store gov auth gateway batch all)"; exit 1 ;;
    esac
fi

echo ">>> Generating SCALE codec ..."

for d in "${DIRS[@]}"; do
    echo "  $d"
    $GEN "$d"
done

echo ">>> Fixing self-import in store/codec_gen.go ..."
# store 包的 codec_gen.go 会错误地 import 自身，需要清理
STORE_GEN="tee-chain/contracts/store/codec_gen.go"
if [ -f "$STORE_GEN" ]; then
    # 删除 self-import 行
    sed -i '/"github.com\/computeflux\/tee-provider\/tee-chain\/contracts\/store"/d' "$STORE_GEN"
    # 删除 store. 前缀
    sed -i 's/store\.EncodeCompact/EncodeCompact/g; s/store\.DecodeCompact/DecodeCompact/g' "$STORE_GEN"
    echo "  fixed $STORE_GEN"
fi

echo ">>> Done."
