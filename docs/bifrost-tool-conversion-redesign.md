# 基于 Bifrost 原生机制的 Anthropic↔OpenAI 工具参数转换方案

## 1. 现状与问题

当前网关在 `/anthropic/v1/messages` 入口已经做了两层工作：

1. **请求转换**：`ConvertAnthropicRequestToOpenAI` 走 Bifrost 的 Responses → Chat → OpenAI 路径：
   ```go
   anthropicReq.ToBifrostResponsesRequest(ctx)
     .ToChatRequest()
   openai.ToOpenAIChatRequest(ctx, chatReq)
   ```
   这说明请求侧已经高度依赖 Bifrost，工具 schema 在转换过程中被规范化为 `schemas.ChatTool`/`schemas.ToolFunctionParameters`。

2. **响应过滤**：非流式/流式响应目前先用 Bifrost 的 Chat → Responses → Anthropic 路径转成 Anthropic 格式，再在**原始 JSON 上**用自定义的 `ToolSchema` 做字段过滤和 required 零值填充。`ToolSchema` 是本地重新定义的轻量结构，和 Bifrost 的 `schemas.ToolFunctionParameters` 是平行实现。

当前遇到的核心问题：

| 问题 | 根因 | 现有网关层兜底能力 |
|------|------|-------------------|
| DeepSeek 对 Bash 等工具生成空 `{}` | 模型未按 schema 填充 required 字段 | 可以补零值，但补成 `command=""` 后 Bash 执行仍失败，导致 Claude Code 循环重试 |
| MiniMax 流式 tool arguments 被截断 | 模型输出 JSON 不完整 / 流式解析鲁棒性不足 | 只能回退到 `{}`，无法修复或优雅降级 |
| MiniMax 多传 `description`、未知字段 | 模型不遵守 `additionalProperties` | 可以按 properties 白名单过滤 |
| schema 只处理了顶层字段 | 自定义 `ToolSchema` 未利用 Bifrost 的 nested schema、OrderedMap、Normalization | 无法处理嵌套对象/数组的 required 补全 |

**结论**：单纯在网关层做 schema 字段兜底无法根治模型输出质量导致的空值/截断问题；需要把工具参数的“描述、校验、修复”全面迁移到 Bifrost 自身的类型和机制上，同时在请求侧增强 schema 以提升模型遵循度。

## 2. 可利用的 Bifrost 原生能力

Bifrost 源码（`github.com/maximhq/bifrost/core`）已经提供我们所需的关键能力：

| 能力 | 所在文件/函数 | 用途 |
|------|--------------|------|
| Anthropic 请求 → Bifrost Responses 请求 | `providers/anthropic/responses.go` `AnthropicMessageRequest.ToBifrostResponsesRequest` | 把 Claude Code 的 Anthropic 请求解析为 Bifrost 中立结构 |
| Responses → Chat 降级 | `schemas/mux.go` `BifrostResponsesRequest.ToChatRequest` | 得到 `schemas.BifrostChatRequest`，内含 `[]schemas.ChatTool` |
| 工具函数参数规范化 | `schemas/chatcompletions.go` `ToolFunctionParameters.Normalized()` | 稳定 key 顺序，便于 prompt caching；可在此基础上做 schema 加固 |
| Chat 请求 → OpenAI 请求 | `providers/openai/chat.go` `openai.ToOpenAIChatRequest` | 生成上游 OpenAI 格式请求；`MarshalJSON` 会自动丢弃 Anthropic server tools |
| OpenAI 响应 → Bifrost Chat 响应 | `schemas.BifrostChatResponse` 直接 unmarshal | 上游 JSON 可直接解析为 Bifrost 中立响应 |
| Bifrost Chat 响应 → Anthropic 响应 | `providers/anthropic/chat.go` `anthropic.ToAnthropicChatResponse` | 比 Chat→Responses→Anthropic 更直接，专用于 Messages API |
| 工具参数结构 | `schemas.ChatAssistantMessageToolCall` / `ChatAssistantMessageToolCallFunction` | `Arguments` 是字符串，可在 Bifrost 层做校验和改写 |
| Provider 特性校验 | `providers/anthropic/utils.go` `ValidateChatToolsForProvider` | 明确哪些 server tools 不能发给 OpenAI 类上游 |

