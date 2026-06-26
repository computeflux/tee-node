// setblspubkey sets the aggregated BLS public key on the BLS gateway contract.
//
// Usage:
//
//	go run ./ext/setblspubkey \
//	  --rpc https://eth-rpc-testnet.polkadot.io \
//	  --bls 0x... \
//	  --key  0x... \
//	  --pubkey 0x...   # 96-byte compressed G2 public key in hex
//
// Environment variables (fallback):
//
//	RPC_URL, BLS_CONTRACT_ADDRESS, PRIVATE_KEY, BLS_PUBKEY
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/model/bls12381"
	teecrypto "github.com/computeflux/tee-provider/pkg/types/crypto"
)

func main() {
	rpcURL := flag.String("rpc", os.Getenv("RPC_URL"), "EVM RPC URL")
	subnetAddr := flag.String("subnet", os.Getenv("SUBNET_CONTRACT_ADDRESS"), "Subnet contract address")
	privKeyHex := flag.String("key", os.Getenv("PRIVATE_KEY"), "Governance private key (hex)")
	pubkeyHex := flag.String("pubkey", os.Getenv("BLS_PUBKEY"), "Aggregated BLS public key, 96-byte compressed G2 (hex)")
	flag.Parse()

	if *rpcURL == "" {
		fmt.Fprintln(os.Stderr, "Error: --rpc or RPC_URL required")
		os.Exit(1)
	}
	if *subnetAddr == "" {
		fmt.Fprintln(os.Stderr, "Error: --subnet or SUBNET_CONTRACT_ADDRESS required")
		os.Exit(1)
	}
	if *privKeyHex == "" {
		fmt.Fprintln(os.Stderr, "Error: --key or PRIVATE_KEY required")
		os.Exit(1)
	}
	if *pubkeyHex == "" {
		fmt.Fprintln(os.Stderr, "Error: --pubkey or BLS_PUBKEY required")
		os.Exit(1)
	}

	// ── 1. Parse private key ──────────────────────────────────
	pkHex := strings.TrimPrefix(*privKeyHex, "0x")
	privateKey, err := crypto.HexToECDSA(pkHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid private key: %v\n", err)
		os.Exit(1)
	}

	// ── 2. Connect to EVM ─────────────────────────────────────
	client, err := ethclient.Dial(*rpcURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: dial RPC: %v\n", err)
		os.Exit(1)
	}
	defer client.Close()

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: chain ID: %v\n", err)
		os.Exit(1)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: transactor: %v\n", err)
		os.Exit(1)
	}

	// ── 3. Parse and convert BLS pubkey ───────────────────────
	pubHex := strings.TrimPrefix(*pubkeyHex, "0x")
	pubBytes, err := hexDecode(pubHex)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: decode pubkey hex: %v\n", err)
		os.Exit(1)
	}
	if len(pubBytes) != 96 {
		fmt.Fprintf(os.Stderr, "Error: pubkey must be 96 bytes (compressed G2), got %d\n", len(pubBytes))
		os.Exit(1)
	}

	pubKey := teecrypto.PubKeyFromBLS12381Byte(pubBytes)
	if pubKey == nil {
		fmt.Fprintln(os.Stderr, "Error: failed to parse BLS public key")
		os.Exit(1)
	}

	pt := pubKey.Point()
	if pt == nil {
		fmt.Fprintln(os.Stderr, "Error: failed to unmarshal BLS public key point")
		os.Exit(1)
	}

	pubEIP, err := bls12381.G2ToEIP2537FromKyber(pt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: convert pubkey to EIP-2537: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Pubkey (EIP-2537): %d bytes\n", len(pubEIP))

	// ── 4. Bind Subnet contract ───────────────────────────────
	subnetContract, err := bindings.NewSubnet(common.HexToAddress(*subnetAddr), client)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: bind Subnet contract: %v\n", err)
		os.Exit(1)
	}

	// ── 5. Send setBlsAggPubkey tx ────────────────────────────
	fmt.Printf("Sender: %s\n", auth.From.Hex())
	fmt.Printf("Subnet contract: %s\n", *subnetAddr)
	fmt.Println("Sending setBlsAggPubkey...")

	tx, err := subnetContract.SetBlsAggPubkey(auth, pubEIP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: send setBlsAggPubkey: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Tx hash: %s\n", tx.Hash().Hex())

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, client, tx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: wait mined: %v\n", err)
		os.Exit(1)
	}
	if receipt.Status != 1 {
		fmt.Fprintln(os.Stderr, "Error: tx failed (status != 1)")
		os.Exit(1)
	}
	fmt.Printf("Success: block %d, gas used %d\n", receipt.BlockNumber, receipt.GasUsed)
}

func hexDecode(s string) ([]byte, error) {
	if len(s)%2 != 0 {
		s = "0" + s
	}
	out := make([]byte, len(s)/2)
	for i := 0; i < len(s); i += 2 {
		b1 := hexValue(s[i])
		b2 := hexValue(s[i+1])
		if b1 < 0 || b2 < 0 {
			return nil, fmt.Errorf("invalid hex char at position %d", i)
		}
		out[i/2] = byte(b1<<4 | b2)
	}
	return out, nil
}

func hexValue(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c - 'a' + 10)
	case c >= 'A' && c <= 'F':
		return int(c - 'A' + 10)
	}
	return -1
}
