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
# 用户通知（actives 消息中心）: NOTIFY_ENABLED（总开关，默认 true）/ NOTIFY_ACTIVES_URL / NOTIFY_PUSH_SECRET
#   TEE_USAGE_SETTLEMENT_ENABLED — 「Usage settlement」结算通知子开关，**默认 false（暂停）**：
#   结算消息量过大，默认不生成/不推送该类型通知；置 "true" 即恢复发送，无需改代码。
#   仅暂停消息，不影响结算/计费主链路，也不影响 model_offline/restored/listed/delisted 等其它通知。
# ⚠️ 编译/运行必须用 ego 工具链（不能用 go build / go run 调用编译；普通 go 产物无法在 TEE 中运行）
GOCACHE="$HOME/.cache/ego-go-build" ego-go build -o ./hack/build/dsecret ./main.go
ego sign ./hack/build/dsecret      # 签名（TEE 校验，生成 dsecret.sig）
OE_SIMULATION=1 ego run ./hack/build/dsecret
# 一键构建（ego-go build + ego sign + 打 docker 镜像）：./hack/build.sh
# 仅语法/单测检查（不产出 TEE 二进制）：go vet ./... && go test ./...
```

## 与各项目的关系
| 项目 | 关系 |
|------|------|
| **web3auth** | `web3auth/back` 功能已全部迁移到 tee-provider（`services/auth` + `services/wallet`）；`web3auth/src` 前端 SDK 保持不变，API 地址指向 tee-provider |
| **dapp / minishop** | 前端 API 基地址指向 tee-provider 节点（`contractDryRun` 读链） |
| **c2c** | 鉴权（`checkSession`）+ 读取合约经济策略（`c2cFxConfig`） |
| **actives / tee-node-monitor** | 链上数据源（合约调用） |
| **contract-deploy** | 合约部署在目标链上，tee-provider 经 `pkg/chains` 交互 |

## 脚本

共 25 个脚本，按用途分组。以下命令默认在 `apps/tee-node` 下执行（`cd apps/tee-node`）；脚本会按自身位置定位项目根，只有 `chain_init.sh` / `chain_gen_node.sh` 以**调用时的当前目录**为基准。

### 构建与代码生成

| 脚本 | 用途 | 用法 |
|------|------|------|
| `hack/build.sh` | 用 ego 工具链编译并签名 TEE 二进制 `dsecret`，同时编译 `sgx-verify-service`，构建并**推送**镜像 `wetee/tee-provider:<YYYY-MM-DD-HH_MM>`，版本写入 `hack/.version`；需 `ego-go`、`docker` 且已登录镜像仓库。 | `./hack/build.sh` |
| `hack/prebuild.sh` | evolve 链路预构建：仅编译 + 签名 `dsecret` / `sgx-verify-service` 到 `hack/build/`，不构建/推送镜像；固定使用 `/opt/ego/bin/ego-go`，可用 `GOCACHE` 覆盖缓存目录。 | `./hack/prebuild.sh` |
| `hack/gen.sh` | 在项目根执行 `go generate ./...`，运行源码里的 `//go:generate` 指令。 | `./hack/gen.sh` |
| `hack/gen_codec.sh` | 为 `tee-chain/runtime/store`、`gov`/`auth`/`gateway`/`batch`/`c2c` 模块与 `pkg/chaintypes` 生成 SCALE codec，并修复 `store/codec_gen.go` 的自引用 import。 | `./hack/gen_codec.sh`（可选 `store` / `gov` / `auth` / `gateway` / `batch` / `c2c` / `all`） |
| `hack/gen_local_contract_abi.sh` | 用 `tools/contractgen` 生成 `gov`/`auth`/`gateway`/`batch`/`c2c` 的 `*_gen.go` 和 `tee-chain/runtime/abis/*.json`，并把 4 个 ABI 同步到 `apps/dapp/public/contract/`；会覆盖已生成文件。 | `./hack/gen_local_contract_abi.sh` |
| `hack/gen_tx_proto.sh` | 在 `pkg/model` 用 `protoc` 生成 `tx.pb.go`（go + vtproto），并复制到 `apps/libos-entry/model/`；需 `protoc`、`protoc-gen-go`、`protoc-gen-go-vtproto`。 | `./hack/gen_tx_proto.sh` |
| `tools/ink-abi-ts-gen/gen.sh` | 把 ink! ABI JSON 生成 TypeScript 类型；生成器缺失或 `main.go` 更新时先 `go build`；不传输出路径时在生成器目录生成 `<abi名>.ts`。 | `(cd tools/ink-abi-ts-gen && ./gen.sh ../../tee-chain/abis/gov.json ./gov.ts)` |

