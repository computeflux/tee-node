package wallet

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"
)

// TxRequest 交易签名请求
type TxRequest struct {
	From         string
	To           string
	Value        string
	Data         string
	Gas          string
	GasPrice     string
	Nonce        string
	ChainID      string
	PasswordHash string
}

// SignTx 签名以太坊交易
func (s *WalletService) SignTx(email string, tx *TxRequest) (string, error) {
	var priv *ecdsa.PrivateKey
	var err error

	if s.HasEncryptedKey(email) {
		priv, err = s.decryptKeyByType(email, tx.PasswordHash)
		if err != nil {
			return "", err
		}
	} else {
		// 没有加密存储 → 从 seed 派生
		priv, err = s.DerivePrivateKey(email)
		if err != nil {
			return "", err
		}
	}

	chainID := big.NewInt(1)
	if tx.ChainID != "" {
		if id, ok := new(big.Int).SetString(cleanHex(tx.ChainID), 16); ok {
			chainID = id
		}
	}

	nonce := uint64(0)
	if tx.Nonce != "" && tx.Nonce != "0x0" {
		n, _ := new(big.Int).SetString(cleanHex(tx.Nonce), 16)
		if n != nil {
			nonce = n.Uint64()
		}
	}

	gasLimit := uint64(0)
	if tx.Gas != "" {
		g, _ := new(big.Int).SetString(cleanHex(tx.Gas), 16)
		if g != nil {
			gasLimit = g.Uint64()
		}
	}

	gasPrice := new(big.Int)
	if tx.GasPrice != "" {
		// 注意：SetString 失败返回 nil，直接赋值会让下面 gasPrice.Sign() panic
		if g, ok := new(big.Int).SetString(cleanHex(tx.GasPrice), 16); ok {
			gasPrice = g
		}
	}

	value := new(big.Int)
	if tx.Value != "" {
		// SetString 失败返回 nil，直接赋值会让 RLP 编码时 panic
		if v, ok := new(big.Int).SetString(cleanHex(tx.Value), 16); ok {
			value = v
		}
	}

	toAddr := ethcrypto.CreateAddress(ethcrypto.PubkeyToAddress(priv.PublicKey), nonce)
	if tx.To != "" {
		toAddr = common.HexToAddress(tx.To)
	}

	var data []byte
	if tx.Data != "" && tx.Data != "0x" {
		data, _ = hex.DecodeString(cleanHex(tx.Data))
	}

	// 默认 gas
	if gasPrice.Sign() == 0 {
		gasPrice.SetString("2000000000000", 10)
	}
	if gasLimit == 0 {
		if len(data) > 0 {
			gasLimit = 300000
		} else {
			gasLimit = 21000
		}
	}

	unsigned := &types.LegacyTx{
		Nonce:    nonce,
		To:       &toAddr,
		Value:    value,
		Gas:      gasLimit,
		GasPrice: gasPrice,
		Data:     data,
	}
	signer := types.NewEIP155Signer(chainID)

	signed, err := types.SignNewTx(priv, signer, unsigned)
	if err != nil {
		return "", fmt.Errorf("sign tx: %w", err)
	}

	raw, err := signed.MarshalBinary()
	if err != nil {
		return "", fmt.Errorf("marshal tx: %w", err)
	}
	return "0x" + hex.EncodeToString(raw), nil
}

// SignMessage 签名消息 (EIP-191)
// passwordHash: 用户设置的交易密码哈希；若用户未设置密码且使用服务端密钥加密，则传空
func (s *WalletService) SignMessage(email, message, passwordHash string) (string, error) {
	var priv *ecdsa.PrivateKey
	var err error

	if s.HasEncryptedKey(email) {
		priv, err = s.decryptKeyByType(email, passwordHash)
		if err != nil {
			return "", err
		}
	} else {
		// 没有加密存储 → 从 seed 派生（首次使用）
		priv, err = s.DerivePrivateKey(email)
		if err != nil {
			return "", err
		}
	}

	// 兼容 MetaMask personal_sign：如果 message 是 hex 字符串，先解码为原始 bytes
	msgBytes, err := decodeMessage(message)
	if err != nil {
		return "", fmt.Errorf("invalid message: %w", err)
	}

	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(msgBytes))
	hash := ethcrypto.Keccak256Hash(append([]byte(prefix), msgBytes...))
	sig, err := ethcrypto.Sign(hash.Bytes(), priv)
	if err != nil {
		return "", fmt.Errorf("sign message: %w", err)
	}
	return "0x" + hex.EncodeToString(sig), nil
}

// decodeMessage 解码 personal_sign 的消息参数。
// MetaMask 行为：如果消息是以 "0x" 开头的 hex 字符串，则解码为原始 bytes；
// 否则将消息作为普通 UTF-8 字符串处理。
func decodeMessage(msg string) ([]byte, error) {
	if strings.HasPrefix(msg, "0x") || strings.HasPrefix(msg, "0X") {
		decoded, err := hex.DecodeString(cleanHex(msg))
		if err == nil {
			return decoded, nil
		}
		// 解码失败则回退为普通字符串（如 "0xhello" 不是合法 hex）
	}
	return []byte(msg), nil
}

func cleanHex(s string) string {
	s = strings.TrimPrefix(s, "0x")
	s = strings.TrimPrefix(s, "0X")
	return s
}
