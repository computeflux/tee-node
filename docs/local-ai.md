# 本地模型接入指南

本文档介绍如何将私有化部署的本地大模型（如 Ollama、vLLM、TGI 等）接入 TEE-Node 网关，作为模型供应商对外提供服务。

## 1. 为什么需要独立的 Local 类型

在旧版本中，本地模型通过 `ProviderOpenAI`（代码 `1`）+ 自定义 `URI` 的方式接入。这带来几个问题：

1. **语义混淆**：本地私有化模型与 OpenAI 官方模型共用同一类型，不利于运营统计和权限隔离。
2. **路由误判**：网关通过 URL 字符串猜测 provider（如 `http://192.168.x.x` 会被误判为 `openai`），导致限流策略、日志标识错误。
3. **网络架构不匹配**：本地模型通常在内网或本机，TEE-Node 无法直接 HTTP 访问。需要通过**反向长连接隧道**让本地客户端主动接入。

### v2 方案核心改进

引入 `ProviderLocal`（代码 `9`）、专用 `LocalAdaptor` 和 **Yamux 反向隧道**：

```
[用户请求]
    │
    ▼
[TEE-Node Gateway] ──Yamux Stream──> [本地客户端 local-client] ──HTTP──> [Ollama/vLLM]
    │                                    │
    ├── TCP Listener :19000              ├── TCP 长连接
    ├── Yamux Session 多路复用            ├── AcceptStream 循环
    ├── 10s 预热后入池                    ├── 转发到本地服务
    └── 独立限流策略                      └── 响应原路返回
```

**性能优势**：
- **零 HTTP Upgrade 开销**：TCP 直连，没有 WebSocket 的握手和帧封装开销
- **真正的多路复用**：Yamux 在单条 TCP 连接上创建虚拟 Stream，每个请求独占一条 Stream，天然隔离
- **低延迟**：Length-prefixed JSON 协议，序列化 overhead 极小
- **自动心跳与死链检测**：Yamux 内置 KeepAlive，断线秒级感知

## 2. 技术栈

| 组件 | 选型 | 说明 |
|------|------|------|
| 传输层 | **TCP** | 物理连接基座，无 HTTP 封装 |
| 多路复用 | **Yamux (libp2p/v5)** | HashiCorp 出品的高性能 mux，单 TCP 连接虚拟出无数 Stream |
| 应用协议 | **Length-prefixed JSON** | 4 字节长度头 + JSON 载荷，简单、无粘包 |
| 客户端 | **Go CLI** | 独立可执行文件，指数退避自动重连 |

## 3. 前提条件

1. 本地模型服务必须实现 **OpenAI 兼容的 Chat Completions API**：
   - `POST /v1/chat/completions`（非流式 + SSE 流式）
   - 请求体格式与 OpenAI 一致
   - 响应中包含 `usage` 字段（用于计费统计）

2. TEE-Node 的 `:19000` 端口可被本地客户端访问（同一机器、内网或公网）。

3. 你拥有一个 TEE-Node 链上的账户（用于注册 Provider 和 Model）。

## 4. 启动本地模型服务

### 4.1 Ollama

```bash
# 安装并启动 Ollama
ollama serve

# 拉取并运行模型（以 llama3 为例）
ollama pull llama3
```

Ollama 默认监听 `http://localhost:11434`，已内置 OpenAI 兼容 API。

### 4.2 vLLM

```bash
python -m vllm.entrypoints.openai.api_server \
  --model meta-llama/Meta-Llama-3-8B-Instruct \
  --port 8000
```

### 4.3 TGI

```bash
text-generation-launcher \
  --model-id meta-llama/Meta-Llama-3-8B-Instruct \
  --port 8080
```

## 5. 在 TEE-Node 注册 Provider

每个账户只能注册一个 Provider。

```graphql
mutation {
  contractCall(
    caller: "YOUR_WALLET_ADDRESS"
    callerType: 1
    nonce: 1
    contract: "gateway"
    method: "create_provider"
    args: [
      "MyLocalAI",           # name: 供应商名称
      "",                    # logoURL: 可选
      "",                    # description: 可选
      "1",                   # region: 1=亚太
      "1000"                 # deposit: 质押金额
    ]
    signature: "SIGNATURE_HEX"
  )
}
```

## 6. 在 TEE-Node 注册 Model（使用 ProviderLocal）

