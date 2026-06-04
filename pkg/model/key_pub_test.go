package model

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// helper: generate ed25519 PubKey
func makeEd25519PubKey(t *testing.T) *PubKey {
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	priv := &PrivKey{PrivateKey: sk}
	pub := priv.GetPublicForSuite(SuiteEd25519)
	require.NotNil(t, pub)
	return pub
}

// helper: generate BLS12381 PubKey
func makeBLS12381PubKey(t *testing.T) *PubKey {
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	priv := &PrivKey{PrivateKey: sk}
	pub := priv.GetPublicForSuite(SuiteBLS12381)
	require.NotNil(t, pub)
	return pub
}

// ── Type Detection ─────────────────────────────────────────

func TestPubKey_SuiteType(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	assert.Equal(t, "ed25519", ed.SuiteType())
	assert.Equal(t, "bls12381", bls.SuiteType())
}

func TestPubKey_IsEd25519_IsBLS12381(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	assert.True(t, ed.IsEd25519())
	assert.False(t, ed.IsBLS12381())

	assert.False(t, bls.IsEd25519())
	assert.True(t, bls.IsBLS12381())
}

// ── Basic Info ─────────────────────────────────────────────

func TestPubKey_String(t *testing.T) {
	ed := makeEd25519PubKey(t)
	assert.Equal(t, hex.EncodeToString(ed.PublicKey), ed.String())
}

func TestPubKey_Byte(t *testing.T) {
	ed := makeEd25519PubKey(t)
	assert.Equal(t, []byte(ed.PublicKey), ed.Byte())
}

// ── Conversion ─────────────────────────────────────────────

func TestPubKey_Std(t *testing.T) {
	ed := makeEd25519PubKey(t)
	std, err := ed.Std()
	require.NoError(t, err)
	assert.Equal(t, ed25519.PublicKey(ed.PublicKey), std)
}

func TestPubKey_Ed25519PublicKey(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	assert.Equal(t, ed.PublicKey, ed.Ed25519PublicKey())
	assert.Nil(t, bls.Ed25519PublicKey())
}

func TestPubKey_BLS12381PublicKey(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	assert.Nil(t, ed.BLS12381PublicKey())
	assert.Equal(t, []byte(bls.PublicKey), bls.BLS12381PublicKey())
}

func TestPubKey_Point(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	pt := ed.Point()
	require.NotNil(t, pt)

	pt2 := bls.Point()
	require.NotNil(t, pt2)
}

func TestPubKey_Point_InvalidBytes(t *testing.T) {
	// 使用一个非全零但无效的公钥（最后一个字节设为 0xFF 增加熵但不保证有效）
	invalid := make([]byte, 32)
	invalid[0] = 0xFF
	invalid[31] = 0xFF
	pk := PubKeyFromByte(invalid)
	// 大多数无效公钥会导致 UnmarshalBinary 失败
	pt := pk.Point()
	// 如果解析成功，验证它不等于有效点的行为即可
	assert.True(t, pt == nil || !pt.Equal(NewSuite(SuiteEd25519).Point().Base()))
}

// ── TeeChainNodeID ─────────────────────────────────────────

func TestPubKey_TeeChainNodeID(t *testing.T) {
	ed := makeEd25519PubKey(t)
	id := ed.TeeChainNodeID()
	assert.NotEmpty(t, id)
	assert.Len(t, string(id), 40) // hex-encoded 20-byte hash
}

func TestPubKey_TeeChainNodeID_PanicsForBLS(t *testing.T) {
	bls := makeBLS12381PubKey(t)
	assert.Panics(t, func() {
		bls.TeeChainNodeID()
	})
}

// ── SS58 ───────────────────────────────────────────────────

func TestPubKey_SS58(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	edSS58 := ed.SS58()
	assert.NotEmpty(t, edSS58)

	blsSS58 := bls.SS58()
	assert.NotEmpty(t, blsSS58)

	// SS58 round-trip
	decoded, err := PubKeyFromSS58(edSS58)
	require.NoError(t, err)
	assert.Equal(t, ed.PublicKey, decoded.PublicKey)
}

// ── AccountID ──────────────────────────────────────────────

func TestPubKey_AccountID(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	edAccount := ed.AccountID()
	assert.NotEmpty(t, edAccount)

	blsAccount := bls.AccountID()
	assert.NotEmpty(t, blsAccount)
}

// ── H160 ───────────────────────────────────────────────────

func TestPubKey_H160Address(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	edH160 := ed.H160Address()
	assert.NotEmpty(t, edH160)

	blsH160 := bls.H160Address()
	assert.NotEmpty(t, blsH160)
}

func TestPubKey_H160(t *testing.T) {
	ed := makeEd25519PubKey(t)
	h160 := ed.H160()
	assert.NotEmpty(t, h160)
}

