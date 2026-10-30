package graph

// 地址详情页（浏览器系列卡片 ④）后端辅助逻辑。
//
// 注意：不要把这些代码放进 chain.resolvers.go —— 该文件由 gqlgen 重新生成，
// 非 resolver 的辅助代码会在重新生成时被丢弃（与 erc20.go 同样的约定）。
// 本文件不受 gqlgen 管理，安全。
//
// 提供两个能力：
//   - 地址余额：复用 teeChain.ContractQuery 走 gov 合约 balance_of（与 dapp 前端同一路径）。
//   - 地址交易列表：从本节点 CometBFT BlockStore 最新区块向前扫描，解码每笔合约交易，
//     按「收 / 发」方向归类（caller=发，transfer 的 to=收），提取金额与对手方。

import (
	"bytes"
	"encoding/hex"
	"time"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	store "github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// addressTxsScanWindow 从最新区块向前扫描的最大区块数（限制单次查询成本）。
const addressTxsScanWindow = 1000

// addressTxsMaxMatches 单次查询最多收集的交易数（达到即截断并标记 hasMore）。
const addressTxsMaxMatches = 500

// addressTxsDefaultPageSize 默认分页大小。
const addressTxsDefaultPageSize = 20

// addressTxsMaxPageSize 分页大小上限。
const addressTxsMaxPageSize = 100

// gov 转账方法选择器（mutates=true，与合约 dispatch 一致）。
var (
	govTransferSelector     = store.MethodToSelectorBytes("transfer")
	govTransferFromSelector = store.MethodToSelectorBytes("transfer_from")
)

// govBalanceOfSelector 是 balance_of 的查询选择器。
// 查询方法按 mutates=false 生成（见 gov_gen.go / abis/gov.json：0x87e0ff32），
// 而 ContractQuery 内部默认按 mutates=true 计算 selector，二者不同，
// 因此这里显式用 mutates=false 生成并以 hex 传入（ContractQuery 对 8 位 hex 直接解析、忽略 mutates）。
var govBalanceOfSelector = store.MethodToSelectorWithMutates("balance_of", false)

// balanceCallerTypes 是真实用户账户可能的 callerType（T）取值。
// T=0 仅对应 TEE 链节点账户，普通用户不会命中；
// T=1 sr25519、T=2 EVM(secp256k1)、T=3 ed25519（见 pkg/model/sig_verify.go）。
// gov.members 以完整 UniAddr（T+V）的 SCALE 编码为 key，T 不同 key 不同，
// 而 DecodeCaller 丢弃了 SS58 prefix、无法从原始公钥推导 T，故逐个尝试取首个非零余额。
var balanceCallerTypes = []uint32{1, 2, 3}

// queryGovBalance 查询地址在 gov 合约中的当前余额（十进制字符串）。
// 依次以 T=1/2/3 编码 owner 调用 balance_of，返回首个非零余额；全为零则返回 "0"。
func queryGovBalance(addrRaw []byte) (string, error) {
	if teeChain == nil {
		return "", gqlerror.Errorf("%s", "chain not ready")
	}

	balanceOfHex := hex.EncodeToString(govBalanceOfSelector[:])

	var lastErr error
	for _, t := range balanceCallerTypes {
		owner := store.UniAddr{T: t, V: addrRaw}
		ownerBytes, err := store.ScaleEncode(owner)
		if err != nil {
			return "", gqlerror.Errorf("%s", "encode owner: "+err.Error())
		}

		out, err := teeChain.ContractQuery(addrRaw, 0, "gov", balanceOfHex, [][]byte{ownerBytes})
		if err != nil {
			lastErr = err
			continue
		}

		var amt store.U256
		if err := store.ScaleDecode(&amt, out); err != nil {
			lastErr = err
			continue
		}
		if amt.Int != nil && amt.Int.Sign() != 0 {
			return amt.String(), nil
		}
	}

	// 所有 T 均无余额（或查询失败）：返回 0，避免对真实用户误报非零。
	if lastErr != nil {
		return "", gqlerror.Errorf("%s", "balance_of: "+lastErr.Error())
	}
	return "0", nil
}

// clampPage 将 (page, size) 收敛为安全的 [start, end) 切片区间。
// page 来自 GraphQL Int 且无上限，直接 (page-1)*size 在 page 极大时会环绕为负数，
// 导致 all[start:end] 负下标 panic；这里先按 total 收敛 page，保证 0 <= start <= end <= total。
func clampPage(page, size, total int) (start, end int) {
	if size <= 0 {
		size = 1
	}
	if page < 1 {
		page = 1
	}
	// 先按 total 收敛 page，避免 (page-1)*size 溢出为负数
	if page-1 > total/size {
		page = total/size + 1
	}
	start = (page - 1) * size
	end = start + size
	if end > total {
		end = total
	}
	return start, end
}

// ss58Of 把原始公钥字节编码为 SS58 地址（ed25519, prefix 42）。空字节返回空串。
func ss58Of(pub []byte) string {
	if len(pub) == 0 {
		return ""
	}
	return crypto.SS58Encode(pub, 42)
}

// decodeTransferArgs 解码 transfer(to, value) 参数：to(UniAddr) + value(U256)。
func decodeTransferArgs(args [][]byte) (to []byte, value string, ok bool) {
	if len(args) < 2 {
		return nil, "", false
	}
	var owner store.UniAddr
	if err := store.ScaleDecode(&owner, args[0]); err != nil {
		return nil, "", false
	}
	var amt store.U256
	if err := store.ScaleDecode(&amt, args[1]); err != nil {
		return nil, "", false
	}
	return owner.V, amt.String(), true
}

// decodeTransferFromArgs 解码 transfer_from(from, to, amount) 参数。
func decodeTransferFromArgs(args [][]byte) (from, to []byte, value string, ok bool) {
	if len(args) < 3 {
		return nil, nil, "", false
	}
	var fromAddr store.UniAddr
	if err := store.ScaleDecode(&fromAddr, args[0]); err != nil {
		return nil, nil, "", false
	}
	var toAddr store.UniAddr
	if err := store.ScaleDecode(&toAddr, args[1]); err != nil {
		return nil, nil, "", false
	}
	var amt store.U256
	if err := store.ScaleDecode(&amt, args[2]); err != nil {
		return nil, nil, "", false
	}
	return fromAddr.V, toAddr.V, amt.String(), true
}

// classifyTx 解码单笔原始交易，判断是否涉及 addrRaw，并返回方向 / 金额 / 对手方。
// 仅处理 gov 合约交易（余额变动来源）；非合约交易（空块 / 纪元 / 主链同步）不涉及地址余额。
// hashHex 为交易哈希（hex，无 0x 前缀），由调用方在 rawTx 仍为 types.Tx 时计算传入。
func classifyTx(rawTx []byte, addrRaw []byte, height, index int, timeStr, hashHex string) (*model.AddressTx, bool) {
	txbox := new(model.Tx)
	if err := protoio.Unmarshal(rawTx, txbox); err != nil {
		return nil, false
	}
	sysCall := new(model.SysCall)
	if err := protoio.Unmarshal(txbox.GetCall(), sysCall); err != nil {
		return nil, false
	}
	contract := sysCall.GetContract()
	if contract == nil || string(contract.GetName()) != "gov" {
		return nil, false
	}

	caller := txbox.GetCaller()
	callerIsAddr := bytes.Equal(caller, addrRaw)

	method := contract.GetMethod()
	var direction string
	var amount *string
	var counterparty *string

	switch {
	case bytes.Equal(method, govTransferSelector):
		// transfer(to, value)：caller 发出，to 接收
		to, value, ok := decodeTransferArgs(contract.GetArgs())
		if !ok {
			return nil, false
		}
		if callerIsAddr {
			direction = "out"
			amount = &value
			cp := ss58Of(to)
			counterparty = &cp
		} else if bytes.Equal(to, addrRaw) {
			direction = "in"
			amount = &value
			cp := ss58Of(caller)
			counterparty = &cp
		} else {
			return nil, false
		}
	case bytes.Equal(method, govTransferFromSelector):
		// transfer_from(from, to, amount)：to 接收；caller 为授权方（spender）
		from, to, value, ok := decodeTransferFromArgs(contract.GetArgs())
		if !ok {
			return nil, false
		}
		if bytes.Equal(to, addrRaw) {
			direction = "in"
			amount = &value
			cp := ss58Of(from)
			counterparty = &cp
		} else if callerIsAddr {
			direction = "out"
			amount = &value
			cp := ss58Of(to)
			counterparty = &cp
		} else {
			return nil, false
		}
	default:
		// 其他 gov 方法（提案 / 投票 / 治理）：仅当 caller 是该地址时算「发出」，无金额 / 对手方
		if !callerIsAddr {
			return nil, false
		}
		direction = "out"
	}

	return &model.AddressTx{
		Hash:         hashHex,
		Height:       height,
		Index:        index,
		Time:         timeStr,
		Direction:    direction,
		Amount:       amount,
		Counterparty: counterparty,
	}, true
}

// scanAddressTxs 从最新区块向前扫描，收集涉及 addrRaw 的交易（时间倒序）。
// 返回：命中的交易列表、窗口内命中总数、是否还有更多（被截断）。
func scanAddressTxs(addrRaw []byte) ([]*model.AddressTx, int, bool, error) {
	node := teechain.TeeChainNode
	if node == nil {
		return nil, 0, false, gqlerror.Errorf("%s", "block store not ready")
	}
	bs := node.BlockStore()
	if bs == nil {
		return nil, 0, false, gqlerror.Errorf("%s", "block store not ready")
	}

	latest := bs.Height()
	base := bs.Base()
	start := latest - addressTxsScanWindow + 1
	if start < base {
		start = base
	}

	matches := make([]*model.AddressTx, 0, 64)
	truncated := false

	for h := latest; h >= start; h-- {
		block := bs.LoadBlock(h)
		if block == nil {
			continue
		}
		timeStr := block.Header.Time.UTC().Format(time.RFC3339)
		for i, rawTx := range block.Data.Txs {
			hashHex := hex.EncodeToString(rawTx.Hash())
			item, ok := classifyTx(rawTx, addrRaw, int(h), i, timeStr, hashHex)
			if !ok {
				continue
			}
			matches = append(matches, item)
			if len(matches) >= addressTxsMaxMatches {
				truncated = true
				break
			}
		}
		if truncated {
			break
		}
	}

	return matches, len(matches), truncated, nil
}
