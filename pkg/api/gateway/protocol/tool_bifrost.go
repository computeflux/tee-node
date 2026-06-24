package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/core/providers/anthropic"
	"github.com/maximhq/bifrost/core/schemas"
)

// ExtractBifrostChatTools 把 Anthropic 请求体解析为 Bifrost 的 []schemas.ChatTool。
// 复用 Bifrost 已有的 Anthropic → Responses → Chat 转换路径，确保响应侧使用的
// schema 和实际发向上游请求体的 schema 完全一致。
func ExtractBifrostChatTools(anthropicBody []byte, targetProvider uint16) []schemas.ChatTool {
	chatReq, err := convertAnthropicBodyToChatReq(anthropicBody, targetProvider)
	if err != nil || chatReq == nil || chatReq.Params == nil {
		return nil
	}

	tools := chatReq.Params.Tools
	for i := range tools {
		if tools[i].Function != nil && tools[i].Function.Parameters != nil {
			tools[i].Function.Parameters = tools[i].Function.Parameters.Normalized()
		}
	}

	// Bifrost 的 Responses -> Chat 降级会丢弃 Anthropic 内置 client/server tool
	//（如 bash、text_editor、computer）。把它们重新合成为 function tool，保证
	// OpenAI 兼容上游能拿到完整 schema。
	serverTools := synthesizeAnthropicServerTools(anthropicBody)
	if len(serverTools) > 0 {
		tools = mergeSynthesizedTools(tools, serverTools)
	}

	return tools
}

// mergeSynthesizedTools 把合成的 Anthropic 内置工具合并进已有工具列表。
// 如果同名工具已存在但缺少 parameters（Bifrost Responses->Chat 降级会保留 name
// 但丢掉 schema），则用合成的 schema 替换；否则保留已有工具避免重复。
func mergeSynthesizedTools(tools []schemas.ChatTool, serverTools []schemas.ChatTool) []schemas.ChatTool {
	idxByName := make(map[string]int, len(tools))
	for i, t := range tools {
		if t.Function != nil {
			idxByName[t.Function.Name] = i
		}
	}
	for _, t := range serverTools {
		if t.Function == nil {
			continue
		}
		if i, ok := idxByName[t.Function.Name]; ok {
			existing := tools[i]
			if existing.Function != nil && (existing.Function.Parameters == nil || parametersEmpty(existing.Function.Parameters)) {
				tools[i] = t
			}
			continue
		}
		idxByName[t.Function.Name] = len(tools)
		tools = append(tools, t)
	}
	return tools
}

// parametersEmpty 判断 ToolFunctionParameters 是否为空 schema（没有 properties 等实质定义）。
func parametersEmpty(p *schemas.ToolFunctionParameters) bool {
	if p == nil {
		return true
	}
	return p.Properties == nil || len(p.Properties.Keys()) == 0
}

// HardenToolParameters 针对 DeepSeek/MiniMax 等容易乱生成参数的模型，给 tool schema
// 增加硬约束，并压缩过长的工具描述以减少上下文长度。返回的是副本，不会修改传入的 tools。
func HardenToolParameters(tools []schemas.ChatTool, model string) []schemas.ChatTool {
	if !shouldHardenToolParameters(model) {
		return tools
	}

	out := make([]schemas.ChatTool, len(tools))
	for i, t := range tools {
		out[i] = t
		if t.Function != nil && t.Function.Parameters != nil {
			fn := *t.Function
			fn.Parameters = hardenParameters(t.Function.Parameters, model)
			out[i].Function = &fn
		}
	}
	out = compressToolDescriptions(out, model)
	return out
}

// shouldHardenToolParameters 按模型名判断是否启用 schema 加固。
func shouldHardenToolParameters(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "deepseek") || strings.Contains(m, "minimax")
}

