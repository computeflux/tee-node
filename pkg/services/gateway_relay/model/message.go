package model

type Message struct {
	Role             string  `json:"role,omitempty"`
	Content          any     `json:"content,omitempty"`
	ReasoningContent any     `json:"reasoning_content,omitempty"`
	Name             *string `json:"name,omitempty"`
	ToolCalls        []Tool  `json:"tool_calls,omitempty"`
	ToolCallId       string  `json:"tool_call_id,omitempty"`
}

type ImageURL struct {
	Url    string `json:"url,omitempty"`
	Detail string `json:"detail,omitempty"`
}

type MessageContent struct {
	Type     string    `json:"type,omitempty"`
	Text     string    `json:"text"`
	ImageURL *ImageURL `json:"image_url,omitempty"`
}
