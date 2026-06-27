#!/usr/bin/env bash
# test.sh — 构建并运行 ego SGX enclave（参考 EGo 官方文档：
#           https://docs.edgeless.systems/ego/workflows/build）
#
# 流程：
#   1. (可选) 重新编译 runtime/runtime.wasm
#   2. 拷贝 runtime.wasm 到本目录（enclave.json files 字段的 source）
#   3. ego-go build  编译 enclave
#   4. ego sign ego  签名（同时把 runtime.wasm 嵌入 enclave image）
#   5. ego run ego   运行 enclave（默认 OE_SIMULATION=1，无 SGX 硬件也能跑）
#
# 用法：
#   hack/... 或者 ./test.sh                  # 不重新编译 wasm
#   ./test.sh --build                        # 先重新编译 wasm 再继续
#   ./test.sh --no-sim                       # 关闭仿真模式（要真实 SGX 硬件）

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_DIR="$SCRIPT_DIR/../runtime"

BUILD=false
SIM=true
DEBUG=false
while [[ $# -gt 0 ]]; do
    case "$1" in
        --build)   BUILD=true; shift ;;
        --no-sim)  SIM=false; shift ;;
        --debug)   DEBUG=true; shift ;;
        -h|--help)
            sed -n '2,12p' "$0"
            cat <<'USAGE'

调试选项：
  --build        先重新编译 wasm（同时编译 prod 和 debug 两个版本）
  --debug        使用 runtime-debug.wasm（-panic=print，保留 panic 信息）
                 如果没加 --build，会跳过 prod 只编译 debug
  --no-sim       关闭 OE_SIMULATION（需要真实 SGX 硬件）
USAGE
            exit 0
            ;;
        *) echo "Unknown arg: $1" >&2; exit 1 ;;
    esac
done

# ── 前置：EGo 工具链 ────────────────────────────────────────
for tool in ego-go ego; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        echo "ERROR: '$tool' not found in PATH" >&2
        echo "  install: https://docs.edgeless.systems/ego/getting-started/install" >&2
        exit 1
    fi
done

# ── 1. (可选) 编译 wasm ────────────────────────────────
if $BUILD; then
    echo "=========================================="
    echo " 1/5 编译 runtime.wasm (TinyGo) ..."
    echo "=========================================="
    if [[ ! -f "$RUNTIME_DIR/build.sh" ]]; then
        echo "ERROR: build.sh not found at $RUNTIME_DIR/build.sh" >&2
        exit 1
    fi
    if $DEBUG; then
        ( cd "$RUNTIME_DIR" && bash build.sh debug )
    else
        ( cd "$RUNTIME_DIR" && bash build.sh )
    fi
fi

# ── 2. 拷贝 wasm 到本目录（enclave.json files.source）────
echo "=========================================="
echo " 2/5 拷贝 wasm → ./ ..."
echo "=========================================="
WASM_SRC_NAME="runtime.wasm"
if $DEBUG; then
    WASM_SRC_NAME="runtime-debug.wasm"
fi
WASM_SRC="$RUNTIME_DIR/$WASM_SRC_NAME"
if [[ ! -f "$WASM_SRC" ]]; then
    echo "ERROR: wasm not found at $WASM_SRC" >&2
    echo "  hint: run with --build" >&2
    exit 1
fi
cp -f "$WASM_SRC" "$SCRIPT_DIR/runtime.wasm"
echo "  ✓ $(stat -c%s "$SCRIPT_DIR/runtime.wasm") bytes (from $WASM_SRC_NAME)"

# ── 3. ego-go build ──────────────────────────────────────
echo ""
echo "=========================================="
echo " 3/5 ego-go build ..."
echo "=========================================="
( cd "$SCRIPT_DIR" && ego-go build -o ego . )
echo "  ✓ built: $SCRIPT_DIR/ego"

# ── 4. ego sign ────────────────────────────────────────────
echo ""
echo "=========================================="
echo " 4/5 ego sign ego (将 runtime.wasm 嵌入 enclave) ..."
echo "=========================================="
( cd "$SCRIPT_DIR" && ego sign ego )
echo "  ✓ signed"

# ── 5. ego run ─────────────────────────────────────────────
echo ""
echo "=========================================="
echo " 5/5 ego run ego ..."
echo "=========================================="
if $SIM; then
    echo "  (OE_SIMULATION=1 — 无需真实 SGX 硬件)"
    OE_SIMULATION=1 ego run ego
else
    ego run ego
fi
