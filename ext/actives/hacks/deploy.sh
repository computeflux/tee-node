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
cd $DIR/..

# -------------------------------------------------
# Config
# -------------------------------------------------
APP_NAME="actives"
REMOTE_HOST="computefluxtest"
REMOTE_DIR="/srv/actives"
DATA_FILE="${DATA_FILE:-}"    # pass DATA_FILE=./formatted.json to import on deploy

# -------------------------------------------------
# Helpers
# -------------------------------------------------
remote_exec() {
    ssh "$REMOTE_HOST" "$@"
}

show_usage() {
    cat << EOF
Usage: $0 <command> [options]

Commands:
  gqlgen      在本地运行 gqlgen generate 生成 GraphQL 代码
  build       构建二进制到 ./hack/build/
  deploy      构建 + 同步远程 + 安装 systemd + 启动
  import      DATA_FILE=./data.json $0 deploy  首次部署时导入数据
  logs        实时查看远程日志
  status      查看远程服务状态
  restart     重启远程服务
  stop        停止远程服务
EOF
}

# -------------------------------------------------
# 1. Generate GraphQL code
# -------------------------------------------------
do_gqlgen() {
    echo "[local] Running gqlgen generate..."
    cd "$DIR/.."
    go run github.com/99designs/gqlgen generate --config gql/gqlgen.yml
    echo "[local] gqlgen done: generated.go + models_gen.go"
}

# -------------------------------------------------
# 2. Build binary
# -------------------------------------------------
do_build() {
    echo "[local] Building $APP_NAME ..."
    mkdir -p "$DIR/../hack/build"

    # Check if ego-go is available (for SGX)
    if command -v ego-go &> /dev/null; then
        ego-go build -o "$DIR/../hack/build/$APP_NAME" .
    else
        go build -o "$DIR/../hack/build/$APP_NAME" .
    fi

    # Sign for SGX if ego is available
    if command -v ego &> /dev/null; then
        echo "[local] Signing with ego..."
        cd "$DIR/../hack/build"
        ego sign "$APP_NAME" 2>/dev/null || echo "[local] No private key for signing, skipping"
        cd -
    fi

    echo "[local] Build done: hack/build/$APP_NAME"
}

# -------------------------------------------------
# 3. Sync to remote
# -------------------------------------------------
sync_to_remote() {
    echo "[local] Syncing binary to $REMOTE_HOST:$REMOTE_DIR ..."
    ssh "$REMOTE_HOST" "mkdir -p $REMOTE_DIR/build"
    rsync -avz -e ssh --delete "$DIR/../hack/build/" "$REMOTE_HOST:$REMOTE_DIR/build/"
}

# 4. Import data (one-time, on first deploy)
do_import() {
    if [ -z "$DATA_FILE" ]; then
        echo "No DATA_FILE set, skipping import."
        echo "  Usage: DATA_FILE=./formatted.json $0 deploy"
        return
    fi
    if [ ! -f "$DATA_FILE" ]; then
        echo "ERROR: $DATA_FILE not found" >&2
        exit 1
    fi

    # 初始化远程数据目录
    ssh "$REMOTE_HOST" "mkdir -p $REMOTE_DIR/data"

    # 同步本地已有数据（如果有）
    LOCAL_DATA="$DIR/../ext/actives/data"
    if [ -d "$LOCAL_DATA" ] && [ -n "$(ls -A "$LOCAL_DATA" 2>/dev/null)" ]; then
        echo "[local] Syncing existing data directory ..."
        rsync -avz -e ssh "$LOCAL_DATA/" "$REMOTE_HOST:$REMOTE_DIR/data/"
    fi

    # 导入 JSON 数据
    echo "[local] Importing $DATA_FILE to $REMOTE_HOST ..."
    rsync -avz -e ssh "$DATA_FILE" "$REMOTE_HOST:$REMOTE_DIR/data_import.json"
    remote_exec "cd $REMOTE_DIR && ./build/actives -db=$REMOTE_DIR/data -data=$REMOTE_DIR/data_import.json"
    echo "[local] Import done."
}

# -------------------------------------------------
# 5. Install systemd service
# -------------------------------------------------
install_systemd_service() {
    echo "[remote] Installing systemd service ${APP_NAME}.service ..."

    ssh "$REMOTE_HOST" "sudo tee /etc/systemd/system/${APP_NAME}.service" > /dev/null << 'SYSTEMD'
[Unit]
Description=ComputeFlux Actives (Early Access Service)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=/srv/actives
ExecStart=/srv/actives/build/actives -port=9100 -db=/srv/actives/data
Restart=always
RestartSec=5
StartLimitInterval=60s
StartLimitBurst=3
StandardOutput=journal
StandardError=journal
SyslogIdentifier=actives

[Install]
WantedBy=multi-user.target
SYSTEMD

    remote_exec "sudo systemctl daemon-reload"
    remote_exec "sudo systemctl enable ${APP_NAME}.service"
}

# -------------------------------------------------
# 6. Start and tail logs
# -------------------------------------------------
start_and_tail_logs() {
    echo "[remote] Starting ${APP_NAME}.service ..."
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
    gqlgen)
        do_gqlgen
        ;;

    build)
        do_gqlgen
        do_build
        ;;

    deploy)
        do_gqlgen
        do_build
        sync_to_remote
        do_import
        install_systemd_service
        start_and_tail_logs
        ;;

    import)
        # 单独导入数据到已在运行的服务
        if [ -z "$DATA_FILE" ]; then
            echo "Usage: DATA_FILE=./data.json $0 import"
            exit 1
        fi
        do_import
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
        remote_exec "sudo systemctl status ${APP_NAME}.service --no-pager"
        ;;

    *)
        show_usage
        exit 1
        ;;
esac
