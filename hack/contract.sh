#!/bin/bash
set -e
# 合约管理脚本: 编译、生成绑定、部署
# 用法:
#   ./hack/contract.sh build    — 编译合约 + 生成 Go binding
#   ./hack/contract.sh deploy   — 部署到 Polkadot testnet
#   ./hack/contract.sh all      — 编译 + 绑定 + 部署

SOURCE="$0"
while [ -h "$SOURCE" ]; do
    DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
    SOURCE="$(readlink "$SOURCE")"
    [[ $SOURCE != /* ]] && SOURCE="$DIR/$SOURCE"
done
DIR="$( cd -P "$( dirname "$SOURCE" )" && pwd )"
ROOT="$( cd "$DIR/../" && pwd )"
CONTRACT_DIR="$ROOT/../contract"
BINDINGS_DIR="$ROOT/pkg/chains/evm/bindings"

CMD="${1:-build}"

case "$CMD" in
    build|compile)
        echo "=== 1. Compile Solidity ==="
        cd "$CONTRACT_DIR"
        npx hardhat compile --force --config hardhat.config.polkadot.js
        echo ""

        echo "=== 2. Generate Go Bindings ==="
        mkdir -p "$BINDINGS_DIR"

        # Token
        python3 -c "import json; json.dump(json.load(open('$CONTRACT_DIR/artifacts/contracts/Token.sol/Token.json'))['abi'], open('/tmp/token_abi.json','w'))"
        abigen --abi /tmp/token_abi.json --pkg bindings --type Token --out "$BINDINGS_DIR/token.go"
        echo "  token.go ($(wc -l < "$BINDINGS_DIR/token.go") lines)"

        # Subnet
        python3 -c "import json; json.dump(json.load(open('$CONTRACT_DIR/artifacts/contracts/Subnet.sol/Subnet.json'))['abi'], open('/tmp/subnet_abi.json','w'))"
        abigen --abi /tmp/subnet_abi.json --pkg bindings --type Subnet --out "$BINDINGS_DIR/subnet.go"
        echo "  subnet.go ($(wc -l < "$BINDINGS_DIR/subnet.go") lines)"

        # Cleanup
        rm -f /tmp/token_abi.json /tmp/subnet_abi.json

        echo ""
        echo "=== 3. Verify Go build ==="
        cd "$ROOT"
        go build ./pkg/chains/evm/...
        echo "Build OK"
        ;;

    deploy)
        echo "=== Deploy to Polkadot testnet ==="
        cd "$CONTRACT_DIR"
        if [ ! -f .env ]; then
            echo "ERROR: .env not found"
            exit 1
        fi
        npx hardhat run deploy_subnet_token.js --config hardhat.config.polkadot.js --network polkadotTestnet
        ;;

    all)
        bash "$0" build
        bash "$0" deploy
        ;;

    *)
        echo "Usage: $0 {build|deploy|all}"
        exit 1
        ;;
esac
