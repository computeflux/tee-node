#!/usr/bin/env bash
set -e

SOURCE_DIR="/srv/token_node1/token_chain_data/"
REMOTE_HOST="computefluxtest"
REMOTE_DIR="/srv/token_node1/token_chain_data"

# 需要保留的节点私钥文件（TEE 密封，每个节点唯一）
SEALED_FILES=("config/p2p_node_key.sealed" "config/validator_dkg_key.sealed")

echo "[remote] Backing up sealed keys on $REMOTE_HOST ..."
BACKUP_DIR=$(ssh $REMOTE_HOST "mktemp -d /tmp/token_node1_sealed_backup_XXXXXX")
for f in "${SEALED_FILES[@]}"; do
    ssh $REMOTE_HOST "if [ -f '$REMOTE_DIR/$f' ]; then cp '$REMOTE_DIR/$f' '$BACKUP_DIR/'; echo '  backed up: $f'; else echo '  skip (not found): $f'; fi"
done

echo ""
echo "[remote] Removing BFT data config wetee under $REMOTE_DIR ..."
ssh $REMOTE_HOST "rm -rf '$REMOTE_DIR'/BFT '$REMOTE_DIR'/data '$REMOTE_DIR'/config '$REMOTE_DIR'/wetee && mkdir -p '$REMOTE_DIR'"

echo ""
echo "[local] Copying $SOURCE_DIR to $REMOTE_HOST:$REMOTE_DIR ..."
rsync -avz -e ssh "$SOURCE_DIR" "$REMOTE_HOST:$REMOTE_DIR"

echo ""
echo "[remote] Restoring sealed keys ..."
for f in "${SEALED_FILES[@]}"; do
    ssh $REMOTE_HOST "if [ -f '$BACKUP_DIR/$(basename $f)' ]; then mkdir -p '$REMOTE_DIR/config'; cp '$BACKUP_DIR/$(basename $f)' '$REMOTE_DIR/$f'; echo '  restored: $f'; else echo '  skip (backup not found): $f'; fi"
done

echo "[remote] Cleaning up backup ..."
ssh $REMOTE_HOST "rm -rf '$BACKUP_DIR'"

echo ""
echo "[local] Copy completed."
