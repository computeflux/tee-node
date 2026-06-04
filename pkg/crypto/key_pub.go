package crypto

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/p2p"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/suites"
	"golang.org/x/crypto/blake2b"

	"github.com/wetee-dao/ink.go/util"
	inkutil "github.com/wetee-dao/ink.go/util"
)

// PubKey wraps an ed25519 or secp256k1 public key.
// The underlying bytes are stored as ed25519.PublicKey (a []byte alias).
// For ed25519: 32 bytes. For secp256k1: 33 (compressed) or 65 (uncompressed).
type PubKey struct {
	ed25519.PublicKey
	suiteType SuiteType
}

// SuiteType returns the suite type string ("ed25519" or "secp256k1").
func (p *PubKey) SuiteType() string {
	return p.suiteType.String()
}

// IsEd25519 returns true if this key uses the Ed25519 suite.
func (p *PubKey) IsEd25519() bool {
	return p.suiteType == SuiteEd25519
}

// IsBLS12381 returns true if this key uses the BLS12-381 suite.
func (p *PubKey) IsBLS12381() bool {
	return p.suiteType == SuiteBLS12381
}

// Suite returns the kyber suite for this key's type.
func (p *PubKey) Suite() suites.Suite {
	return NewSuite(p.suiteType)
}

func (p *PubKey) AccountString() string {
	if p.IsEd25519() {
		return p.SS58()
	}
	return hex.EncodeToString(p.PublicKey)
}

// Point returns the kyber Point representation of this public key.
// Returns nil if unmarshaling fails. Callers must check for nil.
// TODO: change signature to (kyber.Point, error) for better error propagation.
func (p *PubKey) Point() kyber.Point {
	point := p.Suite().Point()
	if err := point.UnmarshalBinary(p.PublicKey); err != nil {
		return nil
	}
	return point
}

// Std returns the Go standard-library public key.
//   - SuiteEd25519 → ed25519.PublicKey
func (p *PubKey) Std() (any, error) {
	return p.PublicKey, nil
}

// Ed25519PublicKey returns the ed25519 public key, or nil if this is not an
// ed25519 key.
func (p *PubKey) Ed25519PublicKey() ed25519.PublicKey {
	if p.IsEd25519() {
		return p.PublicKey
	}
	return nil
}

// BLS12381PublicKey returns the raw BLS12-381 public key bytes (96 bytes),
// or nil if this is not a BLS12-381 key.
func (p *PubKey) BLS12381PublicKey() []byte {
	if p.IsBLS12381() {
		return p.PublicKey
	}
	return nil
}

// String returns the hex-encoded public key.
func (p *PubKey) String() string {
	return hex.EncodeToString(p.PublicKey)
}

// TeeChainNodeID returns the CometBFT P2P node ID (ed25519 only).
func (p *PubKey) TeeChainNodeID() p2p.ID {
	if !p.IsEd25519() {
		panic("not an ed25519 key")
	}
	return p2p.ID(hex.EncodeToString(crypto.AddressHash(p.PublicKey)))
}

// Byte returns the raw public key bytes.
func (p *PubKey) Byte() []byte {
	return p.PublicKey
}

// SS58 returns the SS58 address for this key.
//   - SuiteEd25519  → SS58(public_key, 42)
//   - SuiteBLS12381 → SS58(blake2b(public_key), 42)
func (p *PubKey) SS58() string {
	if p.IsEd25519() {
		return SS58Encode(p.PublicKey, 42)
	}
	// For BLS12381 (and any future non-ed25519 suite), hash the pubkey.
	hash := blake2b.Sum256(p.PublicKey)
	return SS58Encode(hash[:], 42)
}

// H160Address returns the H160 (Ethereum-style) address.
//   - ed25519  → keccak256(pubkey)[12:] per ink.util
//   - bls12381 → keccak256(pubkey)[12:]
func (p *PubKey) H160Address() types.H160 {
	h160, err := util.H160FromPublicKey(p.Byte())
	if err != nil {
		return types.H160{}
	}
	return h160
}

// H160 returns the H160 address for this key.
func (p *PubKey) H160() types.H160 {
	return H160FromPublicKey(p.PublicKey)
}

