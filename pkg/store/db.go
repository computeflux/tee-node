package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/cockroachdb/pebble"
	"github.com/computeflux/tee-provider/pkg/model/protoio"
	"github.com/computeflux/tee-provider/tee-chain/contracts/store"
	"github.com/cosmos/gogoproto/proto"
	ecrypto "github.com/edgelesssys/ego/ecrypto"
)

var SECRET_DB *DB

const (
	DBPath = "./token_chain_data/wetee"
)

// Sealer abstracts how values are sealed/unsealed before being written to pebble.
type Sealer interface {
	Encrypt(plaintext []byte) ([]byte, error)
	Decrypt(ciphertext []byte) ([]byte, error)
}

// ProductSealer seals values with the TEE product key.
// It is used for the application DB so that only enclaves of the same product
// can decrypt the data.
type ProductSealer struct {
	additionalData []byte
}

func (p *ProductSealer) Encrypt(plaintext []byte) ([]byte, error) {
	return ecrypto.SealWithProductKey(plaintext, p.additionalData)
}

func (p *ProductSealer) Decrypt(ciphertext []byte) ([]byte, error) {
	return ecrypto.Unseal(ciphertext, p.additionalData)
}

// DB wraps a pebble DB and a Sealer.
type DB struct {
	*pebble.DB
	sealer Sealer
}

// OpenDB opens a pebble DB at path with the given sealer. It does not set any global singleton.
func OpenDB(sealer Sealer, path string) (*DB, error) {
	db, err := pebble.Open(path, &pebble.Options{})
	if err != nil {
		return nil, err
	}
	return &DB{DB: db, sealer: sealer}, nil
}

// NewDB opens the application DB at the default path sealed with the product key and sets SECRET_DB.
func NewDB() (*DB, error) {
	return NewDBWithPath(DBPath)
}

// NewDBWithPath opens the application DB at the given path sealed with the product key and sets SECRET_DB.
func NewDBWithPath(path string) (*DB, error) {
	db, err := OpenDB(&ProductSealer{}, path)
	if err != nil {
		return nil, err
	}
	SECRET_DB = db
	return SECRET_DB, nil
}

func (db *DB) Seal(plaintext []byte) ([]byte, error) {
	return db.sealer.Encrypt(plaintext)
}

func (db *DB) Unseal(ciphertext []byte) ([]byte, error) {
	return db.sealer.Decrypt(ciphertext)
}

// --- Methods on *DB: the primary API, no global singleton needed ---

func (db *DB) Set(key string, value []byte) error {
	return db.SetKey("", key, value)
}

func (db *DB) Get(key string) ([]byte, error) {
	return db.GetKey("", key)
}

func (db *DB) SetKey(namespace, key string, value []byte) error {
	k := []byte(comboKey(namespace, key))
	val, err := db.Seal(value)
	if err != nil {
		return err
	}
	return db.DB.Set(k, val, pebble.Sync)
}

func (db *DB) GetKey(namespace, key string) ([]byte, error) {
	k := []byte(comboKey(namespace, key))
	value, closer, err := db.DB.Get(k)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return db.Unseal(value)
}

func (db *DB) GetFromByteKey(key []byte) ([]byte, error) {
	value, closer, err := db.DB.Get(key)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return db.Unseal(value)
}

func (db *DB) DeleteKey(namespace, key string) error {
	return db.DB.Delete([]byte(comboKey(namespace, key)), pebble.Sync)
}

func (db *DB) DeleteByteKey(key []byte) error {
	return db.DB.Delete(key, pebble.Sync)
}

// --- Generic helpers operating on an explicit *DB ---
// Go does not support generic methods, so these are package-level functions
// that take the DB as their first argument.

func DBSetJson[T any](db *DB, namespace, key string, val *T) error {
	bt, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return db.SetKey(namespace, key, bt)
}

func DBGetJson[T any](db *DB, namespace, key string) (*T, error) {
	v, err := db.GetKey(namespace, key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if len(v) == 0 {
		return nil, nil
	}
	val := new(T)
	if err := json.Unmarshal(v, val); err != nil {
		return nil, err
	}
	return val, nil
}

func DBGetJsonList[T any](db *DB, namespace, key string) ([]*T, [][]byte, error) {
	rkey := []byte(comboKey(namespace, key))
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: rkey,
		UpperBound: keyUpperBound(rkey),
	})
	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()

	var keys [][]byte
	var list []*T
	for iter.First(); iter.Valid(); iter.Next() {
		v := iter.Value()
		value, err := db.Unseal(v)
		if err != nil {
			return nil, nil, err
		}
		val := new(T)
		if err := json.Unmarshal(value, val); err != nil {
			return nil, nil, fmt.Errorf("unmarshal db entry: %w", err)
		}
		// iter.Key() 指向内部缓冲区，需深拷贝后再保存
		keys = append(keys, copyIterKey(iter.Key()))
		list = append(list, val)
	}

	return list, keys, nil
}

func DBSetCodec[T any](db *DB, namespace, key string, val T) error {
	bt, err := store.ScaleEncode(val)
	if err != nil {
		return err
	}
	return db.SetKey(namespace, key, bt)
}

func DBGetCodec[T any](db *DB, namespace, key string) (*T, error) {
	bt, err := db.GetKey(namespace, key)
	if err != nil {
		if errors.Is(err, pebble.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	val := new(T)
	if err := store.ScaleDecode(val, bt); err != nil {
		return nil, err
	}
	return val, nil
}

func DBGetCodecOrDefault[T any](db *DB, namespace, key string, defaultVal T) (T, error) {
	val, err := DBGetCodec[T](db, namespace, key)
	if err != nil {
		var zero T
		return zero, err
	}
	if val == nil {
		return defaultVal, nil
	}
	return *val, nil
}

func DBGetProtoMessage[T any](db *DB, namespace, key string) (*T, error) {
	v, err := db.GetKey(namespace, key)
	if err != nil {
		return nil, err
	}
	if len(v) == 0 {
		return nil, nil
	}
	val := new(T)
	if err := protoio.ReadMessage(bytes.NewBuffer(v), val); err != nil {
		return nil, err
	}
	return val, nil
}

func DBSetProtoMessage[T proto.Message](db *DB, namespace, key string, value T) error {
	buf := new(bytes.Buffer)
	if err := protoio.WriteMessage(value, buf); err != nil {
		return err
	}
	return db.SetKey(namespace, key, buf.Bytes())
}

func DBGetProtoMessageList[T any](db *DB, namespace, key string) ([]*T, [][]byte, error) {
	rkey := []byte(comboKey(namespace, key))
	iter, err := db.NewIter(&pebble.IterOptions{
		LowerBound: rkey,
		UpperBound: keyUpperBound(rkey),
	})
	if err != nil {
		return nil, nil, err
	}
	defer iter.Close()

	keys := make([][]byte, 0, 100)
	list := make([]*T, 0, 100)
	for iter.First(); iter.Valid(); iter.Next() {
		v := iter.Value()
		// iter.Key() 指向内部缓冲区，需深拷贝后再保存
		keys = append(keys, copyIterKey(iter.Key()))
		value, err := db.Unseal(v)
		if err != nil {
			return nil, nil, err
		}
		val := new(T)
		if err := protoio.ReadMessage(bytes.NewBuffer(value), val); err != nil {
			// Log but don't fail the whole query; one corrupt entry
			// shouldn't block reading of others.
			fmt.Printf("GetProtoMessageList: skip corrupt entry at key %x: %v\n", iter.Key(), err)
			continue
		}
		list = append(list, val)
	}

	return list, keys, nil
}
