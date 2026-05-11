package gov

import (
	"math/big"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wetee-dao/tee-dsecret/pkg/model"
)

// testDBSubdir is the subdirectory where the test database is stored.
const testDBSubdir = "chain_data/wetee"

// testRuntime implements model.ContractApi for tests.
type testRuntime struct {
	height      int64
	caller      model.UniAddr
	txn         *model.Txn
	sudoAccount model.UniAddr
}

func (r *testRuntime) GetHeight() int64         { return r.height }
func (r *testRuntime) GetTxn() *model.Txn       { return r.txn }
func (r *testRuntime) GetCaller() model.UniAddr { return r.caller }
func (r *testRuntime) GetSudoAccount() model.UniAddr {
	return r.sudoAccount
}

// Query 实现跨合约查询（测试环境暂不支持）
func (r *testRuntime) Query(target model.UniAddr, call model.ContractCall) ([]byte, error) {
	return nil, nil
}

// Call 实现跨合约调用（测试环境暂不支持）
func (r *testRuntime) Call(target model.UniAddr, call model.ContractCall) ([]byte, error) {
	return nil, nil
}

// setupTestDB creates a temporary database for testing with an initialized runtime.
// It returns a testRuntime with the database transaction set up.
func setupTestDB(t *testing.T) (rt *testRuntime, cleanup func()) {
	t.Helper()
	tmp := t.TempDir()
	oldWD, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmp))
	require.NoError(t, os.MkdirAll(testDBSubdir, 0o755))

	db, err := model.NewDB()
	require.NoError(t, err)

	txn := db.NewTransaction()
	rt = &testRuntime{txn: txn}

	cleanup = func() {
		_ = txn.Rollback()
		_ = db.Close()
		model.DBINS = nil
		_ = os.Chdir(oldWD)
	}
	return rt, cleanup
}

func TestAmountAddSub(t *testing.T) {
	a := model.Amount{Int: big.NewInt(100)}
	b := model.Amount{Int: big.NewInt(30)}

	sum := model.AmountAdd(a, b)
	require.Equal(t, 0, sum.Int.Cmp(big.NewInt(130)))

	diff := model.AmountSub(a, b)
	require.Equal(t, 0, diff.Int.Cmp(big.NewInt(70)))

	// 零值测试
	zero := model.ZeroAmount
	require.Equal(t, 0, zero.Int.Sign())
}

func TestAllowanceKey(t *testing.T) {
	owner := model.UniAddr{T: 0, V: []byte{0xab}}
	spender := model.UniAddr{T: 0, V: []byte{0xcd}}
	k := allowanceKey(owner, spender)
	// allowanceKey 组合两个地址
	require.Equal(t, []byte{0xab, 0xcd}, k.V)
}