// shouldEnforceAdditionalProperties 仅对 MiniMax 启用 additionalProperties=false。
// DeepSeek 对该字段兼容性不好，会误导致模型输出空 {}，因此在请求侧不加，
// 依赖响应侧按 properties 白名单过滤未知字段。
func shouldEnforceAdditionalProperties(model string) bool {
	m := strings.ToLower(model)
	return strings.Contains(m, "minimax")
}

// compressToolDescriptions 把超长的工具描述截短，降低非 Claude 上游的上下文压力。
// 只保留第一段；若仍超长则按单词边界截断并加 " ..." 后缀。
func compressToolDescriptions(tools []schemas.ChatTool, model string) []schemas.ChatTool {
	if !shouldHardenToolParameters(model) {
		return tools
	}
	const maxLen = 500
	out := make([]schemas.ChatTool, len(tools))
	for i, t := range tools {
		out[i] = t
		if t.Function != nil && t.Function.Description != nil {
			fn := *t.Function
			truncated := truncateDescription(*t.Function.Description, maxLen)
			fn.Description = &truncated
			out[i].Function = &fn
		}
	}
	return out
}

func truncateDescription(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	// 优先保留完整第一段（以空行为界）。
	if idx := strings.Index(s, "\n\n"); idx > 0 {
		first := strings.TrimSpace(s[:idx])
		if len([]rune(first)) <= maxLen {
			return first
		}
	}
	// 在 maxLen 之前最后一个空格处截断，避免切断单词。
	cut := maxLen
	for cut > 0 && runes[cut] != ' ' {
		cut--
	}
	if cut <= 0 {
		cut = maxLen
	}
	return string(runes[:cut]) + " ..."
}

// stripAdditionalPropertiesFalse 递归删除 schema map 中的 additionalProperties:false，
// 用于缓解 DeepSeek 因该限制而保守输出空 {} 的问题。
func stripAdditionalPropertiesFalse(m map[string]any) {
	if m == nil {
		return
	}
	if ap, ok := m["additionalProperties"]; ok && ap == false {
		delete(m, "additionalProperties")
	}
	for _, v := range m {
		switch x := v.(type) {
		case map[string]any:
			stripAdditionalPropertiesFalse(x)
		case []any:
			for _, elem := range x {
				if em, ok := elem.(map[string]any); ok {
					stripAdditionalPropertiesFalse(em)
				}
			}
		}
	}
}

// hardenParameters 给 schema 增加约束，从而减少模型输出空值或多余字段的概率。
// DeepSeek 对 additionalProperties=false 有兼容性问题（会误导致空 {} 输出），
// 因此只对 MiniMax 加 additionalProperties=false；对 required string 则统一加
// minLength=1。
func hardenParameters(p *schemas.ToolFunctionParameters, model string) *schemas.ToolFunctionParameters {
	// 先转成 map 以便灵活修改；丢失的 key 顺序对 DeepSeek/MiniMax 不影响功能。
	b, err := json.Marshal(p)
	if err != nil {
		return p
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return p
	}

	props, _ := m["properties"].(map[string]any)
	required := requiredSet(m)

	for name, propAny := range props {
		prop, ok := propAny.(map[string]any)
		if !ok {
			continue
		}
		typ := typeString(prop["type"])
		if required[name] && typ == "string" && prop["minLength"] == nil {
			prop["minLength"] = int64(1)
		}
		if shouldEnforceAdditionalProperties(model) && typ == "object" && prop["additionalProperties"] == nil {
			prop["additionalProperties"] = false
		}
		props[name] = prop
	}

	// DeepSeek 对 additionalProperties=false 兼容性较差，容易因害怕输出未知字段而直接返回空 {}。
	// 上游过滤逻辑会在响应阶段删除不在 properties 中的字段，因此请求侧可以安全地放宽该限制。
	if !shouldEnforceAdditionalProperties(model) {
		stripAdditionalPropertiesFalse(m)
	}

	b, _ = json.Marshal(m)
	var out schemas.ToolFunctionParameters
	if err := json.Unmarshal(b, &out); err != nil {
		return p
	}
	return &out
}

