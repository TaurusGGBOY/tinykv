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
	"errors"
	pb "github.com/pingcap-incubator/tinykv/proto/pkg/eraftpb"
	"math/rand"
	"time"
)

// None is a placeholder node ID used when there is no leader.
const None uint64 = 0

// StateType represents the role of a node in a cluster.
type StateType uint64

const (
	StateFollower StateType = iota
	StateCandidate
	StateLeader
)

var stmap = [...]string{
	"StateFollower",
	"StateCandidate",
	"StateLeader",
}

func (st StateType) String() string {
	return stmap[uint64(st)]
}

// ErrProposalDropped is returned when the proposal is ignored by some cases,
// so that the proposer can be notified and fail fast.
var ErrProposalDropped = errors.New("raft proposal dropped")

// Config contains the parameters to start a raft.
type Config struct {
	// ID is the identity of the local raft. ID cannot be 0.
	ID uint64

	// peers contains the IDs of all nodes (including self) in the raft cluster. It
	// should only be set when starting a new raft cluster. Restarting raft from
	// previous configuration will panic if peers is set. peer is private and only
	// used for testing right now.
	peers []uint64

	// ElectionTick is the number of Node.Tick invocations that must pass between
	// elections. That is, if a follower does not receive any message from the
	// leader of current term before ElectionTick has elapsed, it will become
	// candidate and start an election. ElectionTick must be greater than
	// HeartbeatTick. We suggest ElectionTick = 10 * HeartbeatTick to avoid
	// unnecessary leader switching.
	ElectionTick int
	// HeartbeatTick is the number of Node.Tick invocations that must pass between
	// heartbeats. That is, a leader sends heartbeat messages to maintain its
	// leadership every HeartbeatTick ticks.
	HeartbeatTick int

	// Storage is the storage for raft. raft generates entries and states to be
	// stored in storage. raft reads the persisted entries and states out of
	// Storage when it needs. raft reads out the previous state and configuration
	// out of storage when restarting.
	Storage Storage
	// Applied is the last applied index. It should only be set when restarting
	// raft. raft will not return entries to the application smaller or equal to
	// Applied. If Applied is unset when restarting, raft might return previous
	// applied entries. This is a very application dependent configuration.
	Applied uint64
}

func (c *Config) validate() error {
	if c.ID == None {
		return errors.New("cannot use none as id")
	}

	if c.HeartbeatTick <= 0 {
		return errors.New("heartbeat tick must be greater than 0")
	}

	if c.ElectionTick <= c.HeartbeatTick {
		return errors.New("election tick must be greater than heartbeat tick")
	}

	if c.Storage == nil {
		return errors.New("storage cannot be nil")
	}

	return nil
}

// Progress represents a follower’s progress in the view of the leader. Leader maintains
// progresses of all followers, and sends entries to the follower based on its progress.
type Progress struct {
	Match, Next uint64
}

type Raft struct {
	id uint64

	Term uint64
	Vote uint64

	// the log
	RaftLog *RaftLog

	// log replication progress of each peers
	Prs map[uint64]*Progress

	// this peer's role
	State StateType

	// votes records
	votes map[uint64]bool

	// msgs need to send
	msgs []pb.Message

	// the leader id
	Lead uint64

	// heartbeat interval, should send
	heartbeatTimeout int
	// baseline of election interval
	electionTimeout       int
	randomElectionTimeout int
	// number of ticks since it reached last heartbeatTimeout.
	// only leader keeps heartbeatElapsed.
	heartbeatElapsed int
	// Ticks since it reached last electionTimeout when it is leader or candidate.
	// Number of ticks since it reached last electionTimeout or received a
	// valid message from current leader when it is a follower.
	electionElapsed int

	// leadTransferee is id of the leader transfer target when its value is not zero.
	// Follow the procedure defined in section 3.10 of Raft phd thesis.
	// (https://web.stanford.edu/~ouster/cgi-bin/papers/OngaroPhD.pdf)
	// (Used in 3A leader transfer)
	leadTransferee uint64

	// Only one conf change may be pending (in the log, but not yet
	// applied) at a time. This is enforced via PendingConfIndex, which
	// is set to a value >= the log index of the latest pending
	// configuration change (if any). Config changes are only allowed to
	// be proposed if the leader's applied index is greater than this
	// value.
	// (Used in 3A conf change)
	PendingConfIndex uint64
}

