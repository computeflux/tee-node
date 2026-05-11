#!/bin/bash
# 基于已有 _chain_init 的 genesis.json 生成新节点配置
# 新节点使用相同的创世区块，拥有独立的 node_key 和 priv_validator_key

set -e

SOURCE="$0"
while [ -h "$SOURCE" ]; do
    DIR="$(cd -P "$(dirname "$SOURCE")" && pwd)"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
SCRIPT_DIR="$(cd -P "$(dirname "$SOURCE")" && pwd)"

# 执行脚本时的当前目录
INVOKE_DIR=$(pwd)
CHAIN_INIT="${INVOKE_DIR}/_chain_init"
GENESIS_SRC="${CHAIN_INIT}/config/genesis.json"
NODE_OUTPUT="${1:-${INVOKE_DIR}/_chain_node}"

# 查找 cometbft
COMETBFT=""
if cand=$(command -v cometbft 2>/dev/null) && [ -x "$cand" ]; then
    COMETBFT="$cand"
elif [ -x "$HOME/go/bin/cometbft" ]; then
    COMETBFT="$HOME/go/bin/cometbft"
elif [ -x "/usr/local/bin/cometbft" ]; then
    COMETBFT="/usr/local/bin/cometbft"
elif [ -x "/usr/bin/cometbft" ]; then
    COMETBFT="/usr/bin/cometbft"
fi
if [ -z "$COMETBFT" ]; then
    echo "错误: 未找到 cometbft 命令"
    exit 1
fi

# 检查创世配置是否存在
if [ ! -f "$GENESIS_SRC" ]; then
    echo "错误: 未找到创世配置 ${GENESIS_SRC}"
    echo "请先运行 chain_init.sh 生成 _chain_init"
    exit 1
fi

echo "=== 生成新节点配置 ==="
echo "创世区块: ${GENESIS_SRC}"
echo "输出目录: ${NODE_OUTPUT}"
echo ""

# 初始化新节点（生成 node_key、priv_validator_key 等）
echo "生成节点密钥..."
rm -rf "$NODE_OUTPUT"
"$COMETBFT" init --home "$NODE_OUTPUT"

# 使用 _chain_init 的 genesis.json 覆盖
echo "应用创世区块..."
cp -f "$GENESIS_SRC" "${NODE_OUTPUT}/config/genesis.json"

echo ""
echo "=== 完成 ==="
echo "已生成 ${NODE_OUTPUT}/"
echo "  config/genesis.json     (来自 _chain_init)"
echo "  config/node_key.json"
echo "  config/priv_validator_key.json"
echo "  data/priv_validator_state.json"
