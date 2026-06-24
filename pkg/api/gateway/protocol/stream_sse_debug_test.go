package protocol

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/maximhq/bifrost/core/schemas"
)

func TestStreamSSEDebug(t *testing.T) {
	cases := []struct {
		sse   string
		tools string
	}{
		{
			sse:   "/srv/token_node0/token_chain_data/gateway_stream_response_anthropic_1782296750617.sse",
			tools: "/srv/token_node0/token_chain_data/gateway_openai_request_final_1782296749415.json",
		},
		{
			sse:   "/srv/token_node0/token_chain_data/gateway_stream_response_anthropic_1782296942229.sse",
			tools: "/srv/token_node0/token_chain_data/gateway_openai_request_final_1782296940869.json",
		},
		{
			sse:   "/srv/token_node0/token_chain_data/gateway_stream_response_anthropic_1782296744205.sse",
			tools: "/srv/token_node0/token_chain_data/gateway_openai_request_final_1782296742931.json",
		},
	}

	for _, tc := range cases {
		sseFile, err := os.Open(tc.sse)
		if err != nil {
			t.Logf("skip %s: %v", tc.sse, err)
			continue
		}
		toolsBytes, err := os.ReadFile(tc.tools)
		if err != nil {
			sseFile.Close()
			t.Logf("skip %s: %v", tc.tools, err)
			continue
		}
		var req struct {
			Tools []schemas.ChatTool `json:"tools"`
		}
		if err := json.Unmarshal(toolsBytes, &req); err != nil {
			sseFile.Close()
			t.Fatalf("unmarshal tools: %v", err)
		}

		var out bytes.Buffer
		_, err = StreamOpenAIResponseToAnthropic(sseFile, &out, true, req.Tools...)
		sseFile.Close()
		if err != nil {
			t.Fatalf("stream convert %s: %v", tc.sse, err)
		}

		outPath := "/tmp/anthropic_out_" + filepath.Base(tc.sse)
		_ = os.WriteFile(outPath, out.Bytes(), 0644)

		fmt.Printf("\n=== %s (output: %s) ===\n", tc.sse, outPath)
		found := false
		for _, line := range bytes.Split(out.Bytes(), []byte("\n")) {
			if !bytes.HasPrefix(line, []byte("data: ")) {
				continue
			}
			data := bytes.TrimPrefix(line, []byte("data: "))
			var event struct {
				Type         string `json:"type"`
				Index        int    `json:"index"`
				ContentBlock struct {
					Type  string         `json:"type"`
					Name  string         `json:"name"`
					Input map[string]any `json:"input"`
				} `json:"content_block"`
			}
			if err := json.Unmarshal(data, &event); err != nil {
				continue
			}
			if event.Type == "content_block_stop" && event.ContentBlock.Type == "tool_use" {
				found = true
				inputJSON, _ := json.Marshal(event.ContentBlock.Input)
				fmt.Printf("tool=%s input=%s\n", event.ContentBlock.Name, string(inputJSON))
			}
		}
		if !found {
			fmt.Println("no tool_use stop events")
		}
	}
}
