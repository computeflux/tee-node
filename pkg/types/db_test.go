package types

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
	os.RemoveAll(DBPath)
	db, err := NewDB()
	if err != nil {
		t.Fatal(err)
	}
	_ = db
	defer SECRET_DB.Close()

	_, err = SECRET_DB.Get("test")
	if !errors.Is(err, pebble.ErrNotFound) {
		t.Error("except ErrNotFound")
	}

	SECRET_DB.Set("test", []byte("test"))
	v, err := SECRET_DB.Get("test")
	if err != nil {
		t.Error(err)
	}
	if string(v) != "test" {
		t.Error("value not equal")
	}
}

func TestSetGetKey(t *testing.T) {
	os.RemoveAll(DBPath)
	db, err := NewDB()
	if err != nil {
		t.Fatal(err)
	}
	_ = db
	defer SECRET_DB.Close()

	_, err = SECRET_DB.GetKey("GG", "test")
	if !errors.Is(err, pebble.ErrNotFound) {
		t.Error("except ErrNotFound")
	}

	SECRET_DB.SetKey("GG", "test", []byte("test"))
	v, err := SECRET_DB.GetKey("GG", "test")
	if err != nil {
		t.Error(err)
	}

	if string(v) != "test" {
		t.Error("value not equal")
	}
}

func TestGetProtoMessage(t *testing.T) {
	os.RemoveAll(DBPath)
	db, err := NewDB()
	if err != nil {
		t.Fatal(err)
	}
	_ = db
	defer SECRET_DB.Close()

	initv := types.ValidatorUpdate{Power: 10000}

	key := "validator" + fmt.Sprint(time.Now().Unix())
	err = DBSetProtoMessage(SECRET_DB, "", key, &initv)
	if err != nil {
		t.Error(err)
	}

	err = DBSetProtoMessage(SECRET_DB, "", "val", &initv)
	if err != nil {
		t.Error(err)
	}

	returnValue, err := DBGetProtoMessage[types.ValidatorUpdate](SECRET_DB, "", key)
	if err != nil {
		t.Error(err)
	}

	if returnValue.Power != 10000 {
		t.Error("value not equal")
	}

	list, _, err := DBGetProtoMessageList[types.ValidatorUpdate](SECRET_DB, "", "validator")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(list)
}

func Test(t *testing.T) {
}
