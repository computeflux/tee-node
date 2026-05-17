#!/bin/bash
set -e
# 部署 Subnet + Token 合约到 Polkadot testnet
# 用法: ./hack/deploy_subnet_token.sh

SOURCE="$0"
while [ -h "$SOURCE" ]; do
    DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
ROOT="$( cd "$DIR/../../" && pwd )"
CONTRACT_DIR="$ROOT/../../contract"

echo "=== Deploy Subnet + Token ==="
echo "Contract dir: $CONTRACT_DIR"

cd "$CONTRACT_DIR"

# 检查 .env
if [ ! -f .env ]; then
    echo "ERROR: .env not found. Copy .env.example to .env and configure."
    exit 1
fi

# 部署
npx hardhat run deploy_subnet_token.js --config hardhat.config.polkadot.js --network polkadotTestnet

echo ""
echo "=== Done ==="
