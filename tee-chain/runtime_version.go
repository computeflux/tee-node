package teechain

import (
	"fmt"
	"os"
	"strconv"

	"github.com/computeflux/tee-provider/pkg/model"
)

const (
	RuntimeVersionKey       = "RUNTIME_VERSION"
	RuntimeMetaKeyPrefix    = "RUNTIME_META_"
	RuntimeAtHeightPrefix   = "RUNTIME_AT_HEIGHT_"
	RuntimeDefaultVersion   = uint32(0)
)



// InitRuntimeVersion 初始化运行时版本系统（在 InitChain 或首次启动时调用）
func InitRuntimeVersion() error {
	current, err := GetCurrentRuntimeVersion()
	if err != nil {
		return err
	}
	// 如果已经初始化过，跳过
	if current > 0 {
		return nil
	}

	// 从文件系统读取 v0 的 WASM
	wasmBytes, err := os.ReadFile("./token_chain_data/runtime_v0.wasm")
	if err != nil {
		return fmt.Errorf("read v0 wasm file: %w", err)
	}

	// 初始化版本 0
	info := &model.RuntimeVersionInfo{
		Version:          RuntimeDefaultVersion,
		ActivationHeight: 0,
		CodeHash:         nil,
		WasmBytes:        wasmBytes,
		Description:      "genesis runtime",
	}
	if err := SetRuntimeVersionInfo(info); err != nil {
		return err
	}
	if err := model.SetCodec("", RuntimeVersionKey, RuntimeDefaultVersion); err != nil {
		return err
	}
	if err := model.SetCodec("", RuntimeAtHeightPrefix+"0", RuntimeDefaultVersion); err != nil {
		return err
	}
	return nil
}

// GetCurrentRuntimeVersion 获取当前激活的运行时版本
func GetCurrentRuntimeVersion() (uint32, error) {
	v, err := model.GetCodecOrDefault("", RuntimeVersionKey, RuntimeDefaultVersion)
	if err != nil {
		return RuntimeDefaultVersion, err
	}
	return v, nil
}

// GetRuntimeVersionAtHeight 获取指定区块高度使用的运行时版本
func GetRuntimeVersionAtHeight(height int64) (uint32, error) {
	// 直接查询该高度记录
	v, err := model.GetCodec[uint32]("", RuntimeAtHeightPrefix+strconv.FormatInt(height, 10))
	if err != nil {
		return RuntimeDefaultVersion, err
	}
	if v != nil {
		return *v, nil
	}

	// 若无精确记录，扫描所有升级记录找到适用的版本
	// 由于升级频率极低，线性扫描是可接受的
	current, err := GetCurrentRuntimeVersion()
	if err != nil {
		return RuntimeDefaultVersion, err
	}

	// 从高版本向低版本扫描
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
func GetRuntimeVersionInfo(version uint32) (*model.RuntimeVersionInfo, error) {
	return model.GetJson[model.RuntimeVersionInfo]("", RuntimeMetaKeyPrefix+strconv.FormatUint(uint64(version), 10))
}

// SetRuntimeVersionInfo 保存版本元数据
func SetRuntimeVersionInfo(info *model.RuntimeVersionInfo) error {
	return model.SetJson("", RuntimeMetaKeyPrefix+strconv.FormatUint(uint64(info.Version), 10), info)
}

// ApplyRuntimeUpgrade 应用运行时升级（用于 v1+，版本号递增）
func ApplyRuntimeUpgrade(txn *model.Txn, info *model.RuntimeVersionInfo, height int64) error {
	if info == nil {
		return fmt.Errorf("runtime upgrade info is nil")
	}

	// 保存版本元数据
	if err := UpdateRuntimeMeta(txn, info); err != nil {
		return err
	}

	// 更新当前版本
	if err := model.TxnSetCodec(txn, "", RuntimeVersionKey, info.Version); err != nil {
		return fmt.Errorf("update runtime version: %w", err)
	}

	// 记录该高度使用的版本
	if err := model.TxnSetCodec(txn, "", RuntimeAtHeightPrefix+strconv.FormatInt(height, 10), info.Version); err != nil {
		return fmt.Errorf("record runtime at height: %w", err)
	}

	return nil
}

// UpdateRuntimeMeta 仅更新版本元数据（不修改当前版本号和高度映射）
// 用于 v0 补充 wasm_bytes，或更新已有版本的元数据
func UpdateRuntimeMeta(txn *model.Txn, info *model.RuntimeVersionInfo) error {
	if info == nil {
		return fmt.Errorf("runtime upgrade info is nil")
	}

	metaKey := model.ComboNamespaceKey("", RuntimeMetaKeyPrefix+strconv.FormatUint(uint64(info.Version), 10))
	if err := model.TxnSetJson(txn, metaKey, info); err != nil {
		return fmt.Errorf("save runtime meta: %w", err)
	}

	return nil
}
