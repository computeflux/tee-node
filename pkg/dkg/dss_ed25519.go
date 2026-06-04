package dkg

import (
	"encoding/json"
	"errors"

	"github.com/computeflux/tee-provider/pkg/dkg/types"
	"github.com/computeflux/tee-provider/pkg/util"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/sign/dss"
	"golang.org/x/crypto/blake2b"
)

func (d *DssSigner) signEd25519(suite dss.Suite, pubs []kyber.Point, long, random *dkgtypes.DistKeyShare, threshold int, msg []byte) ([]byte, error) {
	if len(msg) > 256 {
		h := blake2b.Sum256(msg)
		msg = h[:]
	}

	dssInst, err := dss.NewDSS(
		suite,
		d.dkg.longtermForSuite(d.suite),
		pubs,
		long,
		random,
		msg,
		uint32(threshold),
	)
	if dssInst == nil || err != nil {
		util.LogWithRed("DSS Sign: err", err.Error())
		return nil, errors.New("dss.NewDSS failed")
	}

	for _, bt := range d.sigs {
		sig := &dkgtypes.PartialSigWrap{}
		if err := json.Unmarshal(bt, sig); err != nil {
			util.LogWithRed("DSS Sign: err", err.Error())
			continue
		}

		ps := &dss.PartialSig{
			Partial:   sig.Partial.PriShare,
			SessionID: sig.SessionID,
			Signature: sig.Signature,
		}
		if err := dssInst.ProcessPartialSig(ps); err != nil {
			return nil, err
		}
	}

	return dssInst.Signature()
}

func (d *DssSigner) partialSignEd25519(suite dss.Suite, pubs []kyber.Point, long, random *dkgtypes.DistKeyShare, threshold int, msg []byte) ([]byte, error) {
	if len(msg) > 256 {
		h := blake2b.Sum256(msg)
		msg = h[:]
	}

	dssInst, err := dss.NewDSS(
		suite,
		d.dkg.longtermForSuite(d.suite),
		pubs,
		long,
		random,
		msg,
		uint32(threshold),
	)
	if dssInst == nil || err != nil {
		return nil, errors.New("dss.NewDSS failed")
	}

	sig, err := dssInst.PartialSig()
	if err != nil {
		return nil, errors.New("dss.PartialSig failed")
	}

	sigWrap := dkgtypes.PartialSigWrap{
		Partial:   dkgtypes.PriShareWithSuite{PriShare: sig.Partial, Suite: "ed25519"},
		SessionID: sig.SessionID,
		Signature: sig.Signature,
	}
	return json.Marshal(sigWrap)
}
