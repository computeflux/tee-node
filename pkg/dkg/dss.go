package dkg

import (
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"go.dedis.ch/kyber/v4"
	"go.dedis.ch/kyber/v4/sign/dss"
)

// create dss signer from dkg
func NewDssSigner(dkg *DKG, txIndex int64) *DssSigner {
	signer := &DssSigner{
		dkg:  dkg,
		sigs: make([]*dss.PartialSig, 0),
	}
	// Try to acquire an independent random share from the pool.
	// If the pool is nil, empty, or txIndex <= 0, fallback to using the long share.
	if dkg.RandomPool != nil && txIndex > 0 {
		random, err := dkg.RandomPool.Acquire(txIndex)
		if err == nil {
			signer.randomShare = random
		} else {
			fmt.Println("DssSigner: failed to acquire random share:", err)
		}
	}
	return signer
}

// signer for dss
type DssSigner struct {
	dkg         *DKG
	randomShare *model.DistKeyShare // independent random share from RandomPool; nil means fallback to long share
	sigs        []*dss.PartialSig
}

// set shares for dss
func (d *DssSigner) SetSigs(btsigs [][]byte) {
	sigs := make([]*dss.PartialSig, 0, len(btsigs))
	for _, bt := range btsigs {
		sig := &model.PartialSigWrap{}
		err := json.Unmarshal(bt, sig)
		if err != nil {
			fmt.Println(err)
			continue
		}
		sigs = append(sigs, &dss.PartialSig{
			Partial:   sig.Partial.PriShare,
			SessionID: sig.SessionID,
			Signature: sig.Signature,
		})
	}
	d.sigs = sigs
}

// get ed25519 public key
func (d *DssSigner) Public() []byte {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()
	if d.dkg.DkgPubKey != nil {
		return d.dkg.DkgPubKey.Byte()
	}
	return d.dkg.NewDkgPubKey.Byte()
}

func (d *DssSigner) AccountID() types.AccountID {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()
	if d.dkg.DkgPubKey != nil {
		return d.dkg.DkgPubKey.AccountID()
	}
	return d.dkg.NewDkgPubKey.AccountID()
}

// sign msg aggd share
func (d *DssSigner) Sign(msg []byte) ([]byte, error) {
	pubs, long, random, threshold := d.PubList()
	dss, err := dss.NewDSS(
		d.dkg.Suite,
		d.dkg.Signer.Scalar(),
		pubs,
		long,
		random,
		msg,
		threshold,
	)

	if dss == nil || err != nil {
		fmt.Println(err)
		return nil, errors.New("dss.NewDSS failed")
	}

	for _, sig := range d.sigs {
		err = dss.ProcessPartialSig(sig)
		if err != nil {
			return nil, err
		}
	}

	return dss.Signature()
}

// partial sign msg
func (d *DssSigner) PartialSign(msg []byte) ([]byte, error) {
	pubs, long, random, threshold := d.PubList()
	dss, err := dss.NewDSS(
		d.dkg.Suite,
		d.dkg.Signer.Scalar(),
		pubs,
		long,
		random,
		msg,
		threshold,
	)

	if dss == nil || err != nil {
		return nil, errors.New("dss.NewDSS failed")
	}

	sig, err := dss.PartialSig()
	if err != nil {
		return nil, errors.New("dss.PartialSig failed")
	}
	sigWrap := model.PartialSigWrap{
		Partial:   model.PriShare{PriShare: sig.Partial},
		SessionID: sig.SessionID,
		Signature: sig.Signature,
	}

	return json.Marshal(sigWrap)
}

// pub list returns the participant public keys, the long-term share,
// the random share, and the threshold.
// If an independent random share was acquired from RandomPool, it is used;
// otherwise falls back to the long-term share (preserves backward compatibility).
func (d *DssSigner) PubList() ([]kyber.Point, *model.DistKeyShare, *model.DistKeyShare, int) {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()
	pubs := make([]kyber.Point, 0, len(d.dkg.Nodes))
	if len(d.dkg.NewNodes) > 0 {
		for _, k := range d.dkg.NewNodes {
			pubs = append(pubs, k.ValidatorId.Point())
		}
		random := d.randomShare
		if random == nil {
			random = d.dkg.NewDkgKeyShare
		}
		return pubs, d.dkg.NewDkgKeyShare, random, len(d.dkg.NewNodes) * 2 / 3
	}

	for _, k := range d.dkg.Nodes {
		pubs = append(pubs, k.ValidatorId.Point())
	}
	random := d.randomShare
	if random == nil {
		random = d.dkg.DkgKeyShare
	}
	return pubs, d.dkg.DkgKeyShare, random, d.dkg.Threshold
}

// verify sig
func (d *DssSigner) Verify(msg []byte, signature []byte) bool {
	d.dkg.mu.RLock()
	defer d.dkg.mu.RUnlock()
	if d.dkg.DkgPubKey == nil {
		return false
	}
	return ed25519.Verify(d.dkg.DkgPubKey.Ed25519PublicKey(), msg, signature)
}

// return ed25519 type
func (d *DssSigner) SignType() uint8 {
	return 1
}
