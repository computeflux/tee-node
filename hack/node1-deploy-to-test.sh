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

# sign binary
cd ./hack/build/
ego sign dsecret

cd $DIR/../

ROOT_DIR="$(pwd)"
REMOTE_HOST="computefluxtest"
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
  deploy    同步构建产物到远程，安装/更新 systemd 服务并启动，随后实时 tail 日志
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

    remote_exec "sudo systemctl daemon-reload"
    remote_exec "sudo systemctl enable $SERVICE_NAME"
}

# -------------------------------------------------
# 4. Start service and tail logs
# -------------------------------------------------
start_and_tail_logs() {
    echo "[remote] Starting $SERVICE_NAME ..."
    remote_exec "sudo systemctl restart $SERVICE_NAME"
    sleep 1
    remote_exec "sudo systemctl status $SERVICE_NAME --no-pager"
    echo ""
    echo "========================================="
    echo "  实时日志输出（按 Ctrl+C 退出日志追踪，服务不会停止）"
    echo "========================================="
    remote_exec "sudo journalctl -u $SERVICE_NAME -f  -o cat --no-hostname --since '1 minute ago'"
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
