package server

import (
	"context"
	"github.com/pingcap-incubator/tinykv/kv/coprocessor"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/kv/storage/raft_storage"
	"github.com/pingcap-incubator/tinykv/kv/transaction/latches"
	"github.com/pingcap-incubator/tinykv/kv/transaction/mvcc"
	"github.com/pingcap-incubator/tinykv/kv/util/engine_util"
	coppb "github.com/pingcap-incubator/tinykv/proto/pkg/coprocessor"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
	"github.com/pingcap-incubator/tinykv/proto/pkg/tinykvpb"
	"github.com/pingcap/tidb/kv"
)

var _ tinykvpb.TinyKvServer = new(Server)

// Server is a TinyKV server, it 'faces outwards', sending and receiving messages from clients such as TinySQL.
type Server struct {
	storage storage.Storage

	// (Used in 4A/4B)
	Latches *latches.Latches

	// coprocessor API handler, out of course scope
	copHandler *coprocessor.CopHandler
}

func NewServer(storage storage.Storage) *Server {
	return &Server{
		storage: storage,
		Latches: latches.NewLatches(),
	}
}

// The below functions are Server's gRPC API (implements TinyKvServer).

// Raft commands (tinykv <-> tinykv)
// Only used for RaftStorage, so trivially forward it.
func (server *Server) Raft(stream tinykvpb.TinyKv_RaftServer) error {
	return server.storage.(*raft_storage.RaftStorage).Raft(stream)
}

// Snapshot stream (tinykv <-> tinykv)
// Only used for RaftStorage, so trivially forward it.
func (server *Server) Snapshot(stream tinykvpb.TinyKv_SnapshotServer) error {
	return server.storage.(*raft_storage.RaftStorage).Snapshot(stream)
}

// Transactional API.
func (server *Server) KvGet(_ context.Context, req *kvrpcpb.GetRequest) (*kvrpcpb.GetResponse, error) {
	// Your Code Here (4B).
	response := new(kvrpcpb.GetResponse)
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return response, err
	}
	defer reader.Close()

	txn := mvcc.NewMvccTxn(reader, req.Version)
	lock, err := txn.GetLock(req.Key)
	if lock != nil && lock.Ts <= req.Version {
		response.Error = &kvrpcpb.KeyError{
			Locked: &kvrpcpb.LockInfo{
				Key:         req.Key,
				PrimaryLock: lock.Primary,
				LockVersion: lock.Ts,
				LockTtl:     lock.Ttl,
			},
		}
		return response, nil
	}
	value, err := txn.GetValue(req.Key)
	if value == nil {
		response.NotFound = true
		return response, nil
	}
	if err != nil {
		return response, err
	}
	response.Value = value
	return response, nil
	return response, nil
}

func (server *Server) KvPrewrite(_ context.Context, req *kvrpcpb.PrewriteRequest) (*kvrpcpb.PrewriteResponse, error) {
	// Your Code Here (4B).
	response := new(kvrpcpb.PrewriteResponse)
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return response, err
	}
	defer reader.Close()
	txn := mvcc.NewMvccTxn(reader, req.StartVersion)
	// TODO mutation是什么
	for _, m := range req.Mutations {
		// 写冲突1：最近写的已经比你新了
		write, ts, _ := txn.MostRecentWrite(m.Key)
		if write != nil && req.StartVersion <= ts {
			response.Errors = append(response.Errors, &kvrpcpb.KeyError{
				Conflict: &kvrpcpb.WriteConflict{
					StartTs:    req.StartVersion,
					ConflictTs: ts,
					Key:        m.Key,
					Primary:    req.PrimaryLock,
				},
			})
			continue
		}
		var kind mvcc.WriteKind
		switch m.Op {
		case kvrpcpb.Op_Put:
			kind = mvcc.WriteKindPut
			txn.PutValue(m.Key, m.Value)
		case kvrpcpb.Op_Del:
			kind = mvcc.WriteKindDelete
			txn.DeleteValue(m.Key)
		case kvrpcpb.Op_Rollback:
			kind = mvcc.WriteKindRollback
			// Used by TinySQL but not TinyKV.
		case kvrpcpb.Op_Lock:
		default:

		}
		// 写冲突2：已经有锁了
		lock, _ := txn.GetLock(m.Key)
		if lock != nil && lock.Ts != req.StartVersion {
			response.Errors = append(response.Errors, &kvrpcpb.KeyError{Locked: &kvrpcpb.LockInfo{
				PrimaryLock: lock.Primary,
				LockVersion: lock.Ts,
				Key:         m.Key,
				LockTtl:     lock.Ttl,
			}})
			return response, err
		}
		lock = &mvcc.Lock{
			Primary: req.PrimaryLock,
			Ts:      req.GetStartVersion(),
			Ttl:     req.LockTtl,
			Kind:    kind,
		}
		txn.PutLock(m.Key, lock)
	}
	server.storage.Write(req.Context, txn.Writes())
	return response, nil
}