// AccountID returns the 32-byte account ID.
//   - SuiteEd25519  → first 32 bytes of public key (truncated)
//   - SuiteBLS12381 → blake2b(public_key)
func (p *PubKey) AccountID() types.AccountID {
	if p.IsEd25519() {
		var bt [32]byte
		copy(bt[:], p.PublicKey)
		return types.AccountID(bt)
	}
	// For BLS12381 (and any future non-ed25519 suite), hash the pubkey.
	hash := blake2b.Sum256(p.PublicKey)
	return types.AccountID(hash)
}

// MarshalJSON implements json.Marshaler.
func (p PubKey) MarshalJSON() ([]byte, error) {
	return json.Marshal(p.PublicKey)
}

// UnmarshalJSON implements json.Unmarshaler.
func (p *PubKey) UnmarshalJSON(bt []byte) error {
	var key ed25519.PublicKey = []byte{}
	err := json.Unmarshal(bt, &key)
	if err != nil {
		return err
	}

	p.PublicKey = key
	p.suiteType = suiteTypeFromBytes(key)
	return nil
}

// suiteTypeFromBytes detects suite type from public key byte length.
func suiteTypeFromBytes(pub []byte) SuiteType {
	switch len(pub) {
	case 32:
		return SuiteEd25519
	case 96:
		return SuiteBLS12381
	default:
		return SuiteEd25519
	}
}

// H160FromPublicKey computes the H160 address from raw public key bytes.
func H160FromPublicKey(bytes []byte) types.H160 {
	if inkutil.IsEthDerived(bytes) {
		var byteArray [20]byte
		copy(byteArray[:20], bytes[:])
		return byteArray
	}

	account_hash := inkutil.Keccak256Hash(bytes)
	var byteArray [20]byte
	copy(byteArray[:20], account_hash[12:])
	return byteArray
}

// --- Constructors ---

// PubKeyFromStdPubKey creates a PubKey from a standard ed25519 public key.
func PubKeyFromStdPubKey(pubkey ed25519.PublicKey) (*PubKey, error) {
	return &PubKey{
		PublicKey: pubkey,
		suiteType: SuiteEd25519,
	}, nil
}

// PubKeyFromPoint creates a PubKey from a kyber Point, auto-detecting the suite.
func PubKeyFromPoint(point kyber.Point) (*PubKey, error) {
	buf, err := point.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal point: %w", err)
	}

	st := suiteTypeFromBytes(buf)
	var pk ed25519.PublicKey = buf
	return &PubKey{
		PublicKey: pk,
		suiteType: st,
	}, nil
}

// PubKeyFromPointWithSuite creates a PubKey from a kyber point with an explicit
// suite type.
func PubKeyFromPointWithSuite(point kyber.Point, suiteType SuiteType) (*PubKey, error) {
	buf, err := point.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal point: %w", err)
	}

	var pk ed25519.PublicKey = buf
	return &PubKey{
		PublicKey: pk,
		suiteType: suiteType,
	}, nil
}

// PubKeyFromByte creates a PubKey from raw bytes, auto-detecting the suite.
func PubKeyFromByte(pubkey []byte) *PubKey {
	st := suiteTypeFromBytes(pubkey)
	return &PubKey{
		PublicKey: ed25519.PublicKey(pubkey),
		suiteType: st,
	}
}

// PubKeyFromBLS12381Byte creates a PubKey from BLS12-381 public key bytes
// (96 bytes).
func PubKeyFromBLS12381Byte(pubkey []byte) *PubKey {
	if len(pubkey) != 96 {
		return nil
	}
	return &PubKey{
		PublicKey: ed25519.PublicKey(pubkey),
		suiteType: SuiteBLS12381,
	}
}

// PubKeyFromHex creates a PubKey from a hex-encoded public key string.
func PubKeyFromHex(h string) (*PubKey, error) {
	b, err := hex.DecodeString(h)
	if err != nil {
		return nil, fmt.Errorf("decode hex: %w", err)
	}

	return PubKeyFromByte(b), nil
}

// PubKeyFromSS58 creates a PubKey from an SS58 address.
func PubKeyFromSS58(ss58 string) (*PubKey, error) {
	_, pubkey, err := SS58Decode(ss58)
	if err != nil {
		return nil, fmt.Errorf("decode ss58: %w", err)
	}

	return PubKeyFromByte(pubkey), nil
}
