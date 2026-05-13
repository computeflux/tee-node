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
		if tx.PasswordHash != "" {
			// 用户设了密码 → 用密码解密
			priv, err = s.DecryptPrivateKey(email, tx.PasswordHash)
		} else {
			// 没设密码 → 用服务端密钥解密
			keyBytes, e := s.DecryptWithServerKey(email)
			if e != nil {
				return "", e
			}
			priv, err = ethcrypto.ToECDSA(keyBytes)
		}
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
		gasPrice, _ = new(big.Int).SetString(cleanHex(tx.GasPrice), 16)
	}

	value := new(big.Int)
	if tx.Value != "" {
		value, _ = new(big.Int).SetString(cleanHex(tx.Value), 16)
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
func (s *WalletService) SignMessage(email, message string) (string, error) {
	priv, err := s.DerivePrivateKey(email)
	if err != nil {
		return "", err
	}
	msgBytes := []byte(message)
	prefix := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(msgBytes))
	hash := ethcrypto.Keccak256Hash(append([]byte(prefix), msgBytes...))
	sig, err := ethcrypto.Sign(hash.Bytes(), priv)
	if err != nil {
		return "", fmt.Errorf("sign message: %w", err)
	}
	return "0x" + hex.EncodeToString(sig), nil
}

func cleanHex(s string) string {
	return strings.TrimPrefix(s, "0x")
}
