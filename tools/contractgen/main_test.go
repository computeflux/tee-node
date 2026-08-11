package main

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// ---------- 命名工具 ----------

func TestSmartSnakeCase(t *testing.T) {
	cases := map[string]string{
		"UserID":      "user_id",
		"ID":          "id",
		"GetUserInfo": "get_user_info",
		"URL":         "url",
		"HTTPRequest": "http_request",
		"Tx":          "tx",
		"BlockNumber": "block_number",
		"simple":      "simple",
		"":            "",
	}
	for in, want := range cases {
		require.Equal(t, want, smartSnakeCase(in), "smartSnakeCase(%q)", in)
	}
}

// 非 ASCII 标识符不应 panic（回归测试：原实现用字节索引访问 rune 切片）
func TestSmartSnakeCase_NonASCII(t *testing.T) {
	require.Equal(t, "中文", smartSnakeCase("中文"))
	require.Equal(t, "æbc", smartSnakeCase("Æbc"))
	// 非 ASCII 小写字母不参与 ASCII 大小写判定，'B' 前仍会插下划线
	require.Equal(t, "fooæ_bar", smartSnakeCase("FooÆBar"))
	require.Equal(t, "标识符", smartSnakeCase("标识符"))
}

func TestSnakeCase(t *testing.T) {
	require.Equal(t, "user_i_d", snakeCase("UserID")) // 朴素实现：每个大小写转换处都插下划线
	require.Equal(t, "get_user_info", snakeCase("GetUserInfo"))
	require.Equal(t, "abc", snakeCase("abc"))
}

func TestCamelToSnake(t *testing.T) {
	require.Equal(t, "user_id", camelToSnake("UserId"))
	require.Equal(t, "arg0", camelToSnake("Arg0"))
	require.Equal(t, "abc", camelToSnake("abc"))
}

func TestParamABIName(t *testing.T) {
	require.Equal(t, "user_id", paramABIName("UserId", 0))
	require.Equal(t, "arg0", paramABIName("", 0))
	require.Equal(t, "arg7", paramABIName("", 7))
}

func TestContractNameGoSafe(t *testing.T) {
	require.Equal(t, "my_contract", contractNameGoSafe("my-contract"))
	require.Equal(t, "normal", contractNameGoSafe("normal"))
	require.Equal(t, "a_b_c", contractNameGoSafe("a-b-c"))
}

func TestSelectorHex(t *testing.T) {
	require.Equal(t, "0x00010203", selectorHex([4]byte{0, 1, 2, 3}))
	require.Equal(t, "0xdeadbeef", selectorHex([4]byte{0xde, 0xad, 0xbe, 0xef}))
}

func TestPickSelectorInk_Consistent(t *testing.T) {
	hex1 := pickSelectorInk("transfer", true)
	hex2 := pickSelectorInk("transfer", true)
	require.Equal(t, hex1, hex2)

	// bytes 形式与 hex 形式一致
	sel := pickSelectorInkBytes("transfer", true)
	require.Equal(t, selectorHex(sel), pickSelectorInk("transfer", true))

	// mutates 影响 selector
	require.NotEqual(t, pickSelectorInk("transfer", true), pickSelectorInk("transfer", false))
	// label 影响 selector
	require.NotEqual(t, pickSelectorInk("transfer", true), pickSelectorInk("balance", true))
}

func TestParseSkip(t *testing.T) {
	m := parseSkip("a, b, ,c")
	require.Equal(t, map[string]bool{"a": true, "b": true, "c": true}, m)
	require.Empty(t, parseSkip(""))
	require.Empty(t, parseSkip(" , , "))
}

// ---------- AST 工具 ----------

func parseExprStr(t *testing.T, src string) ast.Expr {
	t.Helper()
	expr, err := parser.ParseExpr(src)
	require.NoError(t, err, "parse %q", src)
	return expr
}

func TestRecvTypeName(t *testing.T) {
	require.Equal(t, "Foo", recvTypeName(parseExprStr(t, "Foo")))
	require.Equal(t, "Foo", recvTypeName(parseExprStr(t, "*Foo")))
	require.Equal(t, "Foo", recvTypeName(parseExprStr(t, "**Foo")))
	require.Equal(t, "", recvTypeName(parseExprStr(t, "map[string]int")))
}

