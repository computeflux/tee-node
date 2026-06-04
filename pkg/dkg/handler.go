package dkg

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/dkgtypes"
	"github.com/computeflux/tee-provider/pkg/domain"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/store"
	"github.com/computeflux/tee-provider/pkg/util"
)

// HandleDkg 处理不同的DKG消息类型
// msg: 被处理的消息对象
// 返回：可能的错误
func (dkg *DKG) handleDkg(msg *model.DkgMessage) error {
	// 根据消息类型执行相应的处理逻辑
	switch msg.Type {
	case "consensus":
		consensusMsg := dkgtypes.ConsensusMsg{}
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
	default:
		// 如果消息类型未知，返回错误
		return fmt.Errorf("unknown message type: %s", msg.Type)
	}
}

// handleSecretSave handles shared secret data persistence from P2P messages.
func (r *DKG) handleSecretSave() {
	r.Peer.Sub("secret", func(msgWrap any) error {
		msg, ok := msgWrap.(*model.DkgMessage)
		if !ok {
			return errors.New("invalid message type")
		}
		// 解析消息
		var datas []domain.Kvs
		err := json.Unmarshal(msg.Payload, &datas)
		if err != nil {
			fmt.Println("Error unmarshalling message data: ", err)
			return err
		}

		for _, data := range datas {
			fmt.Println("-------------------------Save key: ", data.K)
			err := store.SetKey("secret", data.K, data.V)
			if err != nil {
				fmt.Println("Error setting key: ", err)
				continue
			}
		}

		return nil
	})
}
