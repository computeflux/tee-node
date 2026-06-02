package teechain

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"strconv"

	"github.com/computeflux/tee-provider/pkg/model"
)

const (
	RuntimeVersionKey     = "RUNTIME_VERSION"
	RuntimeMetaKeyPrefix  = "RUNTIME_META_"
	RuntimeAtHeightPrefix = "RUNTIME_AT_HEIGHT_"
	RuntimeDefaultVersion = uint32(0)
)

// InitRuntimeVersion 初始化运行时版本系统（在 InitChain 或首次启动时调用）
func InitRuntimeVersion() error {
	current, err := GetCurrentRuntimeVersion()
	if err != nil {
		return err
	}
	if current > 0 {
		return nil
	}

	wasmBytes, err := loadWasmFile("./token_chain_data/runtime_v0.wasm")
	if err != nil {
		return fmt.Errorf("read v0 wasm file: %w", err)
	}

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

// loadWasmFile 加载 WASM 文件，支持 .wasm 和 .wasm.gz 两种格式。
func loadWasmFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		// 尝试 .gz 后缀
		if os.IsNotExist(err) {
			return loadWasmGz(path + ".gz")
		}
		return nil, err
	}
	defer f.Close()

	// 检测 gzip magic bytes (0x1f 0x8b)
	var magic [2]byte
	if _, err := io.ReadFull(f, magic[:]); err != nil {
		return nil, fmt.Errorf("read magic: %w", err)
	}
	if magic[0] == 0x1f && magic[1] == 0x8b {
		// 已经是 gzip 格式，回退到文件开头
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return nil, err
		}
		return readGzip(f)
	}

	// 普通文件，回退并读取全部
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
}

func loadWasmGz(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return readGzip(f)
}

func readGzip(r io.Reader) ([]byte, error) {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	return io.ReadAll(gz)
}

// GetCurrentRuntimeVersion 获取当前激活的运行时版本
func GetCurrentRuntimeVersion() (uint32, error) {
	v, err := model.GetCodecOrDefault("", RuntimeVersionKey, RuntimeDefaultVersion)
	if err != nil {
		return RuntimeDefaultVersion, err
	}
	return v, nil
}

// GetRuntimeVersionAtHeightTxn 从当前交易中优先读取版本（支持同区块内 RuntimeUpgrade 立即生效）。
func GetRuntimeVersionAtHeightTxn(txn *model.Txn, height int64) (uint32, error) {
	v, err := model.TxnGetCodec[uint32](txn, "", RuntimeAtHeightPrefix+strconv.FormatInt(height, 10))
	if err != nil {
		return RuntimeDefaultVersion, err
	}
	if v != nil {
		return *v, nil
	}
	return GetRuntimeVersionAtHeight(height)
}

// GetRuntimeVersionAtHeight 获取指定区块高度使用的运行时版本
func GetRuntimeVersionAtHeight(height int64) (uint32, error) {
	v, err := model.GetCodec[uint32]("", RuntimeAtHeightPrefix+strconv.FormatInt(height, 10))
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

	if err := UpdateRuntimeMeta(txn, info); err != nil {
		return err
	}

	if err := model.TxnSetCodec(txn, "", RuntimeVersionKey, info.Version); err != nil {
		return fmt.Errorf("update runtime version: %w", err)
	}

	if err := model.TxnSetCodec(txn, "", RuntimeAtHeightPrefix+strconv.FormatInt(height, 10), info.Version); err != nil {
		return fmt.Errorf("record runtime at height: %w", err)
	}

	return nil
}

// UpdateRuntimeMeta 仅更新版本元数据（不修改当前版本号和高度映射）
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
