package graph

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"testing"
)

func TestDecryptSecret(t *testing.T) {
	// 生成 RSA 密钥对
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("生成 RSA 密钥对失败: %v", err)
	}

	// 先用公钥加密，再用私钥解密（与 rsaDecryptWithKey 的 PKCS#1 v1.5 模式一致）
	expected := "这是一个测试字符串"
	ciphertext, err := rsa.EncryptPKCS1v15(rand.Reader, &privateKey.PublicKey, []byte(expected))
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}
	encryptedBase64 := base64.StdEncoding.EncodeToString(ciphertext)

	// 解密
	plaintext, err := rsaDecryptWithKey(privateKey, encryptedBase64)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	// 验证解密结果（这里假设加密前的数据是一个简单的字符串）
	if string(plaintext) != expected {
		t.Errorf("解密结果错误: 期望 %s, 实际 %s", expected, string(plaintext))
	}
}
