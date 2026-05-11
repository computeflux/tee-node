package dkg

// import (
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"time"

// 	stypes "github.com/centrifuge/go-substrate-rpc-client/v4/types"
// 	uuid "github.com/satori/go.uuid"
// 	gtypes "github.com/wetee-dao/tee-dsecret/pkg/chains/revive/generated/types"
// 	"github.com/wetee-dao/tee-dsecret/pkg/model"
// )

// // HandleUploadClusterProof 函数处理上传集群证明的逻辑
// func (dkg *DKG) HandleUploadClusterProof(data []byte, msgID string, OrgId string) ([]byte, error) {
// 	// 解析 JSON 数据为 TeeCall 结构体
// 	workerReport := &model.TeeCall{}
// 	err := json.Unmarshal(data, workerReport)
// 	if err != nil {
// 		return nil, err
// 	}

// 	// 如果没有提供 msgID，则生成一个新的 UUID
// 	if msgID == "" {
// 		msgID = uuid.NewV4().String()
// 	}

// 	// 上锁，创建一个接收消息的管道，并解锁
// 	dkg.mu.Lock()
// 	dkg.preRecerve[msgID] = make(chan any)
// 	dkg.mu.Unlock()

// 	// 请求节点验证签名
// 	errNum := 0
// 	for _, node := range dkg.Nodes {
// 		// 向节点发送消息
// 		err := dkg.sendToNode(model.SendToNode(&node.P2pId), &model.DkgMessage{
// 			MsgId:   msgID,
// 			Type:    "sign_cluster_proof",
// 			Payload: data,
// 		})
// 		if err != nil {
// 			// 统计发生错误的次数
// 			errNum++
// 		}
// 	}

// 	// 检查有效响应数量是否达到阈值
// 	if len(dkg.Nodes)-errNum < dkg.Threshold {
// 		return nil, errors.New("not enough nodes")
// 	}

// 	// 初始化变量，用于存储公钥和签名
// 	pubs := make([][32]byte, 0, len(dkg.Nodes))
// 	sigs := make([]gtypes.MultiSignature, 0, len(dkg.Nodes))

// 	// 从通道中接收节点的响应
// 	for i := 0; i > dkg.Threshold; i++ {
// 		select {
// 		case d := <-dkg.preRecerve[msgID]:
// 			// 将接收到的数据转换为 ReportSign 结构体
// 			data := d.(*ReportSign)
// 			// 将公钥和签名添加到各自的切片中
// 			pubs = append(pubs, data.account)
// 			sigs = append(sigs, data.sig)
// 		case <-time.After(30 * time.Second):
// 			// 设置超时时间，打印错误信息，并返回错误
// 			fmt.Println("Timeout receiving from channel")
// 			return nil, fmt.Errorf("timeout receiving from channel")
// 		}
// 	}

// 	// 接收到足够的签名后，解锁并移除预留的消息通道
// 	dkg.mu.Lock()
// 	delete(dkg.preRecerve, msgID)
// 	dkg.mu.Unlock()

// 	// TODO
// 	// // 获取交易帐户
// 	// s, err := dkg.Signer.ToSigner()
// 	// if err != nil {
// 	// 	return nil, errors.New("signer to signer: " + err.Error())
// 	// }

// 	// // 获取 MainChain 结构体，检查元数据
// 	// ins := chain.MainChain

// 	// // 从报告中提取 CID
// 	// cid, err := types.CidFromBytes(workerReport.Report)
// 	// if err != nil {
// 	// 	return nil, errors.New("cid from bytes: " + err.Error())
// 	// }

// 	// // 通过地址获取集群信息
// 	// _, account, _ := subkey.SS58Decode(workerReport.Address)
// 	// var account32 [32]byte
// 	// copy(account32[:], account)
// 	// clusterId, ok, err := worker.GetK8sClusterAccountsLatest(chain.MainChain.Api.RPC.State, account32)
// 	// if err != nil || !ok {
// 	// 	return nil, errors.New("get k8s cluster error")
// 	// }

// 	// // 提交证明
// 	// runtimeCall := dsecret.MakeUploadClusterProofCall(clusterId, cid.Bytes(), pubs, sigs)

// 	// call, err := (runtimeCall).AsCall()
// 	// if err != nil {
// 	// 	return nil, errors.New("(runtimeCall).AsCall() error: " + err.Error())
// 	// }

// 	// // 签署并提交交易
// 	// err = ins.SignAndSubmit(s, call, false)
// 	// if err != nil {
// 	// 	return nil, errors.New("submit: " + err.Error())
// 	// }

// 	// // 设置密钥数据
// 	// go dkg.SetData([]types.Kvs{
// 	// 	{K: cid.String(), V: workerReport.Report},
// 	// })

// 	// // 返回 CID 的字节切片，作为提交成功的证明
// 	// return cid.Bytes(), nil

// 	return nil, nil
// }

// func (dkg *DKG) HandleSignClusterProof(data []byte, msgID string, OrgId string) error {
// 	workerReport := &model.TeeCall{}
// 	err := json.Unmarshal(data, workerReport)
// 	if err != nil {
// 		return fmt.Errorf("unmarshal reencrypt secret reply: %w", err)
// 	}

// 	// 校验 Worker
// 	_, err = dkg.VerifyWorker(workerReport)
// 	if err != nil {
// 		return errors.New("HandleSignClusterProof verify worker: " + err.Error())
// 	}

// 	siger := dkg.Signer.ToSigner()

// 	// 计算 cid
// 	cid, err := model.CidFromBytes(workerReport.Report)
// 	if err != nil {
// 		return errors.New("cid from bytes: " + err.Error())
// 	}

// 	// 签名 report
// 	sig, err := siger.Sign(cid.Bytes())
// 	if err != nil {
// 		return errors.New("sign: " + err.Error())
// 	}

// 	n := dkg.getNode(OrgId)
// 	if n == nil {
// 		return fmt.Errorf("node not found: %s", OrgId)
// 	}

// 	// 回传到事务结点
// 	if err := dkg.sendToNode(model.SendToNode(n), &model.DkgMessage{
// 		MsgId:   msgID,
// 		Type:    "sign_cluster_proof_reply",
// 		Payload: sig,
// 	}); err != nil {
// 		return errors.New("send to node: " + err.Error())
// 	}

// 	return nil
// }

// func (dkg *DKG) HandleSignClusterProofReply(data []byte, msgID string, OrgId string) error {
// 	account := dkg.getNode(OrgId)
// 	if account == nil {
// 		return fmt.Errorf("node not found: %s", OrgId)
// 	}

// 	// 计算 account32
// 	bt := account.PublicKey
// 	var account32 [32]byte
// 	copy(account32[:], bt)

// 	// 如果已经满足签名需求，则直接返回
// 	if _, ok := dkg.preRecerve[msgID]; !ok {
// 		return nil
// 	}

// 	sig := stypes.NewSignature(data)
// 	dkg.preRecerve[msgID] <- &ReportSign{
// 		account: account32,
// 		sig: gtypes.MultiSignature{
// 			IsEd25519:       true,
// 			AsEd25519Field0: sig,
// 		},
// 	}

// 	return nil
// }

// type ReportSign struct {
// 	account [32]byte
// 	sig     gtypes.MultiSignature
// }
