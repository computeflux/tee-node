package localtunnel

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
)

// ProtocolVersion 当前协议版本，用于未来兼容性检查
const ProtocolVersion = 1

// MaxFrameSize 单帧最大大小（64MB）
const MaxFrameSize = 64 * 1024 * 1024

// MaxBodySize 请求/响应体最大大小（16MB）
const MaxBodySize = 16 * 1024 * 1024

// ── Frame Types ──────────────────────────────────────────

type FrameType uint8

const (
	FrameTypeRequest FrameType = 1 // TEE-Node -> LocalClient: HTTP request
	FrameTypeHeader  FrameType = 2 // LocalClient -> TEE-Node: HTTP response header
	FrameTypeData    FrameType = 3 // LocalClient -> TEE-Node: response body chunk (binary)
	FrameTypeDone    FrameType = 4 // LocalClient -> TEE-Node: end of response
	FrameTypeError   FrameType = 5 // LocalClient -> TEE-Node: upstream error
)

// ── Request Frame (JSON) ─────────────────────────────────

type RequestFrame struct {
	Path    string            `json:"path"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers,omitempty"`
	Body    []byte            `json:"body,omitempty"`
}

// ── Response Header Frame (JSON) ─────────────────────────

type HeaderFrame struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers,omitempty"`
}

// ── Error Frame (JSON) ───────────────────────────────────

type ErrorFrame struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

// ── Handshake (JSON) ─────────────────────────────────────

type Handshake struct {
	Version int      `json:"version"` // 协议版本号
	ModelID string   `json:"modelId"` // 主模型标识
	APIKey  string   `json:"apiKey"`  // 认证密钥
	BaseURL string   `json:"baseUrl"` // 本地服务基地址
	Models  []string `json:"models"`  // 支持的所有模型列表
}

// ── Low-level framing ────────────────────────────────────
//
// 所有帧格式：[1B type][4B length][N bytes payload]
//
// 对于 Request/Header/Error/Done 帧，payload 是 JSON。
// 对于 Data 帧，payload 是原始二进制数据（不再 JSON wrap，减少序列化开销）。

// WriteTypedFrame 写入一个带类型的帧
func WriteTypedFrame(w io.Writer, ft FrameType, payload []byte) error {
	if len(payload) > MaxFrameSize {
		return fmt.Errorf("payload exceeds max frame size: %d > %d", len(payload), MaxFrameSize)
	}
	buf := make([]byte, 5+len(payload))
	buf[0] = uint8(ft)
	binary.BigEndian.PutUint32(buf[1:5], uint32(len(payload)))
	copy(buf[5:], payload)
	_, err := w.Write(buf)
	return err
}

// ReadTypedFrame 读取一个带类型的帧，返回 type + payload bytes
func ReadTypedFrame(r io.Reader) (FrameType, []byte, error) {
	var header [5]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return 0, nil, err
	}
	ft := FrameType(header[0])
	length := binary.BigEndian.Uint32(header[1:5])
	if length > MaxFrameSize {
		return 0, nil, fmt.Errorf("frame too large: %d", length)
	}
	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return 0, nil, err
	}
	return ft, payload, nil
}

// ── JSON helpers ─────────────────────────────────────────

// WriteJSONFrame 将对象序列化为 JSON 后写入 typed frame
func WriteJSONFrame(w io.Writer, ft FrameType, v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return WriteTypedFrame(w, ft, data)
}

// ReadJSONFrame 读取 typed frame 并解析 JSON
func ReadJSONFrame(r io.Reader, ft FrameType, v interface{}) error {
	actualType, payload, err := ReadTypedFrame(r)
	if err != nil {
		return err
	}
	if actualType != ft {
		return fmt.Errorf("unexpected frame type: got %d, want %d", actualType, ft)
	}
	return json.Unmarshal(payload, v)
}

// ── Conn helpers ─────────────────────────────────────────

// WriteHandshake 发送注册握手（JSON frame with type 0，兼容旧版）
func WriteHandshake(conn net.Conn, h Handshake) error {
	data, err := json.Marshal(h)
	if err != nil {
		return err
	}
	// 握手使用 length-prefixed JSON（不带 type byte，保持简单）
	if err := binary.Write(conn, binary.BigEndian, uint32(len(data))); err != nil {
		return err
	}
	_, err = conn.Write(data)
	return err
}

// ReadHandshake 读取注册握手
func ReadHandshake(conn net.Conn) (*Handshake, error) {
	var length uint32
	if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
		return nil, err
	}
	if length > MaxFrameSize {
		return nil, fmt.Errorf("handshake too large: %d", length)
	}
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, err
	}
	var h Handshake
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, err
	}
	if h.Version != ProtocolVersion {
		return nil, fmt.Errorf("protocol version mismatch: client=%d server=%d", h.Version, ProtocolVersion)
	}
	return &h, nil
}