// FilterBifrostToolCalls 按请求时注册的 schemas.ChatTool 校验并修复响应中的 tool calls。
func FilterBifrostToolCalls(resp *schemas.BifrostChatResponse, tools []schemas.ChatTool) {
	if resp == nil || len(tools) == 0 {
		return
	}
	schemaMap := buildToolSchemaMap(tools)

	for i := range resp.Choices {
		choice := &resp.Choices[i]
		if choice.ChatNonStreamResponseChoice == nil || choice.ChatNonStreamResponseChoice.Message == nil {
			continue
		}
		msg := choice.ChatNonStreamResponseChoice.Message
		if msg.ChatAssistantMessage == nil || len(msg.ChatAssistantMessage.ToolCalls) == 0 {
			continue
		}
		for j := range msg.ChatAssistantMessage.ToolCalls {
			tc := &msg.ChatAssistantMessage.ToolCalls[j]
			toolName := ""
			if tc.Function.Name != nil {
				toolName = *tc.Function.Name
			}
			tc.Function.Arguments = repairToolArguments(tc.Function.Arguments, toolName, schemaMap)
		}
	}
}

// buildToolSchemaMap 把 []schemas.ChatTool 转成按 function name 索引的 schema map。
func buildToolSchemaMap(tools []schemas.ChatTool) map[string]map[string]any {
	m := make(map[string]map[string]any, len(tools))
	for _, t := range tools {
		if t.Function == nil || t.Function.Parameters == nil {
			continue
		}
		if schema := schemaMapFromParams(t.Function.Parameters); schema != nil {
			m[t.Function.Name] = schema
		}
	}
	return m
}

// schemaMapFromParams 把 ToolFunctionParameters 转成普通 map，便于递归过滤。
func schemaMapFromParams(params *schemas.ToolFunctionParameters) map[string]any {
	if params == nil {
		return nil
	}
	b, err := json.Marshal(params)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil
	}
	return m
}

// repairToolArguments 尝试把上游返回的 tool arguments 按 schema 修复。
// 如果 JSON 非法或截断，先尝试简单 repair，再按 schema 过滤/补零值。
func repairToolArguments(args, toolName string, schemaMap map[string]map[string]any) string {
	if args == "" {
		args = "{}"
	}
	repaired := repairJSONArguments(args)
	var input map[string]any
	if err := json.Unmarshal([]byte(repaired), &input); err != nil {
		input = map[string]any{}
	} else {
		input = filterInputByMap(input, schemaMap[toolName])
	}
	b, err := json.Marshal(input)
	if err != nil {
		return "{}"
	}
	return string(b)
}

// filterInputByMap 按 JSON schema map 过滤输入：删除不在 properties 中的字段，
// 为缺失的 required 字段补零值，支持嵌套 object/array。
func filterInputByMap(input map[string]any, schema map[string]any) map[string]any {
	if schema == nil {
		return input
	}
	typ, _ := schema["type"].(string)
	if typ != "object" {
		return input
	}
	props, _ := schema["properties"].(map[string]any)
	if props == nil {
		return input
	}

	required := requiredSet(schema)
	filtered := make(map[string]any, len(input))
	for k, v := range input {
		if prop, ok := props[k]; ok {
			filtered[k] = filterValueBySchemaMap(v, prop)
		}
	}
	for k := range required {
		if _, ok := filtered[k]; !ok {
			filtered[k] = zeroValueForPropertyMap(props[k])
		}
	}
	return filtered
}

