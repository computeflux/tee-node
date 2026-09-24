// sdkgen 从 ink metadata（ABI JSON）生成 tee-chain 合约的 Go SDK：
// 每个合约一个目录（calls.go / types.go），共享一个 chain 运行时包，
// 调用形态与 go-ink-gen 生成的 dao 一致（Query / DryRun / Exec / CallOf）。
//
// 输出（-out 根目录）：
//
//	<out>/<chain-dir>/client.go        运行时：ChainClient（GraphQL 查询/预演 + CometBFT 提交）、参数、Tx
//	<out>/<chain-dir>/codec.go         SCALE 参数编码 / protobuf wire / 签名原文
//	<out>/<chain-dir>/signer.go        签名器接口（可插拔）+ 调用方类型
//	<out>/<chain-dir>/signer_builtin.go  内置 ed25519（calltype 3）/ sr25519（calltype 1）签名器
//	<out>/<合约>/calls.go              合约调用（QueryX / DryRunX / ExecX / CallOfX）
//	<out>/<合约>/types.go              合约类型（SCALE 编解码，类型映射沿用 go-ink-gen）
//
// 用法：
//
//	sdkgen -abi c2c.json     -out ./sdk -client C2C
//	sdkgen -abi gateway.json -out ./sdk -client Gateway -no-chain
//
// ABI 由 contractgen -abi-out 生成（ink metadata v6）。
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// genFile 一个待写出的生成文件。
type genFile struct {
	name string
	doc  string
	body string
}

func main() {
	abiPath := flag.String("abi", "", "ink metadata ABI JSON 路径（必填）")
	out := flag.String("out", ".", "输出根目录：<out>/<合约>/{calls.go,types.go} 与 <out>/<chain-dir>/*.go")
	client := flag.String("client", "", "合约客户端类型名（默认合约名转驼峰；如 c2c 合约用 C2C）")
	contract := flag.String("contract", "", "链上合约名（默认取 ABI 的 contract.name 小写）")
	pkg := flag.String("pkg", "", "合约包名（默认由合约名推导）")
	chainDir := flag.String("chain-dir", "chain", "共享运行时目录名（package chain）")
	chainImport := flag.String("chain-import", "", "chain 包 import 路径（默认按 go.mod 自动推导）")
	noChain := flag.Bool("no-chain", false, "不生成共享运行时（同输出目录已生成过时用）")
	noSigners := flag.Bool("no-signers", false, "不生成内置签名器（ed25519 / sr25519），仅保留 Signer 接口")
	flag.Parse()

	if *abiPath == "" {
		flag.Usage()
		os.Exit(1)
	}

	err := run(*abiPath, *out, *client, *contract, *pkg, *chainDir, *chainImport, *noChain, *noSigners)
	if err != nil {
		fmt.Fprintln(os.Stderr, "sdkgen:", err)
		os.Exit(1)
	}
}

func run(abiPath, out, client, contract, pkg, chainDir, chainImport string, noChain, noSigners bool) error {
	data, err := os.ReadFile(abiPath)
	if err != nil {
		return err
	}
	g, err := NewReviveGen(data)
	if err != nil {
		return fmt.Errorf("parse abi: %w", err)
	}

	contractName := contract
	if contractName == "" {
		contractName = strings.ToLower(g.Abi.Contract.Name)
	}
	pkgName := pkg
	if pkgName == "" {
		pkgName = pkgNameOf(contractName)
	}
	clientName := client
	if clientName == "" {
		clientName = UnderscoreToCamelCase(contractName)
	}

	funcs, err := g.Parse()
	if err != nil {
		return err
	}
	if g.Names[clientName] {
		return fmt.Errorf("合约类型名 %s 与客户端类型名冲突，请用 -client 指定其他名字", clientName)
	}

	chainPath, err := resolveChainImport(out, chainDir, chainImport)
	if err != nil {
		return err
	}

	callsBody, err := callGen(ContractCallBox{
		PackageName: pkgName,
		Name:        clientName,
		Contract:    contractName,
		Funcs:       funcs,
	})
	if err != nil {
		return err
	}

	// 合约目录：types.go（纯类型）+ calls.go（调用层，引用 chain 包）
	contractDir := filepath.Join(out, pkgName)
	if err := writeGoFile(filepath.Join(contractDir, "types.go"), pkgName, "", g.TypesBody(), nil); err != nil {
		return err
	}
	extra := map[string]importSpec{"chain.": {Path: chainPath, Alias: "chain"}}
	if err := writeGoFile(filepath.Join(contractDir, "calls.go"), pkgName, "", callsBody, extra); err != nil {
		return err
	}

	if noChain {
		return nil
	}

	chainFiles := []genFile{
		{name: "client.go", doc: chainPkgDoc, body: chainClientSrc},
		{name: "codec.go", body: chainCodecSrc},
		{name: "signer.go", body: chainSignerSrc},
	}
	if !noSigners {
		chainFiles = append(chainFiles, genFile{name: "signer_builtin.go", body: chainSignerBuiltinSrc})
	}
	for _, f := range chainFiles {
		path := filepath.Join(out, chainDir, f.name)
		if err := writeGoFile(path, "chain", f.doc, renderSource(f.body), nil); err != nil {
			return err
		}
	}
	return nil
}