func (server *Server) KvCommit(_ context.Context, req *kvrpcpb.CommitRequest) (*kvrpcpb.CommitResponse, error) {
	// Your Code Here (4B).
	response := new(kvrpcpb.CommitResponse)
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return response, err
	}
	defer reader.Close()
	txn := mvcc.NewMvccTxn(reader, req.StartVersion)
	// TODO mutation是什么
	server.Latches.WaitForLatches(req.Keys)
	defer server.Latches.ReleaseLatches(req.Keys)
	for _, key := range req.Keys {
		// 应该放提交的版本
		// 如果没加锁就应该直接返回 后面不执行了
		lock, _ := txn.GetLock(key)
		if lock == nil {
			return response, nil
		}

		if lock.Ts != txn.StartTS {
			response.Error = &kvrpcpb.KeyError{Retryable: "true"}
			return response, nil
		}
		txn.PutWrite(key, req.CommitVersion, &mvcc.Write{
			StartTS: req.StartVersion,
			Kind:    mvcc.WriteKindPut,
		})
		txn.DeleteLock(key)
	}
	server.storage.Write(req.Context, txn.Writes())
	return response, nil
}

func (server *Server) KvScan(_ context.Context, req *kvrpcpb.ScanRequest) (*kvrpcpb.ScanResponse, error) {
	// Your Code Here (4C).
	response := new(kvrpcpb.ScanResponse)
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return response, err
	}
	defer reader.Close()
	txn := mvcc.NewMvccTxn(reader, req.Version)
	scanner := mvcc.NewScanner(req.StartKey, txn)
	defer scanner.Close()
	for len(response.Pairs) < int(req.Limit) {
		key, value, _ := scanner.Next()
		if key == nil {
			break
		}
		if value != nil {
			response.Pairs = append(response.Pairs, &kvrpcpb.KvPair{Key: key, Value: value})
		}
	}
	return response, nil
}

func (server *Server) KvCheckTxnStatus(_ context.Context, req *kvrpcpb.CheckTxnStatusRequest) (*kvrpcpb.CheckTxnStatusResponse, error) {
	// Your Code Here (4C).
	response := new(kvrpcpb.CheckTxnStatusResponse)
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return response, err
	}
	defer reader.Close()
	txn := mvcc.NewMvccTxn(reader, req.LockTs)
	write, ts, err := txn.CurrentWrite(req.PrimaryKey)
	if write != nil {
		response.Action = kvrpcpb.Action_NoAction
		if write.Kind != mvcc.WriteKindRollback {
			response.CommitVersion = ts
		}
		return response, nil
	}
	lock, err := txn.GetLock(req.PrimaryKey)
	if lock != nil && mvcc.PhysicalTime(lock.Ts)+lock.Ttl <= mvcc.PhysicalTime(req.CurrentTs) {
		response.Action = kvrpcpb.Action_TTLExpireRollback
		txn.PutWrite(req.PrimaryKey, req.LockTs, &mvcc.Write{StartTS: req.LockTs, Kind: mvcc.WriteKindRollback})
		txn.DeleteLock(req.PrimaryKey)
		txn.DeleteValue(req.PrimaryKey)
		server.storage.Write(req.Context, txn.Writes())
		return response, nil
	}
	if lock == nil {
		txn.PutWrite(req.PrimaryKey, req.LockTs, &mvcc.Write{StartTS: req.LockTs, Kind: mvcc.WriteKindRollback})
		server.storage.Write(req.Context, txn.Writes())
		response.Action = kvrpcpb.Action_LockNotExistRollback
		return response, nil
	}
	return response, nil
}

