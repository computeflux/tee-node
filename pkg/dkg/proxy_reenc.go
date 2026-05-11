// // / Copyright (c) 2022 Sourcenetwork Developers. All rights reserved.
// // / copy from https://github.com/sourcenetwork/orbis-go

package dkg

// import (
// 	"encoding/json"
// 	"fmt"
// 	"time"

// 	"go.dedis.ch/kyber/v4/share"

// 	proxy_reenc "github.com/wetee-dao/tee-dsecret/pkg/dkg/proxy-reenc"
// 	"github.com/wetee-dao/tee-dsecret/pkg/model"
// )

// // SendEncryptedSecretRequest 发送加密的秘密请求，并等待指定数量的节点响应
// // payload 是加密的秘密请求的负载
// // msgID 是消息的唯一标识符，用于跟踪和匹配响应
// // OrgId 是组织的标识符，用于确定秘密请求的目标组织
// func (d *DKG) SendEncryptedSecretRequest(payload []byte, msgID string, OrgId string) (*model.ReencryptSecret, error) {
// 	// 同步访问共享资源
// 	d.mu.Lock()
// 	// 为消息ID预分配一个通道，用于接收响应
// 	d.preRecerve[msgID] = make(chan any)
// 	d.mu.Unlock()

// 	// 向所有节点发送加密秘密请求
// 	for _, n := range d.Nodes {
// 		err := d.sendToNode(model.SendToNode(&n.P2pId), &model.DkgMessage{
// 			Type:    "reencrypt_secret_request",
// 			Payload: payload,
// 		})
// 		if err != nil {
// 			fmt.Println("send reencrypt secret request: ", err)
// 		}
// 	}

// 	// 准备收集至少达到阈值数量的节点响应
// 	psk := make([]*share.PubShare, 0, len(d.Nodes))
// 	for range d.Threshold {
// 		select {
// 		case d := <-d.preRecerve[msgID]:
// 			data := d.(*share.PubShare)
// 			psk = append(psk, data)
// 		case <-time.After(30 * time.Second):
// 			fmt.Println("Timeout receiving from channel")
// 			return nil, fmt.Errorf("timeout receiving from channel")
// 		}
// 	}

// 	// 同步删除不再需要的消息ID通道
// 	d.mu.Lock()
// 	delete(d.preRecerve, msgID)
// 	d.mu.Unlock()

// 	// 从收集的响应中恢复重加密承诺
// 	xncCmt, err := proxy_reenc.Recover(d.Suite, psk, d.Threshold, len(d.Nodes))
// 	if err != nil {
// 		return nil, fmt.Errorf("recover reencrypt reply: %s", err)
// 	}

// 	// 解析原始请求以获取秘密ID
// 	req := &model.ReencryptSecretRequest{}
// 	err = json.Unmarshal(payload, req)
// 	if err != nil {
// 		return nil, fmt.Errorf("unmarshal ReencryptSecretRequest: %w", err)
// 	}

// 	// 根据请求的秘密ID获取加密的秘密数据
// 	scrt, err := d.GetSecretData(req.SecretId)
// 	if err != nil {
// 		return nil, fmt.Errorf("encrypted secret for %s not found", req.SecretId)
// 	}

// 	// 准备并返回重加密响应
// 	xncCmtBt, _ := xncCmt.MarshalBinary()
// 	return &model.ReencryptSecret{
// 		XncCmt:  xncCmtBt,
// 		EncScrt: scrt.EncScrt,
// 	}, nil
// }

// // HandleProcessReencrypt 处理密文重新加密请求
// // 该函数接收一个密文重新加密请求的字节切片，消息ID和组织ID，
// // 并尝试对指定的密文进行重新加密，最后将重新加密的结果发送给目标组织节点
// // 参数:
// // - reqBt: 密文重新加密请求的字节切片
// // - msgID: 消息ID，用于标识消息
// // - OrgId: 组织ID，用于确定重新加密结果的目标节点
// // 返回值:
// // - error: 如果处理过程中发生错误，返回错误信息
// func (d *DKG) HandleProcessReencrypt(reqBt []byte, msgID string, OrgId string) error {
// 	// 解析密文重新加密请求
// 	req := &model.ReencryptSecretRequest{}
// 	err := json.Unmarshal(reqBt, req)
// 	if err != nil {
// 		return fmt.Errorf("HandleProcessReencrypt unmarshal reencrypt secret request: %w", err)
// 	}

// 	// 获取重新加密所需的公钥和密文
// 	rdrPk := req.RdrPk
// 	scrt, err := d.GetSecretData(req.SecretId)
// 	if err != nil {
// 		return fmt.Errorf("get secret: %w", err)
// 	}

// 	// 获取本节点的份额，并进行重新加密操作
// 	share := d.Share()
// 	reply, err := proxy_reenc.Reencrypt(share, scrt, *rdrPk)
// 	if err != nil {
// 		return fmt.Errorf("reencrypt: %w", err)
// 	}

// 	// 将重新加密得到的密钥份额、挑战和证明序列化
// 	xncski, err := reply.Share.V.MarshalBinary()
// 	if err != nil {
// 		return fmt.Errorf("marshal xncski: %w", err)
// 	}

// 	chlgi, err := reply.Challenge.MarshalBinary()
// 	if err != nil {
// 		return fmt.Errorf("marshal chlgi: %w", err)
// 	}

// 	proofi, err := reply.Proof.MarshalBinary()
// 	if err != nil {
// 		return fmt.Errorf("marshal proofi: %w", err)
// 	}

