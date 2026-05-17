#!/usr/bin/env bash
# Generate *_gen.go (ExecCall / ExecQuery) and side-chain/abis/*.json
# for all contracts: gov, auth, gateway.
ROOT="$(cd "$(dirname "$0")/../../" && pwd)"
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

echo "Done."