func TestTypeString(t *testing.T) {
	fset := token.NewFileSet()
	require.Equal(t, "*model.StoreMapping[Vec[u8], u64]", typeString(fset, parseExprStr(t, "*model.StoreMapping[Vec[u8], u64]")))
	require.Equal(t, "uint32", typeString(fset, parseExprStr(t, "uint32")))
}

func TestIsResultErrorType(t *testing.T) {
	require.True(t, isResultErrorType(parseExprStr(t, "Result[T, error]")))
	require.True(t, isResultErrorType(parseExprStr(t, "store.Result[T, error]")))
	require.True(t, isResultErrorType(parseExprStr(t, "Result[Vec[u8], error]")))
	require.False(t, isResultErrorType(parseExprStr(t, "Result[T, E]")))
	require.False(t, isResultErrorType(parseExprStr(t, "T")))
	require.False(t, isResultErrorType(parseExprStr(t, "error")))
	require.False(t, isResultErrorType(parseExprStr(t, "Result[T]")))
}

// ---------- Store 类型解析 ----------

func TestParseTypePair(t *testing.T) {
	k, v, ok := parseTypePair("K, V")
	require.True(t, ok)
	require.Equal(t, "K", k)
	require.Equal(t, "V", v)

	// 嵌套逗号不分割
	k, v, ok = parseTypePair("Vec[K, X], V")
	require.True(t, ok)
	require.Equal(t, "Vec[K, X]", k)
	require.Equal(t, "V", v)

	// 元组
	k, v, ok = parseTypePair("(A, B), C")
	require.True(t, ok)
	require.Equal(t, "(A, B)", k)

	_, _, ok = parseTypePair("K")
	require.False(t, ok)
}

func TestParseStoreTypes(t *testing.T) {
	// StoreMapping 带包前缀与指针
	k, v, ok := parseStoreMappingType("*model.StoreMapping[Vec[u8], u64]")
	require.True(t, ok)
	require.Equal(t, "Vec[u8]", k)
	require.Equal(t, "u64", v)

	k, v, ok = parseStoreMappingType("store.StoreMapping[u8, u32]")
	require.True(t, ok)
	require.Equal(t, "u8", k)
	require.Equal(t, "u32", v)

	_, _, ok = parseStoreMappingType("StoreValue[u8]")
	require.False(t, ok)

	// StoreValue
	val, ok := parseStoreValueType("*model.StoreValue[Account]")
	require.True(t, ok)
	require.Equal(t, "Account", val)

	// StoreList
	k, v, ok = parseStoreListType("model.StoreList[Vec[u8], Account]")
	require.True(t, ok)
	require.Equal(t, "Vec[u8]", k)
	require.Equal(t, "Account", v)

	// IndexedList
	k, v, ok = parseIndexedListType("model.IndexedList[u8, Account]")
	require.True(t, ok)
	require.Equal(t, "u8", k)
	require.Equal(t, "Account", v)

	// IndexedStore
	id, val, ok := parseIndexedStoreType("model.IndexedStore[Vec[u8], Account]")
	require.True(t, ok)
	require.Equal(t, "Vec[u8]", id)
	require.Equal(t, "Account", val)

	// StoreList2D 三个参数，中间参数含嵌套
	k1, ix, val, ok := parseStoreList2DType("model.StoreList2D[u8, Vec[u16], Account]")
	require.True(t, ok)
	require.Equal(t, "u8", k1)
	require.Equal(t, "Vec[u16]", ix)
	require.Equal(t, "Account", val)

	_, _, _, ok = parseStoreList2DType("model.StoreList[u8, Account]")
	require.False(t, ok)
}

func TestNormalizeStoreType(t *testing.T) {
	require.Equal(t, "StoreMapping[u8, u32]", normalizeStoreType("*model.StoreMapping[u8, u32]"))
	require.Equal(t, "StoreList[u8, u32]", normalizeStoreType("store.StoreList[u8, u32]"))
	require.Equal(t, "StoreValue[u8]", normalizeStoreType("  *StoreValue[u8]  "))
}

// ---------- struct tag 解析 ----------

func TestReflectParseStructTag(t *testing.T) {
	m, ok := reflectParseStructTag(`store:"key:xxx,foo:bar"`, "store")
	require.True(t, ok)
	require.Equal(t, map[string]string{"key": "xxx", "foo": "bar"}, m)

	m, ok = reflectParseStructTag("`json:\"name\"`", "json")
	require.True(t, ok)
	// 纯值（无 k:v 对）不产生任何条目
	require.Empty(t, m)

	_, ok = reflectParseStructTag(`json:"name"`, "store")
	require.False(t, ok)
}

