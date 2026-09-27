#!/usr/bin/env bash
set -e

# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../

# build binary
ego-go build -o ./hack/build/dsecret ./main.go

# build sgx verify service (cgo: 静态链接 /opt/ego/lib 的 liboehostverify；
# 随 sync_build 同步到 test 机 /srv/token_node1/build/ 并在那里本机运行)
CGO_CFLAGS=-I/opt/ego/include CGO_LDFLAGS=-L/opt/ego/lib go build -o ./hack/build/sgx-verify-service ./tools/sgx-verify-service/

# sign binary
cd ./hack/build/
ego sign dsecret

cd $DIR/../

ROOT_DIR="$(pwd)"
REMOTE_HOST="computefluxtest"
# sgx-verify-service 统一用本地服务器的公网地址（服务监听 0.0.0.0:8004）；
# 可能需要 DNS/DDNS + 路由器端口映射 8004 才能通，可用环境变量覆盖: SVS_HOST=1.2.3.4 $0 deploy
SVS_HOST="${SVS_HOST:-xiaobai.asyou.me}"
# /logs 日志查询端点 token（供 tee-node-monitor 远程拉取日志；与监控侧 MONITOR_LOG_TOKEN 一致）
# 可用环境变量覆盖: LOG_API_TOKEN=*** $0 deploy
LOG_API_TOKEN="${LOG_API_TOKEN:-527fe9c13397f554d5cdef09ce6a470c}"
REMOTE_NODE_DIR="/srv/token_node1"
REMOTE_CHAIN_DIR="$REMOTE_NODE_DIR/token_chain_data"
REMOTE_BUILD_DIR="$REMOTE_NODE_DIR/build"
SERVICE_NAME="dsecret-node1.service"

# -------------------------------------------------
# Helpers
# -------------------------------------------------
remote_exec() {
    ssh "$REMOTE_HOST" "$@"
}

show_usage() {
    cat << EOF
Usage: $0 <command>

Commands:
  deploy    同步构建产物到远程，安装/更新 systemd 服务并启动（默认不 tail; TAIL=1 实时看日志）
  logs      实时查看服务日志（journalctl -f）
  status    查看服务运行状态
  restart   重启服务
  stop      停止服务
  enable    设置开机自启
  disable   关闭开机自启
EOF
}

# -------------------------------------------------
# 1. Prepare remote directories and symlink
# -------------------------------------------------
prepare_remote() {
    echo "[remote] Preparing $REMOTE_CHAIN_DIR and /token_chain_data on $REMOTE_HOST..."

    ssh "$REMOTE_HOST" 'bash -s' << 'EOF'
set -e
REMOTE_CHAIN_DIR="/srv/token_node1/token_chain_data"

mkdir -p "$REMOTE_CHAIN_DIR"

if [ -L /token_chain_data ]; then
    CUR=$(readlink /token_chain_data)
    if [ "$CUR" != "$REMOTE_CHAIN_DIR" ]; then
        echo "ERROR: /token_chain_data is a symlink to '$CUR', expected '$REMOTE_CHAIN_DIR'." >&2
        exit 1
    fi
elif [ -e /token_chain_data ]; then
    if [ -d /token_chain_data ]; then
        if [ -n "$(ls -A /token_chain_data 2>/dev/null)" ]; then
            echo "ERROR: /token_chain_data exists as a non-empty directory; please remove/move it." >&2
            exit 1
        fi
        rmdir /token_chain_data
    else
        rm -f /token_chain_data
    fi
    ln -s "$REMOTE_CHAIN_DIR" /token_chain_data
else
    ln -s "$REMOTE_CHAIN_DIR" /token_chain_data
fi
EOF
}

# -------------------------------------------------
# 2. Sync build artifacts to remote
# -------------------------------------------------
sync_build() {
    echo "[local] Syncing build artifacts to $REMOTE_HOST:$REMOTE_BUILD_DIR ..."
    rsync -avz -e ssh --delete "$ROOT_DIR/hack/build/" "$REMOTE_HOST:$REMOTE_BUILD_DIR/"
}

