# Tinykv 2C

> 实现快照

## 题目要求

+ 服务器会丢掉超过阈值的日志
+ 快照是一个类似Append的消息
+ 为了不阻塞网络用的是独立的链接
+ 分块传的Snapshot()
+ leader通过storage.snapshot生成快照
+ follower通过handleSnapshot处理
+ 要处理gc
+ gc在AdminRequest 
+ 更新元数据，安排gc工人
+ snapshot是调用region工人来打快照
+ snap_runner 快照发送和接受
+ ready的时候可以修改snapshot
+ 修改kvdb和raftdb的陈旧状态

## 流程

### 快照

+ 发现nextIndex已经被打成快照了
+ 打快照
+ 发送快照
+ 接收快照：判断快照是不是最新的
+ apply快照：

### gc

+ 收到gc消息 CompactLogRequest
+ 压缩 给自己发个snapshot？

## 需要看和修改的函数

快照：
+ sendAppend():发现已经不能发Entries只能发快照了
+ sendSnapshot：leader调用这个 发送快照给follower
+ Storage.Snapshot():调用这个函数会返回快照，也就是
+ handleSnapshot：follower调用这个来存储快照，基本走了一遍newRaft，会保存快照
+ ApplySnapshot：传进来快照要应用到两个db里面去
+ hasReady：判断一下是否有新的快照
+ raedy()：取新的快照给raedy
+ SaveReadyState：要保存snapshot到kv里面
+ HandleRaftReady：处理snapshot
+ Advanced()：会调用mapbeCompact
+ maybeCompact
+ FirstIndex()

gc：
+ handleAdminRequest
+ proposeAdminRequest
+ processAdminRequest：更改apply消息，执行compact
+ Term：快照的时候，有可能暂时不能访问……

## 数据结构
+ Snapshot：Data快照 MetaData元数据
+ SnapshotMetadata：ConfState peers信息 Index最后一个索引 Term最后一个Term
+ CompactLogRequest: index最后一个index term一个term

## ApplySnapshot

> 快照需要自己手动apply到两个db中

### 要做的事
+ 更新raftstate，applystate等
+ 通过ps.regionSched 发送regionTaskApply任务给region工人
+ ps.clearMeta and ps.clearExtraData 删除过时数据

## peerStorage 和 Raft的思考

+ 顶层是peer
+ raft是在peer.RaftGroup.raft中
+ peerStorage是在peer.storage中
+ Storage是在peer.RaftGroup.raft.RaftLog.Storage中
+ 在做6.824的时候没有这么多结构，一个Raft可以存下所有东西
+ 之前实现raft是只用考虑简单kv（lab4以前），这里要考虑分片了等原因
+ peerStorage在做的事情如下
  + 保存region信息，这个原始raft并不考虑
  + raftState信息，这个以前都存在raft吧，hardState就是论文提到必须持久化的
  + applyState：AppliedIndex这个之前是不用持久化的，因为考虑幂等性可以再apply一次就行
  + TruncatedState：截断信息，这个就是snapshotIndex和snapshotTerm
  + snapState：snapshot类型，闲置，生成中还是应用中，和接收者（？）会存一个snapshot的指针
  + regionSched：通知工人干活了
  + snapTriedCnt：什么计数……
  + Engines：这个是要的，持久化都存在这个里面
  + Tag：用户id
+ 

## 测试

### TestOneSnapshot2C

#### 流程

+ 10条就打开始gc
+ 开三台机器
+ 加两条日志（现加空日志 有3条）
+ Get两条（共有5条）
+ 获取apply状态
+ 添加过滤器（？）
+ 写15条日志（现20条+2快照）
+ 删除1条（现21条+2快照）
+ 睡500ms
+ Get一条（现22条+2快照）
+ Get三条（现25条+2快照）
+ 宕机 重启
+ Get一条（现26条+2快照，如果重新选举就27条）
+ 判断日志是否截断

####  bug
+ bug1: panic: snapshot is temporarily unavailable
  + 如果 Snapshot 还没有生成好，Snapshot 会先返回 raft.ErrSnapshotTemporarilyUnavailable 错误，Leader 就应该放弃本次 Snapshot，等待下一次再次请求 Snapshot。
  + 解决：如果日志还没好 先return
+ bug2: panic: can't get value 76313030 for key 6b313030 [recovered]
  + 快照一直没有打进去
  + 原因：471行存了k100，476行有过滤器的情况下是读不出来的，481行取消过滤器之后应该能够取出来
  + 看了一圈 不懂为什么没取出来……感觉是快照并没有真正进入storage，获取的l.storage.FirstIndex()都不是snapshot的下一个Index
  + 解决：ApplySnapshot函数根本没写……
+ bug3: panic: runtime error: invalid memory address or nil pointer dereference
  + 弯路：要考虑是第一次初始化才清空各个状态信息，ps.isInitialized
  + 解决：空snapshot直接跳过
+ bug4： panic: [region 1] 1 unexpected raft log index: lastIndex 9 < appliedIndex 26 [recovered]
  panic: [region 1] 1 unexpected raft log index: lastIndex 9 < appliedIndex 26
  + 分析：理论上lastIndex更新了，然后通过wb写入就算是持久化好了，为何会出现重启之后lastIndex很小的事情
  + 发现更新乐LastIndex之后，下一回合又变成了8很奇怪
  + 解决：接到日志有以后 r.RaftLog.entries = make([]pb.Entry, 0)
### TestSnapshotRecover2C
+ bug1: test_test.go:314: logs were not trimmed (547 - 5 > 2*100)
  + 问题：截断是谁来控制？是leader还是follower自己
  + 解决：没有处理gc消息
### TestSnapshotUnreliableRecoverConcurrentPartition2C
  + bug1:panic: runtime error: index out of range [68] with length 14
### TestRestoreSnapshot2C
  + bug1:panic: runtime error: index out of range [18446744073709551615] with length 0 [recovered]
    panic: runtime error: index out of range [18446744073709551615] with length 0
    + 访问了一个不能访问的index
  + bug2:panic: requested entry at index is unavailable [recovered]
    + 同上，但是可以通过catch err解决
  + bug3:raft_test.go:1020: sm.Nodes = [], want [1 2 3]
    + 没处理快照传来的node
### TestProvideSnap2C
+ bug1:ndex out of range [18446744073709551603] with length 1 [recovered]
  + becomeleader的时候没有更新next数组
### TestRawNodeRestartFromSnapshot2C
+ bug1: Snapshot:{Data:[] Metadata:conf_state:<nodes:1 nodes:2 > index:2 term:1
  + 弯路：不懂为什么一定要这个ready.Snapshot.Metadata = nil
  + 改了前面的就过不了了……再看看……
  + newLog的时候不把snapshot保存了 因为已经执行了 不pending

## 结果

![](https://raw.githubusercontent.com/TaurusGGBOY/photobed/master/20220328205426.png)

## 技巧

+ peerStorage.Tag就是id！！！

## 天坑
+ r.RaftLog.first 和 r.raftLog.storage.FirstIndex() 和 r.raftLog.FirstIndex()
  + r.RaftLog.first
  + r.raftLog.storage.FirstIndex()
  + r.raftLog.FirstIndex() 这个可以直接删掉
+ apply快照之后，是不是要把所有现有日志先抛掉
  + r.RaftLog.entries = make([]pb.Entry, 0)就好了
  + 因为不一致这一段的话也不重要，几次通信就回来了
  + 后面逻辑更简单点
  + 否则会出现snapshot修改一次applied和lastindex
  + 然后entry不为0 又修改一次的情况
