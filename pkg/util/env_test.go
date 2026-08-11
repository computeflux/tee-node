package util

import (
	"os"
	"testing"
)

func TestGetEnv(t *testing.T) {
	// Test when environment variable is set
	os.Setenv("TEST_KEY", "test_value")
	expected := "test_value"
	result := GetEnv("TEST_KEY", "default_value")
	if result != expected {
		t.Errorf("GetEnv() = %s; expected %s", result, expected)
	}

	// Test when environment variable is not set
	os.Unsetenv("TEST_KEY")
	expected = "default_value"
	result = GetEnv("TEST_KEY", "default_value")
	if result != expected {
		t.Errorf("GetEnv() = %s; expected %s", result, expected)
	}
}

func TestGetEnvInt_Valid(t *testing.T) {
	os.Setenv("TEST_INT_KEY", "42")
	defer os.Unsetenv("TEST_INT_KEY")
	if got := GetEnvInt("TEST_INT_KEY", 7); got != 42 {
		t.Errorf("GetEnvInt() = %d, want 42", got)
	}
}

func TestGetEnvInt_Missing(t *testing.T) {
	os.Unsetenv("TEST_INT_KEY_MISSING")
	if got := GetEnvInt("TEST_INT_KEY_MISSING", 7); got != 7 {
		t.Errorf("GetEnvInt() = %d, want default 7", got)
	}
}

func TestGetEnvInt_Invalid(t *testing.T) {
	// 非法值不应 panic，应回退到默认值
	os.Setenv("TEST_INT_KEY_BAD", "not-a-number")
	defer os.Unsetenv("TEST_INT_KEY_BAD")
	if got := GetEnvInt("TEST_INT_KEY_BAD", 7); got != 7 {
		t.Errorf("GetEnvInt() = %d, want default 7", got)
	}
}

func TestGetEnvInt_Negative(t *testing.T) {
	os.Setenv("TEST_INT_KEY_NEG", "-5")
	defer os.Unsetenv("TEST_INT_KEY_NEG")
	if got := GetEnvInt("TEST_INT_KEY_NEG", 7); got != -5 {
		t.Errorf("GetEnvInt() = %d, want -5", got)
	}
}