## 3. 新方案总体架构

目标：**让工具参数在全链路都以 `schemas.ChatTool` / `schemas.ToolFunctionParameters` 为唯一 canonical schema，并在请求/响应两侧分别做“加固”和“修复”。**

```
Claude Code (Anthropic)
        │
        ▼
AnthropicMessageRequest ──Bifrost──► BifrostResponsesRequest ──ToChatRequest──► BifrostChatRequest
        │                                                                           │
        │                                    ┌──────────────────────────────────────┘
        │                                    │
        │                                    ▼
        │                         []schemas.ChatTool (canonical schemas)
        │                                    │
        │                    ┌───────────────┴───────────────┐
        │                    ▼                               ▼
        │           请求侧：HardenToolParameters        响应侧：FilterBifrostToolCalls
        │           (加固 schema 发给上游)               (按 schema 修复上游返回的参数)
        │                    │                               │
        │                    ▼                               ▼
        │           openai.ToOpenAIChatRequest            anthropic.ToAnthropicChatResponse
        │                    │                               │
        ▼                    ▼                               ▼
   原始 Anthropic JSON   OpenAI JSON                    Anthropic JSON
```

## 4. 详细设计

### 4.1 统一 schema 表示：用 `[]schemas.ChatTool` 替代自定义 `ToolSchema`

新增 `pkg/api/gateway/protocol/tool_bifrost.go`：

```go
// ExtractBifrostChatTools 把 Anthropic 请求体解析为 Bifrost 的 ChatTool 列表。
// 复用已有的 Responses → Chat 转换路径，确保和 upstream 请求体使用的 schema 完全一致。
func ExtractBifrostChatTools(anthropicBody []byte, targetProvider uint16) []schemas.ChatTool
```

实现要点：

1. `json.Unmarshal` 到 `anthropic.AnthropicMessageRequest`。
2. 调用 `req.ToBifrostResponsesRequest(ctx)` → `ToChatRequest()`。
3. 返回 `chatReq.Params.Tools`。
4. 对每个 `tool.Function.Parameters` 调用 `Normalized()`，保证 properties key 顺序和 Bifrost 发向上游时一致。

`relay_handler.go` 中把：

```go
toolSchemas = protocol.ExtractToolSchemas(bodyBytes)
```

替换为：

```go
toolSchemas := protocol.ExtractBifrostChatTools(bodyBytes, modelData.Data.ProviderType)
```

后续过滤全部基于 `schemas.ChatTool`。

### 4.2 请求侧 schema 加固：`HardenToolParameters`

DeepSeek/MiniMax 等 OpenAI 兼容上游对 JSON Schema 的遵循度较弱，需要在发向上游前给 schema 增加“硬约束”。实现一个可选的 schema transformer：

```go
// HardenToolParameters 针对已知容易乱生成参数的模型，给 schema 增加约束。
// 仅当 model 命中 deepseek/minimax 前缀时启用，避免影响 OpenAI/Claude 等规范上游。
func HardenToolParameters(tools []schemas.ChatTool, model string) []schemas.ChatTool
```

具体加固规则（可逐步迭代）：

| 规则 | 作用 | 对应字段 |
|------|------|---------|
| 对顶层 required 的 `string` 字段且未设置 `minLength` 的，设置 `minLength = 1` | 减少 DeepSeek 生成 `""` 或空对象的概率 | `ToolFunctionParameters.MinLength` |
| 对 object 类型的参数且未设置 `additionalProperties` 的，设置为 `false` | 阻止 MiniMax 多传 `description` 等未知字段 | `AdditionalPropertiesBool = false` |
| （可选）给 `bash`/`str_replace_editor` 等关键工具的 `command` 字段注入 `default` 示例 | 让模型更清楚期望的输入形式 | `ToolFunctionParameters.Default` |

实现时注意：

