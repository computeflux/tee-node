# Tailscale Cross-Region TEE Node Proxy

基于 Tailscale mesh VPN 的区块链 TEE 节点跨区域流量转发系统。

## 架构

```
┌──────────────────────┐           ┌──────────────────────┐
│     Region A (US)     │           │     Region B (EU)     │
│                       │           │                       │
│  User                 │           │                       │
│    │                  │           │                       │
│    ▼                  │           │                       │
│  ┌──────────┐         │  TS VPN   │  ┌──────────┐        │
│  │  Proxy   │◄────────┼───────────┼──│ TEE Node │        │
│  │  :8080   │         │           │  │  :9944   │        │
│  └────┬─────┘         │           │  └──────────┘        │
│       │               │           │                       │
│       │ X-Chain-Id:   │           │                       │
│       │   polkadot    │           │                       │
│       │ X-Service:    │           │                       │
│       │   rpc         │           │                       │
│       ▼               │           │                       │
│  Route Table          │           │                       │
│  polkadot+rpc →       │           │                       │
│  tee-node-eu:9944     │           │                       │
└──────────────────────┘           └──────────────────────┘
```

## 工作流程

1. 所有 TEE 节点加入同一个 Tailscale tailnet，通过 WireGuard 建立加密 mesh 网络
2. 每个节点的 Proxy 监听本地端口（默认 `:8080`）
3. 用户请求带 `X-Chain-Id` 和 `X-Service` 头，发送到入口节点
4. Proxy 查询 Route Table，找到目标节点的 Tailscale 地址
5. 通过 Tailscale 加密隧道转发请求到目标节点
6. 响应原路返回

## 配置

```yaml
# config.yaml
tailscale:
  auth_key: "tskey-auth-xxxxx"
  hostname: "tee-node-us-east"

proxy:
  listen_addr: ":8080"

routes:
  - chain_id: "polkadot"
    service: "rpc"
    target: "tee-node-eu-west:9944"

services:
  - chain_id: "polkadot"
    service: "graphql"
    port: 4000
    region: "us-east"
```

## 路由策略

路由分三级优先级：

1. **静态路由**（配置文件 `routes` 字段）
2. **对等节点发现**（自动检测 Tailscale 上广告同类服务的节点）
3. **兜底路由**（`fallback` 字段）

## 使用方式

### 代码集成

```go
import "github.com/computeflux/tee-provider/ext/tailscale"

func main() {
    cfg, _ := tailscale.LoadConfig("config.yaml")
    
    // 自动创建 Node + Proxy + Router 并启动
    log.Fatal(tailscale.SetupAndServe(cfg))
}
```

### curl 示例

```bash
# 通过 US 节点查询 EU 区域的 Polkadot RPC
curl -H "X-Chain-Id: polkadot" \
     -H "X-Service: rpc" \
     -X POST -d '{"method":"chain_getBlock"}' \
     http://tee-node-us-east:8080/

# 通过 query 参数
curl "http://tee-node-us-east:8080/?chain_id=polkadot&service=rpc" \
     -X POST -d '{"method":"chain_getBlock"}'

# 健康检查
curl http://tee-node-us-east:8080/health
```

## 先决条件

1. Tailscale 账号和 auth key
2. 所有节点在同一个 tailnet 中
3. Go 1.21+
