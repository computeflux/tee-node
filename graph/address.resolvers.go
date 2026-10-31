package graph

// 本文件实现 address_balance / address_txs 两个 GraphQL 查询（schema 见
// chain.graphqls）。
//
// 注意：不要把这些代码放进 chain.resolvers.go —— 该文件由 gqlgen 重新生成，
// 手写逻辑会被清掉（同 erc20.go 的约定）。此文件不受 gqlgen 管理，安全。

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/cometbft/cometbft/types"
	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/computeflux/tee-provider/pkg/types/crypto"
	teechain "github.com/computeflux/tee-provider/tee-chain"
	cstore "github.com/computeflux/tee-provider/tee-chain/runtime/store"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// maxAddressTxScanBlocks 是 address_txs 从最新区块向前扫描的块数硬上限。
//
// 侧链交易全部是合约调用，地址相关交易（发起 / 转账收付）只能从区块内交易
// 字节解码识别，没有按地址的索引。扫描开销只与该常量 + pageSize 相关，与链上
// 交易总量无关；2000 块在测试网出块节奏下可覆盖数小时至数天的交易历史。
const maxAddressTxScanBlocks = 2000

// maxAddressTxPageSize 是 address_txs 单页条数上限（匿名可达，防资源放大）。
const maxAddressTxPageSize = 100

// defaultAddressTxPageSize 是 address_txs 默认页大小（与前端 PAGE_SIZE 一致）。
const defaultAddressTxPageSize = 20

// normalizeAddress 把用户输入地址归一化为链上 UniAddr + 展示形式。
//
// 支持三种输入（优先级从高到低）：
//  1. EVM 0x 地址：20 字节 hex（带 0x 前缀）→ UniAddr{T: 2, V: 20 字节}
//     （与 ext/eth2uniaddr、graph/auth_helpers.go 的 EVM 归一化一致）；
//  2. SS58 地址：base58 带校验和 → UniAddr{T: 0, V: 公钥字节}；
//  3. hex 公钥：32 字节（ed25519）/ 96 字节（BLS12-381）→ UniAddr{T: 0, V: 公钥}。
//
// 返回的 display 为归一化后的展示地址（EVM 为 0x 小写 hex，SS58/hex 公钥为
// SS58 编码），供前端回显与跳转。
//
// 链上 UniAddr 的 T 是 callerType 命名空间（见 pkg/model/sig_verify.go）：
//   - 0 = 链节点 / 系统账号（用户交易不可用，graph/contract.resolvers.go 拒绝 callerType==0）
//   - 1 = sr25519（polkadot 签名）
//   - 2 = EVM（secp256k1，20 字节地址）
//   - 3 = ed25519（polkadot 签名）
//
// 因此归一化必须按 payload 长度落到正确的 T，且绝不能用 T:0 查用户地址：
//   - 20 字节 → T:2（EVM，与 ext/eth2uniaddr、graph/auth_helpers.go 一致）；
//   - 32 字节 → 无法从地址本身区分 sr25519(T:1) 与 ed25519(T:3)，返回两个候选；
//   - 96 字节 → BLS12-381（验证节点），保留 T:0 候选（与现有 BLS 处理一致）。
//
// 用「候选集」而非单一 T，避免猜错命名空间导致 balance 恒为 0 / 交易匹配不到。
type normalizedAddr struct {
	display    string
	candidates []cstore.UniAddr
}

func normalizeAddress(input string) (normalizedAddr, error) {
	s := strings.TrimSpace(input)
	if s == "" {
		return normalizedAddr{}, fmt.Errorf("empty address")
	}

	// 1. EVM 0x 地址（20 字节）→ T:2
	if strings.HasPrefix(s, "0x") || strings.HasPrefix(s, "0X") {
		b, err := hex.DecodeString(s[2:])
		if err != nil {
			return normalizedAddr{}, fmt.Errorf("invalid evm address: %w", err)
		}
		if len(b) != 20 {
			return normalizedAddr{}, fmt.Errorf("evm address must be 20 bytes, got %d", len(b))
		}
		return normalizedAddr{
			display:    "0x" + hex.EncodeToString(b),
			candidates: []cstore.UniAddr{{T: 2, V: b}},
		}, nil
	}

	// 2. SS58 地址（base58 带校验和）：按 payload 长度归一化 T
	if _, pub, err := crypto.SS58Decode(s); err == nil && len(pub) > 0 {
		return ss58ToNormalized(pub)
	}

	// 3. hex 公钥（20/32/96 字节）：同样按长度归一化 T
	if b, err := hex.DecodeString(s); err == nil && (len(b) == 20 || len(b) == 32 || len(b) == 96) {
		return hexPubToNormalized(b)
	}

	return normalizedAddr{}, fmt.Errorf("unsupported address format (expected SS58 / hex pubkey / 0x evm address)")
}

