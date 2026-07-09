package teechain

import (
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func TestSignTx_RawSysCallRoundtrip(t *testing.T) {
	priv, _, err := crypto.GenerateEd25519KeyPair(nil)
	require.NoError(t, err)
	signer := priv.ToSigner(crypto.SuiteEd25519)

	call := &model.SysCall{
		Payload: &model.SysCall_Empty{Empty: 1},
	}

	tx, err := signTx(call, signer)
	require.NoError(t, err)
	require.NotNil(t, tx)

	// VerifyTxSigner unmarshals tx.Call directly, so it must be raw SysCall bytes,
	// not a length-delimited protoio frame.
	decoded := new(model.SysCall)
	err = decoded.UnmarshalVT(tx.Call)
	require.NoError(t, err)
	assert.True(t, proto.Equal(call, decoded))
}
