# AI API 网关（Gateway）

本目录包含 AI 模型请求转发、计费、结算和 API Key 管理相关的设计文档。

| 文档 | 说明 |
|------|------|
| [`ai-gateway-design.md`](ai-gateway-design.md) | Gateway Relay 详细设计（中间件链、适配器、路由、限流、监控） |
| [`api-key-design.md`](api-key-design.md) | API Key 链上注册、鉴权与安全模型 |
| [`usage-settle-design.md`](usage-settle-design.md) | 用量上链结算架构（统一批量提交 `[]Settlement`） |
| [`usage-store-settle-design.md`](usage-store-settle-design.md) | 用量存储与结算设计 |
| [`api-history-endpoints.md`](api-history-endpoints.md) | 充值/结算历史查询接口 |
| [`settlement-dual-storage-design.md`](settlement-dual-storage-design.md) | 结算记录双存储设计（Provider 维度 + User 维度） |
| [`optimizer-usage-risk-analysis.md`](optimizer-usage-risk-analysis.md) | Optimizer 用量统计风险分析 |
