package model

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/cometbft/cometbft/abci/types"
	"github.com/pkg/errors"
)

func TestSetGet(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	_, err := Get("test")
	if !errors.Is(err, pebble.ErrNotFound) {
		t.Error("except ErrNotFound")
	}

	Set("test", []byte("test"))
	v, err := Get("test")
	if err != nil {
		t.Error(err)
	}
	if string(v) != "test" {
		t.Error("value not equal")
	}
}

func TestSetGetKey(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	_, err := GetKey("GG", "test")
	if !errors.Is(err, pebble.ErrNotFound) {
		t.Error("except ErrNotFound")
	}

	SetKey("GG", "test", []byte("test"))
	v, err := GetKey("GG", "test")
	if err != nil {
		t.Error(err)
	}

	if string(v) != "test" {
		t.Error("value not equal")
	}
}

func TestGetProtoMessage(t *testing.T) {
	os.RemoveAll(dbPath)
	NewDB()
	defer DBINS.Close()

	initv := types.ValidatorUpdate{Power: 10000}

	key := "validator" + fmt.Sprint(time.Now().Unix())
	err := SetProtoMessage("", key, &initv)
	if err != nil {
		t.Error(err)
	}

	err = SetProtoMessage("", "val", &initv)
	if err != nil {
		t.Error(err)
	}

	returnValue, err := GetProtoMessage[types.ValidatorUpdate]("", key)
	if err != nil {
		t.Error(err)
	}

	if returnValue.Power != 10000 {
		t.Error("value not equal")
	}

	list, _, err := GetProtoMessageList[types.ValidatorUpdate]("", "validator")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(list)
}

func Test(t *testing.T) {
}
