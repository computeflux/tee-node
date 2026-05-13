package model

import (
	"crypto/ecdsa"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
	"golang.org/x/crypto/blake2b"
)

// ---------------------------------------------------------------------------
// Secp256k1Signer — implements chain.SignerType for ECDSA secp256k1
// ---------------------------------------------------------------------------

// Secp256k1Signer implements chain.SignerType using ECDSA over secp256k1.
// Signatures follow the Substrate convention: blake2b hash → secp256k1 sign,
// producing a 64-byte [R || S] signature (no recovery ID).
type Secp256k1Signer struct {
	priv *ecdsa.PrivateKey
	pub  []byte // compressed 33-byte public key
}

// Public returns the compressed secp256k1 public key (33 bytes).
func (s *Secp256k1Signer) Public() []byte {
	return s.pub
}

// PrivateKey returns the underlying ECDSA private key.
func (s *Secp256k1Signer) PrivateKey() *ecdsa.PrivateKey {
	return s.priv
}

// AccountID returns the 32-byte account ID (truncated public key).
func (s *Secp256k1Signer) AccountID() types.AccountID {
	var bt [32]byte
	copy(bt[:], s.pub)
	return types.AccountID(bt)
}

// Sign produces a 64-byte ECDSA signature [R || S] over blake2b(msg).
func (s *Secp256k1Signer) Sign(msg []byte) ([]byte, error) {
	digest := blake2b.Sum256(msg)
	sig, err := ethcrypto.Sign(digest[:], s.priv)
	if err != nil {
		return nil, err
	}
	// sig is 65 bytes [R || S || V]; strip the recovery ID for 64-byte output.
	return sig[:64], nil
}

// Verify checks a 64-byte [R || S] signature against blake2b(msg) and the
// compressed public key.
func (s *Secp256k1Signer) Verify(msg []byte, signature []byte) bool {
	if len(signature) != 64 {
		return false
	}
	digest := blake2b.Sum256(msg)
	return ethcrypto.VerifySignature(s.pub, digest[:], signature)
}

// SignType returns 2, the key type code for secp256k1.
func (s *Secp256k1Signer) SignType() uint8 {
	return 2
}
