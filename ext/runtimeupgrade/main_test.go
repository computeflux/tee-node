package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// ── deriveBIP44 ────────────────────────────────────────────

func TestDeriveBIP44_HardhatVector(t *testing.T) {
	// hardhat/ethers 标准测试助记词 → 默认账户 #0（m/44'/60'/0'/0/0）
	const mnemonic = "test test test test test test test test test test test junk"
	const wantAddr = "0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266"

	priv, addr, err := deriveBIP44(mnemonic)
	if err != nil {
		t.Fatalf("deriveBIP44: %v", err)
	}
	if !strings.EqualFold(addr.Hex(), wantAddr) {
		t.Fatalf("address = %s, want %s", addr.Hex(), wantAddr)
	}
	// 私钥与地址一致
	if got := ethcrypto.PubkeyToAddress(priv.PublicKey); got != addr {
		t.Fatalf("pubkey address %s != derived %s", got.Hex(), addr.Hex())
	}
}

func TestDeriveBIP44_EmptyMnemonic(t *testing.T) {
	// 空助记词不应 panic（PBKDF2 会接受空串），但结果应确定性
	priv, _, err := deriveBIP44("")
	if err != nil {
		t.Fatalf("deriveBIP44(empty): %v", err)
	}
	if priv == nil {
		t.Fatal("nil private key")
	}
}

func TestDeriveBIP44_Deterministic(t *testing.T) {
	const mnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
	p1, a1, err := deriveBIP44(mnemonic)
	if err != nil {
		t.Fatal(err)
	}
	p2, a2, _ := deriveBIP44(mnemonic)
	if a1 != a2 {
		t.Fatalf("not deterministic: %s vs %s", a1.Hex(), a2.Hex())
	}
	if p1.D.BitLen() == 0 || p1.D.Cmp(p2.D) != 0 {
		t.Fatal("private keys differ")
	}
}

// ── signEVMPersonal ────────────────────────────────────────

func TestSignEVMPersonal_RecoversAddress(t *testing.T) {
	priv, addr, err := deriveBIP44("test test test test test test test test test test test junk")
	if err != nil {
		t.Fatal(err)
	}

	msg := []byte("hello world")
	sig, err := signEVMPersonal(msg, priv)
	if err != nil {
		t.Fatalf("signEVMPersonal: %v", err)
	}
	if len(sig) != 65 {
		t.Fatalf("sig len = %d, want 65", len(sig))
	}
	if sig[64] < 27 {
		t.Fatalf("v = %d, want >= 27", sig[64])
	}

	// 按 signEVMPersonal 的格式恢复：call_tee_contract:<hex> + EIP-191 前缀
	data := fmt.Sprintf("call_tee_contract:%s", hex.EncodeToString(msg))
	pre := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(data))
	hash := ethcrypto.Keccak256Hash(append([]byte(pre), []byte(data)...))

	recoverSig := make([]byte, 65)
	copy(recoverSig, sig)
	recoverSig[64] -= 27

	pub, err := ethcrypto.SigToPub(hash.Bytes(), recoverSig)
	if err != nil {
		t.Fatalf("SigToPub: %v", err)
	}
	if got := ethcrypto.PubkeyToAddress(*pub); got != addr {
		t.Fatalf("recovered %s != %s", got.Hex(), addr.Hex())
	}
}

// ── encodeCompact / SCALE ──────────────────────────────────

func TestEncodeCompact(t *testing.T) {
	tests := []struct {
		in   uint64
		want []byte
	}{
		{0, []byte{0x00}},
		{1, []byte{0x04}},
		{63, []byte{0xFC}},                                 // 0x3F: byte<<2 = 0xFC
		{64, []byte{0x01, 0x01}},                           // 0x40: (0x40<<2)|1 = 0x101 → 0x01, 0x40>>6 = 0x01
		{0x3FFF, []byte{0xFD, 0xFF}},                       // 2 字节上限: (0x3F<<2)|1 = 0xFD, 0x3FFF>>6 = 0xFF
		{0x4000, []byte{0x02, 0x00, 0x01, 0x00}},           // 3字节模式起点: (0x4000&0x3F)<<2|2 = 0x02, >>6 = 0x00, >>14 = 0x01, >>22 = 0x00（3字节模式固定输出4字节）
		{0x3FFFFFFF, []byte{0xFE, 0xFF, 0xFF, 0xFF}},       // 4 字节(3字节模式)上限: (0x3F<<2)|2 = 0xFE
		{0x40000000, []byte{0x03, 0x00, 0x00, 0x00, 0x40}}, // 5 字节模式: 0x03 + uint32 LE
	}

	for _, tt := range tests {
		got := encodeCompact(tt.in)
		if len(got) != len(tt.want) {
			t.Fatalf("encodeCompact(%d) = %v (len %d), want %v (len %d)", tt.in, got, len(got), tt.want, len(tt.want))
		}
		for i := range tt.want {
			if got[i] != tt.want[i] {
				t.Fatalf("encodeCompact(%d) = %v, want %v", tt.in, got, tt.want)
			}
		}
	}
}

