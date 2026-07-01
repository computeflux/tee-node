# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../

# ensure host /token_chain_data points to hack/node0/token_chain_data (cannot change enclave.json)
ROOT_DIR="$(pwd)"
NODE_DIR="/srv/token_node0"
NODE_CHAIN_DIR="$NODE_DIR/token_chain_data"

ensure_dir() {
    local dir="$1"
    if mkdir -p "$dir" 2>/dev/null; then
        return 0
    fi
    if command -v sudo >/dev/null 2>&1; then
        echo "Info: need sudo to create '$dir'."
        ensure_sudo || return 1
        sudo mkdir -p "$dir"
        return $?
    fi
    echo "ERROR: cannot create directory '$dir' (no permission, and sudo not available)." 1>&2
    return 1
}

ensure_sudo() {
    # If credentials are already cached, don't prompt.
    if sudo -n true 2>/dev/null; then
        return 0
    fi

    # If we have a TTY, let sudo prompt normally.
    if [ -t 0 ]; then
        sudo -v
        return $?
    fi

    # No TTY (e.g. non-interactive run). Prompt ourselves and feed sudo via stdin.
    printf "sudo password: " 1>&2
    stty -echo 2>/dev/null || true
    IFS= read -r SUDO_PW
    stty echo 2>/dev/null || true
    printf "\n" 1>&2

    if [ -z "$SUDO_PW" ]; then
        echo "ERROR: empty sudo password." 1>&2
        return 1
    fi

    printf "%s\n" "$SUDO_PW" | sudo -S -v >/dev/null
}

ensure_dir "$NODE_CHAIN_DIR" || exit 1

ensure_chain_data_link() {
    local desired="$1"
    local target="/token_chain_data"

    if [ -L "$target" ]; then
        local cur
        cur="$(readlink "$target")"
        if [ "$cur" != "$desired" ]; then
            echo "ERROR: $target is a symlink to '$cur', expected '$desired'." 1>&2
            return 1
        fi
        return 0
    fi

    if [ -e "$target" ]; then
        # If it's an actual directory/file, don't destroy user data silently.
        # Allow auto-fix ONLY when it's safe (empty dir or a file).
        if [ -d "$target" ]; then
            if [ -n "$(ls -A "$target" 2>/dev/null)" ]; then
                echo "ERROR: $target exists as a non-empty directory; please remove/move it to allow linking to '$desired'." 1>&2
                return 1
            fi
            if rmdir "$target" 2>/dev/null; then
                :
            elif command -v sudo >/dev/null 2>&1; then
                echo "Info: need sudo to replace existing empty directory '$target'."
                ensure_sudo || return 1
                sudo rmdir "$target" || return 1
            else
                echo "ERROR: cannot remove empty directory '$target' (no permission, and sudo not available)." 1>&2
                return 1
            fi
        else
            if rm -f "$target" 2>/dev/null; then
                :
            elif command -v sudo >/dev/null 2>&1; then
                echo "Info: need sudo to remove existing '$target'."
                ensure_sudo || return 1
                sudo rm -f "$target" || return 1
            else
                echo "ERROR: cannot remove existing '$target' (no permission, and sudo not available)." 1>&2
                return 1
            fi
        fi
    fi

    if ln -s "$desired" "$target" 2>/dev/null; then
        return 0
    fi

    if command -v sudo >/dev/null 2>&1; then
        echo "Info: need sudo to create '$target' -> '$desired'."
        # Trigger password prompt (if needed) and cache credentials for the following commands.
        ensure_sudo || return 1
        if [ -d "$target" ]; then
            # Don't destroy user data silently.
            if [ -n "$(ls -A "$target" 2>/dev/null)" ]; then
                echo "ERROR: $target exists as a non-empty directory; please remove/move it to allow linking to '$desired'." 1>&2
                return 1
            fi
            sudo rmdir "$target" || return 1
        else
            sudo rm -f "$target" || return 1
        fi
        sudo ln -s "$desired" "$target" || return 1
        return 0
    fi

    echo "ERROR: cannot create $target (no permission, and sudo not available)." 1>&2
    return 1
}

ensure_chain_data_link "$NODE_CHAIN_DIR" || exit 1

# build binary
ego-go build -o ./hack/build/dsecret ./main.go

# sign binary
cd ./hack/build/
ego sign dsecret

export SIDE_CHAIN_PORT=30116
export SIDE_CHAIN_RPC_PORT=30117
export GQL_PORT=30118
export CHAIN_ENV=test

# run from / so relative ./token_chain_data resolves to /token_chain_data
cd /

# ── 确保服务有 Restart 策略（进程退出后 systemd 自动拉起）──
# 用 systemctl edit 追加 drop-in 配置，避免改 /etc/systemd/system/tee-node0.service 原文件。
# Restart=always:        任意退出（含 clean exit code=0）都重启，保证节点持续在线。
# RestartSec=5:          重启间隔 5 秒，避免崩后立刻拉起又崩造成刷日志。
# StartLimitBurst / StartLimitIntervalSec: 60 秒内最多 10 次重启，超过后放弃。
if [ ! -f /etc/systemd/system/tee-node0.service.d/restart.conf ]; then
    echo "Configuring systemd restart policy for tee-node0.service ..."
    sudo mkdir -p /etc/systemd/system/tee-node0.service.d
    sudo tee /etc/systemd/system/tee-node0.service.d/restart.conf >/dev/null <<'EOF'
[Service]
Restart=always
RestartSec=5
StartLimitBurst=10
StartLimitIntervalSec=60
EOF
    sudo systemctl daemon-reload
fi

# ── 进程崩溃被 systemd 拉起时，不要再重新编译，直接复用脚本已构建的二进制 ──
# 原 /etc/systemd/system/tee-node0.service 在 ExecStartPre 里每次启动都跑
# `ego-go build` + `ego sign`，进程崩了被 systemd 拉起时还要先编译，拖慢恢复。
# 用 drop-in 把 ExecStartPre 重置，仅保留系统级目录/符号链接准备。
if [ ! -f /etc/systemd/system/tee-node0.service.d/no-rebuild.conf ]; then
    echo "Configuring systemd to skip rebuild on auto-restart ..."
    sudo mkdir -p /etc/systemd/system/tee-node0.service.d
    sudo tee /etc/systemd/system/tee-node0.service.d/no-rebuild.conf >/dev/null <<'EOF'
[Service]
ExecStartPre=
ExecStartPre=+/bin/bash -c 'mkdir -p /srv/token_node0/token_chain_data && rm -f /token_chain_data && ln -s /srv/token_node0/token_chain_data /token_chain_data'
EOF
    sudo systemctl daemon-reload
fi

# ── 检查并终止已有进程 ──
if systemctl is-active --quiet tee-node0.service 2>/dev/null; then
    echo "Stopping existing tee-node0.service ..."
    sudo systemctl stop tee-node0.service
    echo "Service stopped."
fi

echo "Starting tee-node0.service ..."
sudo systemctl start tee-node0.service

sleep 2
if systemctl is-active --quiet tee-node0.service; then
    echo "Service started successfully (auto-restart enabled)."
    echo "Tailing logs (Ctrl+C to stop watching, node will keep running):"
    journalctl -u tee-node0.service -f -o cat
else
    echo "ERROR: service failed to start. Check status:"
    systemctl status tee-node0.service
    exit 1
fi
