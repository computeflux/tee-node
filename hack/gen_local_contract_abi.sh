#!/usr/bin/env bash
# Generate *_gen.go (ExecCall / ExecQuery) and tee-chain/abis/*.json
# for all contracts: gov, auth, gateway.
ROOT="$(cd "$(dirname "$0")/../" && pwd)"
cd "$ROOT"

GEN="$ROOT/tools/contractgen"

# ── gov ────────────────────────────────────────────────────
echo ">>> gov"
go run "$GEN" \
  -dir ./tee-chain/contracts/gov \
  -out ./tee-chain/contracts/gov/gov_gen.go \
  -mutation GovMutation \
  -query GovQuery \
  -struct Gov \
  -skip-mutation Delete

# ── auth ───────────────────────────────────────────────────
echo ">>> auth"
go run "$GEN" \
  -dir ./tee-chain/contracts/auth \
  -out ./tee-chain/contracts/auth/auth_gen.go \
  -mutation AuthMutation \
  -query AuthQuery \
  -struct Auth

# ── gateway ────────────────────────────────────────────────
echo ">>> gateway"
go run "$GEN" \
  -dir ./tee-chain/contracts/gateway \
  -out ./tee-chain/contracts/gateway/gateway_gen.go \
  -mutation GatewayMutation \
  -query GatewayQuery \
  -struct Gateway

# ── Sync ABI to dapp ───────────────────────────────────────
echo ">>> sync ABI to dapp"
DAPP_CONTRACT_DIR="$ROOT/../dapp/public/contract"
mkdir -p "$DAPP_CONTRACT_DIR"
cp "$ROOT/tee-chain/abis/gateway.json" "$DAPP_CONTRACT_DIR/gateway.json"
cp "$ROOT/tee-chain/abis/auth.json" "$DAPP_CONTRACT_DIR/auth.json"
cp "$ROOT/tee-chain/abis/gov.json" "$DAPP_CONTRACT_DIR/gov.json"
echo "Synced to $DAPP_CONTRACT_DIR"

echo "Done."
