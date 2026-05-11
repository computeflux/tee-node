package main

import (
	"github.com/cometbft/cometbft/p2p"

	"github.com/cometbft/cometbft/privval"
)

func main() {
	if _, err := p2p.LoadOrGenNodeKey("node_key.json"); err != nil {
		panic(err)
	}

	pv := privval.GenFilePV("priv_validator_key.json", "priv_validator_state.json")

	pv.Save()
}
