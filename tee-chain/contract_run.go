package teechain

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"

	"github.com/computeflux/tee-provider/pkg/domain"
	"github.com/computeflux/tee-provider/pkg/store"
)

const (
	RuntimeVersionKey     = "RUNTIME_VERSION"
	RuntimeMetaKeyPrefix  = "RUNTIME_META_"
	RuntimeAtHeightPrefix = "RUNTIME_AT_HEIGHT_"
	RuntimeDefaultVersion = uint32(0)
)

// InitRuntimeV0 启动时校验 genesis.json 包含 app_state.runtime.wasm。
func CheckRuntimeV0() error {
	data, err := os.ReadFile("./token_chain_data/config/genesis.json")
	if err != nil {
		return fmt.Errorf("read genesis.json: %w", err)
	}

	var genesis struct {
		AppState struct {
			Runtime struct {
				Wasm string `json:"wasm"`
			} `json:"runtime"`
		} `json:"app_state"`
	}
	if err := json.Unmarshal(data, &genesis); err != nil {
		return fmt.Errorf("parse genesis.json: %w", err)
	}

	if genesis.AppState.Runtime.Wasm == "" {
		return fmt.Errorf("genesis.json missing app_state.runtime.wasm")
	}

	return nil
}

// InitRuntimeVersion 初始化运行时版本系统（在 InitChain 或首次启动时调用）。
// genesisWasm 为 genesis app_state 中提供的 WASM（可选），若提供则优先使用。
func InitRuntimeVersion(genesisWasm []byte) error {
	if len(genesisWasm) == 0 {
		return fmt.Errorf("genesis app_state.wasm is empty")
	}
	return createRuntimeV0(genesisWasm, "genesis runtime")
}

func createRuntimeV0(wasmBytes []byte, desc string) error {
	info := &domain.RuntimeVersionInfo{
		Version:          RuntimeDefaultVersion,
		ActivationHeight: 0,
		CodeHash:         nil,
		WasmBytes:        wasmBytes,
		Description:      desc,
	}

	if err := SetRuntimeVersionInfo(info); err != nil {
		return err
	}

	if err := store.SetCodec("", RuntimeVersionKey, RuntimeDefaultVersion); err != nil {
		return err
	}

	if err := store.SetCodec("", RuntimeAtHeightPrefix+"0", RuntimeDefaultVersion); err != nil {
		return err
	}
	return nil
}

// GetCurrentRuntimeVersion 获取当前激活的运行时版本
func GetCurrentRuntimeVersion() (uint32, error) {
	v, err := store.GetCodecOrDefault("", RuntimeVersionKey, RuntimeDefaultVersion)
	if err != nil {
		return RuntimeDefaultVersion, err
	}
	return v, nil
}

// GetRuntimeVersionAtHeight 获取指定区块高度使用的运行时版本
func GetRuntimeVersionAtHeight(height int64) (uint32, error) {
	v, err := store.GetCodec[uint32]("", RuntimeAtHeightPrefix+strconv.FormatInt(height, 10))
	if err != nil {
		return RuntimeDefaultVersion, err
	}
	if v != nil {
		return *v, nil
	}

	current, err := GetCurrentRuntimeVersion()
	if err != nil {
		return RuntimeDefaultVersion, err
	}

	for ver := current; ver > 0; ver-- {
		info, err := GetRuntimeVersionInfo(ver)
		if err != nil || info == nil {
			continue
		}
		if height >= info.ActivationHeight {
			return ver, nil
		}
	}

	return RuntimeDefaultVersion, nil
}

// GetRuntimeVersionInfo 获取指定版本的元数据
func GetRuntimeVersionInfo(version uint32) (*domain.RuntimeVersionInfo, error) {
	return store.GetJson[domain.RuntimeVersionInfo]("", RuntimeMetaKeyPrefix+strconv.FormatUint(uint64(version), 10))
}

// SetRuntimeVersionInfo 保存版本元数据
func SetRuntimeVersionInfo(info *domain.RuntimeVersionInfo) error {
	return store.SetJson("", RuntimeMetaKeyPrefix+strconv.FormatUint(uint64(info.Version), 10), info)
}

// ApplyRuntimeUpgrade 应用运行时升级（用于 v1+，版本号递增）
func ApplyRuntimeUpgrade(txn *store.Txn, info *domain.RuntimeVersionInfo, height int64) error {
	if info == nil {
		return fmt.Errorf("runtime upgrade info is nil")
	}

	if err := UpdateRuntimeMeta(txn, info); err != nil {
		return err
	}

	if err := store.TxnSetCodec(txn, "", RuntimeVersionKey, info.Version); err != nil {
		return fmt.Errorf("update runtime version: %w", err)
	}

	if err := store.TxnSetCodec(txn, "", RuntimeAtHeightPrefix+strconv.FormatInt(height, 10), info.Version); err != nil {
		return fmt.Errorf("record runtime at height: %w", err)
	}

	return nil
}

// UpdateRuntimeMeta 仅更新版本元数据（不修改当前版本号和高度映射）
func UpdateRuntimeMeta(txn *store.Txn, info *domain.RuntimeVersionInfo) error {
	if info == nil {
		return fmt.Errorf("runtime upgrade info is nil")
	}

	metaKey := store.ComboNamespaceKey("", RuntimeMetaKeyPrefix+strconv.FormatUint(uint64(info.Version), 10))
	if err := store.TxnSetJson(txn, metaKey, info); err != nil {
		return fmt.Errorf("save runtime meta: %w", err)
	}

	return nil
}
