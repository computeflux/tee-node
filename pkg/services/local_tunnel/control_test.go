package localtunnel

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/libp2p/go-yamux/v5"
)

func TestDynamicModelRegisterUnregister(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv := NewServer(ServerConfig{
		Addr:           listener.Addr().String(),
		WarmupDuration: 0,
		ValidateKey:    func(modelID, apiKey string) (uint64, bool) { return 1, true },
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go srv.handleClientConn(conn)
		}
	}()

	dialer := net.Dialer{Timeout: 2 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	h := Handshake{
		Version: ProtocolVersion,
		ModelID: "model-a",
		APIKey:  "key",
		BaseURL: "http://localhost:1000",
		Models:  []string{"model-a"},
	}
	if err := WriteHandshake(conn, h); err != nil {
		t.Fatalf("handshake: %v", err)
	}

	config := yamux.DefaultConfig()
	session, err := yamux.Client(conn, config, nil)
	if err != nil {
		t.Fatalf("yamux client: %v", err)
	}
	defer session.Close()

	// 等待服务端完成注册
	time.Sleep(50 * time.Millisecond)
	for _, e := range srv.Pool().AllEntries() {
		e.Available = true
	}

	// 动态注册 model-b
	if err := sendControlFrame(session, FrameTypeModelRegister, ModelControlFrame{ModelID: "model-b", BaseURL: "http://localhost:2000"}); err != nil {
		t.Fatalf("register model-b: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	for _, e := range srv.Pool().AllEntries() {
		e.Available = true
	}

	if got := srv.Pool().Count(routeKey(1, "model-a")); got != 1 {
		t.Fatalf("model-a count = %d, want 1", got)
	}
	if got := srv.Pool().Count(routeKey(1, "model-b")); got != 1 {
		t.Fatalf("model-b count = %d, want 1", got)
	}

	// 动态注销 model-b
	if err := sendControlFrame(session, FrameTypeModelUnregister, ModelControlFrame{ModelID: "model-b"}); err != nil {
		t.Fatalf("unregister model-b: %v", err)
	}
	time.Sleep(50 * time.Millisecond)

	if got := srv.Pool().Count(routeKey(1, "model-b")); got != 0 {
		t.Fatalf("model-b count after unregister = %d, want 0", got)
	}
	if got := srv.Pool().Count(routeKey(1, "model-a")); got != 1 {
		t.Fatalf("model-a count after unregister = %d, want 1", got)
	}
}

func sendControlFrame(session *yamux.Session, ft FrameType, ctrl ModelControlFrame) error {
	stream, err := session.OpenStream(context.Background())
	if err != nil {
		return err
	}
	defer stream.Close()

	if err := WriteJSONFrame(stream, ft, ctrl); err != nil {
		return err
	}

	rft, payload, err := ReadTypedFrame(stream)
	if err != nil {
		return err
	}
	if rft == FrameTypeError {
		var e ErrorFrame
		if err := json.Unmarshal(payload, &e); err != nil {
			return err
		}
		return fmt.Errorf("%s", e.Message)
	}
	if rft != FrameTypeHeader {
		return fmt.Errorf("unexpected frame type %d", rft)
	}
	var h HeaderFrame
	if err := json.Unmarshal(payload, &h); err != nil {
		return err
	}
	if h.Status != 200 {
		return fmt.Errorf("control failed status=%d", h.Status)
	}
	rft2, _, err := ReadTypedFrame(stream)
	if err != nil {
		return err
	}
	if rft2 != FrameTypeDone {
		return fmt.Errorf("expected done frame, got %d", rft2)
	}
	return nil
}