// 	// 构建重新加密的密文份额响应
// 	resp := &model.ReencryptedSecretShare{
// 		SecretId: req.SecretId,
// 		Index:    int32(reply.Share.I),
// 		XncSki:   xncski,
// 		Chlgi:    chlgi,
// 		Proofi:   proofi,
// 	}
// 	bt, _ := json.Marshal(resp)

// 	// 获取目标组织的节点信息
// 	n := d.getNode(OrgId)
// 	if n == nil {
// 		return fmt.Errorf("node not found: %s", OrgId)
// 	}

// 	// 向目标节点发送重新加密的密文份额
// 	err = d.sendToNode(model.SendToNode(n), &model.DkgMessage{
// 		MsgId:   msgID,
// 		Type:    "reencrypted_secret_reply",
// 		Payload: bt,
// 	})

// 	if err != nil {
// 		fmt.Println("send reencrypted secretshare: ", err)
// 	}

// 	return nil
// }

// // HandleReencryptedShare 处理重新加密的份额
// // 该函数接收一个重新加密的份额的二进制表示，消息ID和原始ID作为参数
// // 它验证并处理这个份额，将其存储以备后续使用
// // 参数:
// // - reqBt: 重新加密的份额的二进制表示
// // - msgID: 消息的唯一ID
// // - OrgId: 原始份额的发送者ID
// // 返回值:
// // - error: 如果处理过程中出现任何错误
// func (d *DKG) HandleReencryptedShare(reqBt []byte, msgID string, OrgId string) error {
// 	// 解析重新加密的请求
// 	var req model.ReencryptedSecretShare

// 	err := json.Unmarshal(reqBt, &req)
// 	if err != nil {
// 		return fmt.Errorf("unmarshal reencrypt request: %s", err)
// 	}
// 	fmt.Printf("handling PRE response: secretid=%s from=%s \n", req.SecretId, OrgId)

// 	// 获取重新加密的公钥和对应的密码学套件
// 	rdrPk := req.RdrPk
// 	ste := rdrPk.Suite()

// 	// 初始化重新加密回复
// 	reply := proxy_reenc.ReencryptReply{
// 		Share: share.PubShare{
// 			I: uint32(req.Index),
// 			V: ste.Point().Base(),
// 		},
// 		Challenge: ste.Scalar(),
// 		Proof:     ste.Scalar(),
// 	}

// 	// 处理回复中的份额信息
// 	err = reply.Share.V.UnmarshalBinary(req.XncSki)
// 	if err != nil {
// 		return fmt.Errorf("unmarshal xncski: %s", err)
// 	}

// 	// 处理回复中的挑战信息
// 	err = reply.Challenge.UnmarshalBinary(req.Chlgi)
// 	if err != nil {
// 		return fmt.Errorf("unmarshal chlgi: %s", err)
// 	}

// 	// 处理回复中的证明信息
// 	err = reply.Proof.UnmarshalBinary(req.Proofi)
// 	if err != nil {
// 		return fmt.Errorf("unmarshal proofi: %s", err)
// 	}

// 	// 获取分布式密钥的份额和多项式承诺
// 	distKeyShare := d.Share()
// 	poly := share.NewPubPoly(ste, nil, distKeyShare.Commitments())

// 	// 获取与请求的秘密ID相关联的密钥材料
// 	scrt, err := d.GetSecretData(req.SecretId)
// 	if err != nil {
// 		return fmt.Errorf("getting secret: %w", err)
// 	}
// 	rawEncCmt := scrt.EncCmt

// 	// 解析加密的承诺
// 	encCmt := ste.Point().Base()
// 	err = encCmt.UnmarshalBinary(rawEncCmt)
// 	if err != nil {
// 		return fmt.Errorf("unmarshal encrypted commitment: %s", err)
// 	}

// 	// 验证重新加密的回复
// 	fmt.Printf("handling PRE response: verifying reencrypt reply share")
// 	err = proxy_reenc.Verify(*rdrPk, poly, encCmt, reply)
// 	if err != nil {
// 		return fmt.Errorf("verify reencrypt reply: %s", err)
// 	}

// 	// 检查并存储验证通过的重新加密的份额
// 	if _, ok := d.preRecerve[msgID]; !ok {
// 		return nil
// 	}

// 	d.preRecerve[msgID] <- &reply.Share
// 	return nil
// }

// // GetSecretData 通过给定的消息ID从存储中获取加密的密钥数据
// // 参数 storeMsgID 是用于标识存储中特定密钥的字符串
// // 返回值 *model.Secret 是解析后的密钥对象指针，error 是错误信息（如果有的话）
// func (r *DKG) GetSecretData(storeMsgID string) (*model.Secret, error) {
// 	// 从存储中获取与密钥ID对应的加密数据
// 	buf, err := model.GetKey("secret", storeMsgID)
// 	if err != nil {
// 		// 如果获取过程中出现错误，则返回错误信息
// 		return nil, fmt.Errorf("get secret: %w", err)
// 	}

// 	// 创建一个 model.Secret 类型的实例用于存储解码后的密钥数据
// 	s := new(model.Secret)
// 	// 解析获取到的加密数据，将其转换为密钥对象
// 	err = json.Unmarshal(buf, s)
// 	if err != nil {
// 		// 如果解析过程中出现错误，则返回错误信息
// 		return nil, fmt.Errorf("unmarshal encrypted secret: %w", err)
// 	}

// 	// 返回解析后的密钥对象
// 	return s, nil
// }
