package sidechain

import (
	"fmt"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
	proxy_reenc "github.com/wetee-dao/tee-dsecret/pkg/proxy-reenc"
	"go.dedis.ch/kyber/v4/share"
	"go.dedis.ch/kyber/v4/suites"
)

// EncodeDecryptShare encode data to protobuf
func EncodeDecryptShare(reply *proxy_reenc.ReencryptReply, id uint64) (*model.DecryptShare, error) {
	xncski, err := reply.Share.V.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal xncski: %w", err)
	}

	chlgi, err := reply.Challenge.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal chlgi: %w", err)
	}

	proofi, err := reply.Proof.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal proofi: %w", err)
	}

	return &model.DecryptShare{
		ShareIndex: int32(reply.Share.I),
		XncSki:     xncski,
		Chlgi:      chlgi,
		Proofi:     proofi,
	}, nil
}

// DecodeDecryptShare decode data from protobuf
func DecodeDecryptShare(req *model.DecryptShare, ste suites.Suite) (*proxy_reenc.ReencryptReply, error) {
	// 初始化重新加密回复
	reply := &proxy_reenc.ReencryptReply{
		Share: share.PubShare{
			I: uint32(req.ShareIndex),
			V: ste.Point().Base(),
		},
		Challenge: ste.Scalar(),
		Proof:     ste.Scalar(),
	}

	// 处理回复中的份额信息
	err := reply.Share.V.UnmarshalBinary(req.XncSki)
	if err != nil {
		return nil, fmt.Errorf("unmarshal xncski: %s", err)
	}

	// 处理回复中的挑战信息
	err = reply.Challenge.UnmarshalBinary(req.Chlgi)
	if err != nil {
		return nil, fmt.Errorf("unmarshal chlgi: %s", err)
	}

	// 处理回复中的证明信息
	err = reply.Proof.UnmarshalBinary(req.Proofi)
	if err != nil {
		return nil, fmt.Errorf("unmarshal proofi: %s", err)
	}

	return reply, nil
}