func TestScaleEncodePendingRuntimeUpgrade(t *testing.T) {
	ver := uint32(12)
	codeHash := []byte{0x01, 0x02}
	wasm := []byte{0xaa, 0xbb, 0xcc}
	desc := "upgrade test"
	activation := uint64(42)

	got := scaleEncodePendingRuntimeUpgrade(ver, codeHash, wasm, desc, activation)

	// 期望布局: ver(uint32 LE) + compact(len(codeHash)) + codeHash + compact(len(wasm)) + wasm + compact(len(desc)) + desc + activation(uint64 LE)
	want := []byte{12, 0, 0, 0} // ver
	want = append(want, 2<<2)   // compact(2)
	want = append(want, codeHash...)
	want = append(want, 3<<2) // compact(3)
	want = append(want, wasm...)
	want = append(want, byte(len(desc))<<2) // compact(12)
	want = append(want, []byte(desc)...)
	want = append(want, 42, 0, 0, 0, 0, 0, 0, 0) // activation uint64 LE

	if len(got) != len(want) {
		t.Fatalf("len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d = %02x, want %02x (full: %x vs %x)", i, got[i], want[i], got, want)
		}
	}

	// 大长度走多字节 compact
	bigWasm := make([]byte, 200)
	got = scaleEncodePendingRuntimeUpgrade(ver, codeHash, bigWasm, desc, 0)
	if len(got) < len(bigWasm) {
		t.Fatalf("output truncated: %d < %d", len(got), len(bigWasm))
	}
}

func TestUint32Selector(t *testing.T) {
	got := uint32Selector(0xa4771d9e)
	want := [4]byte{0xa4, 0x77, 0x1d, 0x9e}
	if got != want {
		t.Fatalf("got %x, want %x", got, want)
	}
	if got := uint32Selector(0); got != [4]byte{} {
		t.Fatalf("zero selector = %x", got)
	}
	if got := uint32Selector(0xFFFFFFFF); got != [4]byte{0xff, 0xff, 0xff, 0xff} {
		t.Fatalf("max selector = %x", got)
	}
}

func TestUint64LE(t *testing.T) {
	got := uint64LE(0x0102030405060708)
	want := []byte{0x08, 0x07, 0x06, 0x05, 0x04, 0x03, 0x02, 0x01}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("byte %d = %02x, want %02x", i, got[i], want[i])
		}
	}
}

// ── readNativeVersion / evalUint32Expr ─────────────────────

func writeTempGo(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "call.go")
	if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestReadNativeVersion(t *testing.T) {
	t.Run("plain_const", func(t *testing.T) {
		p := writeTempGo(t, "package runtime\n\nconst NativeVersion = 12\n")
		v, err := readNativeVersion(p)
		if err != nil {
			t.Fatal(err)
		}
		if v != 12 {
			t.Fatalf("got %d, want 12", v)
		}
	})

	t.Run("uint32_wrapped", func(t *testing.T) {
		p := writeTempGo(t, "package runtime\n\nconst NativeVersion = uint32(0x1A)\n")
		v, err := readNativeVersion(p)
		if err != nil {
			t.Fatal(err)
		}
		if v != 26 {
			t.Fatalf("got %d, want 26", v)
		}
	})

	t.Run("hex_literal", func(t *testing.T) {
		p := writeTempGo(t, "package runtime\n\nconst NativeVersion = 0x64\n")
		v, err := readNativeVersion(p)
		if err != nil {
			t.Fatal(err)
		}
		if v != 100 {
			t.Fatalf("got %d, want 100", v)
		}
	})

	t.Run("missing_const", func(t *testing.T) {
		p := writeTempGo(t, "package runtime\n\nconst Other = 1\n")
		if _, err := readNativeVersion(p); err == nil {
			t.Fatal("expected error for missing const")
		}
	})

	t.Run("syntax_error", func(t *testing.T) {
		p := writeTempGo(t, "package runtime\n\nconst NativeVersion = \n")
		if _, err := readNativeVersion(p); err == nil {
			t.Fatal("expected error for broken file")
		}
	})

	t.Run("unsupported_expr", func(t *testing.T) {
		p := writeTempGo(t, "package runtime\n\nconst NativeVersion = Other + 1\n")
		if _, err := readNativeVersion(p); err == nil {
			t.Fatal("expected error for unsupported expression")
		}
	})

	t.Run("not_a_file", func(t *testing.T) {
		if _, err := readNativeVersion("/nonexistent/path/call.go"); err == nil {
			t.Fatal("expected error for missing file")
		}
	})
}

