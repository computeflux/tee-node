#!/usr/bin/env bash
# tee-node 预构建（evolve 链路 docker build 前调用，约定：apps/<app>/hack/prebuild.sh）：
# 编译 + 签名 dsecret / sgx-verify-service 到 hack/build/。
# 只做编译，不做 docker build/push（那是 build.sh 的完整部署流程）。
#
# 运行环境（evolve-driver 注入）：ego 工具链挂 /opt/ego（宿主 /opt/ego 只读），
# GOCACHE 独立（ego-go 自带 patch 过的 Go 工具链，与系统 go 共用 cache 会链接失败，
# 见 build.sh 同注释）。
set -euo pipefail
cd "$(dirname "$0")/.."

GOCACHE="${GOCACHE:-$HOME/.cache/ego-go-build}"
export GOCACHE
mkdir -p "$GOCACHE"

# build binary
/opt/ego/bin/ego-go build -o ./hack/build/dsecret ./main.go

# build sgx verify service (cgo: 静态链接 /opt/ego/lib 的 liboehostverify; see tools/sgx-verify-service)
CGO_CFLAGS=-I/opt/ego/include CGO_LDFLAGS=-L/opt/ego/lib /opt/ego/bin/ego-go build -o ./hack/build/sgx-verify-service ./tools/sgx-verify-service/

# sign binary
cd ./hack/build/
/opt/ego/bin/ego sign dsecret

echo "prebuild done: $(ls -la dsecret sgx-verify-service)"