// newRaft return a raft peer with the given config
func newRaft(c *Config) *Raft {
	if err := c.validate(); err != nil {
		panic(err.Error())
	}
	// Your Code Here (2A).
	// TODO 2AA
	rand.Seed(time.Now().Unix())
	r := Raft{
		id:               c.ID,
		State:            StateFollower,
		electionTimeout:  c.ElectionTick,
		heartbeatTimeout: c.HeartbeatTick,
		RaftLog:          newLog(c.Storage),
	}
	hardState, _, _ := r.RaftLog.storage.InitialState()
	r.Term = hardState.GetTerm()
	r.RaftLog.committed = hardState.GetCommit()
	r.Vote = hardState.GetVote()

	r.Prs = make(map[uint64]*Progress)
	lastIndex := r.RaftLog.LastIndex()
	for _, i := range c.peers {
		r.Prs[i] = &Progress{Match: 0, Next: lastIndex + 1}
	}
	r.Prs[r.id] = &Progress{Match: 0, Next: lastIndex + 1}
	r.Prs[r.id].Match = lastIndex
	r.resetState()
	return &r
}

// sendAppend sends an append RPC with new entries (if any) and the
// current commit index to the given peer. Returns true if a message was sent.
func (r *Raft) sendAppend(to uint64) bool {
	// Your Code Here (2A).
	msg := pb.Message{
		MsgType: pb.MessageType_MsgAppend,
		Term:    r.Term,
		From:    r.id,
		To:      to,
		Commit:  r.RaftLog.committed,
	}
	msg.Index = r.Prs[to].Next - 1
	msg.LogTerm, _ = r.RaftLog.Term(msg.Index)

	for i := r.Prs[to].Next; i <= r.RaftLog.LastIndex(); i++ {
		msg.Entries = append(msg.Entries, r.RaftLog.getByIndex(i))
	}
	r.msgs = append(r.msgs, msg)
	return true
}

func (r *Raft) sendAppendResponse(to uint64, index uint64, reject bool) {
	// Your Code Here (2A).
	msg := pb.Message{
		MsgType: pb.MessageType_MsgAppendResponse,
		Term:    r.Term,
		From:    r.id,
		To:      to,
		Index:   index,
		Reject:  reject,
	}
	r.msgs = append(r.msgs, msg)
}

// sendHeartbeat sends a heartbeat RPC to the given peer.
func (r *Raft) sendHeartbeat(to uint64) {
	// Your Code Here (2A).
	msg := pb.Message{
		MsgType: pb.MessageType_MsgHeartbeat,
		To:      to,
		From:    r.id,
		Term:    r.Term,
		Commit:  r.RaftLog.committed,
	}
	r.msgs = append(r.msgs, msg)
}

func (r *Raft) sendHeartbeatResponse(to uint64, reject bool) {
	// Your Code Here (2A).
	msg := pb.Message{
		MsgType: pb.MessageType_MsgHeartbeatResponse,
		Term:    r.Term,
		From:    r.id,
		To:      to,
		Reject:  reject,
	}
	r.msgs = append(r.msgs, msg)
}

func (r *Raft) sendAllHeartbeat() {
	r.resetState()
	for id, _ := range r.Prs {
		if id == r.id {
			continue
		}
		r.sendHeartbeat(id)
	}
}

func (r *Raft) sendAllAppendEntries() {
	r.resetState()
	for id, _ := range r.Prs {
		if id == r.id {
			continue
		}
		r.sendAppend(id)
	}
}

func (r *Raft) sendAllRequestVote() {
	for id, _ := range r.Prs {
		if id == r.id {
			continue
		}
		r.sendRequestVoteToPeer(id)
	}
}

