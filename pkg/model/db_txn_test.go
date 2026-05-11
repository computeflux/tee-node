package model

import (
	"errors"
	"os"
	"testing"

	"github.com/cockroachdb/pebble"
)

func TestTxnDelete(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	tx := DBINS.NewTransaction()

	tx.Set([]byte("222222"), []byte("test"))
	v4, err := tx.Get([]byte("222222"))
	if err != nil {
		t.Error(err)
	}
	if string(v4) != "test" {
		t.Error("value not equal")
	}

	tx.Set([]byte("test1"), []byte("test"))
	v, err := tx.Get([]byte("test1"))
	if err != nil {
		t.Error(err)
	}
	if string(v) != "test" {
		t.Error("value not equal")
	}

	tx.Set([]byte("test2"), []byte("test2"))
	v2, err := tx.Get([]byte("test2"))
	if err != nil {
		t.Error(err)
	}

	if string(v2) != "test2" {
		t.Error("value not equal")
	}

	tx.DeletekeysByPrefix([]byte("test"))

	_, err = tx.Get([]byte("test"))
	if !errors.Is(err, pebble.ErrNotFound) {
		t.Error(err)
	}

	tx.Set([]byte("222222"), []byte("test"))
	v5, err := tx.Get([]byte("222222"))
	if err != nil {
		t.Error(err)
	}
	if string(v5) != "test" {
		t.Error("value not equal")
	}
}
