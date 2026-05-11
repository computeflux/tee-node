package model

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPersistChan(t *testing.T) {
	os.RemoveAll("./chain_data")
	db, err := NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed store.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	key := "test_persist_chan"
	cacheLen := uint16(1000)

	pc, err := NewPersistChan[string](key, cacheLen)
	if err != nil {
		t.Fatalf("Failed to create PersistChan: %v", err)
	}

	// Test Push
	for i := range 2 {
		err = pc.Push("test message " + fmt.Sprint(i))
		if err != nil {
			t.Fatalf("Failed to push message: %v", err)
		}
	}

	// Test Start
	pc.Start(func(msg string) error {
		fmt.Println(msg)
		t.SkipNow()
		return nil
	})
}

func TestPersistChanLoad(t *testing.T) {
	// os.RemoveAll("./chain_data")
	db, err := NewDB()
	if err != nil {
		require.NoErrorf(t, err, "failed store.InitDB")
		os.Exit(1)
	}
	defer db.Close()

	key := "test_persist_chan"
	cacheLen := uint16(1000)

	pc, err := NewPersistChan[string](key, cacheLen)
	if err != nil {
		t.Fatalf("Failed to create PersistChan: %v", err)
	}

	// Test Start
	pc.Start(func(msg string) error {
		fmt.Println(msg)
		t.SkipNow()
		return nil
	})
}