func (r *Raft) sendRequestVoteToPeer(to uint64) {
	msg := pb.Message{
		MsgType: pb.MessageType_MsgRequestVote,
		To:      to,
		From:    r.id,
		Term:    r.Term,
		Index:   r.RaftLog.LastIndex(),
	}
	msg.LogTerm, _ = r.RaftLog.Term(msg.Index)
	r.msgs = append(r.msgs, msg)
}

func (r *Raft) sendVoteResponse(to uint64, reject bool) {
	msg := pb.Message{
		MsgType: pb.MessageType_MsgRequestVoteResponse,
		To:      to,
		From:    r.id,
		Term:    r.Term,
		Reject:  reject,
	}
	r.msgs = append(r.msgs, msg)
}

// tick advances the internal logical clock by a single tick.
func (r *Raft) tick() {
	// Your Code Here (2A).
	switch r.State {
	case StateFollower:
		r.tickElection()
	case StateCandidate:
		r.tickElection()
	case StateLeader:
		r.tickHeartbeat()
	}
}

func (r *Raft) tickElection() {
	r.electionElapsed++
	if r.electionElapsed >= r.randomElectionTimeout {
		r.electionElapsed = 0
		msg := pb.Message{MsgType: pb.MessageType_MsgHup}
		r.Step(msg)
	}
}

func (r *Raft) tickHeartbeat() {
	r.heartbeatElapsed++
	if r.heartbeatElapsed >= r.heartbeatTimeout {
		r.heartbeatElapsed = 0
		msg := pb.Message{MsgType: pb.MessageType_MsgBeat}
		r.Step(msg)
	}
}

// becomeFollower transform this peer's state to Follower
func (r *Raft) becomeFollower(term uint64, lead uint64) {
	// Your Code Here (2A).\
	r.resetState()
	r.Lead = lead
	r.Term = term
	r.State = StateFollower
	r.Vote = None
}

// becomeCandidate transform this peer's state to candidate
func (r *Raft) becomeCandidate() {
	// Your Code Here (2A).
	r.resetState()
	r.Term++
	r.State = StateCandidate
	r.votes = make(map[uint64]bool)
	r.votes[r.id] = true
	r.Vote = r.id
	if len(r.Prs) == 1 {
		r.becomeLeader()
		return
	}
}

func (r *Raft) startElection() {
	r.becomeCandidate()
	r.sendAllRequestVote()
}

// becomeLeader transform this peer's state to leader
func (r *Raft) becomeLeader() {
	// Your Code Here (2A).
	// NOTE: Leader should propose a noop entry on its term
	r.resetState()
	r.State = StateLeader
	r.Lead = r.id
	msg1 := pb.Message{
		From:    r.id,
		To:      r.id,
		MsgType: pb.MessageType_MsgPropose,
		Entries: []*pb.Entry{{Term: r.Term, Index: r.RaftLog.LastIndex() + 1}},
	}
	r.Step(msg1)
	//msg2 := pb.Message{MsgType: pb.MessageType_MsgBeat}
	//r.Step(msg2)
}

// Step the entrance of handle message, see `MessageType`
// on `eraftpb.proto` for what msgs should be handled
func (r *Raft) Step(m pb.Message) error {
	// Your Code Here (2A).
	switch r.State {
	case StateFollower:
		return r.followerHandler(m)
	case StateCandidate:
		return r.candidateHandler(m)
	case StateLeader:
		return r.leaderHandler(m)
	}
	return nil
}

func (r *Raft) followerHandler(m pb.Message) error {
	switch m.MsgType {
	case pb.MessageType_MsgHup:
		r.startElection()
	case pb.MessageType_MsgRequestVote:
		r.handleRequestVote(m)
	case pb.MessageType_MsgHeartbeat:
		r.handleHeartbeat(m)
	case pb.MessageType_MsgAppend:
		r.handleAppendEntries(m)
	default:
		break
	}
	return nil
}