// ss58ToNormalized 把 SS58 解码出的公钥字节按长度归一化为候选 UniAddr 集。
func ss58ToNormalized(pub []byte) (normalizedAddr, error) {
	switch len(pub) {
	case 20:
		// EVM 地址（TxDetail.From 会把 20 字节 caller 渲染成 SS58，回环到这里）→ T:2
		return normalizedAddr{
			display:    "0x" + hex.EncodeToString(pub),
			candidates: []cstore.UniAddr{{T: 2, V: pub}},
		}, nil
	case 32:
		// sr25519(T:1) / ed25519(T:3) 无法从地址区分，两个候选都查
		return normalizedAddr{
			display:    crypto.SS58Encode(pub, 42),
			candidates: []cstore.UniAddr{{T: 1, V: pub}, {T: 3, V: pub}},
		}, nil
	case 96:
		// BLS12-381（验证节点）
		return normalizedAddr{
			display:    crypto.PubKeyFromByte(pub).SS58(),
			candidates: []cstore.UniAddr{{T: 0, V: pub}},
		}, nil
	default:
		return normalizedAddr{}, fmt.Errorf("unsupported ss58 payload length %d", len(pub))
	}
}

// hexPubToNormalized 把 hex 公钥字节按长度归一化为候选 UniAddr 集。
func hexPubToNormalized(b []byte) (normalizedAddr, error) {
	switch len(b) {
	case 20:
		return normalizedAddr{
			display:    "0x" + hex.EncodeToString(b),
			candidates: []cstore.UniAddr{{T: 2, V: b}},
		}, nil
	case 32:
		return normalizedAddr{
			display:    crypto.PubKeyFromByte(b).SS58(),
			candidates: []cstore.UniAddr{{T: 1, V: b}, {T: 3, V: b}},
		}, nil
	case 96:
		return normalizedAddr{
			display:    crypto.PubKeyFromByte(b).SS58(),
			candidates: []cstore.UniAddr{{T: 0, V: b}},
		}, nil
	}
	return normalizedAddr{}, fmt.Errorf("unsupported hex pubkey length %d", len(b))
}

// uniAddrInSet 判断 a 是否等于候选集中任一 UniAddr。
func uniAddrInSet(a cstore.UniAddr, set []cstore.UniAddr) bool {
	for _, c := range set {
		if uniAddrEqual(a, c) {
			return true
		}
	}
	return false
}

// uniAddrDisplay 把链上 UniAddr 转为展示地址：
// T=2 且 20 字节 → 0x EVM 地址；其余按公钥 SS58 编码（无法识别时返回空串）。
func uniAddrDisplay(a cstore.UniAddr) string {
	if a.T == 2 && len(a.V) == 20 {
		return "0x" + hex.EncodeToString(a.V)
	}
	if len(a.V) == 32 || len(a.V) == 96 {
		return crypto.PubKeyFromByte(a.V).SS58()
	}
	return ""
}

