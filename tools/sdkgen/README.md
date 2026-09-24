# sdkgen

从 ink metadata（ABI JSON，由 `contractgen -abi-out` 产出）生成 tee-chain 合约的 Go SDK：
每个合约一个目录（`calls.go` + `types.go`），共享一个 `chain` 运行时包；
调用形态与 go-ink-gen 生成的 dao 代码一致（`Query` / `DryRun` / `Exec` / `CallOf`）。

## 用法

```bash
# 第一个合约：同时生成共享运行时 chain/
go run ./tools/sdkgen -abi tee-chain/runtime/abis/c2c.json -out ./sdk -client C2C

# 同输出目录的后续合约：-no-chain 复用已生成的运行时
go run ./tools/sdkgen -abi tee-chain/runtime/abis/gateway.json -out ./sdk -client Gateway -no-chain
```

| flag | 说明 |
|---|---|
| `-abi` | ink metadata ABI JSON（必填） |
| `-out` | 输出根目录（默认 `.`） |
| `-client` | 合约客户端类型名（默认合约名转驼峰，如 `c2c` 建议显式传 `C2C`） |
| `-contract` | 链上合约名（默认取 ABI 的 `contract.name` 小写） |
| `-pkg` | 合约包名（默认由合约名推导） |
| `-chain-dir` | 共享运行时目录名（默认 `chain`） |
| `-chain-import` | chain 包 import 路径（默认按最近 `go.mod` 的模块路径推导） |
| `-no-chain` | 不生成共享运行时（同输出目录已生成过时用） |
| `-no-signers` | 不生成内置签名器，仅保留 `Signer` 接口 |

## 输出布局

```
<out>/chain/client.go          ChainClient：GraphQL 查询/预演 + CometBFT 提交/确认
<out>/chain/codec.go           SCALE 参数编码 / protobuf wire / 签名原文
<out>/chain/signer.go          Signer 接口（可插拔）+ caller 类型常量
<out>/chain/signer_builtin.go  内置 ed25519（calltype 3）/ sr25519（calltype 1）
<out>/<合约>/calls.go          调用层：QueryX / DryRunX / ExecX / CallOfX
<out>/<合约>/types.go          类型层：SCALE Encode/Decode（类型映射沿用 go-ink-gen）
```

## 调用形态

```go
cli, _ := chain.InitClient(rpcURL, gqlURL, signer) // signer 可为 nil（只读）
c2cIns := c2c.InitC2CContract(cli)

fx, err := c2cIns.QueryGetFxConfig(chain.DefaultDryRunParams()) // 只读查询
err = c2cIns.ExecRegisterCode(hash, ver, chain.DefaultExecParams())        // 提交并确认
err = c2cIns.DryRunRegisterCode(hash, ver, chain.ParamsFromSigner(signer)) // 预演，不落链
tx, err := c2cIns.CallOfRegisterCode(hash, ver, chain.DefaultExecParams()) // 只构造签名交易
```

- `QueryX(args..., params)`：只读，`params` 可带 caller（`chain.EVMParams("0x…")` 查 owner 维度，如
  `get_provider_by_owner`）。
- `DryRunX(args..., params)`：变更预演，合约业务错误在此带出，不消耗 nonce。
- `ExecX(args..., params)`：查询 nonce → 签名 → `broadcast_tx_sync` → 轮询确认；被拒时重放取合约错误。
- `CallOfX(args..., params)`：只构造并签名交易（不广播），返回 `*chain.Tx`（`Bytes()` / `Hash()`）。

## 签名器

`chain.Signer` 只有三个方法，任何方案都能接入：

```go
type Signer interface {
	Sign(msg []byte) ([]byte, error) // 签名原文 = <Bytes>{contract}{method}{args}{nonce}</Bytes>
	Public() []byte                  // Tx.caller（sr25519/ed25519 32B，EVM 20B）
	CallerType() uint32              // 见 chain.CallerType* 常量
}
```

- 内置：`chain.NewEd25519Signer(seedHex)`（calltype 3，直接签名）、
  `chain.NewSr25519Signer(seedHex)`（calltype 1）；seed 为 32 字节 hex。
- 更多方案：实现 `Signer` 即可 —— EVM secp256k1 用 calltype 2；链节点 ed25519 用 calltype 0，
  且 >256 字节原文需先 `chain.PreHashIfLong`（与链上 `Ed25519SignVerify` 一致）；远程 KMS / HSM 同理。
- 已有实现（如 c2c 服务的 `ed25519Signer` / `Sr25519Signer`）方法集与本接口一致，可直接作为
  `chain.Signer` 传入。

## 与链上的一致性

- 只读查询 → `contractDryRun(mut=false)`，返回链上已解包 `Result` 的裸值，按 SCALE 解码。
- 变更预演 → `contractDryRun(mut=true)`，错误经 GraphQL errors 带出。
- 提交 → 每次查询 `contractNonce`（不信任本地缓存）→ 签名 → protobuf
  `Tx{caller/caller_type/call/signature}` → CometBFT `broadcast_tx_sync` → 轮询 `/tx`。
- 签名原文与 `pkg/model.ContractForSig` 一致；参数 SCALE 编码与合约端解码逐字节一致。

## 测试

```bash
go test ./tools/sdkgen/   # 类型去重、参数命名、import 推导、调用形态生成
```
