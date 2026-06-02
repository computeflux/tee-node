// buildwasm 生成运行时 WASM 文件。
//
// 由于当前 Go 合约依赖 PebbleDB(CGO)，无法直接用 GOOS=wasip1 GOARCH=wasm 编译。
// 此工具生成一个有效的最小 WASM 模块，包含：
//   - 导出函数 version() -> i32（返回运行时版本号）
//   - custom section "runtime_meta"（存储版本元数据：源码 hash、ABI、描述等）
//
// 用法:
//   go run tools/buildwasm/main.go -version 0 -contracts ./tee-chain/contracts -abis ./tee-chain/abis -out ./token_chain_data/runtime_v0.wasm
package main

import (
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

// leb128EncodeUint32 将 uint32 编码为 unsigned LEB128
func leb128EncodeUint32(v uint32) []byte {
	var buf []byte
	for {
		b := byte(v & 0x7f)
		v >>= 7
		if v != 0 {
			b |= 0x80
		}
		buf = append(buf, b)
		if v == 0 {
			break
		}
	}
	return buf
}

// leb128EncodeUint32AsVaruint 用于 section size（最大 5 字节）
func leb128EncodeUint32Size(v uint32) []byte {
	return leb128EncodeUint32(v)
}

// buildWasm 构造 WASM 字节码
func buildWasm(version uint32, metaJSON []byte) []byte {
	// WASM 魔数 + 版本
	wasm := []byte{0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00}

	// === Type section (id=1) ===
	// 1 type: () -> i32
	typeSection := []byte{
		0x01,       // num types
		0x60,       // func type
		0x00,       // num params
		0x01, 0x7f, // num results: 1 x i32
	}
	wasm = append(wasm, 0x01) // section id
	wasm = append(wasm, leb128EncodeUint32(uint32(len(typeSection)))...)
	wasm = append(wasm, typeSection...)

	// === Function section (id=3) ===
	funcSection := []byte{0x01, 0x00} // 1 function, type index 0
	wasm = append(wasm, 0x03)
	wasm = append(wasm, leb128EncodeUint32(uint32(len(funcSection)))...)
	wasm = append(wasm, funcSection...)

	// === Export section (id=7) ===
	exportName := []byte("version")
	exportSection := append([]byte{0x01}, leb128EncodeUint32(uint32(len(exportName)))...)
	exportSection = append(exportSection, exportName...)
	exportSection = append(exportSection, 0x00, 0x00) // kind=func, index=0
	wasm = append(wasm, 0x07)
	wasm = append(wasm, leb128EncodeUint32(uint32(len(exportSection)))...)
	wasm = append(wasm, exportSection...)

	// === Code section (id=10) ===
	// body: i32.const <version>, end
	funcBody := []byte{
		0x00, // local count
		0x41, // i32.const
	}
	funcBody = append(funcBody, leb128EncodeUint32(version)...) // version as signed leb128
	funcBody = append(funcBody, 0x0b)                           // end

	// body 用 body_size 包裹
	bodyWithSize := append(leb128EncodeUint32(uint32(len(funcBody))), funcBody...)
	codeSection := append([]byte{0x01}, bodyWithSize...)
	wasm = append(wasm, 0x0a)
	wasm = append(wasm, leb128EncodeUint32(uint32(len(codeSection)))...)
	wasm = append(wasm, codeSection...)

	// === Custom section (id=0): runtime_meta ===
	customName := []byte("runtime_meta")
	customPayload := append(leb128EncodeUint32(uint32(len(customName))), customName...)
	customPayload = append(customPayload, metaJSON...)
	wasm = append(wasm, 0x00)
	wasm = append(wasm, leb128EncodeUint32(uint32(len(customPayload)))...)
	wasm = append(wasm, customPayload...)

	return wasm
}

// hashDir 递归计算目录下所有 .go 文件的 SHA256 hash
func hashDir(dir string) (string, map[string]string, error) {
	h := sha256.New()
	files := make(map[string]string)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".go" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		fh := sha256.Sum256(data)
		files[rel] = fmt.Sprintf("%x", fh)
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", nil, err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), files, nil
}

// hashAbis 计算 ABI 目录下所有 .json 文件的 hash
func hashAbis(dir string) (string, map[string]string, error) {
	h := sha256.New()
	files := make(map[string]string)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		fh := sha256.Sum256(data)
		files[rel] = fmt.Sprintf("%x", fh)
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", nil, err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), files, nil
}

type metadata struct {
	Version     uint32            `json:"version"`
	CodeHash    string            `json:"code_hash"`
	AbiHash     string            `json:"abi_hash"`
	SourceFiles map[string]string `json:"source_files"`
	AbiFiles    map[string]string `json:"abi_files"`
	Description string            `json:"description"`
}

func main() {
	var (
		version     = flag.Uint("version", 0, "runtime version number")
		contractsDir = flag.String("contracts", "./tee-chain/contracts", "contracts source directory")
		abisDir     = flag.String("abis", "./tee-chain/abis", "ABI json directory")
		output      = flag.String("out", "./token_chain_data/runtime_v0.wasm", "output wasm file path")
		description = flag.String("description", "", "runtime description")
	)
	flag.Parse()

	// 计算合约源码 hash
	codeHash, sourceFiles, err := hashDir(*contractsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash contracts dir: %v\n", err)
		os.Exit(1)
	}

	// 计算 ABI hash
	abiHash, abiFiles, err := hashAbis(*abisDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "hash abis dir: %v\n", err)
		os.Exit(1)
	}

	// 构造元数据
	meta := metadata{
		Version:     uint32(*version),
		CodeHash:    codeHash,
		AbiHash:     abiHash,
		SourceFiles: sourceFiles,
		AbiFiles:    abiFiles,
		Description: *description,
	}
	metaJSON, err := json.Marshal(meta)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal metadata: %v\n", err)
		os.Exit(1)
	}

	// 生成 WASM
	wasmBytes := buildWasm(uint32(*version), metaJSON)

	// 确保输出目录存在
	outDir := filepath.Dir(*output)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir: %v\n", err)
		os.Exit(1)
	}

	// 写入文件
	if err := os.WriteFile(*output, wasmBytes, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write wasm: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Generated: %s\n", *output)
	fmt.Printf("  Version:  %d\n", *version)
	fmt.Printf("  CodeHash: %s\n", codeHash)
	fmt.Printf("  AbiHash:  %s\n", abiHash)
	fmt.Printf("  Size:     %d bytes\n", len(wasmBytes))
	fmt.Printf("  Source files: %d\n", len(sourceFiles))
	fmt.Printf("  ABI files:    %d\n", len(abiFiles))
}
