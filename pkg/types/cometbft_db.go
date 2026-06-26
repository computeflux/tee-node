package types

import (
	"fmt"

	dbm "github.com/cometbft/cometbft-db"
	cfg "github.com/cometbft/cometbft/config"
)

// encryptedDB wraps a dbm.DB and seals all values with the TEE product key.
// Keys remain plaintext; only values are sealed.
// The underlying seal is provided by ProductSealer (ecrypto.SealWithProductKey),
// so only enclaves of the same product can unseal the data.
type encryptedDB struct {
	base   dbm.DB
	sealer *ProductSealer
}

// NewEncryptedDB creates a sealed DB wrapper backed by the given dbm.DB.
// Values are sealed with ProductSealer (TEE product key) before being written.
func NewEncryptedDB(base dbm.DB) (*encryptedDB, error) {
	return &encryptedDB{base: base, sealer: &ProductSealer{}}, nil
}

func (db *encryptedDB) Get(key []byte) ([]byte, error) {
	value, err := db.base.Get(key)
	if err != nil {
		return nil, err
	}
	if value == nil {
		return nil, nil
	}
	return db.decryptValue(value)
}

func (db *encryptedDB) Has(key []byte) (bool, error) {
	return db.base.Has(key)
}

func (db *encryptedDB) Set(key []byte, value []byte) error {
	enc, err := db.encryptValue(value)
	if err != nil {
		return err
	}
	return db.base.Set(key, enc)
}

func (db *encryptedDB) SetSync(key []byte, value []byte) error {
	enc, err := db.encryptValue(value)
	if err != nil {
		return err
	}
	return db.base.SetSync(key, enc)
}

func (db *encryptedDB) Delete(key []byte) error {
	return db.base.Delete(key)
}

func (db *encryptedDB) DeleteSync(key []byte) error {
	return db.base.DeleteSync(key)
}

func (db *encryptedDB) Iterator(start, end []byte) (dbm.Iterator, error) {
	itr, err := db.base.Iterator(start, end)
	if err != nil {
		return nil, err
	}
	return &encryptedIterator{base: itr, sealer: db.sealer}, nil
}

func (db *encryptedDB) ReverseIterator(start, end []byte) (dbm.Iterator, error) {
	itr, err := db.base.ReverseIterator(start, end)
	if err != nil {
		return nil, err
	}
	return &encryptedIterator{base: itr, sealer: db.sealer}, nil
}

func (db *encryptedDB) Close() error {
	return db.base.Close()
}

func (db *encryptedDB) NewBatch() dbm.Batch {
	return &encryptedBatch{
		base:   db.base.NewBatch(),
		sealer: db.sealer,
	}
}

func (db *encryptedDB) Print() error {
	return db.base.Print()
}

func (db *encryptedDB) Stats() map[string]string {
	return db.base.Stats()
}

func (db *encryptedDB) Compact(start, end []byte) error {
	return db.base.Compact(start, end)
}

func (db *encryptedDB) encryptValue(value []byte) ([]byte, error) {
	return db.sealer.Encrypt(value)
}

func (db *encryptedDB) decryptValue(value []byte) ([]byte, error) {
	plaintext, err := db.sealer.Decrypt(value)
	if err != nil {
		return nil, err
	}
	if plaintext == nil {
		plaintext = []byte{}
	}
	return plaintext, nil
}

type encryptedIterator struct {
	base   dbm.Iterator
	sealer *ProductSealer
}

func (itr *encryptedIterator) Domain() ([]byte, []byte) {
	return itr.base.Domain()
}

func (itr *encryptedIterator) Valid() bool {
	return itr.base.Valid()
}

func (itr *encryptedIterator) Next() {
	itr.base.Next()
}

func (itr *encryptedIterator) Key() []byte {
	return itr.base.Key()
}

func (itr *encryptedIterator) Value() []byte {
	value := itr.base.Value()
	if value == nil {
		return nil
	}
	plaintext, err := itr.sealer.Decrypt(value)
	if err != nil {
		// Iterator contract doesn't allow returning error from Value().
		// We panic to surface decryption failure immediately.
		panic(fmt.Errorf("encrypted iterator decrypt value for key %x: %w", itr.base.Key(), err))
	}
	return plaintext
}

func (itr *encryptedIterator) Error() error {
	return itr.base.Error()
}

func (itr *encryptedIterator) Close() error {
	return itr.base.Close()
}

type encryptedBatch struct {
	base   dbm.Batch
	sealer *ProductSealer
}

func (b *encryptedBatch) Set(key []byte, value []byte) error {
	enc, err := b.sealer.Encrypt(value)
	if err != nil {
		return err
	}
	return b.base.Set(key, enc)
}

func (b *encryptedBatch) Delete(key []byte) error {
	return b.base.Delete(key)
}

func (b *encryptedBatch) Write() error {
	return b.base.Write()
}

func (b *encryptedBatch) WriteSync() error {
	return b.base.WriteSync()
}

func (b *encryptedBatch) Close() error {
	return b.base.Close()
}

// EncryptedDBProvider returns a CometBFT DBProvider that wraps the default database
// with a TEE product-key sealing layer.
//
// The provider opens the underlying database via cfg.DefaultDBProvider and then
// returns a sealed wrapper. CometBFT uses this provider for block store,
// state store and evidence store.
func EncryptedDBProvider() func(*cfg.DBContext) (dbm.DB, error) {
	return func(ctx *cfg.DBContext) (dbm.DB, error) {
		base, err := cfg.DefaultDBProvider(ctx)
		if err != nil {
			return nil, fmt.Errorf("open base db %s: %w", ctx.ID, err)
		}

		db, err := NewEncryptedDB(base)
		if err != nil {
			_ = base.Close()
			return nil, fmt.Errorf("wrap encrypted db %s: %w", ctx.ID, err)
		}
		return db, nil
	}
}
