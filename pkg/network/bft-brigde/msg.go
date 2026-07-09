package bftbrigde

import "fmt"

// bftMsgWrapper wraps raw vtproto wire bytes for cometbft's gogo-based p2p.
//
// cometbft's proto package builds reflection tables for gogo-generated messages;
// it cannot safely clone or marshal google.golang.org/protobuf / vtproto generated
// messages because their internal protoimpl state fields are not gogo-compatible.
// This wrapper exposes a minimal gogo-compatible proto.Message around deterministic
// vtproto bytes so the actual message can be encoded/decoded by vtproto on our side.
type bftMsgWrapper struct {
	data []byte
}

func (w *bftMsgWrapper) Reset()         { w.data = nil }
func (w *bftMsgWrapper) String() string { return fmt.Sprintf("bftMsgWrapper(%d bytes)", len(w.data)) }
func (w *bftMsgWrapper) ProtoMessage()    {}

// Marshal satisfies gogo's proto.Marshaler so cometbft can send the wrapper without
// building reflection tables for it.
func (w *bftMsgWrapper) Marshal() ([]byte, error) {
	return append([]byte(nil), w.data...), nil
}

// Unmarshal satisfies gogo's proto.Unmarshaler. cometbft writes received bytes here.
func (w *bftMsgWrapper) Unmarshal(data []byte) error {
	w.data = append([]byte(nil), data...)
	return nil
}

func (w *bftMsgWrapper) Bytes() []byte { return w.data }

// ensure bftMsgWrapper satisfies the gogo proto.Message interface at compile time.
var _ interface {
	Reset()
	String() string
	ProtoMessage()
} = (*bftMsgWrapper)(nil)