### 本地链初始化

| 脚本 | 用途 | 用法 |
|------|------|------|
| `hack/chain_init.sh` | 用 `cometbft init` 在当前工作目录生成 `_chain_init/`（`genesis.json`、`node_key.json`、`priv_validator_key.json`、`priv_validator_state.json`）；需 `cometbft` 在 PATH / `~/go/bin` / `/usr/local/bin` / `/usr/bin`。 | `./hack/chain_init.sh`（产物为 `./_chain_init`，不在脚本目录） |
| `hack/chain_gen_node.sh [输出目录]` | 读取当前目录下 `_chain_init/config/genesis.json`，生成共享同一创世块、但 node/validator 密钥独立的新节点配置；⚠️ 会先 `rm -rf` 输出目录（默认 `./_chain_node`）。 | `./hack/chain_gen_node.sh` 或 `./hack/chain_gen_node.sh /srv/token_node3` |

### 节点部署与运行

| 脚本 | 用途 | 用法 |
|------|------|------|
| `hack/node0-run.sh` | 本地 node0 全量部署：ego 编译 + 签名 `dsecret`，同步到硬编码执行路径 `/home/wetee/work/Query-Web3/tee-node/hack/build/dsecret`，写入 `tee-node0.service` 的 systemd drop-in（自动重启、跳过重建、SGX 验证地址、日志 token、通知），然后重启服务。⚠️ 覆盖已部署二进制与 systemd 配置并重启本地节点，需 `sudo`；需已存在 `tee-node0.service`。 | `./hack/node0-run.sh`；`TAIL=1 ./hack/node0-run.sh`（跟随日志） |
| `hack/node1-deploy-to-test.sh [命令]` | 本地 ego 编译 + 签名后，`rsync --delete` 构建产物到 `computefluxtest:/srv/token_node1/build/`，安装/更新远端 `dsecret-node1.service` 与 `sgx-verify-service.service` 并重启；命令默认 `deploy`，另有 `logs`/`status`/`restart`/`stop`/`enable`/`disable`。⚠️ 覆盖远端二进制/unit 并重启远程节点。 | `./hack/node1-deploy-to-test.sh`；`TAIL=1 ./hack/node1-deploy-to-test.sh deploy`；`./hack/node1-deploy-to-test.sh status` |
| `hack/node1-copy-to-test.sh` | 把本地 `/srv/token_node1/token_chain_data/` 同步到 `computefluxtest` 同路径：先备份远端 `config/p2p_node_key.sealed` 与 `config/validator_dkg_key.sealed`，删除远端 `BFT`/`data`/`config`/`wetee`，`rsync` 后恢复 sealed 私钥。⚠️ 清空远端链数据与配置，不可逆（sealed 私钥除外）。 | `./hack/node1-copy-to-test.sh` |
| `hack/node2-deploy-docker.sh` | 读取 `hack/.version`，以 Docker 启动 node2 容器 `tee-provider-2`（端口 30136/30137/30138，挂载 `/srv/token_node2/token_chain_data`，SGX 模拟模式）。⚠️ 先 `docker rm -f tee-provider-2` 删除旧容器。 | `./hack/node2-deploy-docker.sh` |
| `hack/node0-deploy.sh` | 旧 k8s 部署：用 `hack/.version` + `envsubst` 渲染 `hack/k8s-temp.yaml` 为 `hack/k8s.yaml`，删除并重建 `worker-addon` 命名空间的 `dsecret-0` Deployment/Service。⚠️ 删除重建工作负载；k3s 已废弃，需可用 kubeconfig。 | `./hack/node0-deploy.sh` |
| `hack/node1-deploy.sh` | 旧 k8s 部署：同上，部署 `tee-provider-1`（端口 30126/30127/30128）到 `worker-addon`。⚠️ 删除重建工作负载。 | `./hack/node1-deploy.sh` |
| `hack/node2-deploy.sh` | 旧 k8s 部署：同上，部署 `tee-provider-2`（端口 30136/30137/30138）到 `worker-addon`。⚠️ 删除重建工作负载。 | `./hack/node2-deploy.sh` |
| `hack/sgx-verify-service-deploy.sh` | 编译 `sgx-verify-service`（cgo 链接 `/opt/ego` 的 `liboehostverify`），安装到 `/usr/local/bin`，写 systemd 单元并以 `0.0.0.0:8004` 启动，健康检查并确认日志含 `verifier=eclient`。⚠️ 覆盖系统二进制/unit 并重启服务，需 `sudo`。 | `./hack/sgx-verify-service-deploy.sh` |