// AddressBalance is the resolver for the address_balance field.
//
// 地址归一化（SS58 / hex 公钥 / EVM 0x → 候选 UniAddr 集）后，对每个候选 T 走
// gov 合约 balanceOf 只读查询，返回首个非零余额（同一公钥只会在一个 T 命名空间
// 有余额）；全为零则返回 "0"。返回十进制余额字符串（U256）。
func (r *queryResolver) AddressBalance(ctx context.Context, address string) (*model.AddressBalance, error) {
	na, err := normalizeAddress(address)
	if err != nil {
		return nil, gqlerror.Errorf("%s", err.Error())
	}
	if teeChain == nil {
		return nil, gqlerror.Errorf("%s", "chain not ready")
	}

	// balanceOf 是查询方法（mutates=false），其 selector 与 mutates=true 不同；
	// ContractQuery 内部走 MethodToSelectorBytes（默认 mutates=true），故这里直接
	// 传查询 selector 的十六进制（MethodToSelector 会走 hex 分支），避免命中
	// GovQuery.ExecQuery 的 default 分支报 unknown method。
	balance := "0"
	for _, cand := range na.candidates {
		ownerArg, err := cstore.ScaleEncode(cand)
		if err != nil {
			return nil, gqlerror.Errorf("%s", "address_balance: encode owner: "+err.Error())
		}
		data, err := teeChain.ContractQuery(cand.V, cand.T, "gov", govBalanceOfSelectorHex, [][]byte{ownerArg})
		if err != nil {
			return nil, gqlerror.Errorf("%s", "address_balance: "+err.Error())
		}
		amt, err := cstore.DecodeScaleArgBytes[cstore.U256](data)
		if err != nil {
			return nil, gqlerror.Errorf("%s", "address_balance: decode balance: "+err.Error())
		}
		if amt.Int.Sign() > 0 {
			balance = amt.String()
			break
		}
	}

	return &model.AddressBalance{Address: na.display, Balance: balance}, nil
}

// AddressTxs is the resolver for the address_txs field.
//
// 从最新区块向前逐块扫描（块数受 maxAddressTxScanBlocks 约束），解码每笔交易
// （model.Tx → model.SysCall → ContractCall），识别与目标地址相关的交易：
//   - out：交易发起方（caller）= 目标地址；
//   - in：gov 合约 transfer 的收款方 = 目标地址。
//
// 转账类交易附带金额（十进制字符串）与对手方地址（SS58 或 0x EVM）。
// 结果按时间倒序，支持分页（page/pageSize）。
func (r *queryResolver) AddressTxs(ctx context.Context, address string, page *int, pageSize *int) (*model.AddressTxs, error) {
	node := teechain.TeeChainNode
	if node == nil {
		return nil, gqlerror.Errorf("%s", "block store not ready")
	}
	bs := node.BlockStore()
	if bs == nil {
		return nil, gqlerror.Errorf("%s", "block store not ready")
	}

	na, err := normalizeAddress(address)
	if err != nil {
		return nil, gqlerror.Errorf("%s", err.Error())
	}

	p := 1
	if page != nil && *page > 0 {
		p = *page
	}
	ps := defaultAddressTxPageSize
	if pageSize != nil && *pageSize > 0 {
		ps = *pageSize
	}
	if ps > maxAddressTxPageSize {
		ps = maxAddressTxPageSize
	}

	latest := bs.Height()
	if latest <= 0 {
		return &model.AddressTxs{Txs: []*model.AddressTx{}, HasMore: false}, nil
	}

	transferSel := cstore.MethodToSelectorBytes("transfer")
	var matched []*model.AddressTx
	total := 0
	hasMore := false

	scanLimit := int64(maxAddressTxScanBlocks)
	if latest < scanLimit {
		scanLimit = latest
	}
	for h := latest; h > latest-scanLimit && h >= 1; h-- {
		block := bs.LoadBlock(h)
		if block == nil {
			continue
		}
		blockTime := block.Header.Time.UTC().Format(timeRFC3339)
		for i := 0; i < len(block.Data.Txs); i++ {
			item, ok := matchAddressTx(block.Data.Txs[i], h, i, blockTime, na.candidates, transferSel)
			if !ok {
				continue
			}
			total++
			// 分页：只保留当前页条目，凑满一页即停
			if total > (p-1)*ps && total <= p*ps {
				matched = append(matched, item)
			}
			if len(matched) >= ps {
				hasMore = true
				break
			}
		}
		if hasMore {
			break
		}
	}

	return &model.AddressTxs{
		Txs:     matched,
		Total:   intPtr(int64(total)),
		HasMore: hasMore,
	}, nil
}

