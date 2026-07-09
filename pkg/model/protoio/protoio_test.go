package protoio

import (
	"bytes"
	"crypto/sha256"
	"testing"

	"github.com/computeflux/tee-provider/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func newTestTx() *model.Tx {
	return &model.Tx{
		Caller:     []byte("test-caller"),
		CallerType: 12345,
		Call:       []byte("test-call-data"),
		Signature:  []byte("test-signature"),
	}
}

// TestMarshalDeterministicConsistency 测试确定性序列化的一致性
func TestMarshalDeterministicConsistency(t *testing.T) {
	msg := newTestTx()

	var hashes [][32]byte
	for i := 0; i < 10; i++ {
		data, err := MarshalDeterministic(msg)
		require.NoError(t, err)
		hash := sha256.Sum256(data)
		hashes = append(hashes, hash)
	}

	for i := 1; i < len(hashes); i++ {
		assert.Equal(t, hashes[0], hashes[i], "第 %d 次序列化的 hash 与第一次不同", i)
	}
}

// TestWriteMessageAndReadMsg 测试写入和读取消息
func TestWriteMessageAndReadMsg(t *testing.T) {
	original := newTestTx()

	var buf bytes.Buffer
	err := WriteMessage(original, &buf)
	require.NoError(t, err)

	reader := NewDelimitedReader(&buf, 1024*1024)
	decoded := &model.Tx{}
	_, err = reader.ReadMsg(decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Caller, decoded.Caller)
	assert.Equal(t, original.CallerType, decoded.CallerType)
	assert.Equal(t, original.Call, decoded.Call)
	assert.Equal(t, original.Signature, decoded.Signature)
}

// TestMarshalDeterministicWithDifferentMessages 测试不同消息产生不同结果
func TestMarshalDeterministicWithDifferentMessages(t *testing.T) {
	msg1 := &model.Tx{CallerType: 1000}
	msg2 := &model.Tx{CallerType: 2000}

	data1, err := MarshalDeterministic(msg1)
	require.NoError(t, err)
	data2, err := MarshalDeterministic(msg2)
	require.NoError(t, err)

	hash1 := sha256.Sum256(data1)
	hash2 := sha256.Sum256(data2)

	assert.NotEqual(t, hash1, hash2, "不同消息应该产生不同的 hash")
}

// TestMarshalDeterministicEmptyMessage 测试空消息
func TestMarshalDeterministicEmptyMessage(t *testing.T) {
	msg := &model.Tx{}

	data, err := MarshalDeterministic(msg)
	require.NoError(t, err)
	assert.NotNil(t, data)

	data2, err := MarshalDeterministic(msg)
	require.NoError(t, err)
	assert.Equal(t, data, data2)
}

// TestNewDelimitedWriter 测试 DelimitedWriter
func TestNewDelimitedWriter(t *testing.T) {
	msg := newTestTx()

	var buf bytes.Buffer
	writer := NewDelimitedWriter(&buf)
	defer writer.Close()

	n, err := writer.WriteMsg(msg)
	require.NoError(t, err)
	assert.Greater(t, n, 0)

	reader := NewDelimitedReader(&buf, 1024*1024)
	decoded := &model.Tx{}
	_, err = reader.ReadMsg(decoded)
	require.NoError(t, err)
	assert.Equal(t, msg.CallerType, decoded.CallerType)
}

// TestUnmarshalDelimited 测试 UnmarshalDelimited
func TestUnmarshalDelimited(t *testing.T) {
	original := newTestTx()

	var buf bytes.Buffer
	err := WriteMessage(original, &buf)
	require.NoError(t, err)

	decoded := &model.Tx{}
	err = UnmarshalDelimited(buf.Bytes(), decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Caller, decoded.Caller)
	assert.Equal(t, original.CallerType, decoded.CallerType)
}

