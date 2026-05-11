package sidechain

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/centrifuge/go-substrate-rpc-client/v4/types"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/model/protoio"
	proxy_reenc "github.com/wetee-dao/tee-dsecret/pkg/proxy-reenc"
	"go.dedis.ch/kyber/v4/suites"
)

const (
	SecretSpace = "secret"
	DiskSpace   = "disk"
)

func (s *SideChain) Encrypt(data []byte) ([]byte, error) {
	// 获取DKG的公钥，用于加密过程
	suite := suites.MustFind("Ed25519")
	dkgPubKey, err := GetDkgPubkey()
	if err != nil {
		return nil, fmt.Errorf("get dkg pubkey: %w", err)
	}
	dkgPub := model.PubKeyFromByte(dkgPubKey.ToBytes())

	// 加密秘密环境变量
	encCmt, encScrt := proxy_reenc.EncryptSecret(suite, dkgPub.Point(), data)

	// 将加密的承诺（encCmt）转换为字节切片格式
	rawEncCmt, err := encCmt.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal encCmt: %s", err)
	}

	// 将加密的秘密（encScrt）转换为字节切片格式
	rawEncScrt := make([][]byte, len(encScrt))
	for i, encScrti := range encScrt {
		rawEncScrti, err := encScrti.MarshalBinary()
		if err != nil {
			return nil, fmt.Errorf("marshal encScrt: %s", err)
		}
		rawEncScrt[i] = rawEncScrti
	}

	// save to db
	secretStore := &model.SecretStore{
		RawEncCmt:  rawEncCmt,
		RawEncScrt: rawEncScrt,
	}

	buf := new(bytes.Buffer)
	protoio.WriteMessage(secretStore, buf)

	return buf.Bytes(), nil
}

func (s *SideChain) SaveSecret(user types.H160, index uint64, data []byte, txn *model.Txn) error {
	return txn.Set(model.ComboNamespaceKey(SecretSpace, user.Hex()+"_"+fmt.Sprint(index)), data)
}

func (s *SideChain) GetSecrets(user types.H160, indexs []uint64) (map[uint64]*model.SecretStore, error) {
	list, keys, err := model.GetProtoMessageList[model.SecretStore](SecretSpace, user.Hex())
	if err != nil {
		return nil, err
	}

	ids := make(map[uint64]*model.SecretStore)
	for _, index := range indexs {
		ids[index] = new(model.SecretStore)
	}

	for i, key := range keys {
		parts := strings.Split(string(key), "_")
		if len(parts) < 3 {
			return nil, fmt.Errorf("invalid key format: %s", string(key))
		}
		k := parts[2]
		index, err := strconv.ParseUint(k, 10, 64)
		if err != nil {
			return nil, err
		}

		if _, ok := ids[index]; ok {
			ids[index] = list[i]
		}
	}

	return ids, nil
}

func (s *SideChain) SaveDiskKey(user types.H160, index uint64, data []byte, txn *model.Txn) error {
	return txn.Set(model.ComboNamespaceKey(DiskSpace, user.Hex()+"_"+fmt.Sprint(index)), data)
}

func (s *SideChain) GetDiskKeys(user types.H160, indexs []uint64) (map[uint64]*model.SecretStore, error) {
	list, keys, err := model.GetProtoMessageList[model.SecretStore](DiskSpace, user.Hex())
	if err != nil {
		return nil, err
	}

	ids := make(map[uint64]*model.SecretStore)
	for _, index := range indexs {
		ids[index] = new(model.SecretStore)
	}

	for i, key := range keys {
		k := strings.Split(string(key), "_")[2]
		index, err := strconv.ParseUint(k, 10, 64)
		if err != nil {
			return nil, err
		}

		if _, ok := ids[index]; ok {
			ids[index] = list[i]
		}
	}

	return ids, nil
}
