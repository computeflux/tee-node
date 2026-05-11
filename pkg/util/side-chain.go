package util

import (
	"encoding/hex"

	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/p2p"
)

func ToSideChainNodeID(pub []byte) p2p.ID {
	return p2p.ID(hex.EncodeToString(crypto.AddressHash(pub)))
}