### genesis 与运行时

| 脚本 | 用途 | 用法 |
|------|------|------|
| `hack/update-genesis-eth.sh [选项] [ETH地址...]` | 用 `ext/eth2uniaddr` 把 ETH 地址转成 UniAddr，写入节点 genesis 的 `app_state.sudo`；多地址时**只取第一个**。⚠️ 直接修改默认三节点 `/srv/token_node{0,1,2}/token_chain_data/config/genesis.json`，需重启链。 | `./hack/update-genesis-eth.sh 0xAb5801a7D398351b8bE11C439e05C5B3259aeC9B`；`./hack/update-genesis-eth.sh --addresses addrs.txt --nodes /srv/token_node0 /srv/token_node1` |
| `hack/update-genesis-validator.sh <ADDRESS> <PUBKEY> [选项]` | 把 validator 写入节点 genesis 的 `validators` 数组；`--power` 默认 10、`--name` 默认空，`-n/--nodes` 指定节点目录（默认三节点）。⚠️ 会用单个条目**整体替换** `validators`，且 `--power` 以字符串写入（CometBFT 标准字段为数字）；改前备份，改后重启链。 | `./hack/update-genesis-validator.sh 79F5D697DB70EA1C76A5C1ED988A8A3D033535D7 XiBiWNpiWaxoFSGHdXeQIR9uzqK1QNZd0pGhMQJofRw=`；`./hack/update-genesis-validator.sh --power 10 --name node0 79F5D697DB70EA1C76A5C1ED988A8A3D033535D7 XiBiWNpiWaxoFSGHdXeQIR9uzqK1QNZd0pGhMQJofRw=` |
| `hack/update-genesis-wasm.sh [节点目录...]` | 用 TinyGo 编译 `tee-chain/runtime/wasm/runtime/runtime.wasm`，base64 写入各节点 genesis 的 `app_state.runtime.wasm`，并把 `runtime.wasm`/`.gz` 复制到节点数据目录。⚠️ 覆盖 genesis 与数据目录，默认三节点，改后需重启链。 | `./hack/update-genesis-wasm.sh`；`./hack/update-genesis-wasm.sh /srv/token_node0` |
| `hack/update_runtime.sh [选项]` | 通过 GraphQL 提交 `gov.RuntimeUpgrade`：编译 `ext/runtimeupgrade`，默认先编译 WASM，读取 `NativeVersion` 校验版本，上传 `runtime.wasm.gz` 并等终态；选项 `--no-build`、`--wasm`、`--version-src`、`--gql`、`--desc`、`--no-wait`。⚠️ 链上运行时升级，影响全链且需 sudo 助记词。 | `SUDO_MNEMONIC="word1 word2 ... word12" ./hack/update_runtime.sh --gql https://localhost:30115`；`./hack/update_runtime.sh --no-build --wasm tee-chain/runtime/wasm/runtime/runtime.wasm.gz` |

### EVM 合约

| 脚本 | 用途 | 用法 |
|------|------|------|
| `hack/evm/contract.sh [build\|deploy\|all]` | `build`/`compile`：用 Hardhat 编译 `Token`/`Subnet`/`BLS` 并用 `abigen` 生成 Go binding 到 `pkg/chains/evm/bindings/`；`deploy`：部署到 Polkadot testnet；`all` = build + deploy。⚠️ `deploy`/`all` 会上链部署合约。 | `./hack/evm/contract.sh build`；`./hack/evm/contract.sh deploy`；`./hack/evm/contract.sh all` |
| `hack/evm/deploy_subnet_token.sh` | 用 Hardhat 把 `Subnet` + `Token` 部署到 Polkadot testnet，要求合约目录已有 `.env`。⚠️ 上链部署，产生真实交易。 | `./hack/evm/deploy_subnet_token.sh` |

### 清理

| 脚本 | 用途 | 用法 |
|------|------|------|
| `hack/clear.sh` | 重置本地三节点链数据：删除 `/srv/token_node{0,1,2}/token_chain_data/{BFT,wetee,data/cs.wal}`，重写 `priv_validator_state.json`（height 0 / round 0 / step 3）。⚠️ 彻底清空链状态且不可恢复；写 `/srv` 需权限。 | `sudo ./hack/clear.sh` |
| `hack/clean-k3s.sh` | 清理本机废弃 k3s：停止/禁用 `k3s`、`k3s-ui`，kill shim/containerd 残留进程，备份 `/var/lib/rancher/k3s`、`/etc/rancher/k3s`、kubeconfig 和 unit 为 `.bak`，删除 `/run/k3s`；不删除节点链数据。⚠️ 会停止全部 k3s 容器并禁用 k3s，需 root（备份可恢复）。 | `sudo bash hack/clean-k3s.sh` |

