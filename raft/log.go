// Copyright 2015 The etcd Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package raft

import (
	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
)

// RaftLog manage the log entries, its struct look like:
//
//  snapshot/first.....applied....committed....stabled.....last
//  --------|------------------------------------------------|
//                            log entries
//
// for simplify the RaftLog implement should manage all log entries
// that not truncated
type RaftLog struct {
	// storage contains all stable entries since the last snapshot.
	storage Storage

	// committed is the highest log position that is known to be in
	// stable storage on a quorum of nodes.
	committed uint64

	// applied is the highest log position that the application has
	// been instructed to apply to its state machine.
	// Invariant: applied <= committed
	applied uint64

	// log entries with index <= stabled are persisted to storage.
	// It is used to record the logs that are not persisted by storage yet.
	// Everytime handling `Ready`, the unstabled logs will be included.
	stabled uint64

	// all entries that have not yet compact.
	entries []pb.Entry

	// the incoming unstable snapshot, if any.
	// (Used in 2C)
	pendingSnapshot *pb.Snapshot

	// Your Data Here (2A).
	first uint64
}

// newLog returns log using the given storage. It recovers the log
// to the state that it just commits and applies the latest snapshot.
func newLog(storage Storage) *RaftLog {
	// Your Code Here (2A).
	l, _ := storage.FirstIndex()
	h, _ := storage.LastIndex()
	//log.Info(fmt.Sprintf("newLog %v %v", l, h))
	entries, _ := storage.Entries(l, h+1)
	// 不保存snapshot 因为都被执行了
	return &RaftLog{
		storage: storage,
		applied: l - 1,
		stabled: h,
		entries: entries,
		first:   l,
	}
}

// We need to compact the log entries in some point of time like
// storage compact stabled log entries prevent the log entries
// grow unlimitedly in memory
func (l *RaftLog) maybeCompact() {
	// Your Code Here (2C).
	first, err := l.storage.FirstIndex()
	if err != nil {
		panic(err)
	}
	if first > l.first {
		for len(l.entries) > 0 && first > l.entries[0].GetIndex() {
			l.entries = l.entries[1:]
		}
		//log.Info(fmt.Sprintf("maybe compact %v %v", l.first, first))
		l.first = first
	}
}

// unstableEntries return all the unstable entries
func (l *RaftLog) unstableEntries() []pb.Entry {
	// Your Code Here (2A).
	return l.entries[l.getIndex(l.stabled)+1:]
}

// nextEnts returns all the committed but not applied entries
func (l *RaftLog) nextEnts() (ents []pb.Entry) {
	// Your Code Here (2A).
	notApplied := l.getIndex(l.applied) + 1
	committed := l.getIndex(l.committed)
	return l.entries[notApplied : committed+1]
}

// LastIndex return the last index of the log entries
func (l *RaftLog) LastIndex() uint64 {
	// Your Code Here (2A).
	var snapshotIndex uint64
	storageIndex, _ := l.storage.LastIndex()

	if !IsEmptySnap(l.pendingSnapshot) {
		snapshotIndex = l.pendingSnapshot.GetMetadata().GetIndex()
	}
	if len(l.entries) > 0 {
		return max(snapshotIndex, l.entries[len(l.entries)-1].GetIndex())
	}
	return max(storageIndex, snapshotIndex)
}

// Term return the term of the entry in the given index
func (l *RaftLog) Term(i uint64) (uint64, error) {
	// Your Code Here (2A).
	if i >= l.first {
		return l.entries[l.getIndex(i)].Term, nil
	}
	term, err := l.storage.Term(i)
	if err == ErrUnavailable && !IsEmptySnap(l.pendingSnapshot) {
		if i == l.pendingSnapshot.Metadata.Index {
			term = l.pendingSnapshot.Metadata.Term
			err = nil
		} else {
			err = ErrCompacted
		}
	}
	return term, err
}

func (l *RaftLog) getIndex(index uint64) uint64 {
	//firstStorage, _ := l.storage.FirstIndex()
	//lastStorage, _ := l.storage.LastIndex()
	//
	//log.Info(fmt.Sprintf("firstStorage:%v, i:%v l.first:%v, lastIndex:%v, applied:%v", firstStorage, index, l.first, lastStorage, l.applied))

	return index - l.first
}

func (l *RaftLog) getByIndex(index uint64) *pb.Entry {
	return &l.entries[l.getIndex(index)]
}

func (l *RaftLog) toGlobalIndex(index uint64) uint64 {
	return index + l.first
}