```graphql
mutation {
  contractCall(
    caller: "YOUR_WALLET_ADDRESS"
    callerType: 1
    nonce: 2
    contract: "gateway"
    method: "register_model"
    args: [
      "llama3",              # name: 模型名称
      "1",                   # inputModalities: 1=Text
      "128000",              # contextLength
      "5000",                # inPrice
      "2500",                # cachedPrice
      "15000",               # outPrice
      "9",                   # providerType: 9 = ProviderLocal ★关键★
      "",                    # URI: 留空（走隧道，不需要地址）
      "dummy-api-key"        # apiKeys: 认证密钥（与本地客户端 -key 匹配）
    ]
    signature: "SIGNATURE_HEX"
  )
}
```

#### ProviderType 代码表

| 代码 | 类型 | 说明 |
|------|------|------|
| `1` | `ProviderOpenAI` | OpenAI / 云端 OpenAI 兼容 |
| `2` | `ProviderClaude` | Anthropic Claude |
| `3` | `ProviderGemini` | Google Gemini |
| `4` | `ProviderAzure` | Azure OpenAI |
| `5` | `ProviderDeepSeek` | DeepSeek |
| `6` | `ProviderLlama` | Meta Llama |
| `7` | `ProviderMistral` | Mistral AI |
| `8` | `ProviderCohere` | Cohere |
| **`9`** | **`ProviderLocal`** | **本地私有化部署模型（Yamux 反向隧道）** |

## 7. 启动本地客户端

本地客户端是一个独立的 Go 程序，负责建立到 TEE-Node 的 Yamux 隧道，并将请求转发到本地 Ollama。

```bash
cd ext/local-client
go run main.go \
  -gateway tee-node.example.com:19000 \
  -model llama3 \
  -key dummy-api-key \
  -local http://localhost:11434
```

参数说明：

| 参数 | 说明 |
|------|------|
| `-gateway` | TEE-Node 的隧道端口地址（默认 `:19000`） |
| `-model` | 模型标识，必须与合约注册的 `name` 一致 |
| `-key` | API Key，必须与合约注册的 `apiKeys` 之一匹配 |
| `-local` | 本地模型服务地址（如 Ollama） |

客户端启动后会：
1. 通过 TCP 连接到 TEE-Node `:19000`
2. 发送注册握手（modelID, apiKey, baseUrl, models, modelUrls）
3. 创建 Yamux client session
4. 进入 `AcceptStream` 循环，等待 TEE-Node 转发请求
5. 收到请求后转发到本地 Ollama，响应通过同一条 Stream 返回
6. 断线后自动指数退避重连（1s → 2s → 4s ... 30s 封顶）

## 7.1 单客户端注册多个模型

一个 `local-client` 实例可以通过一条 Yamux 连接同时注册多个模型，并根据请求中的模型 ID 将流量路由到不同的本地服务地址。

多模型配置文件示例 `config.json`：

```json
{
  "gateway": "tee-node.example.com:19000",
  "key": "dummy-api-key",
  "local": "http://localhost:11434",
  "models": {
    "qwen2.5": "http://localhost:11434",
    "llama3": "http://localhost:8000"
  }
}
```

启动方式：

```bash
cd ext/local-client
go run main.go -config config.json
```

说明：
- `local` 为默认本地服务地址，未在 `models` 中指定地址的模型会回退到该地址。
- `models` 中的每个键都是要注册的模型 ID，值是该模型对应的本地服务地址。
- TEE-Node 收到任意已注册模型的请求后，会在同一条连接上打开 Stream，并在 `RequestFrame` 中带上目标模型 ID；`local-client` 据此路由到正确的本地地址。

## 7.2 运行时动态增删模型

`local-client` 启动后默认在 `:19001` 暴露一个管理 HTTP 接口（可用 `-admin` 或配置文件中的 `admin` 字段修改地址，空则禁用）。连接建立后，可以通过该接口动态添加或删除模型，无需重启客户端。

### 添加模型

```bash
curl -X POST http://localhost:19001/admin/models \
  -H "Content-Type: application/json" \
  -d '{"modelId":"mistral","url":"http://localhost:8000"}'
```

### 删除模型

```bash
curl -X DELETE http://localhost:19001/admin/models/mistral
```

说明：
- 管理接口会先向 TEE-Node 发送控制帧完成注册/注销，成功后再更新本地模型路由表。
- 添加/删除的模型会在断线重连后保留（已写入运行时模型表）。
- 如果客户端尚未与 TEE-Node 建立连接，调用会返回 `502` 错误。

## 8. 调用本地模型

注册完成且本地客户端在线后，用户即可调用：

```bash
curl -X POST https://tee-node.example.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "model": "MyLocalAI/llama3",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": false
  }'
```

流式调用：

```bash
curl -X POST https://tee-node.example.com/v1/chat/completions \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_ACCESS_TOKEN" \
  -d '{
    "model": "MyLocalAI/llama3",
    "messages": [{"role": "user", "content": "你好"}],
    "stream": true
  }'
```

