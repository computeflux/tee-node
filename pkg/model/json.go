package model

import (
	"errors"

	"google.golang.org/protobuf/encoding/protojson"
)

// encoding/json 无法对 protobuf oneof 接口字段（isXxx_Payload）做反序列化，
// 会报 "cannot unmarshal object into Go struct field ... of type model.isXxx_Payload"，
// 导致依赖 JSON 持久化的队列（PersistChan、bft-brigde 待发队列）在重启后无法恢复消息。
// 这里为所有会被 JSON 持久化的消息类型提供基于 protojson 的编解码：
// protojson 通过 ProtoReflect 反射正确处理 oneof 字段。
//
// 兼容性注意：protojson 输出 camelCase 字段名（如 msgId、txIndex），
// 与旧版 encoding/json 的 snake_case（msg_id、tx_index）不兼容。
// 若仓库里存在旧格式的持久化数据（如历史 PersistChan/队列 JSON），
// 需要手动迁移或清理后此格式才能恢复。

func (x *To) MarshalJSON() ([]byte, error) {
	if x == nil {
		return []byte("null"), nil
	}
	return protojson.Marshal(x)
}

func (x *To) UnmarshalJSON(b []byte) error {
	if x == nil {
		return errors.New("model.To: UnmarshalJSON on nil receiver")
	}
	return protojson.Unmarshal(b, x)
}

func (x *DkgMessage) MarshalJSON() ([]byte, error) {
	if x == nil {
		return []byte("null"), nil
	}
	return protojson.Marshal(x)
}

func (x *DkgMessage) UnmarshalJSON(b []byte) error {
	if x == nil {
		return errors.New("model.DkgMessage: UnmarshalJSON on nil receiver")
	}
	return protojson.Unmarshal(b, x)
}

func (x *BlockPartialSign) MarshalJSON() ([]byte, error) {
	if x == nil {
		return []byte("null"), nil
	}
	return protojson.Marshal(x)
}

func (x *BlockPartialSign) UnmarshalJSON(b []byte) error {
	if x == nil {
		return errors.New("model.BlockPartialSign: UnmarshalJSON on nil receiver")
	}
	return protojson.Unmarshal(b, x)
}

func (x *SecretBox) MarshalJSON() ([]byte, error) {
	if x == nil {
		return []byte("null"), nil
	}
	return protojson.Marshal(x)
}

func (x *SecretBox) UnmarshalJSON(b []byte) error {
	if x == nil {
		return errors.New("model.SecretBox: UnmarshalJSON on nil receiver")
	}
	return protojson.Unmarshal(b, x)
}

func (x *PeerMsg) MarshalJSON() ([]byte, error) {
	if x == nil {
		return []byte("null"), nil
	}
	return protojson.Marshal(x)
}

func (x *PeerMsg) UnmarshalJSON(b []byte) error {
	if x == nil {
		return errors.New("model.PeerMsg: UnmarshalJSON on nil receiver")
	}
	return protojson.Unmarshal(b, x)
}
