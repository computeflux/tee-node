package localtunnel

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/libp2p/go-yamux/v5"
)

func TestProviderIsolation(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer listener.Close()

	srv := NewServer(ServerConfig{
		Addr:           listener.Addr().String(),
		WarmupDuration: 0,
		ValidateKey: func(modelID, apiKey string) (uint64, bool) {
			switch apiKey {
			case "key1":
				return 1, true
			case "key2":
				return 2, true
			default:
				return 0, false
			}
		},
	})

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go srv.handleClientConn(conn)
		}
	}()

	ctx := context.Background()
	dialer := net.Dialer{Timeout: 2 * time.Second}

	// provider 1 注册 model-a
	conn1, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial provider1: %v", err)
	}
	defer conn1.Close()
	if err := WriteHandshake(conn1, Handshake{
		Version: ProtocolVersion,
		ModelID: "model-a",
		APIKey:  "key1",
		BaseURL: "http://localhost:1000",
		Models:  []string{"model-a"},
	}); err != nil {
		t.Fatalf("handshake provider1: %v", err)
	}
	session1, err := yamux.Client(conn1, yamux.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("yamux provider1: %v", err)
	}
	defer session1.Close()

	// provider 2 注册同样的 model-a
	conn2, err := dialer.DialContext(ctx, "tcp", listener.Addr().String())
	if err != nil {
		t.Fatalf("dial provider2: %v", err)
	}
	defer conn2.Close()
	if err := WriteHandshake(conn2, Handshake{
		Version: ProtocolVersion,
		ModelID: "model-a",
		APIKey:  "key2",
		BaseURL: "http://localhost:2000",
		Models:  []string{"model-a"},
	}); err != nil {
		t.Fatalf("handshake provider2: %v", err)
	}
	session2, err := yamux.Client(conn2, yamux.DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("yamux provider2: %v", err)
	}
	defer session2.Close()

	time.Sleep(50 * time.Millisecond)
	for _, e := range srv.Pool().AllEntries() {
		e.mu.Lock()
		e.Available = true
		e.mu.Unlock()
	}

	if got := srv.Pool().Count(routeKey(1, "model-a")); got != 1 {
		t.Fatalf("provider1/model-a count = %d, want 1", got)
	}
	if got := srv.Pool().Count(routeKey(2, "model-a")); got != 1 {
		t.Fatalf("provider2/model-a count = %d, want 1", got)
	}

	// 验证 Pick 按 provider 隔离：provider1 只能拿到自己的 BaseURL
	entry1 := srv.Pool().Pick(routeKey(1, "model-a"))
	if entry1 == nil {
		t.Fatal("no entry for provider1")
	}
	if entry1.BaseURL != "http://localhost:1000" {
		t.Fatalf("provider1 picked wrong base url: %s", entry1.BaseURL)
	}

	entry2 := srv.Pool().Pick(routeKey(2, "model-a"))
	if entry2 == nil {
		t.Fatal("no entry for provider2")
	}
	if entry2.BaseURL != "http://localhost:2000" {
		t.Fatalf("provider2 picked wrong base url: %s", entry2.BaseURL)
	}
}
