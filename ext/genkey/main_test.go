package main

import (
	"os"
	"path/filepath"
	"testing"
)

// TestMainGeneratesKeyFiles 在临时目录中运行 main()，验证生成三个密钥文件。
func TestMainGeneratesKeyFiles(t *testing.T) {
	// t.Chdir 在 Go 1.24+ 可用；main() 使用相对路径写文件
	t.Chdir(t.TempDir())

	main()

	for _, f := range []string{"node_key.json", "priv_validator_key.json", "priv_validator_state.json"} {
		info, err := os.Stat(f)
		if err != nil {
			t.Fatalf("expected %s to be created: %v", f, err)
		}
		if info.Size() == 0 {
			t.Fatalf("%s is empty", f)
		}
	}
}

// TestMainIdempotent 重复运行 main() 不应 panic（node_key.json 已存在时走 Load 路径）。
func TestMainIdempotent(t *testing.T) {
	t.Chdir(t.TempDir())

	main()
	main()

	if _, err := os.Stat("node_key.json"); err != nil {
		t.Fatalf("node_key.json missing after rerun: %v", err)
	}
	// priv_validator_state.json 包含高度等状态，重跑后文件应仍是合法 JSON
	raw, err := os.ReadFile(filepath.Join(".", "priv_validator_state.json"))
	if err != nil {
		t.Fatalf("read priv_validator_state.json: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("priv_validator_state.json empty")
	}
}