// matchAddressTx 判断一笔原始交易是否与目标地址（候选 UniAddr 集）相关，相关则构造 AddressTx。
//
// 识别规则：
//   - 发起方（Tx.Caller，按 Tx.CallerType 构造 UniAddr）∈ 候选集 → out；
//   - gov 合约 transfer 的收款方（args[0]）∈ 候选集 → in（附带金额/对手方）。
func matchAddressTx(rawTx types.Tx, height int64, index int, blockTime string, target []cstore.UniAddr, transferSel []byte) (*model.AddressTx, bool) {
	txBox, err := decodeTxBox(rawTx)
	if err != nil || txBox == nil {
		return nil, false
	}

	caller := cstore.UniAddr{T: txBox.CallerType, V: txBox.Caller}
	var item *model.AddressTx
	switch {
	case uniAddrInSet(caller, target):
		item = &model.AddressTx{Direction: "out"}
	case isGovTransferToTx(txBox, target, transferSel):
		item = &model.AddressTx{Direction: "in"}
	default:
		return nil, false
	}

	item.Hash = hex.EncodeToString(rawTx.Hash())
	item.Height = int(height)
	item.Index = index
	item.Time = blockTime

	// 转账类交易：金额 + 对手方（对手方方向语义不同：
	//   - out（caller=目标）：对手方 = 收款方 args[0]；
	//   - in（目标=收款方）：对手方 = 付款方（交易发起方 caller）。
	// 若对手方恰为目标自身（自转）则留空，避免前端显示“自己转给自己”。
	if call, e := decodeSysCall(txBox.Call); e == nil {
		if cc, ok := call.Payload.(*model.SysCall_Contract); ok && cc.Contract != nil &&
			string(cc.Contract.Name) == "gov" && bytesEqual(cc.Contract.Method, transferSel) &&
			len(cc.Contract.Args) >= 2 {
			var cp cstore.UniAddr
			if item.Direction == "out" {
				cp, _ = cstore.DecodeScaleArgBytes[cstore.UniAddr](cc.Contract.Args[0])
			} else {
				cp = caller
			}
			if !uniAddrInSet(cp, target) {
				if s := uniAddrDisplay(cp); s != "" {
					item.Counterparty = &s
				}
			}
			if amt, e2 := cstore.DecodeScaleArgBytes[cstore.U256](cc.Contract.Args[1]); e2 == nil {
				s := amt.String()
				item.Amount = &s
			}
		}
	}
	return item, true
}

// isGovTransferToTx 判断已解码交易是否为 gov 合约 transfer 且收款方 ∈ 目标候选集。
func isGovTransferToTx(txBox *model.Tx, target []cstore.UniAddr, transferSel []byte) bool {
	call, err := decodeSysCall(txBox.Call)
	if err != nil {
		return false
	}
	cc, ok := call.Payload.(*model.SysCall_Contract)
	if !ok || cc.Contract == nil {
		return false
	}
	if string(cc.Contract.Name) != "gov" || !bytesEqual(cc.Contract.Method, transferSel) {
		return false
	}
	if len(cc.Contract.Args) < 1 {
		return false
	}
	to, err := cstore.DecodeScaleArgBytes[cstore.UniAddr](cc.Contract.Args[0])
	if err != nil {
		return false
	}
	return uniAddrInSet(to, target)
}

// uniAddrEqual 比较两个 UniAddr 是否相同（T 与 V 均相等）。
func uniAddrEqual(a, b cstore.UniAddr) bool {
	if a.T != b.T {
		return false
	}
	return bytesEqual(a.V, b.V)
}

// bytesEqual 比较两个字节切片是否相等（nil 与空切片视为相等）。
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// govBalanceOfSelectorHex 是 gov 合约 balanceOf 查询方法（mutates=false）的
// 4 字节 selector 十六进制（与 abis/gov.json 的 balance_of selector 一致）。
// 查询方法 selector 与 mutates=true 不同，故不能走 MethodToSelectorBytes 默认分支。
const govBalanceOfSelectorHex = "0x87e0ff32"

// timeRFC3339 是区块时间展示格式（与 chain.resolvers.go 一致）。
const timeRFC3339 = "2006-01-02T15:04:05Z07:00"
