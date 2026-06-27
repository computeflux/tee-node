#!/bin/bash
set -e

# ── 将 Validator (Address + PubKey) 写入创世文件 validators 数组 ──
# 用法:
#   ./hack/update-genesis-validator.sh <ADDRESS> <PUBKEY>
#   ./hack/update-genesis-validator.sh <ADDRESS> <PUBKEY> --power 10 --name "node0"
#   ./hack/update-genesis-validator.sh --nodes /srv/token_node0 <ADDRESS> <PUBKEY>

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# ── 解析参数 ──
ADDRESS=""
PUBKEY=""
POWER="10"
NAME=""
NODES=()

while [[ $# -gt 0 ]]; do
    case "$1" in
        --power)
            POWER="$2"; shift 2 ;;
        --name)
            NAME="$2"; shift 2 ;;
        --nodes|-n)
            shift
            while [[ $# -gt 0 && ! "$1" =~ ^-- ]]; do
                NODES+=("$1"); shift
            done
            ;;
        --help|-h)
            echo "用法: $0 [选项] <ADDRESS> <PUBKEY>"
            echo ""
            echo "选项:"
            echo "  --power N          validator power (默认 10)"
            echo "  --name NAME        validator 名称 (默认空)"
            echo "  -n, --nodes DIR... 指定节点目录列表"
            echo ""
            echo "示例:"
            echo "  $0 79F5D697DB70EA1C76A5C1ED988A8A3D033535D7 XiBiWNpiWaxoFSGHdXeQIR9uzqK1QNZd0pGhMQJofRw="
            exit 0
            ;;
        *)
            if [[ -z "$ADDRESS" ]]; then
                ADDRESS="$1"
            elif [[ -z "$PUBKEY" ]]; then
                PUBKEY="$1"
            else
                echo "ERROR: 多余参数: $1"
                exit 1
            fi
            shift
            ;;
    esac
done

# ── 默认节点目录 ──
if [[ ${#NODES[@]} -eq 0 ]]; then
    NODES=(/srv/token_node0 /srv/token_node1 /srv/token_node2)
fi

# ── 校验 ──
if [[ -z "$ADDRESS" || -z "$PUBKEY" ]]; then
    echo "ERROR: 缺少 Address 或 PubKey"
    echo "用法: $0 <ADDRESS> <PUBKEY>"
    exit 1
fi

echo "========================================"
echo " 更新创世文件 validators"
echo "========================================"
echo "  Address : $ADDRESS"
echo "  PubKey  : $PUBKEY"
echo "  Power   : $POWER"
echo "  Name    : $NAME"
echo "  节点目录:"
for node in "${NODES[@]}"; do
    echo "    - $node"
done
echo ""

for node in "${NODES[@]}"; do
    GENESIS="$node/token_chain_data/config/genesis.json"

    if [[ ! -f "$GENESIS" ]]; then
        echo "SKIP: $GENESIS 不存在"
        continue
    fi

    ADDRESS="$ADDRESS" PUBKEY="$PUBKEY" POWER="$POWER" NAME="$NAME" GENESIS_PATH="$GENESIS" python3 -c '
import json, os
addr  = os.environ["ADDRESS"]
pub   = os.environ["PUBKEY"]
power = os.environ["POWER"]
name  = os.environ["NAME"]

with open(os.environ["GENESIS_PATH"], "r") as f:
    data = json.load(f)

entry = {
    "address": addr,
    "pub_key": {"type": "tendermint/PubKeyEd25519", "value": pub},
    "power":   power,
    "name":    name,
}

# 替换现有 validators（单节点创世）
data["validators"] = [entry]

with open(os.environ["GENESIS_PATH"], "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
'
    echo "  updated: $GENESIS"
done

echo ""
echo "全部完成！"