// ── envOr ──────────────────────────────────────────────────

func TestEnvOr(t *testing.T) {
	t.Setenv("TEE_TEST_ENV_VAR", "set-value")
	if got := envOr("TEE_TEST_ENV_VAR", "default"); got != "set-value" {
		t.Fatalf("got %q", got)
	}
	t.Setenv("TEE_TEST_ENV_VAR", "")
	if got := envOr("TEE_TEST_ENV_VAR", "default"); got != "default" {
		t.Fatalf("got %q", got)
	}
	if got := envOr("TEE_TEST_UNSET_VAR_XYZ", "d"); got != "d" {
		t.Fatalf("got %q", got)
	}
}

// ── gql client ─────────────────────────────────────────────

func TestGqlPost_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/gql" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %s", ct)
		}
		w.Write([]byte(`{"data":{"foo":"bar"}}`))
	}))
	defer srv.Close()

	var out struct{ Foo string }
	if err := gqlPost(srv.URL+"/gql", `query{foo}`, nil, &out); err != nil {
		t.Fatal(err)
	}
	if out.Foo != "bar" {
		t.Fatalf("got %q", out.Foo)
	}
}

func TestGqlPost_GraphQLErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errors":[{"message":"boom"}]}`))
	}))
	defer srv.Close()

	if err := gqlPost(srv.URL, `query{foo}`, nil, nil); err == nil {
		t.Fatal("expected gql error")
	}
}

func TestGqlPost_BadJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`not-json`))
	}))
	defer srv.Close()

	if err := gqlPost(srv.URL, `query{foo}`, nil, nil); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestGqlContractNonce(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gqlReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if !strings.Contains(req.Query, "contractNonce") {
			t.Errorf("unexpected query: %s", req.Query)
		}
		w.Write([]byte(`{"data":{"contractNonce":123}}`))
	}))
	defer srv.Close()

	n, err := gqlContractNonce(srv.URL, "0xabc", 2)
	if err != nil {
		t.Fatal(err)
	}
	if n != 123 {
		t.Fatalf("nonce = %d, want 123", n)
	}
}

func TestGqlContractCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req gqlReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if !strings.Contains(req.Query, "contractCall") {
			t.Errorf("unexpected query: %s", req.Query)
		}
		w.Write([]byte(`{"data":{"contractCall":"0xdeadbeef"}}`))
	}))
	defer srv.Close()

	h, err := gqlContractCall(srv.URL, "0xabc", 2, 5, "gov", "runtime_upgrade", []string{"0x01"}, "sigsig")
	if err != nil {
		t.Fatal(err)
	}
	if h != "0xdeadbeef" {
		t.Fatalf("hash = %q", h)
	}
}

// ── waitTxStatus ───────────────────────────────────────────

func TestWaitTxStatus_Success(t *testing.T) {
	responses := []string{
		`{"data":{"txStatus":"PENDING"}}`,
		`{"data":{"txStatus":"SUCCESS"}}`,
	}
	i := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(responses[i]))
		i++
	}))
	defer srv.Close()

	if err := waitTxStatus(srv.URL, "0xhash", 10*time.Second); err != nil {
		t.Fatal(err)
	}
	if i != 2 {
		t.Fatalf("expected 2 polls, got %d", i)
	}
}

func TestWaitTxStatus_Failed(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"txStatus":"FAILED"}}`))
	}))
	defer srv.Close()

	err := waitTxStatus(srv.URL, "0xhash", 10*time.Second)
	if err == nil || !strings.Contains(err.Error(), "tx failed") {
		t.Fatalf("got %v, want tx failed error", err)
	}
}

func TestWaitTxStatus_UnknownFinalStatus(t *testing.T) {
	// 未知状态且不含 PEND 视为成功
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"txStatus":"DONE-ish"}}`))
	}))
	defer srv.Close()

	if err := waitTxStatus(srv.URL, "0xhash", 10*time.Second); err != nil {
		t.Fatal(err)
	}
}

func TestWaitTxStatus_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":{"txStatus":"PENDING"}}`))
	}))
	defer srv.Close()

	// timeout 小于 2s 轮询间隔，第二次轮询前必然超时
	err := waitTxStatus(srv.URL, "0xhash", 50*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("got %v, want timeout", err)
	}
}

func TestWaitTxStatus_GQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"errors":[{"message":"down"}]}`))
	}))
	defer srv.Close()

	if err := waitTxStatus(srv.URL, "0xhash", 10*time.Second); err == nil {
		t.Fatal("expected error")
	}
}
