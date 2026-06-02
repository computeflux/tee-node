#!/bin/bash
set -e
cd "$(dirname "$0")"

# ── 生产构建（体积优先，安全可靠）──
echo ">>> TinyGo 编译 (target=wasi) ..."
tinygo build -target=wasi \
    -no-debug \
    -panic=trap \
    -gc=leaking \
    -o runtime.wasm ./

echo ">>> wasm-opt 压缩 ..."
wasm-opt -Oz --enable-bulk-memory --enable-nontrapping-float-to-int \
    runtime.wasm -o runtime.wasm.tmp 2>&1
mv runtime.wasm.tmp runtime.wasm

echo ">>> 完成: $(ls -lh runtime.wasm | awk '{print $5}')"
gzip -9 -k -f runtime.wasm
echo ">>> gzip: $(ls -lh runtime.wasm.gz | awk '{print $5}')"
echo ">>> 导出: $(strings runtime.wasm | grep -E '^(ping|execute|memory|_start)$' | sort -u | tr '\n' ' ')"

# ── 开发构建（保留 panic 信息，便于调试）──
# tinygo build -target=wasi -no-debug -gc=leaking -o runtime.wasm ./

# ── 选项说明 ──
#   -target=wasi    WASI 目标 (wazero 兼容)
#   -no-debug       去调试信息
#   -panic=trap     panic→trap
#   -gc=leaking     不回收内存 (适合短生命周期)
#   wasm-opt -Oz    Binaryen 后处理
#   gzip            线上传输
