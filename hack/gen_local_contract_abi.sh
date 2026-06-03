#!/usr/bin/env bash
# Generate *_gen.go (ExecCall / ExecQuery) and tee-chain/abis/*.json
# for all contracts: gov, auth, gateway, batch.
ROOT="$(cd "$(dirname "$0")/../" && pwd)"
cd "$ROOT"

GEN="$ROOT/tools/contractgen"

# ── gov ────────────────────────────────────────────────────
echo ">>> gov"
go run "$GEN" \
  -dir ./tee-chain/contracts/module/gov \
  -out ./tee-chain/contracts/module/gov/gov_gen.go \
  -mutation GovMutation \
  -query GovQuery \
  -struct Gov \
  -skip-mutation Delete

# ── auth ───────────────────────────────────────────────────
echo ">>> auth"
go run "$GEN" \
  -dir ./tee-chain/contracts/module/auth \
  -out ./tee-chain/contracts/module/auth/auth_gen.go \
  -mutation AuthMutation \
  -query AuthQuery \
  -struct Auth

# ── gateway ────────────────────────────────────────────────
echo ">>> gateway"
go run "$GEN" \
  -dir ./tee-chain/contracts/module/gateway \
  -out ./tee-chain/contracts/module/gateway/gateway_gen.go \
  -mutation GatewayMutation \
  -query GatewayQuery \
  -struct Gateway

# ── batch ──────────────────────────────────────────────────
echo ">>> batch"
go run "$GEN" \
  -dir ./tee-chain/contracts/module/batch \
  -out ./tee-chain/contracts/module/batch/batch_gen.go \
  -mutation BatchMutation \
  -query BatchQuery \
  -struct Batch

# ── Sync ABI to dapp ───────────────────────────────────────
echo ">>> sync ABI to dapp"
DAPP_CONTRACT_DIR="$ROOT/../dapp/public/contract"
mkdir -p "$DAPP_CONTRACT_DIR"
cp "$ROOT/tee-chain/contracts/abis/gateway.json" "$DAPP_CONTRACT_DIR/gateway.json"
cp "$ROOT/tee-chain/contracts/abis/auth.json" "$DAPP_CONTRACT_DIR/auth.json"
cp "$ROOT/tee-chain/contracts/abis/gov.json" "$DAPP_CONTRACT_DIR/gov.json"
echo "Synced to $DAPP_CONTRACT_DIR"

echo "Done."
