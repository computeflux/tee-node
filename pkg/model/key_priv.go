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
)

// PrivKey wraps an ed25519 private key (64 bytes: 32 seed + 32 public).
// The first 32 bytes serve as a master seed.
type PrivKey struct {
	ed25519.PrivateKey
}

// Scalar returns the scalar for the given suite type.
//
//	SuiteEd25519 → SHA-512(seed), clamped per RFC 8032
func (p *PrivKey) Scalar(st SuiteType) kyber.Scalar {
	switch st {
	case SuiteEd25519:
		return p.ed25519Scalar()
	case SuiteBLS12381:
		return p.bls12381Scalar()
	default:
		return p.ed25519Scalar()
	}
}

// Public returns the public key point for the given suite type.
func (p *PrivKey) Public(st SuiteType) kyber.Point {
	switch st {
	case SuiteEd25519:
		return p.ed25519Public()
	case SuiteBLS12381:
		// For BLS12-381, derive the scalar and multiply by G2 base.
		suite := NewSuite(st)
		scalar := p.Scalar(st)
		return suite.Point().Mul(scalar, nil)
	default:
		return p.ed25519Public()
	}
}

// GetPublicForSuite returns a PubKey for the given suite type.
func (p *PrivKey) GetPublicForSuite(st SuiteType) *PubKey {
	switch st {
	case SuiteEd25519:
		return &PubKey{
			PublicKey: p.PrivateKey.Public().(ed25519.PublicKey),
			suiteType: SuiteEd25519,
		}
	case SuiteBLS12381:
		pt := p.Public(st)
		buf, err := pt.MarshalBinary()
		if err != nil {
			return nil
		}
		return &PubKey{
			PublicKey: ed25519.PublicKey(buf),
			suiteType: SuiteBLS12381,
		}
	default:
		return nil
	}
}

func (p *PrivKey) ed25519Scalar() kyber.Scalar {
	buf := p.PrivateKey
	digest := sha512.Sum512(buf[:32])
	digest[0] &= 0xf8
	digest[31] &= 0x7f
	digest[31] |= 0x40
	return NewSuite(SuiteEd25519).Scalar().SetBytes(digest[:32])
}

func (p *PrivKey) bls12381Scalar() kyber.Scalar {
	buf := p.PrivateKey
	digest := sha512.Sum512(buf[:32])
	digest[0] &= 0xf8
	digest[31] &= 0x7f
	digest[31] |= 0x40
	return NewSuite(SuiteBLS12381).Scalar().SetBytes(digest[:32])
}

func (p *PrivKey) ed25519Public() kyber.Point {
	suite := NewSuite(SuiteEd25519)
	point := suite.Point()
	point.UnmarshalBinary(p.PrivateKey[32:])
	return point
}

func (p *PrivKey) String() string {
	return hex.EncodeToString(p.PrivateKey)
}

// ToSigner returns a chain.SignerType for the given suite type.
//
//	SuiteEd25519 → standard ed25519 signer (P2P/chain identity)
func (p *PrivKey) ToSigner(st SuiteType) chain.SignerType {
	switch st {
	case SuiteEd25519:
		s, err := chain.Ed25519PairFromPk(ed25519.PrivateKey(p.PrivateKey), 42)
		if err != nil {
			panic(err)
		}
		return &s
	default:
		return nil
	}
}

// ToSecp256k1Signer returns an ECDSA secp256k1 signer derived from the master
// seed. This is kept separate from the suite-type-based ToSigner because
// secp256k1 is used for EVM transaction signing, not DKG threshold signing.
func (p *PrivKey) ToSecp256k1Signer() *Secp256k1Signer {
	return deriveSecp256k1Signer(p.PrivateKey[:32])
}

func (p *PrivKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.PrivateKey)
}

func (p *PrivKey) UnmarshalJSON(bt []byte) error {
	var privateKey ed25519.PrivateKey = []byte{}
	if err := json.Unmarshal(bt, &privateKey); err != nil {
		return err
	}
	if len(privateKey) != ed25519.PrivateKeySize {
		return fmt.Errorf("invalid private key length: %d, expected %d", len(privateKey), ed25519.PrivateKeySize)
	}
	p.PrivateKey = privateKey
	return nil
}

// --- Constructors ---

func GenerateEd25519KeyPair(src io.Reader) (*PrivKey, *PubKey, error) {
	pk, sk, err := ed25519.GenerateKey(src)
	if err != nil {
		return nil, nil, err
	}
	return &PrivKey{PrivateKey: sk},
		&PubKey{PublicKey: pk, suiteType: SuiteEd25519},
		nil
}

func PrivateKeyFromStd(privkey ed25519.PrivateKey) (*PrivKey, error) {
	if len(privkey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid private key length: %d", len(privkey))
	}
	return &PrivKey{PrivateKey: privkey}, nil
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
	return &PrivKey{PrivateKey: bt}, nil
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
