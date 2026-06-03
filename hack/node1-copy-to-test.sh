#!/usr/bin/env bash
set -e

SOURCE_DIR="/srv/token_node1/token_chain_data/"
REMOTE_HOST="computefluxtest"
REMOTE_DIR="/srv/token_node1/token_chain_data"

echo "[remote] Removing BFT data config wetee under $REMOTE_DIR on $REMOTE_HOST ..."
ssh $REMOTE_HOST "rm -rf '$REMOTE_DIR'/BFT '$REMOTE_DIR'/data '$REMOTE_DIR'/config '$REMOTE_DIR'/wetee && mkdir -p '$REMOTE_DIR'"

echo "[local] Copying $SOURCE_DIR to $REMOTE_HOST:$REMOTE_DIR ..."
rsync -avz -e ssh "$SOURCE_DIR" "$REMOTE_HOST:$REMOTE_DIR"

echo "[local] Copy completed."
