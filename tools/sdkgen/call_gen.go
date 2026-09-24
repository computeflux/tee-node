// 调用层生成：每个合约一个包，calls.go 提供 go-ink-gen/dao 风格的调用方法
// （只读 QueryX；变更 DryRunX / ExecX / CallOfX），实际收发由 chain 包完成。
package main

import (
	"bytes"
	"strings"
	"text/template"
)

// ContractCallBox calls.go 模板数据。
type ContractCallBox struct {
	PackageName string // 包名（= 合约目录名）
	Name        string // 合约客户端类型名（如 C2C）
	Contract    string // 链上合约名（如 c2c）
	Funcs       []Func
}

// Func 一个 message 的调用形态。
type Func struct {
	Selector   string
	FuncName   string // ABI label（snake_case，如 release）
	ArgStr     string // 实参名列表（逗号结尾，空则空串）
	ArgTypeStr string // 形参列表（逗号结尾，空则空串）
	Return     string // 只读方法返回类型（变更方法不用）
	IsMut      bool
}

func callGen(box ContractCallBox) (string, error) {
	t, err := template.New("call").Funcs(template.FuncMap{
		"CamelCase": func(v string) string {
			parts := strings.Split(v, "::")
			for i := range parts {
				parts[i] = UnderscoreToCamelCase(parts[i])
			}
			return strings.Join(parts, "")
		},
	}).Parse(callTemp)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, box); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// callTemp 生成 calls.go 正文（import 由 goFile 按引用推导）。
//
// 调用形态与 go-ink-gen（dao）一致，差别在链特性：
//   - 只读：QueryX（contractDryRun mut=false，返回已解包 Result 的裸值）
//   - 变更：DryRunX（mut=true 预演，不落链、不消耗 nonce）/ ExecX（提交并确认）/
//     CallOfX（构造签名交易但不提交，供自行广播或转交中继）
//
// 参数顺序与 dao 一致：合约参数在前，params 在最后。
var callTemp = `// {{.Name}}ContractName 链上合约名（contractDryRun / 交易路由用）。
const {{.Name}}ContractName = "{{.Contract}}"

// Init{{.Name}}Contract 绑定链上 {{.Contract}} 合约。
func Init{{.Name}}Contract(client *chain.ChainClient) *{{.Name}} {
	return &{{.Name}}{ChainClient: client, Name: {{.Name}}ContractName}
}

// {{.Name}} {{.Contract}} 合约调用客户端（实现 chain.Ink）。
type {{.Name}} struct {
	// ChainClient tee-chain 客户端（查询 / 提交，见 chain 包）。
	ChainClient *chain.ChainClient
	// Name 链上合约名（默认 {{.Name}}ContractName）。
	Name string
}

// Client 返回链客户端（chain.Ink）。
func (c *{{.Name}}) Client() *chain.ChainClient { return c.ChainClient }

// ContractName 返回链上合约名（chain.Ink）。
func (c *{{.Name}}) ContractName() string { return c.Name }
{{ range .Funcs }}
{{- if .IsMut }}
// DryRun{{CamelCase .FuncName}} 预演 {{.FuncName}}：contractDryRun(mut=true)，只执行不落链、
// 不消耗 nonce；合约业务错误（状态校验失败等）在此带出，可用于提交前校验。
func (c *{{$.Name}}) DryRun{{CamelCase .FuncName}}({{.ArgTypeStr}} params chain.DryRunParams) error {
	return chain.DryRunInk(c, params, chain.ContractInput{
		Selector: "{{.Selector}}",
		Args:     []any{ {{.ArgStr}} },
	})
}

// Exec{{CamelCase .FuncName}} 提交 {{.FuncName}}：查询 nonce → 签名器签名 →
// broadcast_tx_sync → 轮询确认；被链拒绝时用同一次调用重放取合约错误。
func (c *{{$.Name}}) Exec{{CamelCase .FuncName}}({{.ArgTypeStr}} params chain.ExecParams) error {
	return chain.CallInk(c, params, chain.ContractInput{
		Selector: "{{.Selector}}",
		Args:     []any{ {{.ArgStr}} },
	})
}

// CallOf{{CamelCase .FuncName}} 构造 {{.FuncName}} 交易（查询 nonce + 签名），不提交；
// 返回的 Tx 可自行 chain.Broadcast，或转交中继 / 批量提交。
func (c *{{$.Name}}) CallOf{{CamelCase .FuncName}}({{.ArgTypeStr}} params chain.ExecParams) (*chain.Tx, error) {
	return chain.CallOfInk(c, params, chain.ContractInput{
		Selector: "{{.Selector}}",
		Args:     []any{ {{.ArgStr}} },
	})
}
{{- else }}
// Query{{CamelCase .FuncName}} 查询 {{.FuncName}}：contractDryRun(mut=false)，只读、无需签名。
func (c *{{$.Name}}) Query{{CamelCase .FuncName}}({{.ArgTypeStr}} params chain.DryRunParams) ({{.Return}}, error) {
	return chain.QueryInk[{{.Return}}](c, params, chain.ContractInput{
		Selector: "{{.Selector}}",
		Args:     []any{ {{.ArgStr}} },
	})
}
{{- end }}
{{ end }}`
