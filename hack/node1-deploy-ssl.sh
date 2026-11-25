#!/usr/bin/env bash
# Sync the test API TLS cert/key to computefluxtest:
#   - /srv/token_node1/token_chain_data/ssl/{ser.key,ser.pem}  (dsecret-node1.service)
#   - /srv/actives/ssl/{ser.key,ser.pem}                       (actives.service)
# then restart both services.
set -e

REMOTE_HOST="computefluxtest"

SSL_SRC_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/ssl/test" && pwd)"
FULLCHAIN="$SSL_SRC_DIR/test-api.computeflux.ai-fullchain.pem"
PRIVKEY="$SSL_SRC_DIR/test-api.computeflux.ai-privkey.pem"

for f in "$FULLCHAIN" "$PRIVKEY"; do
    if [ ! -f "$f" ]; then
        echo "ERROR: missing source file: $f" >&2
        exit 1
    fi
done

echo "[remote] Backing up current certs on $REMOTE_HOST ..."
BACKUP_DIR=$(ssh "$REMOTE_HOST" "mktemp -d /tmp/ssl_backup_XXXXXX")
ssh "$REMOTE_HOST" "sudo cp /srv/token_node1/token_chain_data/ssl/ser.pem '$BACKUP_DIR/node1-ser.pem' 2>/dev/null && echo '  backed up: token_node1 ser.pem' || echo '  skip (not found): token_node1 ser.pem'"
ssh "$REMOTE_HOST" "sudo cp /srv/token_node1/token_chain_data/ssl/ser.key '$BACKUP_DIR/node1-ser.key' 2>/dev/null && echo '  backed up: token_node1 ser.key' || echo '  skip (not found): token_node1 ser.key'"
ssh "$REMOTE_HOST" "sudo cp /srv/actives/ssl/ser.pem '$BACKUP_DIR/actives-ser.pem' 2>/dev/null && echo '  backed up: actives ser.pem' || echo '  skip (not found): actives ser.pem'"
ssh "$REMOTE_HOST" "sudo cp /srv/actives/ssl/ser.key '$BACKUP_DIR/actives-ser.key' 2>/dev/null && echo '  backed up: actives ser.key' || echo '  skip (not found): actives ser.key'"

echo ""
echo "[remote] Installing cert/key to $REMOTE_HOST ..."
for DEST_DIR in "/srv/token_node1/token_chain_data/ssl" "/srv/actives/ssl"; do
    ssh "$REMOTE_HOST" "sudo mkdir -p '$DEST_DIR'"
    cat "$FULLCHAIN" | ssh "$REMOTE_HOST" "sudo tee '$DEST_DIR/ser.pem' > /dev/null"
    ssh "$REMOTE_HOST" "sudo chmod 644 '$DEST_DIR/ser.pem'"
    cat "$PRIVKEY" | ssh "$REMOTE_HOST" "sudo tee '$DEST_DIR/ser.key' > /dev/null"
    ssh "$REMOTE_HOST" "sudo chmod 600 '$DEST_DIR/ser.key'"
    echo "  installed: $DEST_DIR/{ser.pem,ser.key}"
done

echo ""
echo "[remote] Restarting services ..."
ssh "$REMOTE_HOST" "sudo systemctl restart dsecret-node1 && echo '  restarted: dsecret-node1.service'"
ssh "$REMOTE_HOST" "sudo systemctl restart actives && echo '  restarted: actives.service'"

echo ""
echo "[remote] Cleaning up backup ..."
ssh "$REMOTE_HOST" "rm -rf '$BACKUP_DIR'"

echo ""
echo "Done."
