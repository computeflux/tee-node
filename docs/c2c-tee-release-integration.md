# c2c Release TEE 验证对接说明

> 面向：验证节点侧（TEE 内代码）/ web2 c2c 服务
> 合约入口：`tee-chain/runtime/module/c2c/c2c.go` `C2CMutation.Release(ReleaseWithSig)`

## 1. 变化总结

`Release` 的"代码版本"校验从**用户口头声称**（`CodeVersion` 参数可任意伪造）改为 **TEE 硬件证明**：

| 步骤 | 旧逻辑（已删除） | 新逻辑 |
|---|---|---|
| ② | —（无） | 用户必须携带 TEE quote（`TeeReport`）+ 签发时间（`TeeTime`），`VerifyTEE` 在 TEE 内做真实 quote 验签 |
| ③ | `CodeVersion`（用户传）== 节点注册值 | `VerifyTEE` 返回的 TEE 证明代码指纹（`tee.CodeSignature`）== 节点注册值 |
| ④ | 用户传的 `CodeVersion` 在白名单内 | TEE 证明的代码指纹在白名单内（链上信任锚） |
| ⑤ | —（无） | 链上用 TEE 证明的代码公钥（`tee.CodeSigner`）验证用户携带的 `CodeSignature`，证明放款数据出自被 TEE 证明的代码 |

`ReleaseWithSig` 结构（`c2c/types.go`，SCALE 编码，`TeeTime` 为新增末字段 u64LE）：

```go
type ReleaseWithSig struct {
    Data          Release    // 放款数据（被证明的数据）
    TeeReport     []byte    // TEE quote 字节（model.TeeCall 的 Report 字段，非整个 TeeCall）
    CodeSignature []byte    // TEE 代码对 SCALE(Data) 的 ed25519 签名
    TeeTime       int64     // 节点签发 quote 的时间戳（秒），即签名内容中的 Time
}
```

## 2. 端到端流程

```
web2 c2c 服务（节点身份）                     TEE 节点（TEE 内代码）
     │  收到商家放款请求（构造 Release Data）        │
     │ ──────────── 请求签发 ───────────────────▶  │
     │                                            │  t = now().Unix()
     │                                            │  msg = [Int64ToBytes(t) || 节点公钥 || marshal(TeeCall_Text{Text: SCALE(Data)})]
     │                                            │  sig = ed25519_sign(节点私钥, msg)      (64B)
     │                                            │  quote = TEE 硬件报告(report data = sig)
     │ ◀────────── {TeeReport: quote,              │
     │              TeeTime: t}                    │
     │
     │  用节点私钥签名交易，提交 c2c Release(ReleaseWithSig{Data, TeeReport, CodeSignature, TeeTime})
     ▼
合约（任一 TEE 节点执行区块时）:
  ① caller 是已注册验证节点
  ② VerifyTEE(TeeReport, SCALE(Data), TeeTime):
     a. 时间窗：|区块时间 − TeeTime| ≤ 10s，否则 ErrTeeVerifyFailed（防旧 quote 重放）
     b. quote 硬件验签（SGX: VerifyRemoteReport/sgx-verify；SNP: verify.SnpAttestation）
     c. 节点签名验签：ed25519_verify(caller, [Int64ToBytes(TeeTime) || caller || marshal(Tx)], quote.ReportData)
     d. 返回 {TeeType, CodeSigner, CodeSignature}
  ③ tee.CodeSignature == node.CodeVersion        否则 ErrCodeMismatch
  ④ 白名单 contains(tee.CodeSignature)            否则 ErrCodeNotAuthorized
  ⑤ verifyCodeSig(tee.CodeSigner, SCALE(Data), CodeSignature)  否则 ErrCodeSigInvalid
  ⑥ 订单去重 ⑦ 金额>0 ⑧ 折算复核 ⑩⑪ 限额 ⑫ 佣金 ⑬⑭ escrow 扣减/买方入账
```

## 3. 节点侧对接要求（必须逐条对齐）

### 3.1 签名消息（quote 绑定的内容）

```
msg = Int64ToBytes(t) || 节点公钥 || protoio.MarshalVT(&model.TeeCall_Text{Text: payload})

  Int64ToBytes(t)  : 8 字节大端 uint64（t 为 Unix 秒）
  节点公钥          : 与交易签名、TeeCall.Caller 完全相同的公钥（ed25519，32B）
  payload          : store.ScaleEncode(Release) 的字节（见 3.3）
  TeeCall_Text     : model.TeeCall 的 oneof tx = text，Text 字段 = payload
                     protoio.MarshalVT 序列化（varint tag 8 + length-delimited bytes）
```

- `t` 必须同时用于：签名消息、`ReleaseWithSig.TeeTime` 上报值。**三者不一致必然验签失败。**
- 节点私钥 = 节点交易签名私钥（`VerifyTxSigner` 校验的那把），私钥在 TEE 内。

### 3.2 quote 格式（按节点 TEE 类型）

| TEE | quote 生成 | TeeReport 内容 |
|---|---|---|
| SGX (TeeType=0) | `enclave.GetRemoteReport(sig)`（ego） | 1024 字节原始 report（sig 在 report data 字段） |
| SNP (TeeType=1) | `client.GetExtendedReport(device, sig64)`（go-sev-guest） | `proto.Marshal(sevsnp.Attestation)`（sig 在 Report.ReportData） |
| TDX (TeeType=2) | 未实现（`TdxIssue` 报错） | — |
| 非 TEE (9999) | 无 quote | 验证 fail-closed（③ 必失败） |

