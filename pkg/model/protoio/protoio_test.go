package protoio

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

// testMsg 是一个测试用的 VTMarshaler + VTUnmarshaler 实现。
// 编码格式: uint32 LE 长度 + 载荷字节，SizeVT 返回完整大小。
type testMsg struct {
	Payload []byte
}

func (m *testMsg) SizeVT() int {
	if m == nil {
		return 0
	}
	return 4 + len(m.Payload)
}

func (m *testMsg) MarshalToVT(buf []byte) (int, error) {
	if m == nil {
		return 0, nil
	}
	if len(buf) < m.SizeVT() {
		return 0, errors.New("buffer too small")
	}
	binary.LittleEndian.PutUint32(buf[:4], uint32(len(m.Payload)))
	copy(buf[4:], m.Payload)
	return m.SizeVT(), nil
}

func (m *testMsg) MarshalVT() ([]byte, error) {
	buf := make([]byte, m.SizeVT())
	if _, err := m.MarshalToVT(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

func (m *testMsg) UnmarshalVT(buf []byte) error {
	if len(buf) < 4 {
		return errors.New("too short")
	}
	n := binary.LittleEndian.Uint32(buf[:4])
	if uint32(len(buf)-4) < n {
		return errors.New("truncated payload")
	}
	m.Payload = append([]byte(nil), buf[4:4+n]...)
	return nil
}

func TestMarshal_RoundTrip(t *testing.T) {
	msg := &testMsg{Payload: []byte{0x01, 0x02, 0x03}}
	buf, err := Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if len(buf) != 7 {
		t.Fatalf("len = %d, want 7", len(buf))
	}

	// 确定性：同一消息两次序列化结果必须一致（WriteMessage 注释中的关键保证）
	buf2, err := Marshal(msg)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(buf, buf2) {
		t.Fatal("marshal not deterministic")
	}

	var out testMsg
	if err := Unmarshal(buf, &out); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !bytes.Equal(out.Payload, msg.Payload) {
		t.Fatalf("payload mismatch: %x", out.Payload)
	}
}

func TestMarshal_Nil(t *testing.T) {
	if _, err := Marshal(nil); err == nil {
		t.Fatal("expected error for nil pb")
	}
}

func TestMarshalVT_RoundTrip(t *testing.T) {
	msg := &testMsg{Payload: []byte("hello")}
	buf, err := MarshalVT(msg)
	if err != nil {
		t.Fatalf("MarshalVT: %v", err)
	}

	var out testMsg
	if err := out.UnmarshalVT(buf); err != nil {
		t.Fatal(err)
	}
	if string(out.Payload) != "hello" {
		t.Fatalf("payload = %q", out.Payload)
	}
}

func TestMarshalVT_Nil(t *testing.T) {
	buf, err := MarshalVT(nil)
	if err != nil {
		t.Fatalf("MarshalVT(nil) = %v, want nil error", err)
	}
	if buf != nil {
		t.Fatalf("MarshalVT(nil) = %v, want nil", buf)
	}
}

func TestMarshalVT_UnsupportedType(t *testing.T) {
	if _, err := MarshalVT(42); err == nil {
		t.Fatal("expected error for unsupported type")
	}
	// 指针类型但未实现接口也不行
	var p *int
	if _, err := MarshalVT(p); err == nil {
		t.Fatal("expected error for non-VTMarshaler pointer")
	}
}

func TestWriteMessage(t *testing.T) {
	msg := &testMsg{Payload: []byte{0xaa}}

	var buf bytes.Buffer
	if err := WriteMessage(msg, &buf); err != nil {
		t.Fatalf("WriteMessage: %v", err)
	}

	// 与 Marshal 输出一致
	want, _ := Marshal(msg)
	if !bytes.Equal(buf.Bytes(), want) {
		t.Fatal("WriteMessage output != Marshal output")
	}

	var out testMsg
	if err := out.UnmarshalVT(buf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Payload, msg.Payload) {
		t.Fatal("payload mismatch")
	}
}

func TestWriteMessage_WriteError(t *testing.T) {
	msg := &testMsg{Payload: []byte("x")}

	errWriter := &failWriter{err: errors.New("disk full")}
	if err := WriteMessage(msg, errWriter); err == nil {
		t.Fatal("expected write error")
	}
}

func TestUnmarshal(t *testing.T) {
	t.Run("vt_unmarshaler", func(t *testing.T) {
		msg := &testMsg{Payload: []byte("data")}
		buf, err := Marshal(msg)
		if err != nil {
			t.Fatal(err)
		}

		var out testMsg
		if err := Unmarshal(buf, &out); err != nil {
			t.Fatalf("Unmarshal: %v", err)
		}
		if string(out.Payload) != "data" {
			t.Fatalf("payload = %q", out.Payload)
		}
	})

	t.Run("bad_input", func(t *testing.T) {
		var out testMsg
		if err := Unmarshal([]byte{0x01}, &out); err == nil {
			t.Fatal("expected error for truncated input")
		}
	})

	t.Run("non_vt_unmarshaler", func(t *testing.T) {
		var out int
		if err := Unmarshal([]byte{0x01}, &out); err == nil {
			t.Fatal("expected error for non-VTUnmarshaler")
		}
	})
}

// failWriter 用于模拟写入失败。
type failWriter struct{ err error }

func (w *failWriter) Write(p []byte) (int, error) { return 0, w.err }
