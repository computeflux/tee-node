#!/usr/bin/env bash
set -e

# get shell path
SOURCE="$0"
while [ -h "$SOURCE" ]; do
    DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
cd "$DIR/.."

# -------------------------------------------------
# Config
# -------------------------------------------------
APP_NAME="actives"
REMOTE_HOST="${REMOTE_HOST:-computefluxtest}"
REMOTE_DIR="${REMOTE_DIR:-/srv/actives}"
DB_PATH="${DB_PATH:-./data}"

# -------------------------------------------------
# Helpers
# -------------------------------------------------
remote_exec() { ssh "$REMOTE_HOST" "$@"; }

show_usage() {
    cat << EOF
Usage: $0 <command>

Commands:
  build       构建 server  二进制到 build/
  deploy      构建 → 同步远程 → 安装 systemd → 启动
  logs        实时查看远程日志
  status      查看远程服务状态
  restart     重启远程服务
  stop        停止远程服务
EOF
}

# -------------------------------------------------
# 1. Build
# -------------------------------------------------
do_build() {
    echo "[build] Building server ..."
    mkdir -p build

    go build -o build/server ./cmd/server

    echo "[build] Done: build/server"
}

# -------------------------------------------------
# 2. Sync to remote
# -------------------------------------------------
do_sync() {
    echo "[sync] Syncing to $REMOTE_HOST:$REMOTE_DIR ..."
    ssh "$REMOTE_HOST" "mkdir -p $REMOTE_DIR/build"
    rsync -avz -e ssh build/server "$REMOTE_HOST:$REMOTE_DIR/build/"
    echo "[sync] Done."
}

# -------------------------------------------------
# 3. Install systemd service
# -------------------------------------------------
do_install() {
    echo "[install] Installing systemd service ${APP_NAME}.service ..."

    ssh "$REMOTE_HOST" "sudo tee /etc/systemd/system/${APP_NAME}.service" > /dev/null << SYSTEMD
[Unit]
Description=ComputeFlux Actives (Early Access Service)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=$REMOTE_DIR
ExecStart=$REMOTE_DIR/build/server -port=9100 -db=$REMOTE_DIR/data
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=$APP_NAME

[Install]
WantedBy=multi-user.target
SYSTEMD

    remote_exec "sudo systemctl daemon-reload"
    remote_exec "sudo systemctl enable ${APP_NAME}.service"
    echo "[install] Done."
}

# -------------------------------------------------
# 4. Start and tail logs
# -------------------------------------------------
do_start() {
    echo "[start] Restarting ${APP_NAME}.service ..."
    remote_exec "sudo systemctl restart ${APP_NAME}.service"
    sleep 1
    remote_exec "sudo systemctl status ${APP_NAME}.service --no-pager"
    echo ""
    echo "========================================="
    echo "  Ctrl+C to stop log tailing (service keeps running)"
    echo "========================================="
    remote_exec "sudo journalctl -u ${APP_NAME}.service -f -o cat --no-hostname --since '1 minute ago'"
}

# -------------------------------------------------
# Command dispatch
# -------------------------------------------------
CMD="${1:-deploy}"

case "$CMD" in
    build)
        do_build
        ;;

    deploy)
        do_build
        do_sync
        do_install
        do_start
        ;;

    logs)
        remote_exec "sudo journalctl -u ${APP_NAME}.service -f -o cat --no-hostname --since '1 minute ago'"
        ;;

    status)
        remote_exec "sudo systemctl status ${APP_NAME}.service --no-pager"
        ;;

    restart)
        remote_exec "sudo systemctl restart ${APP_NAME}.service"
        sleep 1
        remote_exec "sudo systemctl status ${APP_NAME}.service --no-pager"
        ;;

    stop)
        remote_exec "sudo systemctl stop ${APP_NAME}.service"
        echo "[stop] ${APP_NAME}.service stopped."
        ;;

    *)
        show_usage
        exit 1
        ;;
esac
