#!/usr/bin/env bash
# update_runtime.sh — 通过 GraphQL 提交 gov.RuntimeUpgrade 交易到 TEE 链。
#
# 用法:
#   hack/update_runtime.sh [--no-build] [--gql URL] [--version-src PATH] \
#                          [--wasm PATH] [--desc TEXT] [--no-wait]
#
# 默认上传 runtime.wasm.gz（链端 engine.go:decompressWasm 按 magic 自动解压），
# 比 .wasm 体积约小一半，绕开 CometBFT 默认 max_tx_bytes=1MB 限制。
# --wasm foo.wasm 时若存在 foo.wasm.gz 会自动改用 gz 版本。
#
# version 从 tee-chain/runtime/module/call.go 的 const NativeVersion 读，
# 工具会自动 GQL 查链上当前 version 并校验一致性（防止传错/编错）。
#
# 工作流（先编译后上传）:
#   1. 编译 ext/runtimeupgrade Go 工具（先编译，fail-fast）
#   2. 编译 runtime.wasm（默认；--no-build 跳过）
#   3. 校验参数 + wasm 文件（>900KB gz 体积预警；链上加载时自动解压）
#   4. 上传（运行工具签名 + GQL contractCall mutation），等终态
#
# 配置来源（高→低优先级）:
#   1. 真实环境变量  SUDO_MNEMONIC / TEE_NODE_GQL
#   2. .env 文件     hack/.env  （或 $ENV_FILE / ./.env / ~/.tee-node.env）
#   3. 命令行参数   --version-src / --wasm / --desc / --gql / ...
#   4. 脚本默认值
#
# ⚠️ 安全提示 ⚠️
#   - SUDO_MNEMONIC 是 sudo 账户私钥种子，拥有全链 sudo 权限
#   - 推荐用 hack/.env（chmod 600）保存，不要直接 export 到 shell history
#   - 模板见 hack/.env.example
#   - 提交走 GQL /gql（不是 CometBFT RPC），所以 --gql 是必须的

set -euo pipefail

# ── 加载 .env ─────────────────────────────────────────────
_SAVED_SUDO_MNEMONIC="${SUDO_MNEMONIC:-}"
_SAVED_TEE_NODE_GQL="${TEE_NODE_GQL:-}"
_SAVED_SKIP_TX_SIZE_CHECK="${SKIP_TX_SIZE_CHECK:-}"

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"

load_env_file() {
    local env_file="${ENV_FILE:-}"
    if [[ -z "$env_file" ]]; then
        for candidate in \
            "$SCRIPT_DIR/.env" \
            "$PROJECT_DIR/.env" \
            "$PWD/.env" \
            "$HOME/.tee-node.env"; do
            if [[ -f "$candidate" ]]; then
                env_file="$candidate"
                break
            fi
        done
    fi
    if [[ -n "$env_file" && -f "$env_file" ]]; then
        set -a
        # shellcheck disable=SC1090
        . "$env_file"
        set +a
        echo "loaded env: $env_file (chmod 600 required)"
    fi
}

load_env_file

[[ -n "$_SAVED_SUDO_MNEMONIC"      ]] && SUDO_MNEMONIC="$_SAVED_SUDO_MNEMONIC"
[[ -n "$_SAVED_TEE_NODE_GQL"       ]] && TEE_NODE_GQL="$_SAVED_TEE_NODE_GQL"
[[ -n "$_SAVED_SKIP_TX_SIZE_CHECK" ]] && SKIP_TX_SIZE_CHECK="$_SAVED_SKIP_TX_SIZE_CHECK"
unset _SAVED_SUDO_MNEMONIC _SAVED_TEE_NODE_GQL _SAVED_SKIP_TX_SIZE_CHECK

WASM_DIR="$PROJECT_DIR/tee-chain/runtime/wasm/runtime"
RUNTIME_TOOL_DIR="$PROJECT_DIR/ext/runtimeupgrade"

# ── 必填校验（趁早） ─────────────────────────────────────
if [[ -z "${SUDO_MNEMONIC:-}" ]]; then
    cat >&2 <<EOF
ERROR: SUDO_MNEMONIC not set.

  配置方式（任选其一）:
    a) hack/.env         (推荐，chmod 600，模板见 hack/.env.example)
       SUDO_MNEMONIC="word1 word2 ... word12"

    b) 当前 shell export:
       export SUDO_MNEMONIC="word1 word2 ... word12"

    c) 单次运行:
       SUDO_MNEMONIC="..." hack/update_runtime.sh   # version 自动从源码读
EOF
    exit 1
fi

# ── 参数解析 ──────────────────────────────────────────
BUILD=true  # 默认编译 WASM；传 --no-build 跳过
WASM_PATH=""
VERSION_SRC="$PROJECT_DIR/tee-chain/runtime/module/call.go"
GQL_URL="${TEE_NODE_GQL:-https://localhost:30115}"
DESC=""
WAIT=true

while [[ $# -gt 0 ]]; do
    case "$1" in
        --no-build)    BUILD=false; shift ;;
        --wasm)        WASM_PATH="$2"; shift 2 ;;
        --version-src) VERSION_SRC="$2"; shift 2 ;;
        --gql)         GQL_URL="$2"; shift 2 ;;
        --desc)        DESC="$2"; shift 2 ;;
        --no-wait)     WAIT=false; shift ;;
        -h|--help)
            sed -n '2,16p' "$0"
            exit 0
            ;;
        *)
            echo "Unknown arg: $1" >&2
            exit 1
            ;;
    esac
