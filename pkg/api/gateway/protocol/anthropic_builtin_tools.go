package protocol

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/maximhq/bifrost/core/schemas"
)

// builtinToolSchemas 保存 Anthropic 内置 client/server tool 的输入 schema。
// 这些工具在 Bifrost 的 Responses -> Chat 降级时会被丢弃，导致上游模型
// 收不到 schema 而随意生成参数，最终触发 Claude Code 的 "Invalid tool parameters"。
// 这里把它们重新合成为 OpenAI function tool，使上游模型按规范生成参数。
//
// 注意：schema 来自 Anthropic 官方文档；版本前缀只决定保留哪些配置字段，
// 输入参数本身在不同版本间基本兼容。
//
// 默认不设置 additionalProperties=false，因为 DeepSeek 对它有兼容性问题，
// 会误导致模型输出空 {}。请求侧仍然用 properties 白名单约束，响应侧也会
// 按同一白名单过滤未知字段。
var builtinToolSchemas = map[string]func() *schemas.ToolFunctionParameters{
	"bash": func() *schemas.ToolFunctionParameters {
		return mustParseParameters(`{
			"type": "object",
			"properties": {
				"command": {"type": "string", "description": "The bash command to run. Must be non-empty."},
				"restart": {"type": "boolean", "description": "Set to true to restart the persistent bash session."}
			},
			"required": ["command"]
		}`)
	},
	"text_editor": func() *schemas.ToolFunctionParameters {
		return mustParseParameters(`{
			"type": "object",
			"properties": {
				"command": {"type": "string", "enum": ["view", "create", "str_replace", "insert", "undo_edit"], "description": "The text editor command to execute."},
				"path": {"type": "string", "description": "The path to the file or directory."},
				"view_range": {"type": "array", "items": {"type": "integer"}, "description": "Optional start and end line numbers (1-indexed) for view command."},
				"old_str": {"type": "string", "description": "The exact text to replace for str_replace command."},
				"new_str": {"type": "string", "description": "The replacement text for str_replace command."},
				"file_text": {"type": "string", "description": "The full content for create command."},
				"insert_line": {"type": "integer", "description": "The line number after which to insert for insert command."},
				"insert_text": {"type": "string", "description": "The text to insert for insert command."}
			},
			"required": ["command", "path"]
		}`)
	},
	"computer": func() *schemas.ToolFunctionParameters {
		return mustParseParameters(`{
			"type": "object",
			"properties": {
				"action": {"type": "string", "enum": ["key", "type", "mouse_move", "left_click", "right_click", "middle_click", "double_click", "screenshot", "cursor_position", "left_mouse_down", "left_mouse_up", "scroll", "hold_key", "left_click_drag", "right_click_drag", "wait"], "description": "The computer action to perform."},
				"coordinate": {"type": "array", "items": {"type": "integer"}, "description": "The [x, y] coordinate for mouse-related actions."},
				"text": {"type": "string", "description": "The text to type or key to press."}
			},
			"required": ["action"]
		}`)
	},
}

// builtinToolDescription 给内置工具一个明确的 function 级 description，
// 帮助上游模型理解何时应该调用该工具。
func builtinToolDescription(toolType string) string {
	switch {
	case strings.HasPrefix(toolType, "bash"):
		return "Run a command in a persistent bash shell. The command parameter is required and must be non-empty."
	case strings.HasPrefix(toolType, "text_editor"):
		return "View and edit text files using commands like view, create, str_replace, insert, and undo_edit."
	case strings.HasPrefix(toolType, "computer"):
		return "Control a computer desktop environment with mouse, keyboard, and screenshot actions."
	}
	return "Anthropic built-in tool"
}

// rawAnthropicTool 只提取判断内置工具和合成 schema 所需的最少字段。
type rawAnthropicTool struct {
	Name        string          `json:"name"`
	Type        string          `json:"type"`
	Description *string         `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema,omitempty"`
}

// synthesizeAnthropicServerTools 从原始 Anthropic 请求体中找出 Anthropic 内置的
// client/server tool（bash/text_editor/computer 等），把它们合成为 OpenAI function
// tool 定义，使 OpenAI 兼容上游（DeepSeek/MiniMax 等）能按规范生成参数。
//
// 对于自定义 function tool（input_schema 已提供），Bifrost 已经能正常处理，这里
// 不会重复合成。
func synthesizeAnthropicServerTools(body []byte) []schemas.ChatTool {
	var req struct {
		Tools []rawAnthropicTool `json:"tools"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil
	}

	var out []schemas.ChatTool
	for _, tool := range req.Tools {
		if tool.Name == "" {
			continue
		}
		params := builtinToolParameters(tool.Type)
		if params == nil {
			continue
		}

		// 自定义 function tool 已经有 input_schema，不需要合成。
		if !isBuiltinToolType(tool.Type) && len(tool.InputSchema) > 0 {
			continue
		}

		desc := builtinToolDescription(tool.Type)
		if tool.Description != nil && *tool.Description != "" {
			desc = *tool.Description
		}

		out = append(out, schemas.ChatTool{
			Type: schemas.ChatToolTypeFunction,
			Function: &schemas.ChatToolFunction{
				Name:        tool.Name,
				Description: &desc,
				Parameters:  params,
			},
		})
	}
	return out
}

// builtinToolParameters 根据 Anthropic tool type（或前缀）返回对应的输入 schema。
func builtinToolParameters(toolType string) *schemas.ToolFunctionParameters {
	for prefix, fn := range builtinToolSchemas {
		if strings.HasPrefix(toolType, prefix+"_") || toolType == prefix {
			return fn()
		}
	}
	return nil
}

// isBuiltinToolType 判断 type 字段是否是已知的 Anthropic 内置工具类型。
func isBuiltinToolType(toolType string) bool {
	return builtinToolParameters(toolType) != nil
}

func mustParseParameters(s string) *schemas.ToolFunctionParameters {
	var p schemas.ToolFunctionParameters
	if err := json.Unmarshal([]byte(s), &p); err != nil {
		panic(fmt.Errorf("parse builtin tool parameters: %w", err))
	}
	return &p
}
