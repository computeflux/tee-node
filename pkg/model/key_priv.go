package model

import (
	"crypto/ed25519"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	oed25519 "github.com/oasisprotocol/curve25519-voi/primitives/ed25519"
	chain "github.com/wetee-dao/ink.go"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/suites"
)

type PrivKey struct {
	ed25519.PrivateKey
	suite suites.Suite
}

func (p *PrivKey) Scalar() kyber.Scalar {
	s, _ := p.ed25519Scalar()
	return s
}

func (p *PrivKey) ed25519Scalar() (kyber.Scalar, error) {
	buf := p.PrivateKey
	if len(buf) < ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: %d", len(buf))
	}

	// hash seed and clamp bytes
	digest := sha512.Sum512(buf[:32])
	digest[0] &= 0xf8
	digest[31] &= 0x7f
	digest[31] |= 0x40
	return p.suite.Scalar().SetBytes(digest[:32]), nil
}

func (p *PrivKey) String() string {
	bt := p.PrivateKey
	return hex.EncodeToString(bt)
}

func (p *PrivKey) GetPublic() *PubKey {
	return &PubKey{
		PublicKey: p.Public().(ed25519.PublicKey),
		suite:     p.suite,
	}
}

func (p *PrivKey) ToSigner() *chain.Signer {
	bt := p.PrivateKey

	var ed25519Key ed25519.PrivateKey = bt
	s, err := chain.Ed25519PairFromPk(ed25519Key, 42)
	if err != nil {
		panic(err)
	}
	return &s
}

func (p PrivKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.PrivateKey)
}

func (p *PrivKey) UnmarshalJSON(bt []byte) error {
	var privateKey ed25519.PrivateKey = []byte{}
	err := json.Unmarshal(bt, &privateKey)
	if err != nil {
		return err
	}

	p.PrivateKey = privateKey
	p.suite = suites.MustFind("Ed25519")
	return nil
}

func GenerateEd25519KeyPair(src io.Reader) (*PrivKey, *PubKey, error) {
	pk, sk, err := ed25519.GenerateKey(src)
	if err != nil {
		return nil, nil, err
	}
	suite := suites.MustFind("Ed25519")

	return &PrivKey{
			PrivateKey: sk,
			suite:      suite,
		}, &PubKey{
			PublicKey: pk,
			suite:     suite,
		}, nil
}

func PrivateKeyFromStd(privkey ed25519.PrivateKey) (*PrivKey, error) {
	if len(privkey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: %d, expected %d", len(privkey), ed25519.PrivateKeySize)
	}
	return &PrivKey{
		PrivateKey: privkey,
		suite:      suites.MustFind("Ed25519"),
	}, nil
}

func PrivateKeyFromHex(s string) (*PrivKey, error) {
	s = strings.TrimPrefix(s, "0x")
	bt, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(bt) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: %d, expected %d", len(bt), ed25519.PrivateKeySize)
	}

	return &PrivKey{
		PrivateKey: bt,
		suite:      suites.MustFind("Ed25519"),
	}, nil
}

func PrivateKeyFromOed25519(privkey oed25519.PrivateKey) (*PrivKey, error) {
	k, err := Oed25519ToStd(privkey)
	if err != nil {
		return nil, err
	}
	return PrivateKeyFromStd(k)
}

func StdToOed25519(privKey ed25519.PrivateKey) (oed25519.PrivateKey, error) {
	if len(privKey) != 64 {
		return nil, fmt.Errorf("invalid privKey length: %d", len(privKey))
	}

	return oed25519.PrivateKey(privKey), nil
}

func Oed25519ToStd(privKey oed25519.PrivateKey) (ed25519.PrivateKey, error) {
	if len(privKey) != 64 {
		return nil, fmt.Errorf("invalid privKey length: %d", len(privKey))
	}

	return ed25519.PrivateKey(privKey), nil
}
