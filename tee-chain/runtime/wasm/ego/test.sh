#!/usr/bin/env bash
# test.sh — 构建并运行 ego SGX enclave（参考 EGo 官方文档：
#           https://docs.edgeless.systems/ego/workflows/build）
#
# 流程：
#   1. (可选) 重新编译 runtime/runtime.wasm
#   2. 拷贝 runtime.wasm.gz 到本目录（enclave.json files 字段的 source）
#      传输量约 .wasm 的一半（gzip 压缩后约 200K），适合链上传输
#   3. ego-go build  编译 enclave
#   4. ego sign ego  签名（同时把 runtime.wasm.gz 嵌入 enclave image）
#   5. ego run ego   运行 enclave（默认 OE_SIMULATION=1，无 SGX 硬件也能跑）
#      enclave 内运行时自动解压 runtime.wasm.gz 给 wasm.Engine
#
# 用法：
#   ./test.sh                  # 用现有 wasm
#   ./test.sh --build          # 先重新编译 wasm 再继续
#   ./test.sh --no-sim         # 关闭仿真模式（要真实 SGX 硬件）

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
RUNTIME_DIR="$SCRIPT_DIR/../runtime"

BUILD=false
SIM=true
while [[ $# -gt 0 ]]; do
    case "$1" in
        --build)   BUILD=true; shift ;;
        --no-sim)  SIM=false; shift ;;
        -h|--help)
            sed -n '2,12p' "$0"
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
    ( cd "$RUNTIME_DIR" && bash build.sh )
fi

# ── 2. 拷贝 runtime.wasm.gz 到本目录 ─────────────────────
echo "=========================================="
echo " 2/5 拷贝 runtime.wasm.gz → ./ ..."
echo "=========================================="
if [[ ! -f "$RUNTIME_DIR/runtime.wasm.gz" ]]; then
    echo "ERROR: wasm.gz not found at $RUNTIME_DIR/runtime.wasm.gz" >&2
    echo "  hint: run with --build" >&2
    exit 1
fi
cp -f "$RUNTIME_DIR/runtime.wasm.gz" "$SCRIPT_DIR/runtime.wasm.gz"
echo "  ✓ $(stat -c%s "$SCRIPT_DIR/runtime.wasm.gz") bytes (gzip 压缩)"

# ── 3. ego-go build ────────────────────────────────
echo ""
echo "=========================================="
echo " 3/5 ego-go build ..."
echo "=========================================="
( cd "$SCRIPT_DIR" && GOCACHE="$HOME/.cache/ego-go-build" ego-go build -o ego . )
echo "  ✓ built: $SCRIPT_DIR/ego"

# ── 4. ego sign ────────────────────────────────────────────
echo ""
echo "=========================================="
echo " 4/5 ego sign ego (将 runtime.wasm.gz 嵌入 enclave) ..."
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
