package dkg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/util"
)

// ── GK 同步（绑定到 DKG 组网流程）──────────────────────────────
//
// 设计目标：新节点只有在成功加入 DKG（完成 initConsensus 或 reConsensus）
// 后，才能通过已有的 P2P 安全通道从已有 GK 的节点同步 Global Key。
//
// 消息类型（复用 DkgMessage，type = "kms_sync_req" / "kms_sync_resp"）
//
// 安全性：
//   - 请求节点必须已经通过 DKG 验证（在 NewNodes / Nodes 列表中）
//   - 响应节点只向已验证的 P2P 节点发送 GK
//   - 所有通信在已有的 TEE P2P 通道内进行

const (
	kmsSyncReq  = "kms_sync_req"
	kmsSyncResp = "kms_sync_resp"
)

var (
	kmsSyncOnce    sync.Once
	kmsSyncTimeout = 60 * time.Second
)

// KmsSyncReq GK 同步请求
type KmsSyncReq struct {
	NodeID string `json:"node_id"` // 请求者 Ed25519 SS58
	Epoch  uint32 `json:"epoch"`   // 当前 DKG epoch
}

// KmsSyncResp GK 同步响应
type KmsSyncResp struct {
	KeyB64  string `json:"key_b64"` // base64(GK)
	Version uint32 `json:"version"` // GK 版本
	Epoch   uint32 `json:"epoch"`   // 响应者 DKG epoch
}

// ── 发起方逻辑：新节点在 DKG 成功后请求 GK ─────────────────────

// SyncGlobalKeyFromPeers 在 DKG 初始化/重新共识成功后调用。
// 本节点如果没有 GK，则向已知 peers 广播请求，等待任意一个有 GK 的节点响应。
func (dkg *DKG) SyncGlobalKeyFromPeers() {
	// 如果本地已有 GK，无需同步
	if model.HasGlobalKey() {
		util.LogWithYellow("TEE KMS", "local GK already exists, skip sync")
		return
	}

	// 只尝试一次
	kmsSyncOnce.Do(func() {
		go dkg.doSyncGlobalKey()
	})
}

func (dkg *DKG) doSyncGlobalKey() {
	util.LogWithYellow("TEE KMS", "start syncing GK from peers after DKG join...")

	req := KmsSyncReq{
		NodeID: dkg.Signer.GetPublicForSuite(model.SuiteEd25519).SS58(),
	}
	dkg.mu.RLock()
	req.Epoch = dkg.Epoch
	if dkg.Epoch == 0 {
		req.Epoch = dkg.NewEpoch
	}
	dkg.mu.RUnlock()

	bt, err := json.Marshal(req)
	if err != nil {
		util.LogError("TEE KMS", "marshal sync req:", err)
		return
	}

	// 广播给所有已知节点（包括旧节点和新节点）
	msg := &model.DkgMessage{
		Type:    kmsSyncReq,
		Payload: bt,
	}

	// 先发给旧节点
	dkg.mu.RLock()
	olds := make([]*model.Validator, len(dkg.Nodes))
	copy(olds, dkg.Nodes)
	dkg.mu.RUnlock()

	for _, node := range olds {
		if node == nil {
			continue
		}
		_ = dkg.sendToNode(model.SendToNode(&node.P2pId), msg)
	}

	// 等待响应（通过 handleKmsSync 回调写入）
	time.Sleep(kmsSyncTimeout)

	if model.HasGlobalKey() {
		util.LogWithGreen("TEE KMS", "GK synced successfully from peers")
	} else {
		util.LogWithRed("TEE KMS", "GK sync timeout — no peer responded with GK")
	}
}

// ── 响应方逻辑：已有 GK 的节点处理请求 ─────────────────────────

func (dkg *DKG) handleKmsSync(msg *model.DkgMessage) error {
	switch msg.Type {
	case kmsSyncReq:
		return dkg.handleKmsSyncReq(msg)
	case kmsSyncResp:
		return dkg.handleKmsSyncResp(msg)
	}
	return nil
}