func (r *Raft) candidateHandler(m pb.Message) error {
	switch m.MsgType {
	case pb.MessageType_MsgHup:
		r.startElection()
	case pb.MessageType_MsgRequestVote:
		r.handleRequestVote(m)
	case pb.MessageType_MsgRequestVoteResponse:
		r.handleResponseVote(m)
	case pb.MessageType_MsgHeartbeat:
		r.handleHeartbeat(m)
	case pb.MessageType_MsgAppend:
		r.handleAppendEntries(m)
	case pb.MessageType_MsgPropose:
	default:
		break
	}
	return nil
}

func (r *Raft) leaderHandler(m pb.Message) error {
	switch m.MsgType {
	case pb.MessageType_MsgHup:
	case pb.MessageType_MsgRequestVote:
		r.handleRequestVote(m)
	case pb.MessageType_MsgHeartbeat:
		r.handleHeartbeat(m)
	case pb.MessageType_MsgBeat:
		r.sendAllHeartbeat()
	case pb.MessageType_MsgAppend:
		r.handleAppendEntries(m)
	case pb.MessageType_MsgAppendResponse:
		r.handleAppendEntriesResponse(m)
	case pb.MessageType_MsgPropose:
		r.handlePropose(m)
	case pb.MessageType_MsgHeartbeatResponse:
		r.handHeartbeatResponse(m)
	default:
		break
	}
	return nil
}

// handleAppendEntries handle AppendEntries RPC request
func (r *Raft) handleAppendEntries(m pb.Message) {
	// Your Code Here (2A).
	if m.GetTerm() < r.Term {
		r.sendAppendResponse(m.GetFrom(), None, true)
		return
	}

	if m.GetTerm() >= r.Term {
		r.becomeFollower(m.GetTerm(), m.GetFrom())
	}

	if m.GetIndex() > r.RaftLog.LastIndex() {
		r.sendAppendResponse(m.GetFrom(), r.RaftLog.LastIndex(), true)
		return
	}

	if term, _ := r.RaftLog.Term(m.GetIndex()); term != m.GetLogTerm() {
		r.sendAppendResponse(m.GetFrom(), m.GetIndex()-1, true)
		return
	}

	// 更新stable为最新有效的 要么有些失效stabled需要回退 要么他就不动
	for _, entry := range m.Entries {
		if entry.Index < r.RaftLog.first {
			continue
		}
		if entry.GetIndex() > r.RaftLog.LastIndex() {
			r.RaftLog.entries = append(r.RaftLog.entries, *entry)
			continue
		}
		term, _ := r.RaftLog.Term(entry.GetIndex())
		if term != entry.GetTerm() {
			r.RaftLog.entries[r.RaftLog.getIndex(entry.GetIndex())] = *entry
			r.RaftLog.entries = r.RaftLog.entries[:r.RaftLog.getIndex(entry.GetIndex())+1]
			r.RaftLog.stabled = min(r.RaftLog.stabled, entry.GetIndex()-1)
		}
	}

	if m.GetCommit() > r.RaftLog.committed {
		r.RaftLog.committed = min(m.GetCommit(), m.GetIndex()+uint64(len(m.Entries)))
	}
	r.sendAppendResponse(m.GetFrom(), r.RaftLog.LastIndex(), false)
}

func (r *Raft) handHeartbeatResponse(m pb.Message) {
	if r.Term > m.GetTerm() {
		return
	}
	if r.Term < m.GetTerm() {
		r.becomeFollower(m.GetTerm(), None)
		return
	}
	if !m.Reject {
		r.sendAppend(m.GetFrom())
	}
}

func (r *Raft) handleAppendEntriesResponse(m pb.Message) {
	// Your Code Here (2A).
	if r.Term > m.GetTerm() {
		return
	}
	if m.Reject {
		r.Prs[m.GetFrom()].Next = r.Prs[m.GetFrom()].Match + 1
		r.sendAppend(m.GetFrom())
		return
	}
	if r.Term < m.GetTerm() {
		r.becomeFollower(m.GetTerm(), m.GetFrom())
		return
	}

	if r.Prs[m.GetFrom()].Match <= m.GetIndex() {
		r.Prs[m.GetFrom()].Match = m.GetIndex()
		r.Prs[m.GetFrom()].Next = r.Prs[m.GetFrom()].Match + 1
		r.PushUpCommit()
	}
}

