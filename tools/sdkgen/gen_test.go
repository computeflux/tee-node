package main

import (
	"strings"
	"testing"
)

// testABI 最小 ABI：一个只读方法（返回 Result<Cfg, Err>）+ 两个变更方法，
// 类型表里含同名（UniAddr）重复注册 —— 复刻 contractgen 对 gateway ABI 里
// UniAddr type 13 / 72 的处理。dupV 用于切换第二个 UniAddr 的布局（是否与第一个一致）。
func testABI(dupV int) string {
	return `{
  "contract": {"name": "tst", "version": "0.1.0"},
  "spec": {
    "constructors": [],
    "messages": [
      {
        "label": "get_cfg", "mutates": false, "selector": "0x0b8cefa6",
        "args": [{"label": "who", "type": {"displayName": ["UniAddr"], "type": 13}}],
        "returnType": {"displayName": ["ink", "MessageResult"], "type": 11}
      },
      {
        "label": "set_cfg", "mutates": true, "selector": "0x0b78c824",
        "args": [{"label": "type", "type": {"displayName": ["Vec"], "type": 14}}],
        "returnType": {"displayName": ["ink", "MessageResult"], "type": 11}
      },
      {
        "label": "del_cfg", "mutates": true, "selector": "0x27dbde64",
        "args": [{"label": "addr", "type": {"displayName": ["UniAddr"], "type": 72}}],
        "returnType": {"displayName": ["ink", "MessageResult"], "type": 11}
      }
    ]
  },
  "types": [
    {"id": 0,  "type": {"def": {"primitive": "u8"}}},
    {"id": 2,  "type": {"def": {"primitive": "u32"}}},
    {"id": 3,  "type": {"def": {"primitive": "u64"}}},
    {"id": 11, "type": {"path": ["Result"], "def": {"variant": {"variants": [
      {"name": "Ok", "index": 0, "fields": [{"name": "", "type": 40}]},
      {"name": "Err", "index": 1, "fields": [{"name": "", "type": 41}]}
    ]}}}},
    {"id": 12, "type": {"def": {"sequence": {"type": 0}}}},
    {"id": 13, "type": {"path": ["UniAddr"], "def": {"composite": {"fields": [
      {"name": "T", "type": 2}, {"name": "V", "type": 12}
    ]}}}},
    {"id": 14, "type": {"def": {"sequence": {"type": 0}}}},
    	{"id": 15, "type": {"def": {"sequence": {"type": 0}}}},
    {"id": 16, "type": {"path": ["string"], "def": {"sequence": {"type": 0}}}},
    {"id": 40, "type": {"path": ["Cfg"], "def": {"composite": {"fields": [
      {"name": "a", "type": 3}, {"name": "name", "type": 16}
    ]}}}},
    {"id": 41, "type": {"path": ["Err"], "def": {"variant": {"variants": [
      {"name": "Bad", "index": 0, "fields": []}
    ]}}}},
    {"id": 72, "type": {"path": ["UniAddr"], "def": {"composite": {"fields": [
      {"name": "T", "type": 2}, {"name": "V", "type": ` + itoa(dupV) + `}
    ]}}}}
  ]
}`
}

func itoa(v int) string {
	if v == 12 {
		return "12"
	}
	return "3"
}

func newTestGen(t *testing.T, abi string) *ReviveGen {
	t.Helper()
	g, err := NewReviveGen([]byte(abi))
	if err != nil {
		t.Fatalf("NewReviveGen: %v", err)
	}
	return g
}

// TestParse 校验调用列表：selector / 参数名（关键字加后缀）/ 只读返回值解包 Result。
func TestParse(t *testing.T) {
	g := newTestGen(t, testABI(12))
	funcs, err := g.Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(funcs) != 3 {
		t.Fatalf("funcs = %d, want 3", len(funcs))
	}

	get := funcs[0]
	if get.Selector != "0x0b8cefa6" || get.FuncName != "get_cfg" || get.IsMut {
		t.Errorf("query func = %+v", get)
	}
	if get.ArgTypeStr != "who UniAddr," || get.ArgStr != "who," {
		t.Errorf("query args = %q / %q", get.ArgTypeStr, get.ArgStr)
	}
	if get.Return != "Cfg" {
		t.Errorf("query return = %q, want Cfg（Result 的 Ok 侧）", get.Return)
	}

	set := funcs[1]
	if !set.IsMut {
		t.Errorf("set_cfg 应为变更方法: %+v", set)
	}
	// ABI 参数名为 Go 关键字 type → 加下划线后缀
	if set.ArgTypeStr != "type_ []byte," || set.ArgStr != "type_," {
		t.Errorf("mutation args = %q / %q", set.ArgTypeStr, set.ArgStr)
	}
}

