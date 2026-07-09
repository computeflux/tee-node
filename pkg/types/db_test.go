package types

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/model"
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

	initv := model.Tx{
		Caller:     []byte("test-caller"),
		CallerType: 1,
		Call:       []byte("test-call"),
		Signature:  []byte("test-signature"),
	}

	key := "tx" + fmt.Sprint(time.Now().Unix())
	err = DBSetProtoMessage(SECRET_DB, "", key, &initv)
	if err != nil {
		t.Error(err)
	}

	err = DBSetProtoMessage(SECRET_DB, "", "tx", &initv)
	if err != nil {
		t.Error(err)
	}

	returnValue, err := DBGetProtoMessage[model.Tx](SECRET_DB, "", key)
	if err != nil {
		t.Error(err)
	}

	if returnValue.CallerType != 1 {
		t.Error("value not equal")
	}

	list, _, err := DBGetProtoMessageList[model.Tx](SECRET_DB, "", "tx")
	if err != nil {
		t.Error(err)
	}

	fmt.Println(list)
}

func Test(t *testing.T) {
}
