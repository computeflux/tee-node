package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"unicode"

	"golang.org/x/crypto/blake2b"
)

// 本文件 ABI JSON 字段布局与 wrevive/crates/wrevive-macro/src/abi.rs 中 emit_abi 生成的 ink 风格一致：
// messages 项含 args / default / docs / label / mutates / payable / returnType / selector；
// selector 为 4 字节十六进制字符串（见 selectorHex）。

type abiTypeRef struct {
	Display []string
	TypeID  int
}

// contractNameGoSafe 将合约名中的 '-' 替换为 '_'，与 abi.rs contract_name_go_safe 一致（避免下游生成非法标识符）。
func contractNameGoSafe(name string) string {
	return strings.ReplaceAll(name, "-", "_")
}

// selectorHex 将 4 字节 selector 格式化为带 0x 前缀的十六进制，与 abi.rs selector_hex 一致。
func selectorHex(sel [4]byte) string {
	return fmt.Sprintf("0x%02x%02x%02x%02x", sel[0], sel[1], sel[2], sel[3])
}

// pickSelectorInk 由 label + mutates 派生 selector，与旧版 contractgen（无模板时）行为一致。
func pickSelectorInk(abiLabel string, mutates bool) string {
	var buf strings.Builder
	fmt.Fprintf(&buf, "wetee/contractgen/v1:%s:mutates=%v", abiLabel, mutates)
	h := blake2b.Sum256([]byte(buf.String()))
	var sel [4]byte
	copy(sel[:], h[:4])
	return selectorHex(sel)
}

// pickSelectorInkBytes 返回 selector 的 4 字节数组形式
func pickSelectorInkBytes(abiLabel string, mutates bool) [4]byte {
	var buf strings.Builder
	fmt.Fprintf(&buf, "wetee/contractgen/v1:%s:mutates=%v", abiLabel, mutates)
	h := blake2b.Sum256([]byte(buf.String()))
	var sel [4]byte
	copy(sel[:], h[:4])
	return sel
}

// writeABI 在内存中构建 ink metadata v6 并写入 -abi-out。
func writeABI(outPath, contractName string, mutMethods, qMethods []*methodSig, errVariants []errorVariant, structTypes map[string]structType) error {
	root, err := buildInkRoot(contractName, mutMethods, qMethods, errVariants, structTypes)
	if err != nil {
		return err
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, append(out, '\n'), 0o644)
}

// abiMessageJSON 生成与 abi.rs messages.push 结构一致的单个 message 对象。
func abiMessageJSON(args []any, label string, mutates bool, returnType map[string]any, selector string) map[string]any {
	return map[string]any{
		"args":       args,
		"default":    false,
		"docs":       []any{},
		"label":      label,
		"mutates":    mutates,
		"payable":    false,
		"returnType": returnType,
		"selector":   selector,
	}
}

func paramABIName(name string, idx int) string {
	if name == "" {
		return fmt.Sprintf("arg%d", idx)
	}
	return camelToSnake(name)
}

func camelToSnake(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