**依赖与常见坑**
- `build.sh` / `prebuild.sh` / `node0-run.sh` / `node1-deploy-to-test.sh` 需要 ego 工具链；`prebuild.sh` 固定 `/opt/ego/bin/ego-go`。ego-go 必须用独立 `GOCACHE`（`$HOME/.cache/ego-go-build`），但 `node0-run.sh` 裸调 `ego-go build` 未设置，与 `build.sh` 不一致。
- `node0-deploy.sh` / `node1-deploy.sh` / `node2-deploy.sh` 是 **k8s 旧链路**，依赖 `hack/.version`、`envsubst`、`kubectl` 与 `worker-addon` 命名空间；`clean-k3s.sh` 已注明 k3s 废弃，当前 node0 用 systemd、node2 用 docker。
- `node1-copy-to-test.sh` / `node1-deploy-to-test.sh` 硬编码远端 `computefluxtest`，需免密 SSH + `rsync`；后者还需要远端免密 `sudo`。远端服务 GQL 端口为 `443`。
- `node0-run.sh` 依赖已存在的 `tee-node0.service`，并会覆盖其 drop-in；`SVS_HOST` 默认 `xiaobai.asyou.me`，`LOG_API_TOKEN` 未显式设置时每次随机生成（监控侧需同步），`TAIL=1` 才会跟随日志。
- genesis 脚本默认改 `/srv/token_node0`、`/srv/token_node1`、`/srv/token_node2`，只改文件不重启链；`update-genesis-validator.sh` 整体替换 `validators` 且 `power` 以字符串写入，用前备份并核对；`update-genesis-eth.sh` 多地址只写第一个。
- `update_runtime.sh` 的 `SUDO_MNEMONIC` 是 sudo 账户助记词（敏感，建议放 `hack/.env` 并 `chmod 600`）；还支持 `TEE_NODE_GQL`、`SKIP_TX_SIZE_CHECK=1`、`ENV_FILE`。WASM 超过 900000 字节会二次确认。
- `hack/evm/contract.sh` 期望 `apps/tee-node/contract/`，`hack/evm/deploy_subnet_token.sh` 期望仓库根 `contract/`——当前仓库均无此目录，直接运行会在 `cd` 处失败；另需 `npx hardhat`、`abigen`、`python3`、`go` 与合约 `.env`。
- `gen_codec.sh` 文件头写的是 `generate_codec.sh`，实际文件名是 `gen_codec.sh`；报错提示的模块列表漏了 `c2c`（`c2c` 实际可用）。`gen_tx_proto.sh` 末尾复制到 `apps/libos-entry/model/`，当前仓库无 `apps/libos-entry`，该步会失败。
- `tools/ink-abi-ts-gen/gen.sh` 会先 `cd` 到自身目录，ABI 与输出路径都按该目录解析，需要用 `../../tee-chain/abis/...` 这类相对生成器目录的路径。

## 约定
- 遵循仓库根级约定（见上级 `AGENTS.md`）。
- 独立 go.mod 模块，不随主仓库 go.sum 同步。
- 改 GraphQL schema 后重新 `gqlgen` 生成 `graph/` 产物；改合约相关逻辑后同步 ABI。
- **编译必须用 ego 工具链，不要用 `go build` / `go run`**：`GOCACHE="$HOME/.cache/ego-go-build" ego-go build -o ./hack/build/dsecret ./main.go` → `ego sign ./hack/build/dsecret` → `OE_SIMULATION=1 ego run ./hack/build/dsecret`（一键 `./hack/build.sh`）。ego-go 自带 patch 过的 Go 工具链，与系统 go 共用 build cache 会混入不兼容目标文件（链接报 `invalid relocation ... not in [0,N)`）→ 必须用独立 GOCACHE。
- 改动后跑 `go vet ./...` / `go test ./...` 做语法与单测检查（不产出 TEE 二进制）。
- 信任模型：不信任单个节点，信任 TEE 硬件 + >2/3 诚实（BFT）+ DKG 门限——涉及密码学/共识路径的改动须评估对三者的影响。
