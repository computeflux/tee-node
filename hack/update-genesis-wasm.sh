#!/bin/bash
set -e

# ── 编译最新 WASM + 更新创世文件 ──
# 用法:
#   ./scripts/update-genesis-wasm.sh              # 默认更新所有 /srv/token_node*
#   ./scripts/update-genesis-wasm.sh /path/a /path/b   # 指定节点目录

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
RUNTIME_DIR="$PROJECT_DIR/tee-chain/runtime/wasm/runtime"

# 节点目录列表
if [ $# -eq 0 ]; then
    NODES=(/srv/token_node0 /srv/token_node1 /srv/token_node2)
else
    NODES=("$@")
fi

echo "========================================"
echo " 1/4 编译 WASM (TinyGo) ..."
echo "========================================"
cd "$RUNTIME_DIR"
bash build.sh

WASM_FILE="$RUNTIME_DIR/runtime.wasm"
WASM_GZ_FILE="$RUNTIME_DIR/runtime.wasm.gz"
if [ ! -f "$WASM_FILE" ]; then
    echo "ERROR: build failed, runtime.wasm not found"
    exit 1
fi
echo "WASM: $(ls -lh "$WASM_FILE" | awk '{print $5}')"
echo "GZIP: $(ls -lh "$WASM_GZ_FILE" | awk '{print $5}')"

echo ""
echo "========================================"
echo " 2/4 Base64 编码 ..."
echo "========================================"
WASM_B64=$(base64 -w0 "$WASM_FILE")
echo "Base64 长度: $(echo -n "$WASM_B64" | wc -c)"

echo ""
echo "========================================"
echo " 3/4 更新创世文件 ..."
echo "========================================"
for node in "${NODES[@]}"; do
    GENESIS="$node/token_chain_data/config/genesis.json"
    DATA_DIR="$node/token_chain_data"

    if [ ! -f "$GENESIS" ]; then
        echo "SKIP: $GENESIS 不存在"
        continue
    fi

    # 更新 genesis.json 中的 wasm 字段
    echo -n "$WASM_B64" > /tmp/wasm_b64.txt
    GENESIS_PATH="$GENESIS" python3 -c '
import json, os
genesis_path = os.environ["GENESIS_PATH"]
with open("/tmp/wasm_b64.txt", "r") as f:
    wasm_b64 = f.read().strip()
with open(genesis_path, "r") as f:
    data = json.load(f)
data["app_state"]["runtime"]["wasm"] = wasm_b64
with open(genesis_path, "w") as f:
    json.dump(data, f, indent=2)
    f.write("\n")
'
    rm -f /tmp/wasm_b64.txt
    echo "  genesis: $GENESIS"

    # 复制 WASM 文件到数据目录
    mkdir -p "$DATA_DIR"
    cp "$WASM_FILE" "$DATA_DIR/runtime.wasm"
    cp "$WASM_GZ_FILE" "$DATA_DIR/runtime.wasm.gz"
    echo "  wasm:    $DATA_DIR/runtime.wasm"
    echo "  gzip:    $DATA_DIR/runtime.wasm.gz"
done

echo ""
echo "========================================"
echo " 4/4 校验 ..."
echo "========================================"
echo "参考: $(md5sum "$WASM_FILE" | cut -d' ' -f1)  $WASM_FILE"
for node in "${NODES[@]}"; do
    f="$node/token_chain_data/runtime.wasm"
    if [ -f "$f" ]; then
        echo "$node: $(md5sum "$f" | cut -d' ' -f1)"
    fi
done
echo ""
echo "全部完成！"
