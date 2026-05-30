#!/usr/bin/env bash
set -e

SOURCE_DIR="/srv/token_node1/"
REMOTE_HOST="47.79.98.142"
REMOTE_DIR="/srv/token_node1/"

echo "[remote] Clearing $REMOTE_DIR on $REMOTE_HOST ..."
ssh -p 22 "root@$REMOTE_HOST" "rm -rf '$REMOTE_DIR' && mkdir -p '$REMOTE_DIR'"

echo "[local] Copying $SOURCE_DIR to root@$REMOTE_HOST:$REMOTE_DIR ..."
rsync -avz -e "ssh -p 22" --delete "$SOURCE_DIR" "root@$REMOTE_HOST:$REMOTE_DIR"

echo "[local] Copy completed."