响应头会自动带上 `X-Provider-Type: local`。

## 9. Yamux 隧道协议细节

### 9.1 连接建立

```
[LocalClient] ──TCP──> [TEE-Node:19000]
    │
    ├── 1. 发送 Handshake (length-prefixed JSON)
    │       {"version":1,"modelId":"llama3","apiKey":"xxx","baseUrl":"http://localhost:11434","models":["llama3"],"modelUrls":{"llama3":"http://localhost:11434"}}
    │
    ├── 2. TEE-Node 验证 API Key、检查最大连接数
    ├── 3. TEE-Node 创建 yamux.Server(session)
    ├── 4. LocalClient 创建 yamux.Client(session)
    └── 5. 连接进入 10s 预热期，之后标记为可用
```

### 9.2 请求转发（单条 Stream）

用户请求到达时，TEE-Node 从连接池获取 session，调用 `session.OpenStream()` 创建新 Stream：

```
TEE-Node -> LocalClient (同一条 Stream):
  [1B type=1][4B len][JSON: {"modelId":"llama3","path":"/v1/chat/completions","method":"POST","headers":{},"body":"..."}]

LocalClient -> TEE-Node (同一条 Stream):
  [1B type=2][4B len][JSON: {"status":200,"headers":{...}}]
  [1B type=3][4B len][<binary data>]      <-- Data 帧直接传输二进制，无 JSON wrap
  [1B type=3][4B len][<binary data>]
  [1B type=4][4B len=0]                   <-- Done 帧
```

每个用户请求独占一条 Yamux Stream，天然隔离，互不影响。

### 9.3 性能对比

| 指标 | WebSocket 方案 | Yamux 方案 |
|------|---------------|-----------|
| 握手开销 | HTTP Upgrade + 帧协商 | 零（TCP 直连） |
| 消息帧开销 | 2~14 字节 WebSocket 头 | 5 字节（1B type + 4B len） |
| Data 帧序列化 | JSON marshal（base64） | 二进制零拷贝 |
| 多路复用 | 单连接共享（需自己分帧） | Yamux 原生 Stream 隔离 |
| 流控 | 应用层实现 | Yamux 内置滑动窗口 |
| 心跳 | 自己实现 Ping/Pong | Yamux 内置 KeepAlive |
| 断线感知 | 数秒 ~ 数十秒 | 秒级（KeepAlive timeout） |
| 流式延迟 | 帧封装延迟 + JSON 解析 | 近乎零拷贝 |

## 10. 更新模型配置

```graphql
mutation {
  contractCall(
    caller: "YOUR_WALLET_ADDRESS"
    callerType: 1
    nonce: 3
    contract: "gateway"
    method: "update_model"
    args: [
      "1",                   # modelID
      "1", "128000", "3000", "1500", "10000",
      "9",                   # providerType: 保持 ProviderLocal
      ""                     # URI 仍留空
    ]
    signature: "SIGNATURE_HEX"
  )
}
```

## 11. 下线/停用模型

```graphql
mutation {
  contractCall(
    caller: "YOUR_WALLET_ADDRESS"
    callerType: 1
    nonce: 4
    contract: "gateway"
    method: "deactivate_model"
    args: ["1"]
    signature: "SIGNATURE_HEX"
  )
}
```

## 12. 计费与结算

本地模型与云端模型共用同一套计费体系。Ollama、vLLM、TGI 默认都会在响应中返回 `usage` 字段（`prompt_tokens` + `completion_tokens`），网关据此统计用量并扣费/结算。

## 13. 故障排查

| 现象 | 原因 | 解决 |
|------|------|------|
| "no available local client" | 本地客户端未连接或仍在 10s 预热期 | 检查客户端是否在线，等待预热完成 |
| "open yamux stream" 失败 | 本地客户端已断线但连接池未清理 | 等待 30s 心跳清理，或重启客户端 |
| 本地模型返回 502 | Ollama 未启动或端口错误 | 检查 `ollama serve` 和 `-local` 参数 |
| 流式响应中断 | 网络不稳定或本地模型超时 | 检查客户端日志和 Ollama 日志 |

## 14. 安全建议

1. **防火墙**：TEE-Node 的 `:19000` 应仅对可信内网开放，或前置 TLS（未来可扩展为 `tls.Listen`）。
2. **API Key**：合约中 `apiKeys` 与客户端 `-key` 必须匹配，防止未授权客户端接入。
3. **连接数上限**：Yamux `MaxStreamWindowSize` 已设为 1MB，可根据本地 GPU 显存调整并发。
4. **网络隔离**：本地客户端与 Ollama 建议在同一台机器或同一内网，避免公网暴露 Ollama 端口。
