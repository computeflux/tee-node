package model

import (
	"fmt"

	"github.com/cometbft/cometbft/p2p"
	chain "github.com/wetee-dao/ink.go"
)

// GetKey get p2p key
func GetP2PKey() (*chain.Signer, *PrivKey, error) {
	nodeKey, err := p2p.LoadNodeKey("./chain_data/config/node_key.json")
	if err != nil {
		fmt.Println("failed to load node key:", err)
		return nil, nil, err
	}

	privateKey, err := PrivateKeyFromOed25519(nodeKey.PrivKey.Bytes())
	if err != nil {
		fmt.Println("Marshal PKG_PK error:", err)
		return nil, nil, err
	}

	return privateKey.ToSigner(), privateKey, nil
}
