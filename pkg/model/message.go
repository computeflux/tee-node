package model

// P2P 请求 Message 消息体
// type Message struct {
// 	// 消息ID
// 	MsgID string `json:"msg_id"`
// 	// 来源ID
// 	OrgId   string `json:"org_id,omitempty"`
// 	Type    string `json:"type"`
// 	Payload []byte `json:"payload"`
// 	// 错误信息
// 	Error string `json:"error"`
// }

type Kvs struct {
	K string `json:"key"`
	V []byte `json:"value"`
}