func (server *Server) KvBatchRollback(_ context.Context, req *kvrpcpb.BatchRollbackRequest) (*kvrpcpb.BatchRollbackResponse, error) {
	// Your Code Here (4C).
	response := new(kvrpcpb.BatchRollbackResponse)
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return response, err
	}
	defer reader.Close()
	txn := mvcc.NewMvccTxn(reader, req.StartVersion)
	for _, key := range req.Keys {
		write, ts, _ := txn.CurrentWrite(key)
		if write != nil && ts == req.StartVersion && write.Kind == mvcc.WriteKindRollback {
			return response, nil
		}
		if write != nil && ts >= req.StartVersion {
			response.Error = &kvrpcpb.KeyError{Abort: "true"}
			return response, nil
		}
		lock, _ := txn.GetLock(key)
		if lock != nil && lock.Ts == req.StartVersion {
			txn.DeleteLock(key)
			txn.DeleteValue(key)
		}
		txn.PutWrite(key, req.StartVersion, &mvcc.Write{StartTS: req.StartVersion, Kind: mvcc.WriteKindRollback})
	}
	server.storage.Write(req.Context, txn.Writes())

	return response, nil
}

func (server *Server) KvResolveLock(_ context.Context, req *kvrpcpb.ResolveLockRequest) (*kvrpcpb.ResolveLockResponse, error) {
	// Your Code Here (4C).
	response := new(kvrpcpb.ResolveLockResponse)
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		return response, err
	}
	defer reader.Close()
	//txn := mvcc.NewMvccTxn(reader, req.StartVersion)
	iter := reader.IterCF(engine_util.CfLock)
	defer iter.Close()
	keys := make([][]byte, 0)
	for ; iter.Valid(); iter.Next() {
		item := iter.Item()
		value, _ := item.ValueCopy(nil)
		lock, _ := mvcc.ParseLock(value)
		if lock != nil && lock.Ts == req.StartVersion {
			keys = append(keys, item.KeyCopy(nil))
		}
	}
	if req.CommitVersion == 0 {
		server.KvBatchRollback(nil, &kvrpcpb.BatchRollbackRequest{
			Context:      req.Context,
			StartVersion: req.StartVersion,
			Keys:         keys,
		})
	} else {
		server.KvCommit(nil, &kvrpcpb.CommitRequest{
			Context:       req.Context,
			StartVersion:  req.StartVersion,
			Keys:          keys,
			CommitVersion: req.CommitVersion,
		})
	}

	return response, nil
}

// SQL push down commands.
func (server *Server) Coprocessor(_ context.Context, req *coppb.Request) (*coppb.Response, error) {
	resp := new(coppb.Response)
	reader, err := server.storage.Reader(req.Context)
	if err != nil {
		if regionErr, ok := err.(*raft_storage.RegionError); ok {
			resp.RegionError = regionErr.RequestErr
			return resp, nil
		}
		return nil, err
	}
	switch req.Tp {
	case kv.ReqTypeDAG:
		return server.copHandler.HandleCopDAGRequest(reader, req), nil
	case kv.ReqTypeAnalyze:
		return server.copHandler.HandleCopAnalyzeRequest(reader, req), nil
	}
	return nil, nil
}
