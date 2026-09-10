#!/usr/bin/env bash
# Generate *_gen.go (ExecCall / ExecQuery) and tee-chain/abis/*.json
# for all contracts: gov, auth, gateway, batch, c2c.
ROOT="$(cd "$(dirname "$0")/../" && pwd)"
cd "$ROOT"

GEN="$ROOT/tools/contractgen"

# ── gov ────────────────────────────────────────────────────
echo ">>> gov"
go run "$GEN" \
  -dir ./tee-chain/runtime/module/gov \
  -out ./tee-chain/runtime/module/gov/gov_gen.go \
  -mutation GovMutation \
  -query GovQuery \
  -struct Gov \
  -skip-mutation Delete

# ── auth ───────────────────────────────────────────────────
echo ">>> auth"
go run "$GEN" \
  -dir ./tee-chain/runtime/module/auth \
  -out ./tee-chain/runtime/module/auth/auth_gen.go \
  -mutation AuthMutation \
  -query AuthQuery \
  -struct Auth

# ── gateway ────────────────────────────────────────────────
echo ">>> gateway"
go run "$GEN" \
  -dir ./tee-chain/runtime/module/gateway \
  -out ./tee-chain/runtime/module/gateway/gateway_gen.go \
  -mutation GatewayMutation \
  -query GatewayQuery \
  -struct Gateway

# ── batch ──────────────────────────────────────────────────
echo ">>> batch"
go run "$GEN" \
  -dir ./tee-chain/runtime/module/batch \
  -out ./tee-chain/runtime/module/batch/batch_gen.go \
  -mutation BatchMutation \
  -query BatchQuery \
  -struct Batch

# ── c2c（积分兑换结算；无 gql 接口，仅合约调用）──────────
echo ">>> c2c"
go run "$GEN" \
  -dir ./tee-chain/runtime/module/c2c \
  -out ./tee-chain/runtime/module/c2c/c2c_gen.go \
  -mutation C2CMutation \
  -query C2CQuery \
  -struct C2C

# ── Sync ABI to dapp ───────────────────────────────────────
echo ">>> sync ABI to dapp"
DAPP_CONTRACT_DIR="$ROOT/../dapp/public/contract"
mkdir -p "$DAPP_CONTRACT_DIR"
cp "$ROOT/tee-chain/runtime/abis/gateway.json" "$DAPP_CONTRACT_DIR/gateway.json"
cp "$ROOT/tee-chain/runtime/abis/auth.json" "$DAPP_CONTRACT_DIR/auth.json"
cp "$ROOT/tee-chain/runtime/abis/gov.json" "$DAPP_CONTRACT_DIR/gov.json"
cp "$ROOT/tee-chain/runtime/abis/c2c.json" "$DAPP_CONTRACT_DIR/c2c.json"
echo "Synced to $DAPP_CONTRACT_DIR"

echo "Done."