done

# ── 1. 先编译（Go 工具）── fail-fast ───────────────────────
echo "=========================================="
echo " 1/4 编译 ext/runtimeupgrade Go 工具 ..."
echo "=========================================="
TOOL_BIN="$(mktemp -t runtimeupgrade.XXXXXX)"
trap 'rm -f "$TOOL_BIN"' EXIT
( cd "$PROJECT_DIR" && go build -o "$TOOL_BIN" ./ext/runtimeupgrade/ )
echo "→ $TOOL_BIN ($(stat -c%s "$TOOL_BIN") bytes)"

# ── 2. 编译 WASM（默认；--no-build 跳过） ──────────────────
if $BUILD; then
    echo ""
    echo "=========================================="
    echo " 2/4 编译 runtime.wasm (TinyGo) ..."
    echo "=========================================="
    if [[ ! -d "$WASM_DIR" ]]; then
        echo "ERROR: WASM source dir not found: $WASM_DIR" >&2
        exit 1
    fi
    ( cd "$WASM_DIR" && bash build.sh )
else
    echo " 2/4 跳过 WASM 编译（用现有 .wasm；删 --no-build 触发重编）"
fi

# ── 3. 校验 ──────────────────────────────────────────────
echo ""
echo "=========================================="
echo " 3/4 校验参数与 WASM ..."
echo "=========================================="

# 默认 WASM 路径：build.sh 已同时产出 runtime.wasm 和 runtime.wasm.gz (gzip -9)。
# 链端 gov.RuntimeUpgrade 直接存原字节、engine.go:decompressWasm 按 magic 自动解压，
# 所以传 .gz 上去 tx 体积直接减半，不影响链上加载。codehash 是对所传字节 blake2b，
# 工具/链端一致，不会有 hash 错配。
if [[ -z "$WASM_PATH" ]]; then
    WASM_PATH="$WASM_DIR/runtime.wasm.gz"
fi
if [[ ! -f "$WASM_PATH" ]]; then
    echo "ERROR: wasm not found: $WASM_PATH (use --build to compile, or --wasm <path>)" >&2
    exit 1
fi

# 贴心：用户显式传了 foo.wasm，但旁边有 foo.wasm.gz → 自动改用 gz 版本（半体积）。
# 想强制用 raw 的话，传一个没有 .gz 兄弟的路径，或传目录外的任意文件名即可。
if [[ "$WASM_PATH" == *.wasm && ! -f "$WASM_PATH.gz" ]]; then
    : # no-op: 没兄弟就走原文件
elif [[ "$WASM_PATH" == *.wasm && -f "$WASM_PATH.gz" ]]; then
    echo "auto: 发现 $WASM_PATH.gz，改用 gzip 版本（链端 engine.go:decompressWasm 自动解压）"
    WASM_PATH="$WASM_PATH.gz"
fi

WASM_SIZE=$(stat -c%s "$WASM_PATH")
echo "wasm: $WASM_PATH ($WASM_SIZE bytes)"

# tx 大小预警：GQL 默认没有 body 限制（gin 默认 32MB），但链上 mempool 仍可能拒收大 tx。
# 真要发 >1MB WASM 还得调 CometBFT max_tx_bytes + consensus max_block_size。
if (( WASM_SIZE > 900000 )); then
    echo ""
    echo "WARN: wasm is $WASM_SIZE bytes, encoded tx will be >1MB" >&2
    echo "      tee-chain CometBFT default max_tx_bytes=1MB → tx 可能被 mempool 拒收" >&2
    echo "      fix: tee-chain/mod.go rpcConf.MaxBodyBytes / consensusConf.MaxBlockSize" >&2
    echo "      or:  split wasm upload (out of scope for this script)" >&2
    echo ""
    if [[ "${SKIP_TX_SIZE_CHECK:-}" != "1" ]]; then
        read -r -p "继续提交? [y/N] " ans
        [[ "$ans" =~ ^[Yy]$ ]] || { echo "aborted"; exit 2; }
    fi
fi

if [[ -z "$DESC" ]]; then
    DESC="runtime upgrade (read NativeVersion from $VERSION_SRC)"
fi
echo "version-src: $VERSION_SRC  desc: $DESC  gql: $GQL_URL  wait: $WAIT"

# ── 4. 上传（签名 + GQL contractCall） ────────────────────
echo ""
echo "=========================================="
echo " 4/4 上传 RuntimeUpgrade 交易 (via GQL) ..."
echo "=========================================="

TOOL_ARGS=(
    --wasm        "$WASM_PATH"
    --desc        "$DESC"
    --version-src "$VERSION_SRC"
    --gql         "$GQL_URL"
    --mnemonic    "$SUDO_MNEMONIC"
)
$WAIT || TOOL_ARGS+=(--no-wait)

SUDO_MNEMONIC="$SUDO_MNEMONIC" "$TOOL_BIN" "${TOOL_ARGS[@]}"

echo ""
echo "Done. 交易执行状态可在以下方式查询："
echo "  - GQL:  query { txStatus(hash: \"<tx-hash>\") }"
echo "  - 日志: journalctl -u tee-node0 -f | grep -E 'RuntimeUpgrade|runtime_upgrade'"
