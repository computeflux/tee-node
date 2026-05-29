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

// ── GK + 配置 同步（绑定到 DKG 组网流程）──────────────────────────────
//
// 设计目标：新节点在 DKG 组网成功后，一次性从已有节点同步 Global Key + 所有密封配置。
// 将 GK 同步和配置同步合并为一次请求/响应，减少 P2P 往返。
//
// 消息类型（复用 DkgMessage，type = "kms_sync_req" / "kms_sync_resp"）
//
// 安全性：
//   - 请求节点必须已经通过 DKG 验证（在 NewNodes / Nodes 列表中）
//   - 响应节点只向已验证的 P2P 节点发送 GK + 配置
//   - 所有通信在已有的 TEE P2P 通道内进行
//   - 配置是 GK 加密后的密文，没有 GK 无法解密

const (
	kmsSyncReq  = "kms_sync_req"
	kmsSyncResp = "kms_sync_resp"
)

var (
	kmsSyncOnce    sync.Once
	kmsSyncTimeout = 60 * time.Second
)

// KmsSyncReq 同步请求（GK + 配置）
type KmsSyncReq struct {
	NodeID string `json:"node_id"` // 请求者 Ed25519 SS58
	Epoch  uint32 `json:"epoch"`   // 当前 DKG epoch
}

// KmsSyncResp 同步响应（GK + 所有 namespace 配置）
// ConfigValues: namespace -> key -> GK 加密后的 value
type KmsSyncResp struct {
	KeyB64       string                       `json:"key_b64"`       // base64(GK)
	Version      uint32                       `json:"version"`       // GK 版本
	Epoch        uint32                       `json:"epoch"`         // 响应者 DKG epoch
	ConfigValues map[string]map[string][]byte `json:"config_values"` // GK 加密后的配置
}

// ── 发起方逻辑：新节点在 DKG 成功后请求 GK + 配置 ─────────────────────

// SyncGlobalKeyFromPeers 在 DKG 初始化/重新共识成功后调用。
// 本节点如果没有 GK，则向已知 peers 广播请求，同时同步所有密封配置。
func (dkg *DKG) SyncGlobalKeyFromPeers() {
	// 如果本地已有 GK 且已有配置，无需同步
	if model.HasGlobalKey() && model.HasConfigNamespace("auth") {
		util.LogWithYellow("TEE KMS", "local GK and config already exist, skip sync")
		return
	}

	// 只尝试一次
	kmsSyncOnce.Do(func() {
		go dkg.doSyncAll()
	})
}

func (dkg *DKG) doSyncAll() {
	util.LogWithYellow("TEE KMS", "start syncing GK + config from peers after DKG join...")

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

	// 广播给所有旧节点
	msg := &model.DkgMessage{
		Type:    kmsSyncReq,
		Payload: bt,
	}

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

	gkOk := model.HasGlobalKey()
	cfgOk := model.HasConfigNamespace("auth")
	if gkOk && cfgOk {
		util.LogWithGreen("TEE KMS", "GK + config synced successfully from peers")
	} else if gkOk {
		util.LogWithYellow("TEE KMS", "GK synced, but config sync timeout")
	} else {
		util.LogWithRed("TEE KMS", "GK sync timeout — no peer responded")
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

// handleKmsSyncReq 已有 GK 的节点收到同步请求
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
		util.LogWithYellow("TEE KMS", "reject sync req from unknown node:", req.NodeID)
		return nil
	}

	// 导出 GK
	keyB64, version, err := model.ExportGlobalKeyBase64()
	if err != nil {
		util.LogError("TEE KMS", "export GK failed:", err)
		return nil
	}

	// 导出所有 namespace 配置
	configValues := make(map[string]map[string][]byte)
	gk, _ := model.GetGlobalKey()
	if gk != nil {
		for _, ns := range []string{"auth"} {
			if model.HasConfigNamespace(ns) {
				values, _, err := model.ExportConfigMap(ns, gk)
				if err == nil {
					configValues[ns] = values
				}
			}
		}
	}

	dkg.mu.RLock()
	epoch := dkg.Epoch
	if epoch == 0 {
		epoch = dkg.NewEpoch
	}
	dkg.mu.RUnlock()

	resp := KmsSyncResp{
		KeyB64:       keyB64,
		Version:      version,
		Epoch:        epoch,
		ConfigValues: configValues,
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

	util.LogWithYellow("TEE KMS", "sending GK + config to new node:", req.NodeID)
	return dkg.sendToNode(model.SendToNode(fromPub), &model.DkgMessage{
		Type:    kmsSyncResp,
		Payload: bt,
	})
}

// handleKmsSyncResp 新节点收到 GK + 配置响应
func (dkg *DKG) handleKmsSyncResp(msg *model.DkgMessage) error {
	var resp KmsSyncResp
	if err := json.Unmarshal(msg.Payload, &resp); err != nil {
		return fmt.Errorf("unmarshal kms sync resp: %w", err)
	}

	// 验证响应者身份：必须在当前 Nodes 中
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
		util.LogWithYellow("TEE KMS", "reject sync resp from unknown peer:", msg.From)
		return nil
	}

	// 导入 GK（如果还没有）
	if !model.HasGlobalKey() {
		key, err := base64.StdEncoding.DecodeString(resp.KeyB64)
		if err != nil {
			return fmt.Errorf("decode base64 GK: %w", err)
		}
		if err := model.ImportGlobalKey(key, resp.Version); err != nil {
			return fmt.Errorf("import GK: %w", err)
		}
		util.LogWithGreen("TEE KMS", "GK synced from peer:", msg.From, "version:", resp.Version)
	}

	// 导入配置（如果还没有）
	if resp.ConfigValues != nil {
		for ns, values := range resp.ConfigValues {
			if model.HasConfigNamespace(ns) {
				continue
			}
			if err := model.ImportConfigMap(ns, values, resp.Version); err != nil {
				util.LogError("TEE KMS", "import config namespace", ns, ":", err)
			} else {
				util.LogWithGreen("TEE KMS", "config namespace", ns, "synced from peer")
			}
		}
	}

	return nil
}
