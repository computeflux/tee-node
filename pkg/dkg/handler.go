package dkg

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/wetee-dao/tee-dsecret/pkg/model"
	"github.com/wetee-dao/tee-dsecret/pkg/util"
)

// HandleDkg 处理不同的DKG消息类型
// msg: 被处理的消息对象
// 返回：可能的错误
func (dkg *DKG) handleDkg(msg *model.DkgMessage) error {
	// 根据消息类型执行相应的处理逻辑
	switch msg.Type {
	case "consensus":
		consensusMsg := model.ConsensusMsg{}
		err := json.Unmarshal(msg.Payload, &consensusMsg)
		if err != nil {
			return fmt.Errorf("unmarshal consensus msg: %w", err)
		}
		// 开始共识
		err = dkg.startConsensus(consensusMsg)
		if err != nil {
			util.LogError("DEAL <<<<<<<< ERROR", "HandleDeal:", err)
		}
		return err
	case "consensus_to_newpoch":
		err := dkg.RevPartialSig(msg.From, msg.Payload)
		if err != nil {
			util.LogError("DEAL <<<<<<<< ERROR", "SideKeyRebuild:", err)
		}
		return err
	case "deal":
		// 处理交易消息
		err := dkg.handleDeal(msg.From, msg.Payload)
		if err != nil {
			util.LogError("DEAL <<<<<<<< ERROR", "HandleDeal:", err)
		}
		return err
	case "deal_resp":
		// 处理交易响应消息
		err := dkg.handleDealResp(msg.From, msg.Payload)
		if err != nil {
			util.LogError("DEAL <<<<<<<<<<<<<<<< ERROR", "HandleDealResp:", err)
		}
		return err
	case "random_deal_batch":
		err := dkg.handleRandomDealBatch(msg.From, msg.Payload)
		if err != nil {
			util.LogError("RandomDKG", "handleRandomDealBatch error:", err)
		}
		return err
	case "random_deal_resp_batch":
		err := dkg.handleRandomDealRespBatch(msg.From, msg.Payload)
		if err != nil {
			util.LogError("RandomDKG", "handleRandomDealRespBatch error:", err)
		}
		return err
	// case "justification":
	// 	// 处理证明消息
	// 	err := dkg.HandleJustification(msg.From, msg.Payload)
	// 	if err != nil {
	// 		util.LogError("DEAL <<<<<<<<<<<<<<<<<<<<<<<<<<<<<<<< ERROR", "HandleJustification:", err)
	// 	}
	// 	return err
	default:
		// 如果消息类型未知，返回错误
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// HandleWorker 处理来自worker的消息
// msg: 待处理的消息
// func (dkg *DKG) handleWorker(msg *model.DkgMessage) error {
// TODO
// 检查链的元数据
// err := chain.MainChain.CheckMetadata()
// if err != nil {
// 	// 记录检查元数据时的错误
// 	util.LogError("CheckMetadata", err)
// 	return err
// }

// 根据消息类型处理不同的逻辑
// switch msg.Type {
// /// -------------------- Proof -----------------------
// case "upload_cluster_proof":
// 	// 处理上传集群证明的消息
// 	hash, err := dkg.HandleUploadClusterProof(msg.Payload, msg.MsgId, msg.From)
// 	if msg.From != "" && msg.MsgId != "" {
// 		// 获取发送方节点
// 		n := dkg.getNode(msg.From)
// 		if n == nil {
// 			// 如果节点不存在，返回错误
// 			return fmt.Errorf("node not found: %s", msg.From)
// 		}
// 		errStr := ""
// 		if err != nil {
// 			// 如果有错误，记录错误信息
// 			errStr = err.Error()
// 		}
// 		// 发送回复消息给节点
// 		if err := dkg.sendToNode(model.SendToNode(n), &model.DkgMessage{
// 			MsgId:   msg.MsgId,
// 			Type:    "upload_cluster_proof_reply",
// 			Payload: hash,
// 			Error:   errStr,
// 		}); err != nil {
// 			// 发送消息失败，返回错误
// 			return errors.New("send to node: " + err.Error())
// 		}
// 	}
// 	return err
// case "sign_cluster_proof":
// 	// 处理签名集群证明的消息
// 	err := dkg.HandleSignClusterProof(msg.Payload, msg.MsgId, msg.From)
// 	if err != nil {
// 		// 记录签名集群证明时的错误
// 		util.LogError("WORKER", "HandleSignClusterProof err: ", err)
// 	}
// 	return err
// case "sign_cluster_proof_reply":
// 	// 处理签名集群证明回复的消息
// 	err := dkg.HandleSignClusterProofReply(msg.Payload, msg.MsgId, msg.From)
// 	if err != nil {
// 		// 记录签名集群证明回复时的错误
// 		util.LogError("WORKER", "HandleSignClusterProofReply err: ", err)
// 	}
// 	return err
// /// -------------------- Reencrypt -----------------------
// case "reencrypt_secret_remote_request":
// 	// 发送加密的密钥请求
// 	key, err := dkg.SendEncryptedSecretRequest(msg.Payload, msg.MsgId, msg.From)
// 	if msg.From != "" && msg.MsgId != "" {
// 		// 获取发送方节点
// 		n := dkg.getNode(msg.From)
// 		if n == nil {
// 			// 如果节点不存在，返回错误
// 			return fmt.Errorf("node not found: %s", msg.From)
// 		}

// 		errStr := ""
// 		var keyBt []byte
// 		if err != nil {
// 			// 如果有错误，记录错误信息
// 			errStr = err.Error()
// 		} else {
// 			// 将密钥转换为字节
// 			keyBt, _ = json.Marshal(key)
// 		}

// 		// 发送回复消息给节点
// 		if err := dkg.sendToNode(model.SendToNode(n), &model.DkgMessage{
// 			MsgId:   msg.MsgId,
// 			Type:    "reencrypt_secret_remote_reply",
// 			Payload: keyBt,
// 			Error:   errStr,
// 		}); err != nil {
// 			// 发送消息失败，返回错误
// 			return errors.New("send to node: " + err.Error())
// 		}
// 	}
// 	return err
// case "reencrypt_secret_request":
// 	// 处理重新加密密钥请求的消息
// 	err := dkg.HandleProcessReencrypt(msg.Payload, msg.MsgId, msg.From)
// 	if err != nil {
// 		// 记录处理重新加密密钥请求时的错误
// 		util.LogError("secret", "HandleReencryptSecretRequest err: ", err)
// 	}
// 	return err
// case "reencrypted_secret_reply":
// 	// 处理重新加密的密钥回复的消息
// 	err := dkg.HandleReencryptedShare(msg.Payload, msg.MsgId, msg.From)
// 	if err != nil {
// 		// 记录处理重新加密的密钥回复时的错误
// 		util.LogError("secret", "HandleReencryptSecretRequest err: ", err)
// 	}
// 	return err
// 	default:
// 		// 默认情况下不执行任何操作，返回nil
// 		return nil
// 	}
// }

// Handle pub msg data save
func (r *DKG) handleSecretSave() {
	r.Peer.Sub("secret", func(msgWrap any) error {
		msg, ok := msgWrap.(*model.DkgMessage)
		if !ok {
			return errors.New("invalid message type")
		}
		// 解析消息
		var datas []model.Kvs
		err := json.Unmarshal(msg.Payload, &datas)
		if err != nil {
			fmt.Println("Error unmarshalling message data: ", err)
			return err
		}

		for _, data := range datas {
			fmt.Println("-------------------------Save key: ", data.K)
			err := model.SetKey("secret", data.K, data.V)
			if err != nil {
				fmt.Println("Error setting key: ", err)
				continue
			}
		}

		return nil
	})
}
