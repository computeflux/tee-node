package dkg

// import (
// 	"context"
// 	"encoding/json"
// 	"fmt"

// 	proxy_reenc "github.com/wetee-dao/tee-dsecret/pkg/dkg/proxy-reenc"
// 	"github.com/wetee-dao/tee-dsecret/pkg/model"
// )

// // SetSecretEnv 通过DKG流程设置秘密环境变量
// // 它接收环境变量，将其加密并存储在系统中
// // 参数:
// //   - ctx: 请求的上下文，用于取消请求或传递其他信息
// //   - env: 要存储的环境变量
// //
// // 返回值:
// //   - SecretEnvWithHash: 包含存储的秘密环境变量的哈希和环境变量本身（未加密）
// //   - error: 如果在处理请求时发生错误，则返回错误
// func (r *DKG) SetSecretEnv(ctx context.Context, env model.Env) (*model.SecretEnvWithHash, error) {
// 	// 将环境变量序列化为JSON格式
// 	scrt, err := json.Marshal(env)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// 创建用于显示的谜文，以显示环境变量的长度而不暴露其真实值
// 	lenvs := make([]*model.LenValue, len(env.Envs))
// 	lfiles := make([]*model.LenValue, len(env.Files))
// 	for i, v := range env.Envs {
// 		lenvs[i] = &model.LenValue{
// 			K: v.K,
// 			V: len(v.V),
// 		}
// 	}
// 	for i, v := range env.Files {
// 		lfiles[i] = &model.LenValue{
// 			K: v.K,
// 			V: len(v.V),
// 		}
// 	}
// 	lenEnv := &model.SecretEnv{
// 		Envs:  lenvs,
// 		Files: lfiles,
// 	}
// 	// 序列化显示用的谜文
// 	pub, _ := json.Marshal(lenEnv)

// 	// 获取DKG的公钥，用于加密过程
// 	dkgPub := r.DkgPubKey

// 	// 加密秘密环境变量
// 	encCmt, encScrt := proxy_reenc.EncryptSecret(r.Suite, dkgPub.Point(), scrt)
// 	// 将加密的承诺（encCmt）转换为字节切片格式
// 	rawEncCmt, err := encCmt.MarshalBinary()
// 	if err != nil {
// 		return nil, fmt.Errorf("marshal encCmt: %s", err)
// 	}

// 	// 将加密的秘密（encScrt）转换为字节切片格式
// 	rawEncScrt := make([][]byte, len(encScrt))
// 	for i, encScrti := range encScrt {
// 		rawEncScrti, err := encScrti.MarshalBinary()
// 		if err != nil {
// 			return nil, fmt.Errorf("marshal encScrt: %s", err)
// 		}
// 		rawEncScrt[i] = rawEncScrti
// 	}

// 	// 创建秘密对象，包含加密后的承诺和秘密
// 	secret := &model.Secret{
// 		EncCmt:  rawEncCmt,
// 		EncScrt: rawEncScrt,
// 	}

// 	// 序列化秘密对象，准备存储
// 	payload, err := json.Marshal(secret)
// 	if err != nil {
// 		return nil, fmt.Errorf("marshal secret: %w", err)
// 	}

// 	// 从序列化的数据生成唯一标识符（CID）
// 	cid, err := model.CidFromBytes(payload)
// 	if err != nil {
// 		return nil, fmt.Errorf("cid from bytes: %w", err)
// 	}

// 	// 使用CID作为标识符，异步存储加密后的环境变量和显示用的谜文
// 	storeMsgID := cid.String()
// 	go r.SetData([]model.Kvs{
// 		{K: storeMsgID, V: payload},
// 		{K: storeMsgID + "_pub", V: pub},
// 	})

// 	// 返回包含CID和未加密环境变量的结构体
// 	return &model.SecretEnvWithHash{
// 		Hash:   storeMsgID,
// 		Secret: lenEnv,
// 	}, nil
// }

// // GetSecretPubEnvData 从存储中获取加密的环境密钥，并将其解析为 SecretEnvWithHash 类型
// // 此方法主要用于从标识为 "secret" 的存储键中获取与 storeMsgID 匹配的公钥信息
// // 它接受一个上下文参数 ctx，该参数主要用于取消操作，以及一个用于标识存储消息的 ID storeMsgID
// // 返回值是 SecretEnvWithHash 类型的指针，其中包含了解密环境密钥及其对应的哈希值，
// // 以及一个错误类型，用于指示操作过程中是否发生了错误
// func (r *DKG) GetSecretPubEnvData(ctx context.Context, storeMsgID string) (*model.SecretEnvWithHash, error) {
// 	// 从存储中获取特定标识的密钥信息
// 	buf, err := model.GetKey("secret", storeMsgID+"_pub")
// 	if err != nil {
// 		// 如果获取密钥过程中发生错误，则返回错误信息
// 		return nil, fmt.Errorf("get secret: %w", err)
// 	}

// 	// 创建 SecretEnv 类型的实例，用于存储解析后的密钥信息
// 	s := new(model.SecretEnv)
// 	// 将获取到的密钥信息解析为 SecretEnv 类型
// 	err = json.Unmarshal(buf, s)
// 	if err != nil {
// 		// 如果解析过程中发生错误，则返回错误信息
// 		return nil, fmt.Errorf("unmarshal encrypted secret: %w", err)
// 	}

// 	// 返回解析后的密钥信息及其对应的哈希值
// 	return &model.SecretEnvWithHash{
// 		Hash:   storeMsgID,
// 		Secret: s,
// 	}, nil
// }
