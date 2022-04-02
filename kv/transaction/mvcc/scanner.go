package mvcc

import (
	"bytes"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
)

// Scanner is used for reading multiple sequential key/value pairs from the storage layer. It is aware of the implementation
// of the storage layer and returns results suitable for users.
// Invariant: either the scanner is finished and cannot be used, or it is ready to return a value immediately.
type Scanner struct {
	// Your Data Here (4C).
	nextKey []byte
	txn     *MvccTxn
	iter    engine_util.DBIterator
}

// NewScanner creates a new scanner ready to read from the snapshot in txn.
func NewScanner(startKey []byte, txn *MvccTxn) *Scanner {
	// Your Code Here (4C).
	return &Scanner{nextKey: startKey, txn: txn, iter: txn.Reader.IterCF(engine_util.CfWrite)}
}

func (scan *Scanner) Close() {
	// Your Code Here (4C).
	scan.iter.Close()
}

// Next returns the next key/value pair from the scanner. If the scanner is exhausted, then it will return `nil, nil, nil`.
func (scan *Scanner) Next() ([]byte, []byte, error) {
	// Your Code Here (4C).
	scan.iter.Seek(EncodeKey(scan.nextKey, scan.txn.StartTS))
	if !scan.iter.Valid() || scan.nextKey == nil {
		return nil, nil, nil
	}
	item := scan.iter.Item()
	keyCopy := item.KeyCopy(nil)
	key := DecodeUserKey(keyCopy)

	// 如果第一个的key就不一样 说明啥 说明就没有这个key 应该返回他后面的那个key
	if !bytes.Equal(key, scan.nextKey) {
		scan.nextKey = key
		return scan.Next()
	}

	// 找nextKey
	for scan.iter.Next(); scan.iter.Valid(); scan.iter.Next() {
		nextItem := scan.iter.Item()
		nextKeyCopy := nextItem.KeyCopy(nil)
		nextKey := DecodeUserKey(nextKeyCopy)
		if !bytes.Equal(nextKey, scan.nextKey) {
			scan.nextKey = nextKey
			break
		}
	}

	if !scan.iter.Valid() {
		scan.nextKey = nil
	}

	valueCopy, _ := item.ValueCopy(nil)
	write, _ := ParseWrite(valueCopy)
	if write.Kind == WriteKindDelete {
		return key, nil, nil
	}
	value, _ := scan.txn.Reader.GetCF(engine_util.CfDefault, EncodeKey(key, write.StartTS))
	return key, value, nil
}
