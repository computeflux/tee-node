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

# mutation{
#   setAuthConfig(cfg:{
#     corsOrigins: "*",
#     smtpHost: "smtp.feishu.cn",
#     smtpPort: 465,
#     smtpUser: "bai@asyou.me",
#     smtpPassword: "lgMoryDJQuM88BoL",
#     smtpFrom: "bai@asyou.me",
#     smtpFromName: "compute flux",
#     mockEmail: true,
#     googleClientID: "560483017331-cjvd4d7ed90ddibspo72fmdmc87nb91v.apps.googleusercontent.com",
#     googleClientSecret: "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",
#     googleRedirectURL: "http://xiaobai.asyou.me:5174/oauth/callback.html",
#     githubClientID: "Ov23lioDBYC59lwL07p6",
#     githubClientSecret: "efd1ff9c815b4353d4f57ed85847c17c2f498df5",
#     githubRedirectURL: "http://xiaobai.asyou.me:5174/oauth/callback.html",
#     twitterClientID: "MFEzY1J1SWsyQlZyVXAxeTF1TDM6MTpjaQ",
#     twitterClientSecret: "M74QGQdYEq-8Uyl3TTUVbWOqvYWEYHVLz5uibXbv_R9ARJbptE",
#     twitterRedirectURL: "http://xiaobai.asyou.me:5174/oauth/callback.html",
#   })
# }

# run from / so relative ./token_chain_data resolves to /token_chain_data
cd / 
OE_SIMULATION=1 ego run "$ROOT_DIR/hack/build/dsecret"