func TestExtractKeyPrefix(t *testing.T) {
	// 行尾注释 // keyPfx: pre_
	field := parseField(t, "A string // keyPfx: pre_")
	require.Equal(t, "pre_", extractKeyPrefix(field, "keyPfx"))

	// 无注释则不加后缀的自动补 _
	field = parseField(t, "A string")
	require.Equal(t, "", extractKeyPrefix(field, "keyPfx"))

	// struct tag 形式，自动补下划线
	field = parseField(t, "A string `store:\"keyPfx:tagged\"`")
	require.Equal(t, "tagged_", extractKeyPrefix(field, "keyPfx"))

	// tag 中不存在 keyPfx 时为空
	field = parseField(t, "A string `store:\"key:other\"`")
	require.Equal(t, "", extractKeyPrefix(field, "keyPfx"))
}

func parseField(t *testing.T, src string) *ast.Field {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "test.go", "package p\ntype S struct {\n\t"+src+"\n}\n", parser.ParseComments)
	require.NoError(t, err)
	ts := f.Decls[0].(*ast.GenDecl).Specs[0].(*ast.TypeSpec)
	st := ts.Type.(*ast.StructType)
	require.Len(t, st.Fields.List, 1)
	return st.Fields.List[0]
}

// ---------- key 生成 ----------

func TestStoreKey(t *testing.T) {
	require.Equal(t, "tx", storeKey("TxStore"))
	require.Equal(t, "user", storeKey("UserStore"))
	require.Equal(t, "block_number", storeKey("BlockNumberStore"))
	require.Equal(t, "id", storeKey("IDStore"))
	require.Equal(t, "plain", storeKey("Plain"))
}

func TestStoreKeyPrefix(t *testing.T) {
	require.Equal(t, "tx_", storeKeyPrefix("TxStore"))
	require.Equal(t, "tx_", storeKeyPrefix("TxsStore")) // 复数去 s
	require.Equal(t, "user_", storeKeyPrefix("UsersStore"))
	require.Equal(t, "address_", storeKeyPrefix("AddressStore"))
	require.Equal(t, "block_number_", storeKeyPrefix("BlockNumberStore"))
}

// ---------- 重复方法检测 ----------

func TestFindDuplicateLabels(t *testing.T) {
	mut := []*methodSig{
		{goName: "MutTransfer", caseName: "transfer"},
	}
	query := []*methodSig{
		{goName: "QueryTransfer", caseName: "transfer"},
		{goName: "QueryBalance", caseName: "balance"},
	}

	dups := findDuplicateLabels(mut, query)
	require.Len(t, dups, 1)
	require.Equal(t, "transfer", dups[0].caseName)
	require.Equal(t, "MutTransfer", dups[0].mutGoName)
	require.Equal(t, "QueryTransfer", dups[0].queryGoName)

	require.Empty(t, findDuplicateLabels(mut, nil))
}

// ---------- ABI 输出 ----------

func TestAbiMessageJSON(t *testing.T) {
	m := abiMessageJSON([]any{}, "transfer", true, map[string]any{"type": 3}, "0x12345678")
	require.Equal(t, "transfer", m["label"])
	require.Equal(t, true, m["mutates"])
	require.Equal(t, "0x12345678", m["selector"])
	require.Equal(t, false, m["payable"])
	require.Equal(t, false, m["default"])
}

func TestWriteABI(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "meta.json")

	err := writeABI(out, "my_contract", nil, nil, nil, nil)
	require.NoError(t, err)

	raw, err := os.ReadFile(out)
	require.NoError(t, err)
	require.True(t, bytes.HasSuffix(raw, []byte("\n")))

	// 输出必须是合法 JSON 且包含合约名
	var root map[string]any
	require.NoError(t, json.Unmarshal(raw, &root))
	contract := root["contract"].(map[string]any)
	require.Equal(t, "my_contract", contract["name"])
}

func TestInitAbi(t *testing.T) {
	raw := []byte(`{"contract":{"name":"c","version":"1"},"spec":{},"types":[]}`)
	abi, err := InitAbi(raw)
	require.NoError(t, err)
	require.Equal(t, "c", abi.Contract.Name)

	_, err = InitAbi([]byte("not json"))
	require.Error(t, err)
}
