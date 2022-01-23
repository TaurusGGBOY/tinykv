package standalone_storage

import (
	"github.com/Connor1996/badger"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
)

type KvReader struct {
	db    *badger.DB
	txn *badger.Txn
	iter *engine_util.BadgerIterator
}

func NewKvReader(d *badger.DB) *KvReader {
	return &KvReader{
		db:    d,
	}
}

func (r *KvReader) GetCF(cf string, key []byte) ([]byte, error) {
	txn := r.db.NewTransaction(false)
	defer txn.Discard()
	val, err := engine_util.GetCFFromTxn(txn, cf, key)
	if err == badger.ErrKeyNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (r *KvReader) IterCF(cf string) engine_util.DBIterator {
	r.txn = r.db.NewTransaction(false)
	r.iter = engine_util.NewCFIterator(cf, r.txn)
	return r.iter
}

func (r *KvReader) Close() {
	r.iter.Close()
	r.txn.Discard()
}
