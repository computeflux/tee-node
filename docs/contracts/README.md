# 智能合约与 WASM

本目录包含合约存储、安全和 WASM 运行时相关的设计文档。

| 文档 | 说明 |
|------|------|
| [`contract-storage-design.md`](contract-storage-design.md) | 合约存储分层设计（StoreMapping / StoreList / IndexedList / StoreIndex） |
| [`contract-security-analysis.md`](contract-security-analysis.md) | 合约安全审计与 Nonce 设计方案 |
| [`go-contract-to-wasm-migration.md`](go-contract-to-wasm-migration.md) | Go 原生合约迁移至 WASM 的技术路径与实施记录 |
| [`wasm-runtime-host-functions.md`](wasm-runtime-host-functions.md) | WASM Host Functions 完整方案（含历史设计与实际落地） |
| [`minimal-wasm-host-functions.md`](minimal-wasm-host-functions.md) | WASM Host Functions 实际实现（wazero + TinyGo，11 个 host functions） |
