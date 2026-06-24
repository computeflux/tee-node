package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// 这个脚本直接按 OpenAI 格式请求 DeepSeek，绕过 tee-node 网关，
// 用来验证 DeepSeek 本身对 Anthropic 内置工具 schema 的 function calling 能力。
//
// 用法：
//   DEEPSEEK_API_KEY=sk-xxx go run ./cmd/test_deepseek_tool
//
// 可选环境变量：
//   BASE_URL  默认 https://api.deepseek.com
//   MODEL     默认 deepseek-v4-pro
func main() {
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "请设置环境变量 DEEPSEEK_API_KEY")
		os.Exit(1)
	}

	baseURL := os.Getenv("BASE_URL")
	if baseURL == "" {
		baseURL = "https://api.deepseek.com"
	}
	model := os.Getenv("MODEL")
	if model == "" {
		model = "deepseek-v4-pro"
	}

	reqBody := map[string]any{
		"model":       model,
		"messages":    []map[string]string{{"role": "user", "content": "list files"}},
		"stream":      false,
		"max_tokens":  64000,
		"tools":       builtinTools(),
	}

	reqJSON, err := json.MarshalIndent(reqBody, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "marshal request:", err)
		os.Exit(1)
	}

	fmt.Println("=== request body ===")
	fmt.Println(string(reqJSON))

	client := &http.Client{Timeout: 60 * time.Second}
	req, err := http.NewRequest("POST", baseURL+"/chat/completions", bytes.NewReader(reqJSON))
	if err != nil {
		fmt.Fprintln(os.Stderr, "create request:", err)
		os.Exit(1)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "do request:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read response:", err)
		os.Exit(1)
	}

	fmt.Println("\n=== response status ===")
	fmt.Println(resp.Status)

	var pretty any
	if err := json.Unmarshal(respBody, &pretty); err == nil {
		b, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Println("\n=== response body ===")
		fmt.Println(string(b))
	} else {
		fmt.Println("\n=== raw response ===")
		fmt.Println(string(respBody))
	}

	// 单独打印 tool_calls 的参数，方便诊断
	var parsed struct {
		Choices []struct {
			Message struct {
				ToolCalls []struct {
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &parsed); err == nil && len(parsed.Choices) > 0 {
		calls := parsed.Choices[0].Message.ToolCalls
		if len(calls) > 0 {
			fmt.Println("\n=== tool_calls ===")
			for _, tc := range calls {
				fmt.Printf("name: %s\narguments: %s\n\n", tc.Function.Name, tc.Function.Arguments)
			}
		} else {
			fmt.Println("\n=== no tool_calls in response ===")
		}
	}
}

func builtinTools() []map[string]any {
	return []map[string]any{
		{
			"type": "function",
			"function": map[string]any{
				"name":        "bash",
				"description": "Run a command in a bash shell Example: {\"command\":\"example\"}",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"description": "The bash command to run. Must be non-empty.",
							"minLength":   1,
						},
						"restart": map[string]any{
							"type":        "boolean",
							"description": "Set to true to restart the persistent bash session.",
						},
					},
					"required": []string{"command"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "text_editor",
				"description": "Edit text files Example: {\"command\":\"view\",\"path\":\"example\"}",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"command": map[string]any{
							"type":        "string",
							"enum":        []string{"view", "create", "str_replace", "insert", "undo_edit"},
							"description": "The text editor command to execute.",
							"minLength":   1,
						},
						"path": map[string]any{
							"type":        "string",
							"description": "The path to the file or directory.",
							"minLength":   1,
						},
						"view_range": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "integer"},
							"description": "Optional start and end line numbers (1-indexed) for view command.",
						},
						"old_str": map[string]any{
							"type":        "string",
							"description": "The exact text to replace for str_replace command.",
						},
						"new_str": map[string]any{
							"type":        "string",
							"description": "The replacement text for str_replace command.",
						},
						"file_text": map[string]any{
							"type":        "string",
							"description": "The full content for create command.",
						},
						"insert_line": map[string]any{
							"type":        "integer",
							"description": "The line number after which to insert for insert command.",
						},
						"insert_text": map[string]any{
							"type":        "string",
							"description": "The text to insert for insert command.",
						},
					},
					"required": []string{"command", "path"},
				},
			},
		},
		{
			"type": "function",
			"function": map[string]any{
				"name":        "computer",
				"description": "Control the computer Example: {\"action\":\"key\"}",
				"parameters": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"action": map[string]any{
							"type": "string",
							"enum": []string{
								"key", "type", "mouse_move", "left_click", "right_click",
								"middle_click", "double_click", "screenshot", "cursor_position",
								"left_mouse_down", "left_mouse_up", "scroll", "hold_key",
								"left_click_drag", "right_click_drag", "wait",
							},
							"description": "The computer action to perform.",
							"minLength":   1,
						},
						"coordinate": map[string]any{
							"type":        "array",
							"items":       map[string]any{"type": "integer"},
							"description": "The [x, y] coordinate for mouse-related actions.",
						},
						"text": map[string]any{
							"type":        "string",
							"description": "The text to type or key to press.",
						},
					},
					"required": []string{"action"},
				},
			},
		},
	}
}
