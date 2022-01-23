package standalone_storage

import (
	"github.com/Connor1996/badger"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
)

type KvReader struct {
	iters []engine_util.DBIterator
	db    *badger.DB
}

func NewKvReader(d *badger.DB) *KvReader {
	return &KvReader{
		iters: make([]engine_util.DBIterator, 0),
		db:    d,
	}
}

func (r *KvReader) GetCF(cf string, key []byte) ([]byte, error) {
	txn := r.db.NewTransaction(false)
	defer txn.Discard()
	val, err := engine_util.GetCFFromTxn(txn, cf, key)
	if err != nil && err != badger.ErrKeyNotFound {
		return nil, err
	}
	return val, nil
}

func (r *KvReader) IterCF(cf string) engine_util.DBIterator {
	txn := r.db.NewTransaction(false)
	defer txn.Discard()
	iter := engine_util.NewCFIterator(cf, txn)
	r.iters = append(r.iters, iter)
	return iter
}

func (r *KvReader) Close() {
	for i := range r.iters {
		r.iters[i].Close()
	}
}
