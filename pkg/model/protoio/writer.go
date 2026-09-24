package protoio

import (
	"errors"
	"fmt"
	"io"
)

// WriteMessage writes a single protobuf message to w using deterministic serialization.
// This ensures consistent hash values across multiple serializations.
// 注意：不写入长度前缀，调用方需保证每个 value 恰好是一条消息（如单个 DB key）。
func WriteMessage(msg VTMarshaler, w io.Writer) error {
	buf := make([]byte, msg.SizeVT())
	if _, err := msg.MarshalToVT(buf); err != nil {
		return err
	}

	_, err := w.Write(buf)
	return err
}

func Marshal(pb VTMarshaler) ([]byte, error) {
	if pb == nil {
		return nil, errors.New("pb is nil")
	}

	buf := make([]byte, pb.SizeVT())
	if _, err := pb.MarshalToVT(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// VTMarshaler VT（vtproto）序列化的最小接口。
//
// 注意：这里不要求 MarshalVT() 方法——protoc-gen-go-vtproto 对 oneof 包装消息
// （如 model.TeeCall_Text）只生成 SizeVT + MarshalToVT（+ MarshalToSizedBufferVT），
// 不生成 MarshalVT()；若接口要求它，这些类型会断言失败，
// 导致 SgxIssue/SgxVerify/SnpIssue/SnpVerify 等对 call.Tx 的 MarshalVT 报
// "unsupported tee tx type"。本仓库所有接口调用点（WriteMessage/Marshal/MarshalVT/
// DBSetProtoMessage 等）只使用 SizeVT/MarshalToVT；直接调 .MarshalVT() 的代码
// 都是针对具体消息类型（其自身有该方法），不经过此接口。
type VTMarshaler interface {
	SizeVT() int
	MarshalToVT([]byte) (int, error)
}

type VTUnmarshaler interface {
	UnmarshalVT([]byte) error
}

func MarshalVT(tx any) ([]byte, error) {
	if tx == nil {
		return nil, nil
	}

	m, ok := tx.(VTMarshaler)
	if !ok {
		return nil, fmt.Errorf("unsupported tee tx type %T", tx)
	}

	buf := make([]byte, m.SizeVT())
	if _, err := m.MarshalToVT(buf); err != nil {
		return nil, err
	}
	return buf, nil
}
