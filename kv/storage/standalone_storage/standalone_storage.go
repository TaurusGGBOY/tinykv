package standalone_storage

import (
	"github.com/pingcap-incubator/tinykv/kv/config"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
)

// StandAloneStorage is an implementation of `Storage` for a single-node TinyKV instance. It does not
// communicate with other nodes and all data is stored locally.
type StandAloneStorage struct {
	// Your Data Here (1).
	e *engine_util.Engines
}

func NewStandAloneStorage(conf *config.Config) *StandAloneStorage {
	// Your Code Here (1).
	raftPath := conf.DBPath + "/raft"
	r := engine_util.CreateDB(raftPath, true)

	kvPath := conf.DBPath + "/kv"
	k := engine_util.CreateDB(kvPath, false)

	return &StandAloneStorage{
		e: engine_util.NewEngines(k, r, kvPath, raftPath),
	}
}

func (s *StandAloneStorage) Start() error {
	// Your Code Here (1).
	return nil
}

func (s *StandAloneStorage) Stop() error {
	// Your Code Here (1).
	err := s.e.Close()
	return err
}

func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storage.StorageReader, error) {
	// Your Code Here (1).
	return &KvReader{db: s.e.Kv}, nil
}

func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error {
	// Your Code Here (1).
	for i := range batch {
		b := batch[i]
		k := b.Key()
		v := b.Value()
		cf := b.Cf()
		if v == nil {
			err := engine_util.DeleteCF(s.e.Kv, cf, k)
			if err != nil {
				return err
			}
		} else {
			err := engine_util.PutCF(s.e.Kv, cf, k, v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
