package model

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/computeflux/tee-provider/pkg/util"
)

const (
	KMSKeyspace  = "TEEKMS"
	GKRefKey     = "global_key"
	GKVersionKey = "global_version"
	AESKeySize   = 32
)

// SealedGK 被本地 sealing key 保护的 GK
type SealedGK struct {
	Ciphertext []byte `json:"ciphertext"`
	SealedAt   int64  `json:"sealedAt"`
	Version    uint32 `json:"version"`
}

// GlobalKey 全局 KMS 密钥（仅在 enclave 内存中使用）
type GlobalKey struct {
	Key     []byte `json:"key"`
	Version uint32 `json:"version"`
}

// KMSKey 用 GK 加密后的 API Key
type KMSKey struct {
	Ref        string `json:"ref"`
	Ciphertext []byte `json:"ciphertext"`
	Nonce      []byte `json:"nonce"`
}

// ── GK 生成 ───────────────────────────────────────────────

// GenerateGlobalKey 生成新的全局密钥
func GenerateGlobalKey() (*GlobalKey, error) {
	key := make([]byte, AESKeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("generate GK: %w", err)
	}
	return &GlobalKey{Key: key, Version: 1}, nil
}

// ── GK Seal / Unseal ─────────────────────────────────────

// SealGlobalKey 用本地 sealing key 加密 GK 后存入 pebble
func SealGlobalKey(gk *GlobalKey) error {
	plaintext, err := json.Marshal(gk)
	if err != nil {
		return err
	}
	sealed, err := util.SealWithProductKey(plaintext, []byte("TEEKMS_GK"))
	if err != nil {
		return fmt.Errorf("seal GK: %w", err)
	}
	sg := SealedGK{
		Ciphertext: sealed,
		SealedAt:   time.Now().Unix(),
		Version:    gk.Version,
	}
	b, _ := json.Marshal(sg)
	if err := SetKey(KMSKeyspace, GKRefKey, b); err != nil {
		return err
	}
	verB, _ := json.Marshal(gk.Version)
	return SetKey(KMSKeyspace, GKVersionKey, verB)
}

// HasGlobalKey 检查本地是否已存储 GK。
func HasGlobalKey() bool {
	_, err := GetKey(KMSKeyspace, GKRefKey)
	return err == nil
}

// UnsealGlobalKey 从 pebble 加载并解封 GK
func UnsealGlobalKey() (*GlobalKey, error) {
	b, err := GetKey(KMSKeyspace, GKRefKey)
	if err != nil {
		return nil, fmt.Errorf("GK not initialized")
	}
	var sg SealedGK
	if err := json.Unmarshal(b, &sg); err != nil {
		return nil, err
	}
	plaintext, err := util.Unseal(sg.Ciphertext, []byte("TEEKMS_GK"))
	if err != nil {
		return nil, fmt.Errorf("unseal GK: %w", err)
	}
	var gk GlobalKey
	if err := json.Unmarshal(plaintext, &gk); err != nil {
		return nil, err
	}
	return &gk, nil
}

// ── AES-GCM 加密 / 解密 ──────────────────────────────────

// EncryptWithGK 用 GK 加密明文
func EncryptWithGK(gk *GlobalKey, ref string, plaintext string) (*KMSKey, error) {
	block, err := aes.NewCipher(gk.Key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	aad := fmt.Sprintf("%s:%d", ref, gk.Version)
	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), []byte(aad))
	return &KMSKey{
		Ref:        ref,
		Ciphertext: ciphertext,
		Nonce:      nonce,
	}, nil
}

// DecryptWithGK 用 GK 解密
func DecryptWithGK(gk *GlobalKey, ek *KMSKey) (string, error) {
	block, err := aes.NewCipher(gk.Key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	aad := fmt.Sprintf("%s:%d", ek.Ref, gk.Version)
	plaintext, err := gcm.Open(nil, ek.Nonce, ek.Ciphertext, []byte(aad))
	if err != nil {
		return "", fmt.Errorf("decrypt failed: %w", err)
	}
	return string(plaintext), nil
}

// ── 单例缓存（运行时内存） ───────────────────────────────

var (
	gkCache     *GlobalKey
	gkCacheErr  error
	gkCacheInit bool
)

// GetGlobalKey 获取全局密钥（带内存缓存）
func GetGlobalKey() (*GlobalKey, error) {
	if gkCacheInit {
		return gkCache, gkCacheErr
	}
	gkCache, gkCacheErr = UnsealGlobalKey()
	gkCacheInit = true
	return gkCache, gkCacheErr
}

// ClearGlobalKeyCache 清除缓存（GK 轮换后调用）
func ClearGlobalKeyCache() {
	gkCache = nil
	gkCacheInit = false
}

// HashRef 将 ref 转为确定性 32-byte hash（用于链上 key）
func HashRef(ref string) []byte {
	h := sha256.Sum256([]byte(ref))
	return h[:]
}

// ── 启动初始化 ───────────────────────────────────────────

// InitGlobalKeyAtStartup 节点启动时调用：
//   - 若本地已存在 SealedGK，自动解封并加载到内存缓存
//   - 若不存在，返回 error 提示需要从其他节点同步（或首次启动时生成）
func InitGlobalKeyAtStartup() (*GlobalKey, error) {
	gk, err := GetGlobalKey()
	if err == nil {
		return gk, nil
	}
	// 本地不存在，返回具体错误让上层决定（生成或同步）
	return nil, fmt.Errorf("global key not found locally: %w (new node must sync from peers or generate)", err)
}

// GenerateAndSealGlobalKey 首次启动时生成 GK 并密封存储。
// 仅应在全网首次部署时由一个节点调用，其他节点应通过 RA+TLS 同步。
func GenerateAndSealGlobalKey() (*GlobalKey, error) {
	gk, err := GenerateGlobalKey()
	if err != nil {
		return nil, err
	}
	if err := SealGlobalKey(gk); err != nil {
		return nil, err
	}
	// 同时加载到内存缓存
	gkCache = gk
	gkCacheErr = nil
	gkCacheInit = true
	return gk, nil
}

// ImportGlobalKey 从其他 TEE 节点同步后导入 GK。
// plaintext 是通过 RA+TLS enclave-to-enclave 传输的明文 GK。
func ImportGlobalKey(plaintext []byte, version uint32) error {
	gk := &GlobalKey{Key: plaintext, Version: version}
	if err := SealGlobalKey(gk); err != nil {
		return err
	}
	gkCache = gk
	gkCacheErr = nil
	gkCacheInit = true
	return nil
}

// ── 导出 / 导入（部署时管理员安全传递） ──────────────────

// ExportGlobalKeyBase64 导出当前 GK 为 base64 字符串（供管理员复制到新节点）。
// 注意：导出的字符串包含明文 GK，必须通过安全渠道传递（如 K8s secret）。
func ExportGlobalKeyBase64() (string, uint32, error) {
	gk, err := GetGlobalKey()
	if err != nil {
		return "", 0, err
	}
	return base64.StdEncoding.EncodeToString(gk.Key), gk.Version, nil
}

// ImportGlobalKeyBase64 从 base64 字符串导入 GK。
// 用于新节点通过环境变量 TEE_KMS_IMPORT_KEY 获取 GK。
func ImportGlobalKeyBase64(b64 string, version uint32) error {
	key, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return fmt.Errorf("invalid base64 GK: %w", err)
	}
	if len(key) != AESKeySize {
		return fmt.Errorf("invalid GK length: expected %d, got %d", AESKeySize, len(key))
	}
	return ImportGlobalKey(key, version)
}
