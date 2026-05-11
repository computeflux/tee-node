package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
	"math/big"
	"net"
	"net/http"
	"time"
)

var certPool *x509.CertPool

func main() {
	// 生成服务器证书
	serverCertPEM, _, serverCertDER, serverPrivKey, _, err := Ed25519Cert("localhost",
		[]net.IP{net.ParseIP("127.0.0.1")}, []string{"localhost"})
	if err != nil {
		log.Fatalf("server create cert err: %v", err)
	}

	// 生成客户端证书
	clientCertPEM, _, clientCertDER, clientPrivKey, _, err := Ed25519Cert("client.local",
		nil, nil)
	if err != nil {
		log.Fatalf("client create cert failed: %v", err)
	}

	// 客户端信任服务器的证书，即添加服务器证书到 CA 池（因自签名）
	certPool = x509.NewCertPool()
	certPool.AppendCertsFromPEM(serverCertPEM)

	// Golang tls.Certificate
	serverCert := tls.Certificate{Certificate: [][]byte{serverCertDER}, PrivateKey: serverPrivKey}
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCert},
		MinVersion:   tls.VersionTLS13,

		// 验证客户端证书
		InsecureSkipVerify: true,
		ClientAuth:         tls.RequireAnyClientCert,
	}

	// HTTP handler
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if err := validateTLS(r); err != nil {
			http.Error(w, "Certificate invalid: "+err.Error(), http.StatusUnauthorized)
			return
		}

		io.WriteString(w, "Hello mTLS with Ed25519!\n")
	})

	// HTTP handler
	http.HandleFunc("/load", func(w http.ResponseWriter, r *http.Request) {
		certPool.AppendCertsFromPEM(clientCertPEM)
		io.WriteString(w, "Load!\n")
	})

	server := &http.Server{
		Addr:      ":8443",
		TLSConfig: tlsConfig,
	}

	log.Println("Starting server at https://localhost:8443")
	go server.ListenAndServeTLS("", "")
	time.Sleep(2 * time.Second)

	// block, _ := pem.Decode(serverCertPEM)
	// if block == nil {
	// 	log.Fatal("failed to decode PEM block from cert")
	// }
	// cert, err := x509.ParseCertificate(block.Bytes)
	// if err != nil {
	// 	log.Fatalf("failed to parse X509 certificate: %v", err)
	// }
	// pub := cert.PublicKey.(ed25519.PublicKey)
	// fmt.Println("Public key:", hex.EncodeToString(pub))
	// fmt.Println("Public key:", hex.EncodeToString(pubKey))

	client(serverCertPEM, clientCertPEM, clientCertDER, clientPrivKey)
}

func validateTLS(r *http.Request) error {
	if r.TLS == nil {
		return errors.New("no tls")
	}

	if len(r.TLS.PeerCertificates) == 0 {
		return errors.New("no tls")
	}

	cert := r.TLS.PeerCertificates[0]

	pub := cert.PublicKey.(ed25519.PublicKey)
	fmt.Println("Public key:", hex.EncodeToString(pub))

	opts := x509.VerifyOptions{
		Roots: certPool,
	}

	_, err := cert.Verify(opts)
	return err
}

func client(serverCertPEM []byte, clientCertPEM []byte, clientCertDER []byte, clientPrivKey ed25519.PrivateKey) {
	// 服务器信任客户端证书，添加客户端证书到 CA 池
	CertPool := x509.NewCertPool()
	CertPool.AppendCertsFromPEM(serverCertPEM)

	// 构造 tls.Certificate
	clientCert := tls.Certificate{Certificate: [][]byte{clientCertDER}, PrivateKey: clientPrivKey}

	conf := &tls.Config{
		Certificates:       []tls.Certificate{clientCert},
		RootCAs:            CertPool,
		ServerName:         "localhost", // 和 server 证书 CN/DNS 匹配
		InsecureSkipVerify: false,
		MinVersion:         tls.VersionTLS13,
	}

	client := &http.Client{Transport: &http.Transport{TLSClientConfig: conf}}

	resp, err := client.Get("https://localhost:8443/load")
	if err != nil {
		log.Fatalf("failed to GET: %v", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read body: %v", err)
	}

	fmt.Printf("Response Status: %s\n", resp.Status)
	fmt.Printf("Response Body:\n%s\n", string(body))

	fmt.Println("--------------------------------------------------------")

	resp, err = client.Get("https://localhost:8443")
	if err != nil {
		log.Fatalf("failed to GET: %v", err)
	}
	defer resp.Body.Close()

	body, err = io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("failed to read body: %v", err)
	}

	fmt.Printf("Response Status: %s\n", resp.Status)
	fmt.Printf("Response Body:\n%s\n", string(body))
}

// CreateSelfSignedCert 生成 Ed25519 自签名证书和对应私钥
func Ed25519Cert(commonName string, ips []net.IP, dns []string) (cert, key []byte, der []byte, priv ed25519.PrivateKey, pub ed25519.PublicKey, err error) {
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: commonName,
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           ips,
		DNSNames:              dns,
	}

	der, err = x509.CreateCertificate(rand.Reader, &template, &template, pubKey, privKey)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}

	// 证书 PEM
	cert = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})

	// 私钥 PEM（PKCS#8格式）
	keyBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		return nil, nil, nil, nil, nil, err
	}
	key = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyBytes})

	return cert, key, der, privKey, pubKey, nil
}