// TestParseDedupeByName 同名类型（不同 type id）只生成一次；同名不同布局报错。
func TestParseDedupeByName(t *testing.T) {
	g := newTestGen(t, testABI(12))
	funcs, err := g.Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	_ = funcs
	body := g.TypesBody()
	if n := strings.Count(body, "type UniAddr struct"); n != 1 {
		t.Errorf("UniAddr 生成 %d 次，want 1:\n%s", n, body)
	}
	if !strings.Contains(body, "type Cfg struct") {
		t.Errorf("缺少 Cfg 类型:\n%s", body)
	}

	// 布局不同的同名类型无法共存
	g = newTestGen(t, testABI(3))
	if _, err := g.Parse(); err == nil {
		t.Fatal("同名不同类型布局应报错，实际通过")
	} else if !strings.Contains(err.Error(), "UniAddr") {
		t.Errorf("错误信息应提到类型名: %v", err)
	}
}

// TestInkString ink 的 String（ABI 里 path=["string"] 的 Vec<u8>）映射为 Go string。
func TestInkString(t *testing.T) {
	g := newTestGen(t, testABI(12))
	if _, err := g.Parse(); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	body := g.TypesBody()
	if !strings.Contains(body, "Name   string") {
		t.Errorf("String 字段应映射为 string:\n%s", body)
	}
	if strings.Contains(body, "Name   []byte") {
		t.Errorf("String 字段不应映射为 []byte:\n%s", body)
	}
}

// TestCallGen 校验 calls.go 正文的调用形态（Query / DryRun / Exec / CallOf）。
func TestCallGen(t *testing.T) {
	g := newTestGen(t, testABI(12))
	funcs, err := g.Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	body, err := callGen(ContractCallBox{PackageName: "tst", Name: "Tst", Contract: "tst", Funcs: funcs})
	if err != nil {
		t.Fatalf("callGen: %v", err)
	}

	for _, want := range []string{
		`const TstContractName = "tst"`,
		"func InitTstContract(client *chain.ChainClient) *Tst {",
		"func (c *Tst) QueryGetCfg(who UniAddr, params chain.DryRunParams) (Cfg, error) {",
		"chain.QueryInk[Cfg](c, params, chain.ContractInput{",
		"func (c *Tst) DryRunSetCfg(type_ []byte, params chain.DryRunParams) error {",
		"func (c *Tst) ExecSetCfg(type_ []byte, params chain.ExecParams) error {",
		"func (c *Tst) CallOfSetCfg(type_ []byte, params chain.ExecParams) (*chain.Tx, error) {",
		`Selector: "0x0b8cefa6"`,
		`Selector: "0x0b78c824"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("calls.go 缺少 %q\n%s", want, body)
		}
	}
}

// TestGoFile 校验文件组装：头部 + package + import（含 chain 别名）+ 可格式化。
func TestGoFile(t *testing.T) {
	body := "func Use(c *chain.ChainClient) types.U256 {\n\treturn types.U256{}\n}\n"
	src, err := goFile("tst", "", body, map[string]importSpec{
		"chain.": {Path: "example.com/sdk/chain", Alias: "chain"},
	})
	if err != nil {
		t.Fatalf("goFile: %v", err)
	}
	out := string(src)
	for _, want := range []string{
		"// Code generated by sdkgen; DO NOT EDIT.",
		"package tst",
		`chain "example.com/sdk/chain"`,
		`"github.com/centrifuge/go-substrate-rpc-client/v4/types"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("缺少 %q\n%s", want, out)
		}
	}
}

// TestImportBlock 校验按引用推导 import：注释里的引用不算，未引用不引入。
func TestImportBlock(t *testing.T) {
	body := "// 注释里的 types.U256 不算引用\n" +
		"var url = \"http://example.com\"\n" +
		"var n uint64 = 1\n"
	block := importBlock(body, nil)
	if strings.Contains(block, "types") {
		t.Errorf("注释里的引用不应引入 import:\n%s", block)
	}
	if strings.Contains(block, "net/http") {
		t.Errorf("字符串字面量里的 http 不应引入 import:\n%s", block)
	}
	if block != "" {
		t.Errorf("无引用时不应有 import 块:\n%s", block)
	}

	block = importBlock("var x = fmt.Sprintf(\"%d\", 1)\n", nil)
	if !strings.Contains(block, `"fmt"`) {
		t.Errorf("应引入 fmt:\n%s", block)
	}
}

// TestGoParamName ABI 参数名 → Go 形参名。
func TestGoParamName(t *testing.T) {
	cases := []struct {
		label string
		i     int
		want  string
	}{
		{"owner", 0, "owner"},
		{"code_hash", 0, "code_hash"},
		{"type", 0, "type_"},     // Go 关键字
		{"params", 0, "params_"}, // 生成方法已占用
		{"c", 0, "c_"},           // 方法接收者
		{"", 3, "arg3"},          // 无名参数
	}
	for _, c := range cases {
		if got := goParamName(c.label, c.i); got != c.want {
			t.Errorf("goParamName(%q, %d) = %q, want %q", c.label, c.i, got, c.want)
		}
	}
}

// TestPkgNameOf 合约名 → 包名。
func TestPkgNameOf(t *testing.T) {
	cases := map[string]string{
		"c2c":         "c2c",
		"gateway":     "gateway",
		"My-Contract": "my_contract",
		"9lives":      "p9lives",
	}
	for in, want := range cases {
		if got := pkgNameOf(in); got != want {
			t.Errorf("pkgNameOf(%q) = %q, want %q", in, got, want)
		}
	}
}