// handleKmsSyncReq 已有 GK 的节点收到新节点的 GK 请求
func (dkg *DKG) handleKmsSyncReq(msg *model.DkgMessage) error {
	// 本节点没有 GK，无法响应
	if !model.HasGlobalKey() {
		return nil
	}

	var req KmsSyncReq
	if err := json.Unmarshal(msg.Payload, &req); err != nil {
		return fmt.Errorf("unmarshal kms sync req: %w", err)
	}

	// 验证请求者身份：必须在当前 Nodes 或 NewNodes 中
	dkg.mu.RLock()
	allNodes := make([]*model.Validator, 0, len(dkg.Nodes)+len(dkg.NewNodes))
	allNodes = append(allNodes, dkg.Nodes...)
	allNodes = append(allNodes, dkg.NewNodes...)
	dkg.mu.RUnlock()

	var found bool
	for _, n := range allNodes {
		if n == nil {
			continue
		}
		if n.ValidatorId.SS58() == req.NodeID {
			found = true
			break
		}
	}
	if !found {
		util.LogWithYellow("TEE KMS", "reject GK sync req from unknown node:", req.NodeID)
		return nil
	}

	// 导出 GK
	keyB64, version, err := model.ExportGlobalKeyBase64()
	if err != nil {
		util.LogError("TEE KMS", "export GK failed:", err)
		return nil
	}

	dkg.mu.RLock()
	epoch := dkg.Epoch
	if epoch == 0 {
		epoch = dkg.NewEpoch
	}
	dkg.mu.RUnlock()

	resp := KmsSyncResp{
		KeyB64:  keyB64,
		Version: version,
		Epoch:   epoch,
	}
	bt, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal kms sync resp: %w", err)
	}

	// 直接回复给请求者（msg.From 是对方 P2P ID）
	fromPub, err := model.PubKeyFromHex(msg.From)
	if err != nil {
		return fmt.Errorf("parse from pubkey: %w", err)
	}

	util.LogWithYellow("TEE KMS", "sending GK to new node:", req.NodeID)
	return dkg.sendToNode(model.SendToNode(fromPub), &model.DkgMessage{
		Type:    kmsSyncResp,
		Payload: bt,
	})
}

// handleKmsSyncResp 新节点收到 GK 响应
func (dkg *DKG) handleKmsSyncResp(msg *model.DkgMessage) error {
	// 如果本地已有 GK，忽略
	if model.HasGlobalKey() {
		return nil
	}

	var resp KmsSyncResp
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return fmt.Errorf("unmarshal kms sync resp: %w", err)
	}

	// 验证响应者身份：必须在当前 Nodes 中（必须是老节点）
	dkg.mu.RLock()
	allNodes := make([]*model.Validator, 0, len(dkg.Nodes)+len(dkg.NewNodes))
	allNodes = append(allNodes, dkg.Nodes...)
	allNodes = append(allNodes, dkg.NewNodes...)
	dkg.mu.RUnlock()

	var found bool
	for _, n := range allNodes {
		if n == nil {
			continue
		}
		if n.P2pId.String() == msg.From {
			found = true
			break
		}
	}
	if !found {
		util.LogWithYellow("TEE KMS", "reject GK sync resp from unknown peer:", msg.From)
		return nil
	}

	// 导入 GK
	key, err := base64.StdEncoding.DecodeString(resp.KeyB64)
	if err != nil {
		return fmt.Errorf("decode base64 GK: %w", err)
	}
	if err := model.ImportGlobalKey(key, resp.Version); err != nil {
		return fmt.Errorf("import GK: %w", err)
	}

	util.LogWithGreen("TEE KMS", "GK synced from peer:", msg.From, "version:", resp.Version, "epoch:", resp.Epoch)
	return nil
}