// TestReadMessage 测试 ReadMessage
func TestReadMessage(t *testing.T) {
	original := newTestTx()

	var buf bytes.Buffer
	err := WriteMessage(original, &buf)
	require.NoError(t, err)

	decoded := &model.Tx{}
	err = ReadMessage(&buf, decoded)
	require.NoError(t, err)

	assert.Equal(t, original.Caller, decoded.Caller)
	assert.Equal(t, original.CallerType, decoded.CallerType)
}

// TestMarshalDeterministicWithProtoMessage 测试使用标准 proto.Message
func TestMarshalDeterministicWithProtoMessage(t *testing.T) {
	msg := newTestTx()

	data, err := MarshalDeterministic(msg)
	require.NoError(t, err)
	assert.NotNil(t, data)
	assert.Greater(t, len(data), 0)

	decoded := &model.Tx{}
	err = proto.Unmarshal(data, decoded)
	require.NoError(t, err)
	assert.Equal(t, msg.Caller, decoded.Caller)
	assert.Equal(t, msg.CallerType, decoded.CallerType)
	assert.Equal(t, msg.Call, decoded.Call)
	assert.Equal(t, msg.Signature, decoded.Signature)
}

// BenchmarkMarshalDeterministic 基准测试
func BenchmarkMarshalDeterministic(b *testing.B) {
	msg := &model.Tx{
		Caller:     []byte("benchmark-caller"),
		CallerType: 1234567890,
		Call:       make([]byte, 1024),
		Signature:  []byte("benchmark-signature"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := MarshalDeterministic(msg)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// TestWriteMessageHashConsistency 测试 WriteMessage 的 hash 一致性
func TestWriteMessageHashConsistency(t *testing.T) {
	msg := newTestTx()

	var hashes [][32]byte
	for i := 0; i < 10; i++ {
		var buf bytes.Buffer
		err := WriteMessage(msg, &buf)
		require.NoError(t, err)

		data := buf.Bytes()
		hash := sha256.Sum256(data)
		hashes = append(hashes, hash)
	}

	for i := 1; i < len(hashes); i++ {
		assert.Equal(t, hashes[0], hashes[i], "第 %d 次 WriteMessage 的 hash 与第一次不同", i)
	}
}

// TestWriteMessageAndMarshalDeterministicConsistency 测试 WriteMessage 和 MarshalDeterministic 的一致性
func TestWriteMessageAndMarshalDeterministicConsistency(t *testing.T) {
	msg := newTestTx()

	var buf bytes.Buffer
	err := WriteMessage(msg, &buf)
	require.NoError(t, err)
	writeMsgData := buf.Bytes()

	marshalData, err := MarshalDeterministic(msg)
	require.NoError(t, err)

	varintLen := 1
	for i := 0; i < len(writeMsgData); i++ {
		if writeMsgData[i] < 0x80 {
			varintLen = i + 1
			break
		}
	}

	length := uint64(0)
	for i := 0; i < varintLen; i++ {
		length |= uint64(writeMsgData[i]&0x7F) << (7 * i)
	}
	assert.Equal(t, len(marshalData), int(length), "长度前缀应该等于 MarshalDeterministic 的数据长度")

	assert.Equal(t, marshalData, writeMsgData[varintLen:], "WriteMessage 的内容应该与 MarshalDeterministic 一致")

	writeMsgHash := sha256.Sum256(writeMsgData[varintLen:])
	marshalHash := sha256.Sum256(marshalData)
	assert.Equal(t, marshalHash, writeMsgHash, "WriteMessage 和 MarshalDeterministic 的 hash 应该一致")
}

// BenchmarkWriteMessage 基准测试
func BenchmarkWriteMessage(b *testing.B) {
	msg := &model.Tx{
		Caller:     []byte("benchmark-caller"),
		CallerType: 1234567890,
		Call:       make([]byte, 1024),
		Signature:  []byte("benchmark-signature"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var buf bytes.Buffer
		err := WriteMessage(msg, &buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}
