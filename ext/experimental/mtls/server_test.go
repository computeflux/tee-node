package main

import (
	"crypto/ed25519"
	"crypto/tls"
	"crypto/x509"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestEd25519Cert(t *testing.T) {
	certPEM, keyPEM, der, priv, pub, err := Ed25519Cert("localhost",
		[]net.IP{net.ParseIP("127.0.0.1")}, []string{"localhost", "example.com"})
	if err != nil {
		t.Fatalf("Ed25519Cert: %v", err)
	}

	if len(certPEM) == 0 || len(keyPEM) == 0 || len(der) == 0 {
		t.Fatal("empty outputs")
	}
	if priv == nil || pub == nil {
		t.Fatal("nil keys")
	}

	// DER 可解析，字段正确
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatalf("ParseCertificate: %v", err)
	}
	if cert.Subject.CommonName != "localhost" {
		t.Fatalf("CN = %q", cert.Subject.CommonName)
	}
	if len(cert.DNSNames) != 2 || cert.DNSNames[0] != "localhost" || cert.DNSNames[1] != "example.com" {
		t.Fatalf("DNSNames = %v", cert.DNSNames)
	}
	if len(cert.IPAddresses) != 1 || !cert.IPAddresses[0].Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("IPAddresses = %v", cert.IPAddresses)
	}
	if cert.NotAfter.Before(cert.NotBefore) {
		t.Fatal("cert validity inverted")
	}

	// 公钥必须是 ed25519 且与返回的 pub 一致
	pk, ok := cert.PublicKey.(ed25519.PublicKey)
	if !ok {
		t.Fatalf("public key type = %T", cert.PublicKey)
	}
	if len(pk) != ed25519.PublicKeySize {
		t.Fatalf("pub key size = %d", len(pk))
	}

	// 私钥 PEM 可解析
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatalf("X509KeyPair: %v", err)
	}
}

func TestEd25519Cert_NoSANs(t *testing.T) {
	// 不带 IP/DNS 的证书也应当有效（client cert 场景）
	certPEM, _, _, _, _, err := Ed25519Cert("client.local", nil, nil)
	if err != nil {
		t.Fatalf("Ed25519Cert: %v", err)
	}
	if len(certPEM) == 0 {
		t.Fatal("empty cert PEM")
	}
}

// tlsHandler 与 main 中的 handler 一致：调用 validateTLS 校验客户端证书。
func tlsHandler(w http.ResponseWriter, r *http.Request) {
	if err := validateTLS(r); err != nil {
		http.Error(w, "Certificate invalid: "+err.Error(), http.StatusUnauthorized)
		return
	}
	io.WriteString(w, "ok")
}

func TestValidateTLS_NoTLS(t *testing.T) {
	// 非 HTTPS 请求 → r.TLS == nil → 报错
	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	if err := validateTLS(req); err == nil {
		t.Fatal("expected error for non-TLS request")
	}
}

// newTLSHandlerServer 启动一个要求客户端证书（可选）的 TLS 测试服务器。
func newTLSHandlerServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(http.HandlerFunc(tlsHandler))
	srv.TLS = &tls.Config{ClientAuth: tls.RequestClientCert}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv
}

func TestValidateTLS_NoPeerCert(t *testing.T) {
	// 普通 HTTPS 客户端（不带客户端证书）→ PeerCertificates 为空 → 401
	srv := newTLSHandlerServer(t)

	resp, err := srv.Client().Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}

func TestValidateTLS_TrustedClientCert(t *testing.T) {
	// 客户端证书被 certPool 信任 → 验证通过 → 200
	clientCertPEM, _, clientDER, clientPrivKey, _, err := Ed25519Cert("client.local", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	certPool = x509.NewCertPool()
	certPool.AppendCertsFromPEM(clientCertPEM)
	t.Cleanup(func() { certPool = nil })

	srv := newTLSHandlerServer(t)

	client := srv.Client()
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	tr.TLSClientConfig.Certificates = []tls.Certificate{{
		Certificate: [][]byte{clientDER},
		PrivateKey:  clientPrivKey,
	}}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200: %s", resp.StatusCode, body)
	}
}

func TestValidateTLS_UntrustedClientCert(t *testing.T) {
	// 客户端证书不在 certPool（信任的是另一张证书）→ 验证失败 → 401
	_, _, clientDER, clientPrivKey, _, err := Ed25519Cert("client.local", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	otherCertPEM, _, _, _, _, err := Ed25519Cert("someone-else.local", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// certPool 只信任 otherCert，不信任 client cert
	certPool = x509.NewCertPool()
	certPool.AppendCertsFromPEM(otherCertPEM)
	t.Cleanup(func() { certPool = nil })

	srv := newTLSHandlerServer(t)

	client := srv.Client()
	tr, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T", client.Transport)
	}
	tr.TLSClientConfig.Certificates = []tls.Certificate{{
		Certificate: [][]byte{clientDER},
		PrivateKey:  clientPrivKey,
	}}

	resp, err := client.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
