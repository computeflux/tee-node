package gov

import (
	"errors"

	"github.com/computeflux/tee-provider/tee-chain/runtime/module/gov/migrations"
)

// RuntimeUpgrade 运行时升级：授权、验证、存储版本元数据并更新当前版本。
// 系统调用（dkgpub）直接通过；非系统调用需 sudo 权限。
// 整个升级流程完全在合约内完成，无需链上层额外处理。
func (d GovMutation) RuntimeUpgrade(upgrade PendingRuntimeUpgrade) error {
	if !d.api.CallbySystem() {
		if err := d.ensureSudo(); err != nil {
			return err
		}
	}

	if len(upgrade.WasmBytes) == 0 {
		return errors.New("runtime upgrade: wasm_bytes is empty")
	}

	// 版本检查：必须为当前版本 + 1
	cver, err := d.runtimeVersion.Get(d.api.GetTxn())
	if err != nil {
		return err
	}
	currentVersion := uint32(0)
	if cver != nil {
		currentVersion = *cver
	}
	if upgrade.Version != currentVersion+1 {
		return errors.New("runtime upgrade: invalid version (must be current + 1)")
	}

	// 记录激活高度
	height := uint64(d.api.GetHeight())
	upgrade.ActivationHeight = height

	// 存储版本元数据
	if err := d.runtimeMetas.Set(d.api.GetTxn(), upgrade.Version, upgrade); err != nil {
		return err
	}

	// 更新当前版本
	if err := d.runtimeVersion.Set(d.api.GetTxn(), upgrade.Version); err != nil {
		return err
	}

	// 记录版本激活高度
	if err := d.runtimeAtHeight.Set(d.api.GetTxn(), height, upgrade.Version); err != nil {
		return err
	}

	return nil
}

func (d GovMutation) Migration(newVersion uint32) error {
	if !d.api.CallbySystem() {
		return errors.New("must call by system")
	}

	if newVersion == 14 {
		return migrations.V14()
	}
	return nil
}

// RuntimeVersion 查询当前运行时版本号。
func (q GovQuery) RuntimeVersion() (*uint32, error) {
	return q.runtimeVersion.Get(q.api.GetTxn())
}

// RuntimeMeta 查询指定版本的元数据。
func (q GovQuery) RuntimeMeta(version uint32) (*PendingRuntimeUpgrade, error) {
	return q.runtimeMetas.Get(q.api.GetTxn(), version)
}

// RuntimeAtHeight 查询指定高度激活的版本号。
func (q GovQuery) RuntimeAtHeight(height int64) (*uint32, error) {
	return q.runtimeAtHeight.Get(q.api.GetTxn(), uint64(height))
}
