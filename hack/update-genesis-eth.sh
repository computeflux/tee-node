#!/bin/bash
set -e

# ── 将 ETH 地址转换为 UniAddr 并更新创世文件 sudo 字段 ──
# 用法:
#   ./scripts/update-genesis-eth.sh 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B
#   ./scripts/update-genesis-eth.sh 0xADDR1 --nodes /srv/token_node0 /srv/token_node1
#   ./scripts/update-genesis-eth.sh --addresses addr.txt --nodes /srv/token_node0

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
ETH2UNIADDR_DIR="$PROJECT_DIR/ext/eth2uniaddr"

# ── 解析参数 ──
ADDRESSES=()
NODES=()
while [[ $# -gt 0 ]]; do
    case "$1" in
        --addresses|-a)
            # 从文件读取地址（每行一个）
            if [[ -f "$2" ]]; then
                while IFS= read -r line || [[ -n "$line" ]]; do
                    line="$(echo "$line" | sed 's/#.*//' | xargs)"
                    [[ -n "$line" ]] && ADDRESSES+=("$line")
                done < "$2"
            else
                echo "ERROR: 文件不存在: $2"
                exit 1
            fi
            shift 2
            ;;
        --nodes|-n)
            shift
            while [[ $# -gt 0 && ! "$1" =~ ^-- ]]; do
                NODES+=("$1")
                shift
            done
            ;;
        --help|-h)
            echo "用法: $0 [选项] [ETH地址...]"
            echo ""
            echo "选项:"
            echo "  -a, --addresses FILE    从文件读取 ETH 地址（每行一个）"
            echo "  -n, --nodes DIR...      指定节点目录列表"
            echo "  -h, --help             显示帮助信息"
            echo ""
            echo "示例:"
            echo "  $0 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"
            echo "  $0 -a addresses.txt -n /srv/token_node0 /srv/token_node1"
            exit 0
            ;;
        *)
            ADDRESSES+=("$1")
            shift
            ;;
    esac
done

# ── 默认节点目录 ──
if [[ ${#NODES[@]} -eq 0 ]]; then
    NODES=(/srv/token_node0 /srv/token_node1 /srv/token_node2)
fi

# ── 校验 ──
if [[ ${#ADDRESSES[@]} -eq 0 ]]; then
    echo "ERROR: 至少需要一个 ETH 地址"
    echo "用法: $0 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B"
    echo "      $0 --addresses addrs.txt"
    exit 1
fi

echo "========================================"
echo " 0/5 ETH → UniAddr 转换 + 创世更新"
echo "========================================"
echo "ETH 地址数量: ${#ADDRESSES[@]}"
echo "节点目录:"
for node in "${NODES[@]}"; do
    echo "  - $node"
done
echo ""

# ── 1/5 编译 / 运行 eth2uniaddr ──
echo "========================================"
echo " 1/5 转换 ETH → UniAddr ..."
echo "========================================"
cd "$PROJECT_DIR"

# 如果存在预编译的二进制，直接使用；否则 go run
ETH2UNI_BIN="$PROJECT_DIR/hack/build/eth2uniaddr"
if [[ -x "$ETH2UNI_BIN" ]]; then
    echo "使用预编译: $ETH2UNI_BIN"
else
    echo "编译 eth2uniaddr (首次较慢，请耐心等待)..."
    mkdir -p "$PROJECT_DIR/hack/build"
    go build -o "$ETH2UNI_BIN" "$ETH2UNIADDR_DIR"
    echo "编译完成: $ETH2UNI_BIN"
fi

UNIADDRS=()
for i in "${!ADDRESSES[@]}"; do
    addr="${ADDRESSES[$i]}"
    uniaddr=$("$ETH2UNI_BIN" "$addr")
    UNIADDRS[$i]="$uniaddr"
    echo "  [$((i+1))] $addr → $uniaddr"
done

# ── 多地址时选择哪个写入 sudo ──
if [[ ${#UNIADDRS[@]} -gt 1 ]]; then
    SUDO_ADDR="${UNIADDRS[0]}"
    echo ""
    echo "WARNING: 提供了 ${#UNIADDRS[@]} 个地址，默认将第一个作为 sudo 写入:"
    echo "  sudoaddr = ${ADDRESSES[0]} → $SUDO_ADDR"
else
    SUDO_ADDR="${UNIADDRS[0]}"
fi

sudo_addr_eth="${ADDRESSES[0]}"

echo ""
echo "========================================"
echo " 2/5 检查 UniAddr 格式 ..."
echo "========================================"
echo "UniAddr hex: $SUDO_ADDR"
echo "长度: $(echo -n "$SUDO_ADDR" | wc -c) chars"

echo ""
echo "========================================"
echo " 3/5 更新创世文件 app_state.sudo ..."
echo "========================================"
for node in "${NODES[@]}"; do
    GENESIS="$node/token_chain_data/config/genesis.json"

    if [[ ! -f "$GENESIS" ]]; then
        echo "SKIP: $GENESIS 不存在"
        continue
    fi

    # 更新 genesis.json 中的 sudo 字段
    SUDO_ADDR="$SUDO_ADDR" GENESIS_PATH="$GENESIS" python3 -c '
import json, os
sudo_addr = os.environ["SUDO_ADDR"]
genesis_path = os.environ["GENESIS_PATH"]

with open(genesis_path, "r") as f:
    data = json.load(f)

data["app_state"]["sudo"] = sudo_addr

with open(genesis_path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
'
    echo "  updated: $GENESIS → app_state.sudo = $SUDO_ADDR"
done

echo ""
echo "========================================"
echo " 4/5 校验 ..."
echo "========================================"
for node in "${NODES[@]}"; do
    GENESIS="$node/token_chain_data/config/genesis.json"
    if [[ -f "$GENESIS" ]]; then
        sudo_val=$(python3 -c "
import json
with open('$GENESIS') as f:
    d = json.load(f)
print(d.get('app_state', {}).get('sudo', 'NOT_SET'))
")
        if [[ "$sudo_val" == "$SUDO_ADDR" ]]; then
            echo "  $node: ✅ sudo = $SUDO_ADDR"
        else
            echo "  $node: ❌ sudo = $sudo_val (期望 $SUDO_ADDR)"
        fi
    fi
done

echo ""
echo "========================================"
echo " 5/5 导出地址映射（方便查阅） ..."
echo "========================================"
echo "ETH → UniAddr 映射表:"
for i in "${!ADDRESSES[@]}"; do
    printf "  %-45s → %s\n" "${ADDRESSES[$i]}" "${UNIADDRS[$i]}"
done
echo ""
echo "sudo 地址:"
echo "  ETH:     ${ADDRESSES[0]}"
echo "  UniAddr: $SUDO_ADDR"

echo ""
echo "全部完成！"