quote 类型必须与执行合约的节点 TEE 类型一致（运行时以本机 `teetypes.TeeType` 组装 TeeCall 后走对应 `VerifyReport` 分支）。

### 3.3 SCALE(Data) 字节布局（payload，须与合约端逐字节一致）

`Release` 的 SCALE 编码（`c2c/codec_gen.go`）：

```
[Merchant.T: u32LE][compact(len(Merchant.V))][Merchant.V]
[Buyer.T: u32LE]  [compact(len(Buyer.V))]   [Buyer.V]
[Amount: 32B 小端 u256]
[compact(len(OrderNo))][OrderNo]
[FiatCents: u64LE]
[compact(len(Currency))][Currency]
[LockedRate: u64LE]
[FeePct: u64LE]
```

节点侧直接用同一套 Go 代码（`store.ScaleEncode(r)`, r 为同一 `Release` 值）生成 payload，避免手写编码不一致。

### 3.4 时间窗

- 合约校验：`|区块时间 − TeeTime| ≤ teeTimeDrift`（当前 10 秒，`runtime/runtime.go` 常量）。
- 节点应在执行该交易的区块内签发 quote（区块时间由 proposer 时钟设定，节点签发用本机时钟，两者通常差 < 1s；10s 窗口容忍时钟偏差）。
- 超窗 → `ErrTeeVerifyFailed`。旧 quote 无法重放（TeeTime 被节点签名锚定，伪造需 TEE 内私钥）。

## 4. 代码签名（CodeSignature）算法

合约端 `verifyCodeSig`（`c2c.go`，纯标准库 ed25519 + blake2b，可移植到任何语言）：

```
if len(tee.CodeSigner) != 32: 失败
msg = payload
if len(msg) > 256: msg = blake2b-256(msg)
ed25519_verify(tee.CodeSigner, msg, CodeSignature)
```

- 代码密钥对（ed25519）在 TEE 飞地内生成、私钥不出飞地。
- 用户侧 `CodeSignature = ed25519_sign(代码私钥, 同一规则下的 msg)`，web2 侧生成（或 TEE 内代签后返回）。
- 测试参考：`c2c/c2c_test.go` `signRelease` / `releaseWithSig`。

## 5. "代码版本"标识的对齐（⚠️ 当前唯一未闭环点）

`VerifyReport` 返回的三要素必须与链上两处注册值对齐：

| 位置 | 值 |
|---|---|
| `RegisterCode(codeHash, ...)` 白名单 | 代码版本标识 |
| `RegisterNode(node, codeHash)` 节点注册 | 同上（节点只能为其自身代码背书） |
| `VerifyReport` 返回的 `CodeSignature` | 必须 == 上面两处 |

- **SGX**：当前 `SgxVerify` 返回 `CodeSigner = MRSIGNER`、`CodeSignature = MRENCLAVE`（32B）。即"代码版本" = MRENCLAVE，白名单/节点注册须填同一 MRENCLAVE。
- **SNP**：当前返回空 → ③ 必失败（fail-closed）。**需补实现**：从 SNP 报告中取代码度量值（如 measured 的 code hash）作为 CodeSignature，并定义对应的 CodeSigner。
- 若希望"代码版本"是自定义 hash（如代码文件 sha256）而非硬件度量值，需节点侧维护 `硬件度量值 → 自定义 hash` 映射，并在 `VerifyReport` 中做转换。

## 6. 错误码速查

| 错误 | 触发条件 |
|---|---|
| `ErrNotNode` | ① caller 未注册为节点 |
| `ErrTeeReportMissing` | ② TeeReport 为空 |
| `ErrTeeVerifyFailed` | ② 时间窗超界 / quote 硬件验签失败 / 节点签名验签失败 / TEE 类型未知 / debug 模式 |
| `ErrCodeMismatch` | ③ TEE 证明代码指纹 ≠ 节点注册值（含非 TEE 节点 CodeSignature 为空） |
| `ErrCodeNotAuthorized` | ④ 代码指纹不在白名单（含被 RevokeCode 撤销） |
| `ErrCodeSigInvalid` | ⑤ CodeSigner 非 32B / ed25519 验签失败 |
| `ErrDuplicateRelease` | ⑥ 订单号已放款 |

## 7. 涉及文件

- 合约：`tee-chain/runtime/module/c2c/c2c.go`（Release ①-⑤）、`types.go`（ReleaseWithSig）、`codec_gen.go`（SCALE 布局）
- 运行时：`tee-chain/runtime/runtime.go`（`VerifyTEE` + `teeTimeDrift`）、`store/api.go`（接口）、`store/types.go`（TeeVerifyInfo）
- TEE 验签：`pkg/types/teetypes/{tee,tee_sgx,tee_snp,tee_tdx}.go`
- WASM 路径：`tee-chain/runtime/wasm/host.go`（verify_tee host 函数，7 参）、`wasm/runtime/{host,main}.go`
- ABI：`tee-chain/runtime/abis/c2c.json`（ReleaseWithSig 增加 TeeTime: i64）
