#!/bin/bash
# 区块链创世区块初始化脚本
# 生成 node_key.json、priv_validator_key.json、priv_validator_state.json、genesis.json 等创世节点文件
# 基于 CometBFT (Tendermint) 链

set -e

# 获取脚本所在目录
SOURCE="$0"
while [ -h "$SOURCE" ]; do
    DIR="$(cd -P "$(dirname "$SOURCE")" && pwd)"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
SCRIPT_DIR="$(cd -P "$(dirname "$SOURCE")" && pwd)"
# 执行脚本时的当前目录（非脚本所在目录）
INVOKE_DIR=$(pwd)
CHAIN_DATA="${INVOKE_DIR}/_chain_init"

# 查找 cometbft (PATH、go/bin、常见安装路径)
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
    echo "错误: 未找到 cometbft 命令，请先安装 CometBFT"
    echo "  go install github.com/cometbft/cometbft/cmd/cometbft@latest"
    exit 1
fi

echo "=== CometBFT 区块链创世初始化 ==="
echo ""

echo "生成创世区块和节点密钥..."
"$COMETBFT" init --home "$CHAIN_DATA"

echo ""
echo "=== 初始化完成 ==="
echo "已生成 ${CHAIN_DATA}/config/{genesis.json,node_key.json,priv_validator_key.json}"
echo "       ${CHAIN_DATA}/data/priv_validator_state.json"
echo ""
echo "chain_id 可在 genesis.json 中查看"
