package crypto

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"strings"

	cmeted "github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"

	"github.com/computeflux/tee-provider/pkg/util"
)

// ToCometNodeKey converts a *PrivKey to a fully initialized *p2p.NodeKey.
func ToCometNodeKey(priv *PrivKey) *p2p.NodeKey {
	return &p2p.NodeKey{PrivKey: cmeted.PrivKey(priv.PrivateKey)}
}

// ToCometFilePV creates a *privval.FilePV from a *PrivKey — zero file I/O.
func ToCometFilePV(priv *PrivKey, keyPath, statePath string) *privval.FilePV {
	cometPriv := cmeted.PrivKey(priv.PrivateKey)
	return privval.NewFilePV(cometPriv, keyPath, statePath)
}

// PrintValidatorForGenesis prints the CometBFT validator address and pub_key
// so the operator can add them to genesis.json > validators.
// Output format matches the genesis.json validator entry:
//
//	{"address": "<HEX40>", "pub_key": {"type": "tendermint/PubKeyEd25519", "value": "<BASE64>"}, "power": "10", "name": ""}
func PrintValidatorForGenesis(priv *PrivKey, name string) {
	cometPriv := cmeted.PrivKey(priv.PrivateKey)
	pubKey := cometPriv.PubKey().Bytes() // 32 bytes
	h := sha256.Sum256(pubKey)
	address := strings.ToUpper(hex.EncodeToString(h[:20]))
	pubB64 := base64.StdEncoding.EncodeToString(pubKey)

	util.LogWithGreen("===================== CometBFT Validator ===================", "")
	util.LogWithGreen("Address", address)
	util.LogWithGreen("PubKey ", pubB64)
	if name != "" {
		util.LogWithGreen("  Name", name)
	}
	util.LogWithGreen("============================================================", "")
}
