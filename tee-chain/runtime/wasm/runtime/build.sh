#!/bin/bash
set -e
cd "$(dirname "$0")"

# ── 生产构建（体积优先，安全可靠）──
echo ">>> TinyGo 编译 (target=wasip1, buildmode=c-shared) ..."
tinygo build -buildmode=c-shared -target=wasip1 \
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
echo ">>> 导出: $(strings runtime.wasm | grep -E '^(ping|execute|get_buffer|alloc|memory|_initialize)$' | sort -u | tr '\n' ' ')"

# ── 开发构建（保留 panic 信息，便于调试）──
# tinygo build -target=wasi -no-debug -gc=leaking -o runtime.wasm ./

# ── 选项说明 ──
#   -buildmode=c-shared   reactor 模式（导出函数，不自动执行 _start）
#   -target=wasip1       WASI preview1 目标 (wazero 兼容)
#   -no-debug            去调试信息
#   -panic=trap          panic→trap
#   -gc=leaking          不回收内存 (适合短生命周期)
#   wasm-opt -Oz         Binaryen 后处理
#   gzip                 线上传输
