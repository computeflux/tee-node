# get shell path
SOURCE="$0"
while [ -h "$SOURCE"  ]; do
    DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /*  ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE"  )" && pwd  )"
cd $DIR/../

ROOT_DIR="$(pwd)"
REMOTE_HOST="computefluxtest"
REMOTE_NODE_DIR="/srv/token_node1"
REMOTE_CHAIN_DIR="$REMOTE_NODE_DIR/token_chain_data"
REMOTE_BUILD_DIR="$REMOTE_NODE_DIR/build"

# -------------------------------------------------
# 1. Prepare remote directories and symlink
# -------------------------------------------------
echo "[remote] Preparing $REMOTE_CHAIN_DIR and /token_chain_data on $REMOTE_HOST..."

ssh $REMOTE_HOST 'bash -s' << 'EOF'
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

# -------------------------------------------------
# 2. Sync build artifacts to remote
# -------------------------------------------------
echo "[local] Syncing build artifacts to $REMOTE_HOST:$REMOTE_BUILD_DIR ..."
rsync -avz -e ssh --delete "$ROOT_DIR/hack/build/" "$REMOTE_HOST:$REMOTE_BUILD_DIR/"

# -------------------------------------------------
# 3. Run node on remote host (same style as node0-run.sh)
# -------------------------------------------------
echo "[remote] Starting node1 on $REMOTE_HOST..."

ssh -tt $REMOTE_HOST 'bash -s' << 'EOF'
set -e
REMOTE_BUILD_DIR="/srv/token_node1/build"

export SIDE_CHAIN_PORT=30126
export SIDE_CHAIN_RPC_PORT=30127
export GQL_PORT=88
export CHAIN_ENV=test

cd /
OE_SIMULATION=1 ego run "$REMOTE_BUILD_DIR/dsecret"
EOF