// filterValueBySchemaMap 递归过滤单个值。
func filterValueBySchemaMap(v any, prop any) any {
	propMap, ok := prop.(map[string]any)
	if !ok {
		return v
	}
	typ := typeString(propMap["type"])
	switch typ {
	case "object":
		m, ok := v.(map[string]any)
		if !ok {
			return zeroValueForPropertyMap(propMap)
		}
		return filterInputByMap(m, propMap)
	case "array":
		arr, ok := v.([]any)
		if !ok {
			return zeroValueForPropertyMap(propMap)
		}
		out := make([]any, len(arr))
		for i, elem := range arr {
			out[i] = filterValueBySchemaMap(elem, propMap["items"])
		}
		return out
	default:
		return v
	}
}

// zeroValueForPropertyMap 根据 JSON schema 类型返回零值。
func zeroValueForPropertyMap(prop any) any {
	propMap, ok := prop.(map[string]any)
	if !ok {
		return ""
	}
	typ := typeString(propMap["type"])
	if strings.Contains(typ, "string") {
		return ""
	}
	if strings.Contains(typ, "integer") || strings.Contains(typ, "number") {
		return 0
	}
	if strings.Contains(typ, "boolean") {
		return false
	}
	if strings.Contains(typ, "array") {
		return []any{}
	}
	if strings.Contains(typ, "object") {
		return map[string]any{}
	}
	return ""
}

// requiredSet 从 schema map 中提取 required 字段集合。
func requiredSet(schema map[string]any) map[string]bool {
	set := make(map[string]bool)
	req, _ := schema["required"].([]any)
	for _, r := range req {
		if s, ok := r.(string); ok {
			set[s] = true
		}
	}
	return set
}

// typeString 把 schema 的 type 字段转成字符串；支持 JSON Schema 的数组类型。
func typeString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case []any:
		for _, item := range x {
			if s, ok := item.(string); ok {
				return s
			}
		}
	case []string:
		if len(x) > 0 {
			return x[0]
		}
	}
	return ""
}

// repairJSONArguments 对上游可能截断的 JSON 做轻量修复。
// 仅做括号/字符串平衡；修复失败则返回原字符串，由调用方再回退到 {}。
func repairJSONArguments(s string) string {
	if json.Valid([]byte(s)) {
		return s
	}
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return s
	}

	var openBraces, openBrackets int
	inString := false
	escape := false

	for _, r := range trimmed {
		if escape {
			escape = false
			continue
		}
		if r == '\\' {
			escape = true
			continue
		}
		if r == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		switch r {
		case '{':
			openBraces++
		case '}':
			openBraces--
		case '[':
			openBrackets++
		case ']':
			openBrackets--
		}
	}

	repaired := trimmed
	if inString {
		// 如果最后一个字符串未完成，先补关闭引号。
		repaired += "\""
	}
	for openBrackets > 0 {
		repaired += "]"
		openBrackets--
	}
	for openBraces > 0 {
		repaired += "}"
		openBraces--
	}

	if json.Valid([]byte(repaired)) {
		return repaired
	}
	return s
}

// convertAnthropicBodyToChatReq 把 Anthropic 请求体转成 BifrostChatRequest。
// 供请求转换和 schema 提取共用同一套逻辑。
func convertAnthropicBodyToChatReq(body []byte, targetProvider uint16) (*schemas.BifrostChatRequest, error) {
	var anthropicReq anthropic.AnthropicMessageRequest
	if err := json.Unmarshal(body, &anthropicReq); err != nil {
		return nil, fmt.Errorf("parse anthropic request: %w", err)
	}

	ctx := schemas.NewBifrostContext(nil, schemas.NoDeadline)
	responsesReq := anthropicReq.ToBifrostResponsesRequest(ctx)
	if responsesReq == nil {
		return nil, fmt.Errorf("ToBifrostResponsesRequest returned nil")
	}
	chatReq := responsesReq.ToChatRequest()
	if chatReq == nil {
		return nil, fmt.Errorf("ToChatRequest returned nil")
	}
	chatReq.Provider = providerTypeToBifrost(targetProvider)
	return chatReq, nil
}
