package model

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/computeflux/tee-provider/pkg/util"
)

const ConfigKeyspace = "TEEKMS_CFG"

// SealedConfigValue 单个 key 的密封值
type SealedConfigValue struct {
	Ciphertext []byte `json:"ciphertext"` // sealing_key(KMSKey)
	SealedAt   int64  `json:"sealedAt"`
	Version    uint32 `json:"version"` // GK version
}

// configFullKey 生成 pebble 中的完整 key：namespace/key
func configFullKey(namespace, key string) string {
	if namespace == "" {
		return key
	}
	return namespace + "/" + key
}

// ── 单条 Seal / Get ──────────────────────────────────────

// SealConfigValue 用 GK 加密单个值，再用本地 sealing key 密封存入 pebble
func SealConfigValue(namespace, key, value string, gk *GlobalKey) error {
	// 先用 GK 加密
	enc, err := EncryptWithGK(gk, configFullKey(namespace, key), value)
	if err != nil {
		return fmt.Errorf("encrypt config value %s: %w", key, err)
	}
	encBytes, _ := json.Marshal(enc)

	// 再用 sealing key 密封
	sealed, err := util.SealWithProductKey(encBytes, []byte("TEEKMS_CFG"))
	if err != nil {
		return fmt.Errorf("seal config value %s: %w", key, err)
	}

	sc := SealedConfigValue{
		Ciphertext: sealed,
		SealedAt:   time.Now().Unix(),
		Version:    gk.Version,
	}
	b, _ := json.Marshal(sc)
	return SetKey(ConfigKeyspace, configFullKey(namespace, key), b)
}

// GetConfigValue 从密封存储读取单个值（自动 unseal + GK 解密）
func GetConfigValue(namespace, key string) (string, error) {
	b, err := GetKey(ConfigKeyspace, configFullKey(namespace, key))
	if err != nil {
		return "", fmt.Errorf("config key %s not found", key)
	}

	var sc SealedConfigValue
	if err := json.Unmarshal(b, &sc); err != nil {
		return "", err
	}

	// 1. 本地解封
	encBytes, err := util.Unseal(sc.Ciphertext, []byte("TEEKMS_CFG"))
	if err != nil {
		return "", fmt.Errorf("unseal config %s: %w", key, err)
	}

	// 2. 解析 GK 加密结构
	var enc KMSKey
	if err := json.Unmarshal(encBytes, &enc); err != nil {
		return "", err
	}

	// 3. 用 GK 解密
	gk, err := GetGlobalKey()
	if err != nil {
		return "", fmt.Errorf("get global key: %w", err)
	}
	plaintext, err := DecryptWithGK(gk, &enc)
	if err != nil {
		return "", fmt.Errorf("decrypt config %s: %w", key, err)
	}
	return plaintext, nil
}

// HasConfigKey 检查某个 key 是否已密封存储
func HasConfigKey(namespace, key string) bool {
	_, err := GetKey(ConfigKeyspace, configFullKey(namespace, key))
	return err == nil
}

// HasConfigNamespace 检查某个 namespace 下是否存在任何配置
func HasConfigNamespace(namespace string) bool {
	keys, err := ListConfigKeys(namespace)
	return err == nil && len(keys) > 0
}

// ListConfigKeys 列出某个 namespace 下已存储的所有 key（不含 namespace 前缀）
func ListConfigKeys(namespace string) ([]string, error) {
	prefix := configFullKey(namespace, "")
	var keys []string

	// 遍历 pebble 中 ConfigKeyspace 的所有 key
	all, err := ListKeysByPrefix(ConfigKeyspace, prefix)
	if err != nil {
		return nil, err
	}

	for _, k := range all {
		k = strings.TrimPrefix(k, prefix)
		if k != "" {
			keys = append(keys, k)
		}
	}
	return keys, nil
}

// ── 批量 Seal ────────────────────────────────────────────

// SealConfigMap 批量密封一组 key-value（常用于从环境变量初始化）
func SealConfigMap(namespace string, kv map[string]string, gk *GlobalKey) error {
	for k, v := range kv {
		if err := SealConfigValue(namespace, k, v, gk); err != nil {
			return fmt.Errorf("seal %s: %w", k, err)
		}
	}
	return nil
}

// SealAllConfigFromEnv 从环境变量批量读取并密封。
// 只密封 env 中存在的 key，不存在的跳过（允许部分配置可选）。
func SealAllConfigFromEnv(namespace string, keys []string, gk *GlobalKey) error {
	for _, key := range keys {
		val := util.GetEnv(key, "")
		if val == "" {
			continue // 环境变量未设置，跳过
		}
		if err := SealConfigValue(namespace, key, val, gk); err != nil {
			return fmt.Errorf("seal env %s: %w", key, err)
		}
	}
	return nil
}

// ── 节点间同步（复用已有 P2P 通道）────────────────────────

// ExportConfigMap 导出某个 namespace 下所有 GK 加密后的配置（不解密，用于 P2P 同步）
func ExportConfigMap(namespace string, gk *GlobalKey) (map[string][]byte, uint32, error) {
	keys, err := ListConfigKeys(namespace)
	if err != nil {
		return nil, 0, err
	}

	result := make(map[string][]byte, len(keys))
	for _, key := range keys {
		b, err := GetKey(ConfigKeyspace, configFullKey(namespace, key))
		if err != nil {
			continue
		}
		var sc SealedConfigValue
		if err := json.Unmarshal(b, &sc); err != nil {
			continue
		}
		// 本地解封获取 GK 加密的数据（仍然不是明文）
		encBytes, err := util.Unseal(sc.Ciphertext, []byte("TEEKMS_CFG"))
		if err != nil {
			continue
		}
		result[key] = encBytes
	}
	return result, gk.Version, nil
}

// ImportConfigMap 从其他节点导入 GK 加密后的配置，再用本地 sealing key 密封
func ImportConfigMap(namespace string, values map[string][]byte, version uint32) error {
	for key, encBytes := range values {
		sealed, err := util.SealWithProductKey(encBytes, []byte("TEEKMS_CFG"))
		if err != nil {
			return fmt.Errorf("seal imported config %s: %w", key, err)
		}
		sc := SealedConfigValue{
			Ciphertext: sealed,
			SealedAt:   time.Now().Unix(),
			Version:    version,
		}
		b, _ := json.Marshal(sc)
		if err := SetKey(ConfigKeyspace, configFullKey(namespace, key), b); err != nil {
			return err
		}
	}
	return nil
}
