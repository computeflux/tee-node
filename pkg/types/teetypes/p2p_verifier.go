package teetypes

import (
	"bytes"
	"fmt"
	"time"

	"github.com/cometbft/cometbft/crypto"
	"github.com/cometbft/cometbft/crypto/ed25519"
	"google.golang.org/protobuf/proto"

	"github.com/computeflux/tee-provider/pkg/model"
	chain "github.com/wetee-dao/ink.go"
)

// P2PTEEVerifier implements github.com/cometbft/cometbft/p2p.TEEAttestationVerifier.
// It exchanges TEE remote attestation reports during the CometBFT p2p handshake.
type P2PTEEVerifier struct {
	signer chain.SignerType
}

// NewP2PTEEVerifier creates a verifier that uses the given signer to generate
// TEE attestation reports. The signer is only used to identify the caller to
// IssueReport; the actual attestation evidence is produced by the TEE.
func NewP2PTEEVerifier(signer chain.SignerType) *P2PTEEVerifier {
	return &P2PTEEVerifier{signer: signer}
}

// GenerateReport creates a TEE attestation report bound to the local p2p key.
func (v *P2PTEEVerifier) GenerateReport() ([]byte, error) {
	if v.signer == nil {
		return nil, fmt.Errorf("signer is nil")
	}

	call := &model.TeeCall{
		ChainId: 0,
		Caller:  v.signer.Public(),
		Tx:      &model.TeeCall_Text{Text: v.signer.Public()},
	}

	if err := IssueReport(v.signer, call); err != nil {
		return nil, fmt.Errorf("issue report: %w", err)
	}

	return proto.Marshal(call)
}

// VerifyReport verifies a remote peer's TEE attestation report and checks that
// it is bound to the provided remote p2p public key.
func (v *P2PTEEVerifier) VerifyReport(report []byte, remotePubKey crypto.PubKey) error {
	if remotePubKey == nil {
		return fmt.Errorf("remote public key is nil")
	}

	if len(report) == 0 {
		return fmt.Errorf("empty attestation report")
	}

	var call model.TeeCall
	if err := proto.Unmarshal(report, &call); err != nil {
		return fmt.Errorf("unmarshal report: %w", err)
	}

	// Bind report to the authenticated p2p public key. The peer must have placed
	// its own ed25519 public key in the Text field before generating the report.
	text := call.GetText()
	if !bytes.Equal(text, remotePubKey.Bytes()) {
		return fmt.Errorf("report is not bound to remote p2p key")
	}

	result, err := VerifyReport(&call)
	if err != nil {
		return fmt.Errorf("verify report: %w", err)
	}

	fmt.Println("CodeSigner", result.CodeSigner)
	fmt.Println("CodeSignature", result.CodeSignature)

	if result == nil {
		return fmt.Errorf("verify report returned nil result")
	}

	if time.Now().Unix()-call.Time > 3 {
		return fmt.Errorf("repost is out time")
	}

	// TODO: add policy checks here (e.g. whitelist code signer/product id, time window).
	return nil
}

// Ensure interface compatibility at compile time.
var _ interface {
	GenerateReport() ([]byte, error)
	VerifyReport(report []byte, remotePubKey crypto.PubKey) error
} = (*P2PTEEVerifier)(nil)

// Ed25519PubKeyFromBytes converts raw ed25519 public key bytes to a
// github.com/cometbft/cometbft/crypto.PubKey. It is a helper for callers that
// store the p2p key in a different representation.
func Ed25519PubKeyFromBytes(b []byte) crypto.PubKey {
	return ed25519.PubKey(b)
}