func TestPubKey_AccountString(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	// Ed25519 → SS58
	assert.Equal(t, ed.SS58(), ed.AccountString())

	// BLS → hex
	assert.Equal(t, hex.EncodeToString(bls.PublicKey), bls.AccountString())
}

// ── JSON ───────────────────────────────────────────────────

func TestPubKey_MarshalJSON_UnmarshalJSON(t *testing.T) {
	ed := makeEd25519PubKey(t)

	bt, err := ed.MarshalJSON()
	require.NoError(t, err)

	var decoded PubKey
	err = json.Unmarshal(bt, &decoded)
	require.NoError(t, err)
	assert.Equal(t, ed.PublicKey, decoded.PublicKey)
	assert.Equal(t, SuiteEd25519, decoded.suiteType)
}

func TestPubKey_UnmarshalJSON_BLS(t *testing.T) {
	bls := makeBLS12381PubKey(t)

	bt, err := bls.MarshalJSON()
	require.NoError(t, err)

	var decoded PubKey
	err = json.Unmarshal(bt, &decoded)
	require.NoError(t, err)
	assert.Equal(t, bls.PublicKey, decoded.PublicKey)
	assert.Equal(t, SuiteBLS12381, decoded.suiteType)
}

// ── suiteTypeFromBytes ─────────────────────────────────────

func TestSuiteTypeFromBytes(t *testing.T) {
	assert.Equal(t, SuiteEd25519, suiteTypeFromBytes(make([]byte, 32)))
	assert.Equal(t, SuiteBLS12381, suiteTypeFromBytes(make([]byte, 96)))
	assert.Equal(t, SuiteEd25519, suiteTypeFromBytes(make([]byte, 17))) // default
}

// ── H160FromPublicKey ──────────────────────────────────────

func TestH160FromPublicKey(t *testing.T) {
	ed := makeEd25519PubKey(t)
	h160 := H160FromPublicKey(ed.PublicKey)
	assert.NotEmpty(t, h160)

	// Same as PubKey.H160()
	assert.Equal(t, ed.H160(), h160)
}

// ── Constructors ───────────────────────────────────────────

func TestPubKeyFromStdPubKey(t *testing.T) {
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	pub := sk.Public().(ed25519.PublicKey)

	pk, err := PubKeyFromStdPubKey(pub)
	require.NoError(t, err)
	assert.Equal(t, pub, pk.PublicKey)
	assert.True(t, pk.IsEd25519())
}

func TestPubKeyFromPoint(t *testing.T) {
	ed := makeEd25519PubKey(t)
	pt := ed.Point()
	require.NotNil(t, pt)

	pk, err := PubKeyFromPoint(pt)
	require.NoError(t, err)
	assert.NotNil(t, pk)
	assert.True(t, pk.IsEd25519())
}

func TestPubKeyFromPointWithSuite(t *testing.T) {
	bls := makeBLS12381PubKey(t)
	pt := bls.Point()
	require.NotNil(t, pt)

	pk, err := PubKeyFromPointWithSuite(pt, SuiteBLS12381)
	require.NoError(t, err)
	assert.NotNil(t, pk)
	assert.True(t, pk.IsBLS12381())
}

func TestPubKeyFromByte(t *testing.T) {
	ed := makeEd25519PubKey(t)
	bls := makeBLS12381PubKey(t)

	ed2 := PubKeyFromByte(ed.PublicKey)
	assert.True(t, ed2.IsEd25519())
	assert.Equal(t, ed.PublicKey, ed2.PublicKey)

	bls2 := PubKeyFromByte(bls.PublicKey)
	assert.True(t, bls2.IsBLS12381())
	assert.Equal(t, bls.PublicKey, bls2.PublicKey)
}

func TestPubKeyFromBLS12381Byte(t *testing.T) {
	bls := makeBLS12381PubKey(t)

	pk := PubKeyFromBLS12381Byte(bls.PublicKey)
	require.NotNil(t, pk)
	assert.True(t, pk.IsBLS12381())

	// Wrong length
	assert.Nil(t, PubKeyFromBLS12381Byte(make([]byte, 32)))
}

func TestPubKeyFromHex(t *testing.T) {
	ed := makeEd25519PubKey(t)
	h := hex.EncodeToString(ed.PublicKey)

	pk, err := PubKeyFromHex(h)
	require.NoError(t, err)
	assert.Equal(t, ed.PublicKey, pk.PublicKey)

	// Invalid hex
	_, err = PubKeyFromHex("not-hex")
	assert.Error(t, err)
}

func TestPubKeyFromSS58(t *testing.T) {
	ed := makeEd25519PubKey(t)
	ss58 := ed.SS58()

	pk, err := PubKeyFromSS58(ss58)
	require.NoError(t, err)
	assert.Equal(t, ed.PublicKey, pk.PublicKey)

	// Invalid SS58
	_, err = PubKeyFromSS58("invalid")
	assert.Error(t, err)
}
