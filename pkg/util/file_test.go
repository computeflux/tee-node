package util

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsFileExists_Existing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "exists.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !IsFileExists(path) {
		t.Errorf("IsFileExists(%s) = false, want true", path)
	}
}

func TestIsFileExists_Missing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.txt")
	if IsFileExists(path) {
		t.Errorf("IsFileExists(%s) = true, want false", path)
	}
}

func TestIsFileExists_EmptyPath(t *testing.T) {
	if IsFileExists("") {
		t.Error("IsFileExists(\"\") = true, want false")
	}
}
