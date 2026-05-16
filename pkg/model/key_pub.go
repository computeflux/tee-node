package model

import (
	gocrypto "crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"encoding/hex"
	"encoding/json"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/p2p"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
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

// IsSecp256k1 returns true if this key uses the secp256k1 suite.
func (p *PubKey) IsSecp256k1() bool {
	return p.suiteType == SuiteSecp256k1
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
	buf := p.PublicKey
	if p.IsSecp256k1() {
		ecpub, err := decompressSecp256k1Pub(buf)
		if err != nil {
			return nil
		}
		// 用标准库 Marshal 得到 04||x||y 后走 UnmarshalBinary
		raw := elliptic.Marshal(ecpub.Curve, ecpub.X, ecpub.Y)
		point := p.Suite().Point()
		if err := point.UnmarshalBinary(raw); err != nil {
			return nil
		}
		return point
	}
	point := p.Suite().Point()
	if err := point.UnmarshalBinary(buf); err != nil {
		return nil
	}
	return point
}

// Std returns the Go standard-library public key.
//   - SuiteEd25519   → ed25519.PublicKey
//   - SuiteSecp256k1 → *ecdsa.PublicKey (decompressed)
func (p *PubKey) Std() (gocrypto.PublicKey, error) {
	if p.IsSecp256k1() {
		return decompressSecp256k1Pub(p.PublicKey)
	}
	return p.PublicKey, nil
}

// decompressSecp256k1Pub decompresses a 33-byte or 65-byte secp256k1 public
// key into a *ecdsa.PublicKey.
func decompressSecp256k1Pub(pub []byte) (*ecdsa.PublicKey, error) {
	switch len(pub) {
	case 33:
		// Compressed: decompress via ethcrypto
		return ethcrypto.DecompressPubkey(pub)
	case 64:
		// Uncompressed without 0x04 prefix
		return ethcrypto.UnmarshalPubkey(append([]byte{0x04}, pub...))
	case 65:
		return ethcrypto.UnmarshalPubkey(pub)
	default:
		return nil, fmt.Errorf("invalid secp256k1 public key length: %d", len(pub))
	}
}

// Ed25519PublicKey returns the ed25519 public key, or nil if this is not an
// ed25519 key.
func (p *PubKey) Ed25519PublicKey() ed25519.PublicKey {
	if p.IsEd25519() {
		return p.PublicKey
	}
	return nil
}

// Secp256k1PublicKey returns the raw secp256k1 public key bytes (33 or 65
// bytes), or nil if this is not a secp256k1 key.
func (p *PubKey) Secp256k1PublicKey() []byte {
	if p.IsSecp256k1() {
		return p.PublicKey
	}
	return nil
}

func (p *PubKey) Secp256k1CompressPubkey() []byte {
	if !p.IsSecp256k1() {
		return nil
	}
	switch len(p.PublicKey) {
	case 33:
		// 已经是压缩格式
		return p.PublicKey
	case 65:
		// 未压缩 04||x||y → 压缩 02/03||x
		pub, err := ethcrypto.UnmarshalPubkey(p.PublicKey)
		if err != nil {
			return nil
		}
		return ethcrypto.CompressPubkey(pub)
	case 64:
		// 未压缩缺 0x04 前缀
		pub, err := ethcrypto.UnmarshalPubkey(append([]byte{0x04}, p.PublicKey...))
		if err != nil {
			return nil
		}
		return ethcrypto.CompressPubkey(pub)
	default:
		return nil
	}
}

// String returns the hex-encoded public key.
func (p *PubKey) String() string {
	return hex.EncodeToString(p.PublicKey)
}

// SideChainNodeID returns the CometBFT P2P node ID (ed25519 only).
func (p *PubKey) SideChainNodeID() p2p.ID {
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
//   - SuiteEd25519   → SS58(public_key, 42)
//   - SuiteSecp256k1 → SS58(blake2b(public_key), 42)
func (p *PubKey) SS58() string {
	if p.IsSecp256k1() {
		hash := blake2b.Sum256(p.PublicKey)
		return SS58Encode(hash[:], 42)
	}
	k, err := p.Std()
	if err != nil {
		fmt.Printf("PubKey.SS58: Std() error: %v\n", err)
		return ""
	}
	return SS58Encode(k.(ed25519.PublicKey), 42)
}

// H160Address returns the H160 (Ethereum-style) address.
//   - ed25519   → keccak256(pubkey)[12:] per ink.util
//   - secp256k1 → keccak256(uncompressed_pubkey_without_prefix)[12:]
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

// ChainH160 returns the H160 address as a chain-agnostic type.
func (p *PubKey) ChainH160() ChainH160 {
	h := H160FromPublicKey(p.PublicKey)
	var out ChainH160
	copy(out[:], h[:])
	return out
}

// AccountID returns the 32-byte account ID.
//   - SuiteEd25519   → first 32 bytes of public key (truncated)
//   - SuiteSecp256k1 → blake2b(public_key) per Substrate convention
func (p *PubKey) AccountID() types.AccountID {
	if p.IsSecp256k1() {
		hash := blake2b.Sum256(p.PublicKey)
		return types.AccountID(hash)
	}
	var bt [32]byte
	copy(bt[:], p.PublicKey)
	return types.AccountID(bt)
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
	case 33, 65:
		return SuiteSecp256k1
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

// PubKeyFromSecp256k1Byte creates a PubKey from secp256k1 public key bytes
// (33 or 65 bytes).
func PubKeyFromSecp256k1Byte(pubkey []byte) *PubKey {
	if len(pubkey) != 33 && len(pubkey) != 65 {
		return nil
	}
	return &PubKey{
		PublicKey: ed25519.PublicKey(pubkey),
		suiteType: SuiteSecp256k1,
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
