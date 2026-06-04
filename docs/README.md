# TEE Node 文档目录

> 本文档按模块/功能域组织，便于按关注点快速定位。

---

## 目录结构

| 目录 | 内容 |
|------|------|
| [`overview/`](overview/) | 项目总体架构、模块列表、融合方案 |
| [`chain/`](chain/) | 侧链核心（CometBFT ABCI、交易、Relay、Epoch） |
| [`contracts/`](contracts/) | 智能合约系统（存储、安全、WASM 运行时） |
| [`gateway/`](gateway/) | AI API 网关（路由、计费、结算、API Key） |
| [`auth/`](auth/) | Web3Auth 与统一鉴权 |
| [`kimi-testing-progress.md`](kimi-testing-progress.md) | 单元测试与合约审核进度报告 |
| [`plans/`](plans/) | 历史重构与规划文档 |

---

## 快速索引

### 项目概览
- [`overview/architecture-overview.md`](overview/architecture-overview.md) — 总体架构设计
- [`overview/module-feature-list.md`](overview/module-feature-list.md) — 模块与功能列表
- [`overview/mirgration.md`](overview/mirgration.md) — web3auth + token 融合方案

### 侧链
- [`chain/tee-chain-design.md`](chain/tee-chain-design.md) — 侧链逻辑架构
- [`chain/relay-design.md`](chain/relay-design.md) — 主链→侧链 Relay 同步
- [`chain/tee-tx-retry-design.md`](chain/tee-tx-retry-design.md) — 交易失败重试机制
- [`chain/unified-tx-design.md`](chain/unified-tx-design.md) — 统一交易设计

### 合约与 WASM
- [`contracts/contract-storage-design.md`](contracts/contract-storage-design.md) — 合约存储分层设计
- [`contracts/contract-security-analysis.md`](contracts/contract-security-analysis.md) — 合约安全审计
- [`contracts/go-contract-to-wasm-migration.md`](contracts/go-contract-to-wasm-migration.md) — Go 合约→WASM 迁移方案
- [`contracts/wasm-runtime-host-functions.md`](contracts/wasm-runtime-host-functions.md) — WASM Host Functions 完整方案
- [`contracts/minimal-wasm-host-functions.md`](contracts/minimal-wasm-host-functions.md) — WASM Host Functions 实际实现

### AI Gateway
- [`gateway/ai-gateway-design.md`](gateway/ai-gateway-design.md) — Gateway 详细设计
- [`gateway/api-key-design.md`](gateway/api-key-design.md) — API Key 设计
- [`gateway/usage-settle-design.md`](gateway/usage-settle-design.md) — 用量上链结算
- [`gateway/usage-store-settle-design.md`](gateway/usage-store-settle-design.md) — 用量存储与结算
- [`gateway/api-history-endpoints.md`](gateway/api-history-endpoints.md) — 历史查询接口
- [`gateway/settlement-dual-storage-design.md`](gateway/settlement-dual-storage-design.md) — 结算双存储设计
- [`gateway/optimizer-usage-risk-analysis.md`](gateway/optimizer-usage-risk-analysis.md) — Optimizer 用量风险分析

### 认证
- [`auth/unified-auth-design.md`](auth/unified-auth-design.md) — 统一鉴权设计

### 规划（历史）
- [`plans/model-refactor-plan.md`](plans/model-refactor-plan.md) — Model 重构计划
- [`plans/query-refactor-plan.md`](plans/query-refactor-plan.md) — Query 重构计划
