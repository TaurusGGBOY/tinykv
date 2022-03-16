package server

import (
	"context"
	"github.com/pingcap-incubator/tinykv/kv/storage"
	"github.com/pingcap-incubator/tinykv/proto/pkg/kvrpcpb"
)

// The functions below are Server's Raw API. (implements TinyKvServer).
// Some helper methods can be found in sever.go in the current directory

// RawGet return the corresponding Get response based on RawGetRequest's CF and Key fields
func (server *Server) RawGet(_ context.Context, req *kvrpcpb.RawGetRequest) (*kvrpcpb.RawGetResponse, error) {
	// Your Code Here (1).
	r, err := server.storage.Reader(nil)
	if err != nil {
		return nil, err
	}
	val, err := r.GetCF(req.Cf, req.Key)

	if err != nil {
		return nil, err
	}

	if val == nil {
		return &kvrpcpb.RawGetResponse{NotFound: true}, nil
	}
	return &kvrpcpb.RawGetResponse{Value: val}, nil
}

// RawPut puts the target data into storage and returns the corresponding response
func (server *Server) RawPut(_ context.Context, req *kvrpcpb.RawPutRequest) (*kvrpcpb.RawPutResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be modified
	batch := []storage.Modify{{storage.Put{Key: req.Key, Value: req.Value, Cf: req.Cf}}}
	err := server.storage.Write(nil, batch)
	return &kvrpcpb.RawPutResponse{}, err
}

// RawDelete delete the target data from storage and returns the corresponding response
func (server *Server) RawDelete(_ context.Context, req *kvrpcpb.RawDeleteRequest) (*kvrpcpb.RawDeleteResponse, error) {
	// Your Code Here (1).
	// Hint: Consider using Storage.Modify to store data to be deleted
	batch := []storage.Modify{{storage.Delete{Key: req.Key, Cf: req.Cf}}}
	err := server.storage.Write(nil, batch)
	return &kvrpcpb.RawDeleteResponse{}, err
}

// RawScan scan the data starting from the start key up to limit. and return the corresponding result
func (server *Server) RawScan(_ context.Context, req *kvrpcpb.RawScanRequest) (*kvrpcpb.RawScanResponse, error) {
	// Your Code Here (1).
	r, err := server.storage.Reader(nil)
	if err != nil {
		return nil, err
	}
	iter := r.IterCF(req.Cf)
	response := &kvrpcpb.RawScanResponse{}
	for iter.Seek(req.StartKey); iter.Valid() && len(response.Kvs)<int(req.Limit); iter.Next() {
		item := iter.Item()
		key := item.Key()
		val, err := item.Value()
		if err == nil {
			response.Kvs = append(response.Kvs, &kvrpcpb.KvPair{Key: key, Value: val})
		}
	}
	r.Close()
	return response, nil
}
