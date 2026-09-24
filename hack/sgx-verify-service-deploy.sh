# 部署 sgx-verify-service 为本地 systemd 服务（不用 docker）。
# 验证逻辑由 ego eclient 完成（cgo 静态链接 /opt/ego/lib 的 liboehostverify）。
# 所有节点（本地 node0/node2、远程 test node1）通过 HTTP 调用它。
# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../
set -e

# 构建（cgo：需要 ego 的 C 头文件和库）
CGO_CFLAGS=-I/opt/ego/include CGO_LDFLAGS=-L/opt/ego/lib go build -o ./hack/build/sgx-verify-service ./tools/sgx-verify-service/

# 必须先停服务再替换二进制：运行中的可执行文件无法覆盖（cp: Text file busy）
sudo systemctl stop sgx-verify-service 2>/dev/null || true

sudo cp -f ./hack/build/sgx-verify-service /usr/local/bin/sgx-verify-service
sudo tee /etc/systemd/system/sgx-verify-service.service >/dev/null <<'EOF'
[Unit]
Description=SGX verify service (ego eclient)
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/sgx-verify-service -addr 0.0.0.0:8004
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF
sudo systemctl daemon-reload
sudo systemctl enable sgx-verify-service
sudo systemctl start sgx-verify-service

# 健康检查 + 确认跑的是新版（新版启动日志含 verifier=eclient，旧 exec 版没有）
sleep 1
if curl -sf 127.0.0.1:8004/healthz >/dev/null 2>&1; then
    if journalctl -u sgx-verify-service -n 5 --no-pager 2>/dev/null | grep -q "verifier=eclient"; then
        echo "sgx-verify-service OK (eclient 版): http://127.0.0.1:8004 (远程调用 http://xiaobai.asyou.me:8004)"
    else
        echo "ERROR: healthz 通过，但日志中没有 verifier=eclient，可能仍是旧版本在运行。"
        journalctl -u sgx-verify-service -n 20 --no-pager
        exit 1
    fi
else
    echo "ERROR: service health check failed, check logs:"
    journalctl -u sgx-verify-service -n 20 --no-pager
    exit 1
fi
