# tee-node（tee-provider — TEE 子网）

Go 项目，独立 go.mod。基于 **TEE（Trusted Execution Environment）** 的去中心化子网：多 TEE 节点经 **CometBFT** 共识维护状态一致，用 **DKG（阈值签名）+ 代理重加密（Proxy Re-Encryption）+ TEE 远程证明** 提供安全密钥管理与秘密分发。可作任意公链的子网，将链下安全计算、认证服务（Web3Auth）和 API 网关能力带入生态。

> 完整设计见 `Readme.md`（架构 / 合约层 Auth+Gateway / DKG 密钥生命周期 / 部署 / 安全模型）。

## 技术栈
- **Go**（独立 `go.mod`）
- **共识**：`tee-chain/`（CometBFT ABCI 应用、纪元管理、HubCall 主链同步、多链连接、交易管线）
- **密码学**：`pkg/dkg`（kyber 阈值签名）、`pkg/proxy-reenc`（代理重加密）、`pkg/chains`（公链接口抽象）、`pkg/util`
- **GraphQL**：`graph/`（`gqlgen.yml` 生成；`contractCall` / `contractDryRun` / 秘密管理 / TEE 报告 / 链信息）
- **TEE 远程证明**：Edgeless ego（Intel SGX）/ go-sev-guest（AMD SEV）；Vote Extensions 嵌入证明
- 服务层：Web3Auth（OTP/社交 OAuth/钱包/交易签名）+ API 网关（Token/多供应商 Relay/速率限制/计费）+ 秘密管理

## 目录结构
- `main.go` — 入口
- `tee-chain/` — 侧链/共识核心
- `pkg/` — 密码学与链抽象层
- `graph/` — GraphQL API
- `config/` — 配置
- `ext/` — 扩展
- `tools/` `tools.go` `sdkgen` — 工具与 SDK 生成
- `hack/` — 运维脚本；`docs/` — 文档
- `Dockerfile` — 容器镜像

## 启动方式
```bash
cd apps/tee-node
export CHAIN_ENV=local            # 主链环境: local / test / main
export GQL_PORT=61000             # GraphQL 端口（c2c 默认指向 :8080；生产 :30118）
export SIDE_CHAIN_PORT=61001       # P2P 端口
export OE_MODE=SIM                # TEE 模式: SIM（开发）/ HW（生产）
# 邮箱（Web3Auth 服务）: SMTP_HOST/PORT/FROM/USER/PASSWORD，MOCK_EMAIL=true 开发模式控制台打印 OTP
# OAuth: GOOGLE/GITHUB/TWITTER_CLIENT_ID/SECRET
# LOG_API_TOKEN  /api/logs 日志查询端点 token（供 tee-node-monitor 远程拉取日志；未设置则端点 404 关闭）
go run main.go
go build ./...
go test ./...
```

## 与各项目的关系
| 项目 | 关系 |
|------|------|
| **web3auth** | `web3auth/back` 功能已全部迁移到 tee-provider（`services/auth` + `services/wallet`）；`web3auth/src` 前端 SDK 保持不变，API 地址指向 tee-provider |
| **dapp / minishop** | 前端 API 基地址指向 tee-provider 节点（`contractDryRun` 读链） |
| **c2c** | 鉴权（`checkSession`）+ 读取合约经济策略（`c2cFxConfig`） |
| **actives / tee-node-monitor** | 链上数据源（合约调用） |
| **contract-deploy** | 合约部署在目标链上，tee-provider 经 `pkg/chains` 交互 |

## 约定
- 遵循仓库根级约定（见上级 `AGENTS.md`）。
- 独立 go.mod 模块，不随主仓库 go.sum 同步。
- 改 GraphQL schema 后重新 `gqlgen` 生成 `graph/` 产物；改合约相关逻辑后同步 ABI。
- 改动后跑 `go vet` / `go build ./...` / `go test ./...`。
- 信任模型：不信任单个节点，信任 TEE 硬件 + >2/3 诚实（BFT）+ DKG 门限——涉及密码学/共识路径的改动须评估对三者的影响。