- 修改的是 `[]schemas.ChatTool` 的副本，不要污染原始请求里缓存的 schema。
- 只在目标模型名匹配 `deepseek-*` / `minimax-*` 时启用，默认不改动。
- 加固后的 `Parameters` 仍通过 `openai.ToOpenAIChatRequest` 序列化，Bifrost 的 `MarshalJSON` 会自动处理 `OrderedMap`。

### 4.3 响应侧修复：在 Bifrost 对象层过滤 tool arguments

#### 4.3.1 非流式路径

把 `ConvertOpenAIResponseToAnthropic` 改为：

```go
var chatResp schemas.BifrostChatResponse
if err := json.Unmarshal(body, &chatResp); err != nil { ... }

// 用 Bifrost ChatTool 修复 tool arguments
filterBifrostToolCalls(&chatResp, tools)

anthropicResp := anthropic.ToAnthropicChatResponse(&chatResp)
out, err := json.Marshal(anthropicResp)
```

不再走 `Chat → Responses → Anthropic` 的间接路径，直接使用 `anthropic.ToAnthropicChatResponse`。

#### 4.3.2 `filterBifrostToolCalls` 实现

```go
// filterBifrostToolCalls 按请求时注册的 schemas.ChatTool 校验并修复响应中的 tool calls。
func filterBifrostToolCalls(resp *schemas.BifrostChatResponse, tools []schemas.ChatTool)
```

对每个 `choice.Message.ChatAssistantMessage.ToolCalls`：

1. 解析 `Function.Arguments` 字符串为 `map[string]any`。
2. 按工具名找到对应的 `schemas.ChatTool`。
3. 调用 `filterInputBySchema(input, params)`：
   - 以 `Parameters.Properties` 为白名单，删除未知 key（解决 MiniMax 多传 `description`）。
   - 对 `Parameters.Required` 中缺失的字段，递归按 property type 生成零值（string `""`, number `0`, bool `false`, array `[]`, object `{}`）。
   - 支持嵌套 object / array（通过递归）。
4. 如果解析失败或修复后仍不满足关键约束（如 required string 为空），根据策略处理：
   - **当前默认**：按原逻辑回退到 `{}`（兼容现有行为）。
   - **可选激进策略**：把该 tool_use 从响应中移除或改为 `text` 错误提示，避免 Claude Code 进入重试循环。
5. 重新 `json.Marshal` 并写回 `Function.Arguments`。

这样 `anthropic.ToAnthropicChatResponse` 看到的 `Arguments` 已经是合法且符合 schema 的 JSON，不会再被替换成 `{}`。

#### 4.3.3 流式路径

流式仍使用现有的 `StreamOpenAIResponseToAnthropic`，但做三处升级：

1. `toolSchemas` 类型改为 `[]schemas.ChatTool`。
2. 在 `content_block_stop` / `finish` 时，把累积的 `tc.arguments` 解析为 `map[string]any` 后调用基于 `schemas.ChatTool` 的 `filterInputBySchema`。
3. **截断鲁棒性**：
   - 当前实现一旦 `json.Unmarshal` 失败就回退 `{}`。
   - 新实现引入一个轻量级 JSON repair：如果 `tc.arguments` 不完整（常见如缺少 `}`），尝试用 `github.com/tidwall/sjson` 补全或截断到最后一个完整 key；失败再回退 `{}`。
   - 对于已确认截断的 tool call，可以选择不发 `content_block_stop`（让客户端感知为未完成），但 Anthropic 协议要求必须关闭；因此优先 repair + 日志。

> 注：Bifrost 内部没有现成的“流式 OpenAI SSE → Anthropic SSE”反向转换器，完全替换流式转换成本较高。先保留现有 SSE 状态机，只把 schema 层替换为 Bifrost 原生结构。

### 4.4 与 Bifrost `ValidateChatToolsForProvider` 结合

在请求转换后、发向上游前，可以显式调用：

```go
keep, dropped := anthropic.ValidateChatToolsForProvider(chatReq.Params.Tools, provider)
if len(dropped) > 0 {
    util.DebugLog("[relay] dropped unsupported server tools for OpenAI path: %v", dropped)
}
chatReq.Params.Tools = keep
```

