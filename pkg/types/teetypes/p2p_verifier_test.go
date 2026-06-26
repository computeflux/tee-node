package teetypes

import (
	"testing"

	"github.com/cometbft/cometbft/crypto/ed25519"
	"github.com/stretchr/testify/require"

	"github.com/computeflux/tee-provider/pkg/types/crypto"
)

func TestP2PTEEVerifierRoundtrip(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519KeyPair(nil)
	require.NoError(t, err)

	signer := priv.ToSigner(crypto.SuiteEd25519)
	verifier := NewP2PTEEVerifier(signer)

	report, err := verifier.GenerateReport()
	require.NoError(t, err)
	require.NotEmpty(t, report)

	// Verify against the same public key the report was bound to.
	remotePub := ed25519.PubKey(signer.Public())
	err = verifier.VerifyReport(report, remotePub)
	require.NoError(t, err)
}

func TestP2PTEEVerifierWrongKey(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519KeyPair(nil)
	require.NoError(t, err)

	signer := priv.ToSigner(crypto.SuiteEd25519)
	verifier := NewP2PTEEVerifier(signer)

	report, err := verifier.GenerateReport()
	require.NoError(t, err)

	// Verify against a different key should fail because the report is bound
	// to the signer's public key.
	otherPriv := ed25519.GenPrivKey()
	err = verifier.VerifyReport(report, otherPriv.PubKey())
	require.Error(t, err)
	require.Contains(t, err.Error(), "not bound to remote p2p key")
}

func TestP2PTEEVerifierEmptyReport(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519KeyPair(nil)
	require.NoError(t, err)

	signer := priv.ToSigner(crypto.SuiteEd25519)
	verifier := NewP2PTEEVerifier(signer)

	remotePriv := ed25519.GenPrivKey()
	err = verifier.VerifyReport(nil, remotePriv.PubKey())
	require.Error(t, err)
	require.Contains(t, err.Error(), "empty attestation report")
}

func TestP2PTEEVerifierNilSigner(t *testing.T) {
	verifier := NewP2PTEEVerifier(nil)
	_, err := verifier.GenerateReport()
	require.Error(t, err)
	require.Contains(t, err.Error(), "signer is nil")
}