func (r *Raft) PushUpCommit() {
	shouldBeat := false
	for i := r.RaftLog.committed + 1; i <= r.RaftLog.LastIndex(); i++ {
		cnt := 0
		for _, progress := range r.Prs {
			if progress.Match >= i {
				cnt++
			}
		}
		if cnt <= len(r.Prs)/2 {
			break
		}
		term, _ := r.RaftLog.Term(i)
		if term == r.Term {
			r.RaftLog.committed = i
			shouldBeat = true
		}
	}
	if shouldBeat {
		r.sendAllAppendEntries()
	}
}

// handleHeartbeat handle Heartbeat RPC request
func (r *Raft) handleHeartbeat(m pb.Message) {
	// Your Code Here (2A).
	if r.Term > m.GetTerm() {
		r.sendHeartbeatResponse(m.GetFrom(), true)
		return
	}
	if r.Term <= m.GetTerm() {
		r.becomeFollower(m.GetTerm(), m.GetFrom())
	}
	if r.RaftLog.LastIndex() < m.GetCommit() {
		r.sendHeartbeatResponse(m.GetFrom(), false)
	}
	r.resetState()
}

// handleSnapshot handle Snapshot RPC request
func (r *Raft) handleSnapshot(m pb.Message) {
	// Your Code Here (2C).
}

func (r *Raft) handleRequestVote(m pb.Message) {
	if r.Term > m.GetTerm() {
		r.sendVoteResponse(m.GetFrom(), true)
		return
	}

	if r.Term < m.GetTerm() {
		r.becomeFollower(m.GetTerm(), None)
	}

	// 选举限制
	lastIndex := r.RaftLog.LastIndex()
	lastTerm, _ := r.RaftLog.Term(lastIndex)
	if m.GetLogTerm() < lastTerm || (m.GetLogTerm() == lastTerm && m.GetIndex() < lastIndex) {
		r.sendVoteResponse(m.GetFrom(), true)
		return
	}

	// vote for fromId
	if r.Vote == m.GetFrom() || r.Vote == None {
		r.Vote = m.GetFrom()
		r.resetState()
		r.sendVoteResponse(m.GetFrom(), false)
		return
	}
	r.sendVoteResponse(m.GetFrom(), true)
}

func (r *Raft) handleResponseVote(m pb.Message) {
	if r.State != StateCandidate {
		return
	}
	if r.Term < m.GetTerm() {
		r.becomeFollower(m.GetTerm(), None)
		return
	}
	if r.Term > m.GetTerm() {
		return
	}

	if m.Reject {
		r.votes[m.GetFrom()] = false
	} else {
		r.votes[m.GetFrom()] = true
	}

	cnt := 0
	for _, vote := range r.votes {
		if vote {
			cnt++
		}
	}
	if cnt > len(r.Prs)/2 {
		r.becomeLeader()
		return
	}

	// 如果就算剩下的人全投赞成，并且加上现在的赞成票，还是小于等于一半，就直接终止吧
	if (len(r.Prs)-len(r.votes))+cnt <= len(r.Prs)/2 {
		r.becomeFollower(r.Term, None)
		return
	}
}

func (r *Raft) handlePropose(m pb.Message) {
	for _, entry := range m.Entries {
		entry.Term = r.Term
		entry.Index = r.RaftLog.LastIndex() + 1
		r.RaftLog.entries = append(r.RaftLog.entries, *entry)
	}
	r.Prs[r.id].Match = r.RaftLog.LastIndex()
	r.Prs[r.id].Next = r.Prs[r.id].Match + 1
	r.PushUpCommit()
	r.sendAllAppendEntries()
}

// addNode add a new node to raft group
func (r *Raft) addNode(id uint64) {
	// Your Code Here (3A).
}

// removeNode remove a node from raft group
func (r *Raft) removeNode(id uint64) {
	// Your Code Here (3A).
}

func (r *Raft) resetState() {
	r.electionElapsed = 0
	r.heartbeatElapsed = 0
	r.randomElectionTimeout = r.electionTimeout + rand.Intn(r.electionTimeout)
}
