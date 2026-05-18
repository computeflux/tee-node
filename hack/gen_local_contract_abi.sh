#!/usr/bin/env bash
# Generate *_gen.go (ExecCall / ExecQuery) and side-chain/abis/*.json
# for all contracts: gov, auth, gateway.
ROOT="$(cd "$(dirname "$0")/../" && pwd)"
cd "$ROOT"

GEN="$ROOT/tools/contractgen"

# ── gov ────────────────────────────────────────────────────
echo ">>> gov"
go run "$GEN" \
  -dir ./side-chain/contracts/gov \
  -out ./side-chain/contracts/gov/gov_gen.go \
  -mutation GovMutation \
  -query GovQuery \
  -struct Gov \
  -skip-mutation Delete

# ── auth ───────────────────────────────────────────────────
echo ">>> auth"
go run "$GEN" \
  -dir ./side-chain/contracts/auth \
  -out ./side-chain/contracts/auth/auth_gen.go \
  -mutation AuthMutation \
  -query AuthQuery \
  -struct Auth

# ── gateway ────────────────────────────────────────────────
echo ">>> gateway"
go run "$GEN" \
  -dir ./side-chain/contracts/gateway \
  -out ./side-chain/contracts/gateway/gateway_gen.go \
  -mutation GatewayMutation \
  -query GatewayQuery \
  -struct Gateway

# ── Sync ABI to dapp ───────────────────────────────────────
echo ">>> sync ABI to dapp"
DAPP_CONTRACT_DIR="$ROOT/../dapp/public/contract"
mkdir -p "$DAPP_CONTRACT_DIR"
cp "$ROOT/side-chain/abis/gateway.json" "$DAPP_CONTRACT_DIR/gateway.json"
cp "$ROOT/side-chain/abis/auth.json" "$DAPP_CONTRACT_DIR/auth.json"
cp "$ROOT/side-chain/abis/gov.json" "$DAPP_CONTRACT_DIR/gov.json"
echo "Synced to $DAPP_CONTRACT_DIR"

echo "Done."

