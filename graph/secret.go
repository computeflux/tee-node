package graph

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"fmt"

	"github.com/computeflux/tee-provider/pkg/types/crypto"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

// rsaDecryptWithKey 直接使用 *rsa.PrivateKey 解密数据
// 参数:
//
//	privateKey: 已加载的 RSA 私钥对象
//	encryptedBase64: Base64 编码的密文
//	useOAEP: 是否使用 OAEP 模式（true 为 OAEP，false 为 PKCS#1 v1.5）
//
// 返回:
//
//	解密后的明文
//	可能的错误
func rsaDecryptWithKey(privateKey *rsa.PrivateKey, encryptedBase64 string) ([]byte, error) {
	// 1. 解码 Base64（jsencrypt 加密后默认返回 Base64 字符串）
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return nil, fmt.Errorf("Base64 decode error: %v", err)
	}

	// 2. 解密（jsencrypt 默认使用 PKCS#1 v1.5 模式）
	plaintext, err := rsa.DecryptPKCS1v15(
		rand.Reader,
		privateKey,
		ciphertext,
	)
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt error: %v", err)
	}

	return plaintext, nil
}

func checkSecretOwnership(ctx context.Context, user string) (*crypto.PubKey, error) {
	token := GatewayTokenFromContext(ctx)
	if token == nil {
		return nil, gqlerror.Errorf("authentication required")
	}

	pubkey, err := crypto.PubKeyFromSS58(user)
	if err != nil {
		return nil, gqlerror.Errorf("PubKeyFromSS58 error: %s", err.Error())
	}
	pubAddr := pubkey.H160Address()

	targetAddr := store.UniAddr{T: 2, V: pubAddr[:]}
	if targetAddr.T != token.Addr.T || !bytes.Equal(targetAddr.V, token.Addr.V) {
		return nil, gqlerror.Errorf("user address does not match authenticated token owner")
	}

	return pubkey, nil
}
