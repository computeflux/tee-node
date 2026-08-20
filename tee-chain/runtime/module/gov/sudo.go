package gov

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/computeflux/tee-provider/tee-chain/runtime/store"
)

// ── Sudo ─────────────────────────────────────────────────────

// ensureSudo 检查调用者是否为 sudo 账户且 sudo 未被禁用。
func (d Gov) ensureSudo() error {
	sudoAddr, err := d.sudo.Get(d.api.GetTxn())
	if err != nil || sudoAddr == nil {
		return ErrSudoNotSet
	}
	disabled, err := d.sudoDisabled.Get(d.api.GetTxn())
	if err != nil {
		return err
	}
	if disabled != nil && *disabled {
		return ErrSudoDisabled
	}
	caller := d.api.GetCaller()
	if !bytes.Equal(sudoAddr.V, caller.V) || sudoAddr.T != caller.T {
		return ErrNotSudo
	}
	return nil
}

// SetSudo 设置 sudo 账户（仅系统调用）。
func (d GovMutation) SetSudo(addr store.UniAddr) error {
	if !d.api.CallbySystem() {
		return errors.New("must call by system")
	}
	return d.sudo.Set(d.api.GetTxn(), addr)
}

// DisableSudo 永久禁用 sudo 权限（仅 sudo 账户可调用，不可逆）。
func (d GovMutation) DisableSudo() error {
	if err := d.ensureSudo(); err != nil {
		return err
	}
	return d.sudoDisabled.Set(d.api.GetTxn(), true)
}

// CallAsSudo 以 sudo 权限批量执行合约调用（仅 sudo 账户，未禁用时可调用）。
func (d GovMutation) CallAsSudo(calls []store.InContractCall) error {
	if err := d.ensureSudo(); err != nil {
		return err
	}
	for i, call := range calls {
		if len(call.Name) == 0 {
			return fmt.Errorf("sudo call %d: empty contract name", i)
		}

		if len(call.Method) == 0 {
			return fmt.Errorf("sudo call %d: empty method selector", i)
		}

		_, err := d.api.Call(d.api.GetSystemAccount(), &call)
		if err != nil {
			return fmt.Errorf("sudo call %d (%s): %w", i, string(call.Name), err)
		}
	}
	return nil
}

// Sudo 查询当前 sudo 账户地址。
func (q GovQuery) Sudo() (*store.UniAddr, error) {
	return q.sudo.Get(q.api.GetTxn())
}

// IsSudoDisabled 查询 sudo 是否已被永久禁用。
func (q GovQuery) IsSudoDisabled() (bool, error) {
	v, err := q.sudoDisabled.Get(q.api.GetTxn())
	if err != nil || v == nil {
		return false, err
	}
	return *v, nil
}
