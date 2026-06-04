# 侧链（Tee Chain）

本目录包含侧链核心（CometBFT ABCI）相关的设计文档。

| 文档 | 说明 |
|------|------|
| [`tee-chain-design.md`](tee-chain-design.md) | 侧链逻辑架构图（初始化、秘密上传/解密、DKG、交易处理、代理重加密） |
| [`relay-design.md`](relay-design.md) | 主链→侧链 Relay 同步架构（信任 RPC + nonce 防重放） |
| [`tee-tx-retry-design.md`](tee-tx-retry-design.md) | 交易失败重试机制设计 |
| [`unified-tx-design.md`](unified-tx-design.md) | 统一交易格式与生命周期设计 |