# -------------------------------------------------
# 3. Install / update systemd service
# -------------------------------------------------
install_systemd_service() {
    echo "[remote] Installing systemd service $SERVICE_NAME ..."

    # 生成服务文件并通过 stdin 写入远程
    ssh "$REMOTE_HOST" "sudo tee /etc/systemd/system/$SERVICE_NAME" > /dev/null << 'EOF'
[Unit]
Description=DSecret Node1 for TEE-Chain
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment="SIDE_CHAIN_PORT=30126"
Environment="SIDE_CHAIN_RPC_PORT=30127"
Environment="GQL_PORT=443"
Environment="CHAIN_ENV=test"
Environment="OE_SIMULATION=1"
WorkingDirectory=/
ExecStart=/bin/bash -lc 'exec ego run /srv/token_node1/build/dsecret'
Restart=always
RestartSec=5
StartLimitInterval=60s
StartLimitBurst=3
StandardOutput=journal
StandardError=journal
SyslogIdentifier=dsecret-node1

[Install]
WantedBy=multi-user.target
EOF

    # sgx 验证服务在 test 机本机运行（二进制由 sync_build 同步到 $REMOTE_BUILD_DIR）：
    # 纯软件验证不需要 SGX 硬件，服务地址用 127.0.0.1。
    ssh "$REMOTE_HOST" "sudo tee /etc/systemd/system/sgx-verify-service.service" > /dev/null << EOF
[Unit]
Description=SGX verify service (ego eclient)
After=network.target

[Service]
Type=simple
ExecStart=$REMOTE_BUILD_DIR/sgx-verify-service -addr 127.0.0.1:8004
Restart=always
RestartSec=2

[Install]
WantedBy=multi-user.target
EOF

    # dsecret 通过 SGX_VERIFY_SERVICE_URL 调用本机服务。用 drop-in 注入，不改原单元文件。
    remote_exec "sudo mkdir -p /etc/systemd/system/$SERVICE_NAME.d"
    ssh "$REMOTE_HOST" "sudo tee /etc/systemd/system/$SERVICE_NAME.d/svs.conf" > /dev/null << EOF
[Service]
Environment="SGX_VERIFY_SERVICE_URL=http://$SVS_HOST:8004"
EOF

    # /logs 端点 token（未设置时端点 404 关闭）；drop-in 注入，不改原单元文件
    ssh "$REMOTE_HOST" "sudo tee /etc/systemd/system/$SERVICE_NAME.d/logs.conf" > /dev/null << EOF
[Service]
Environment="LOG_API_TOKEN=$LOG_API_TOKEN"
EOF

    remote_exec "sudo systemctl daemon-reload"
    remote_exec "sudo systemctl enable $SERVICE_NAME"
    remote_exec "sudo systemctl enable sgx-verify-service"
}

# -------------------------------------------------
# 4. Start service and tail logs
# -------------------------------------------------
start_and_tail_logs() {
    echo "[remote] Starting sgx-verify-service and $SERVICE_NAME ..."
    remote_exec "sudo systemctl restart sgx-verify-service"
    remote_exec "sudo systemctl restart $SERVICE_NAME"
    sleep 1
    remote_exec "sudo systemctl status $SERVICE_NAME --no-pager"
    # 默认不 tail(部署脚本不该永久阻塞); TAIL=1 $0 deploy 可实时看日志
    if [ "${TAIL:-0}" = "1" ]; then
        echo ""
        echo "========================================="
        echo "  实时日志输出（按 Ctrl+C 退出日志追踪，服务不会停止）"
        echo "========================================="
        remote_exec "sudo journalctl -u $SERVICE_NAME -f  -o cat --no-hostname --since '1 minute ago'"
    fi
}

# -------------------------------------------------
# Command dispatch
# -------------------------------------------------
CMD="${1:-deploy}"

case "$CMD" in
    deploy)
        prepare_remote
        sync_build
        install_systemd_service
        start_and_tail_logs
        ;;

    logs)
        echo "[remote] Tailing logs for $SERVICE_NAME ..."
        remote_exec "sudo journalctl -u $SERVICE_NAME -f  -o cat --no-hostname --since '1 minute ago'"
        ;;

    status)
        remote_exec "sudo systemctl status $SERVICE_NAME --no-pager"
        ;;

    restart)
        echo "[remote] Restarting $SERVICE_NAME ..."
        remote_exec "sudo systemctl restart $SERVICE_NAME"
        sleep 1
        remote_exec "sudo systemctl status $SERVICE_NAME --no-pager"
        ;;

    stop)
        echo "[remote] Stopping $SERVICE_NAME ..."
        remote_exec "sudo systemctl stop $SERVICE_NAME"
        remote_exec "sudo systemctl status $SERVICE_NAME --no-pager"
        ;;

    enable)
        remote_exec "sudo systemctl enable $SERVICE_NAME"
        echo "[remote] $SERVICE_NAME enabled."
        ;;

    disable)
        remote_exec "sudo systemctl disable $SERVICE_NAME"
        echo "[remote] $SERVICE_NAME disabled."
        ;;

    *)
        show_usage
        exit 1
        ;;
esac