`openai.OpenAIChatRequest.MarshalJSON` 本来也会丢弃 Anthropic server tools，显式调用可以：

- 提前记录日志，便于排查“工具消失”问题。
- 对自定义 provider 也保持一致行为。

## 5. 实施步骤

1. **新增 `tool_bifrost.go`**
   - `ExtractBifrostChatTools`
   - `filterInputBySchema` / `filterBifrostToolCalls`
   - `HardenToolParameters`（默认关闭，按模型名开关）
   - 配套零值/类型处理辅助函数
2. **改造 `relay_handler.go`**
   - 用 `ExtractBifrostChatTools` 替换 `ExtractToolSchemas`
   - 在请求转换后把 `chatReq.Params.Tools` 传入 converter，供响应侧使用
   - 对 DeepSeek/MiniMax 模型调用 `HardenToolParameters`
3. **改造 `anthropic.go` 非流式响应**
   - `ConvertOpenAIResponseToAnthropic` 走 `schemas.BifrostChatResponse → filterBifrostToolCalls → anthropic.ToAnthropicChatResponse`
   - 移除对 `filterAnthropicResponseToolInputs` 的依赖，或保留作为兼容兜底
4. **改造 `stream.go` 流式响应**
   - `toolSchemas` 改为 `[]schemas.ChatTool`
   - 用 Bifrost schema 替换 `ToolSchema.FilterInput`
   - 加入 JSON repair 逻辑
5. **删除/弃用 `tool_schema.go`**
   - 若 `filterAnthropicResponseToolInputs` 仍需保留做兜底，可保留；否则删除自定义 `ToolSchema`
6. **测试**
   - 单元测试：DeepSeek 空 `{}`、MiniMax 多传字段、嵌套 required 补全、截断 repair
   - 集成测试：真实 Claude Code → gateway → DeepSeek/MiniMax 的 Bash 调用是否不再循环
7. **灰度与观测**
   - 通过 `util.DebugLog` 输出：schema 数量、被过滤字段、修复动作、repair 触发次数
   - 对 DeepSeek/MiniMax 先小流量验证，确认无回归后全量

## 6. 风险与回退

| 风险 | 缓解措施 |
|------|---------|
| `HardenToolParameters` 改变模型行为 | 仅对已知问题模型启用；可配置开关 |
| 响应侧从 Responses 路径切到 Chat 路径导致字段差异 | `ToAnthropicChatResponse` 是 Bifrost 为 Messages API 设计的原生转换；对比测试 usage/thinking/stop_reason 等字段 |
| 流式 JSON repair 引入不可预期输出 | repair 失败回退 `{}`；记录原始截断片段便于事后分析 |
| 删除 `ToolSchema` 后外部测试依赖破坏 | `tool_schema_test.go` 同步改写为基于 `schemas.ChatTool` 的测试 |
| 嵌套 schema 零值生成错误 | 单元测试覆盖 object/array/anyOf 等复杂结构 |

## 7. 预期收益

- **Schema 唯一源**：不再维护本地 `ToolSchema` 和 Bifrost `ToolFunctionParameters` 两套结构，减少不一致。
- **更强的校验**：利用 Bifrost 的完整 JSON Schema 字段（`additionalProperties`、`minLength`、`anyOf` 等）。
- **请求侧质量提升**：通过 schema 加固降低 DeepSeek/MiniMax 生成空/非法参数的概率。
- **响应侧修复前置**：在 Bifrost 对象层修复，避免 `ToAnthropicChatResponse` 把非法 JSON 替换成 `{}`。
- **可观测性增强**：显式记录 dropped server tools、过滤动作、repair 触发次数。

## 8. 下一步建议

按“实施步骤”顺序开发，优先完成 1-4（非流式改造 + 流式 schema 替换），即可解决当前 Claude Code 报 `Invalid tool parameters` 和 Bash 空命令循环的问题。`HardenToolParameters` 和 JSON repair 可作为第二阶段补充，用于进一步降低 MiniMax 截断和 DeepSeek 空参数的发生率。
