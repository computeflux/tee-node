#!/bin/bash
# build.sh — 编译 runtime.wasm（TinyGo reactor 模式）
#
# 用法:
#   bash build.sh           # 默认：编译生产版本（-panic=trap）+ 调试版本（-panic=print）
#   bash build.sh prod      # 只编译生产版本
#   bash build.sh debug     # 只编译调试版本

set -e
cd "$(dirname "$0")"

MODE="${1:-all}"  # all | prod | debug

build_prod() {
    echo ">>> [prod] TinyGo 编译 (-panic=trap, -no-debug) ..."
    tinygo build -buildmode=c-shared -target=wasip1 \
        -no-debug \
        -panic=trap \
        -gc=leaking \
        -o runtime.wasm ./

    echo ">>> [prod] wasm-opt 压缩 ..."
    wasm-opt -Oz --enable-bulk-memory --enable-nontrapping-float-to-int \
        runtime.wasm -o runtime.wasm.tmp 2>&1
    mv runtime.wasm.tmp runtime.wasm

    echo ">>> [prod] 完成: $(ls -lh runtime.wasm | awk '{print $5}')"
    gzip -9 -k -f runtime.wasm
    echo ">>> [prod] gzip: $(ls -lh runtime.wasm.gz | awk '{print $5}')"
}

build_debug() {
    echo ">>> [debug] TinyGo 编译 (-panic=print, 保留 panic 信息) ..."
    tinygo build -buildmode=c-shared -target=wasip1 \
        -panic=print \
        -gc=leaking \
        -o runtime-debug.wasm ./

    echo ">>> [debug] 完成: $(ls -lh runtime-debug.wasm | awk '{print $5}')"
}

case "$MODE" in
    prod)
        build_prod
        ;;
    debug)
        build_debug
        ;;
    all|*)
        build_prod
        build_debug
        echo ""
        echo ">>> 导出 (prod): $(strings runtime.wasm | grep -E '^(ping|execute|get_buffer|memory|_initialize)$' | sort -u | tr '\n' ' ')"
        echo ">>> 导出 (debug): $(strings runtime-debug.wasm | grep -E '^(ping|execute|get_buffer|memory|_initialize)$' | sort -u | tr '\n' ' ')"
        ;;
esac
