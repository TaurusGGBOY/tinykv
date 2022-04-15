# Tinykv 3B

> 实现split

## 题目要求
+ 代码：peer_msg_handler 和 peer
+ 实现领导转移
  + rawNode的propose transferLeader命令 调用TransferLeader方法
+ 实现成员变更
  + 了解RegionEpoch，metapb.Region的元信息
  + 成员变更或者分裂 要改变epoch  conf_ver++
  + 分裂时候 version++
  + 用来保证最近的region信息在网络隔离但是有两个leader在同一个region
  + propose ProposeConfChange
  + committed之后 改变regionLocalState 包括 regionepoch 和 region里面的peers
  + 调用ApplyConfChange()
  + 执行AddNote maybeCreatePeer()。这是Log Term 和 Index为0。然后leader会知道他没有数据了 会直接发送快照
  + 执行removeNode 应该调用destroyPeer()
  + 更新GlobalContext 里面的storeMeta
  + 考虑confchange的幂等性
+ 实现region分裂
  + region方便scan
  + region有startkey和endkey
  + 一开始只有范围["","")
  + 每次SplitRegionCheckTickInterval都会检查regionsize，如果超了就要分裂  在split_check。在MsgSplitRegion的onPrepareSplitRegion
  + scheduler分配id
  + onPrepareSplitRegion 看onAskSplit
  + 实现处理splitAdminCommand看router.go
  + 一个region会继承，只用改range和epoch，另一个要创建新的元信息
  + peer应该通过createPeer创建 注册在router.regions里。信息要插入regionRanges 在 StoreMeta
  + checkSnapshot检查快照是否越界了
  + ExceedEndKey比较region的endkey 如果endkey 等于空 和 key比空大 就要处理ErrRegionNotFound ErrKeyNotInRegion`, `ErrEpochNotMatch`.等

## processConfChange

+ 当confChange的log被提出来之后对entry进行处理
+ unmarsal之后看cc.ChangeType
+ 添加结点
  + 如果已经有这个结点了，就应该跳过本次entry直接apply
  + 如果没有这个结点
  + region.peers里添加结点
  + confVer++
  + writeRegionState
  + storageMeta.regions[region.Id]=region
  + insertPeerCache
+ 删除结点
  + 和insert反向操作就行

## 测试

### TestBasicConfChange3B

+ bug1:panic: request timeout [recovered] panic: request timeout
  + 这个时候集群里只有1,3两台

### TestBasicConfChange3B

### 流程

+ 起1 2 3 4 5
+ 领导转移到1
+ 删除1 2 3 4 （集群只剩1）
+ put (k1, v1) （存的k1）
+ 2 应该get k1位空
+ 加peer(2,2)（集群：1 2）
+ Put Get k2（存 k1 k2）
+ 2 应该k1 k2都能获取
+ 集群获取k1 confVersion应该 > 1（等于5？）
+ 添加（3,3）（集群 1 2 3）
+ 删除（2,2）（集群 1 3）
+ Put k3
+ 3 能获取k1 k2 k3
+ 2 应该获取不到 k1 k2
+ 添加（2,2） （集群 1 2 3）
+ 2应该能获取k1 k2 k3
+ 删除（2,2）（集群1 3）
+ 添加（2,4）（集群1 3 4）
+ 删除（3,3）（集群1 4）
+ put k4
+ 2不能查到k1 k4
+ 3能查到k1 k4

### bug

+ bug1: [error] handle raft message failed storeID 2, tombstone peer [epoch: conf_ver:1 version:1 ] received an invalid message MsgRequestVoteResponse, ignore
+ bug2: timeout
  + 定位了一下是MustPut操作阻塞
  + 继续定位 发现只有两台机器的时候 发送的心跳无法接收 怀疑是add node 的时候peer的信息没有更新完全 
  + 没有想到怎么处理 继续看 TestConfChangeRecover3B错误原因 看能否排查出这里的问题
  + 查找1：cluster.MustAddPeer(1, NewPeer(2, 4))这个地方给貌似没有添加进去
  + 查找2：ConfChangeType_RemoveNode的destroyPeer没有执行 
  + 岔路：看来是遇到了大家都会遇到的一个bug 双结点 删除结点 集群失效bug（暂时不是这个地方）
  + 整体的问题还是一个：重新加入（2,4)结点 之后1没法给4发消息了，收不到（没有调用handler）
  + TestConfChangeSnapshotUnreliableRecoverConcurrentPartition3B都能过 basic过不了……滑天下之大稽
  + 尝试：用正确代码测试一下添加（2,4）之后，会不会调用newPeer， 我现在的代码是不走的。
  + 破案了：在processConfChange的时候 让d.proposals==nil的时候就不继续进行，然而 后面还要destroy self的 导致他没有自毁掉
  + 解决： 把response放后面 先执行destroy等操作
+ bug3:timeout
  + 现象：maybeCreatePeer()无法创建peer
  + 查找：断点 发现maybeCreatePeer一直收到的是append消息，但是根据IsInitialMsg函数定义 只有收到请求投票以及心跳（0==msg.commit的心跳） 才是合法的初始消息
  + 破案了：心跳里面带了committed 然而实际不让带 无语 导致一直不能判断这个结点是新的 有毒吧
  + 解决：无语了 发现心跳是不能带committed 而是应该在心跳response中带committed 然后判断是否发append
bug4:panic: [region 1] 2 unexpected raft log index: lastIndex 0 < appliedIndex 8
  + 现象：初始化peer之后，raftState.LastIndex = 0，但是要求为 appliedIndex
  + 探索：看函数里从db中拿出来的raftLocalState，确实拿出来就是applied = 8，这就很奇怪了，因为断点追踪到是进行了deleteMeta了的，也就是说里面的applyState应该是被删除了才对
  + 首先排除是否搞成了是同一个engine，看起来排除了 
  + ![](https://raw.githubusercontent.com/TaurusGGBOY/photobed/master/20220414203108.png)
  + 通过打印日志 发现确实是删掉了，在删除之后马上查 是查不到的，那么只能是删掉之后又写上去了
  + 找到了！找到了！找了两天！
  + 原因：在process消息之后，如果我自己destroy了，就必须要退出，不然可能后面会往db里面写东西，导致初始化的时候判断这不是一个初始结点
  + 解决：process之后如果d.stopped==true 就直接return
bug4:panic: [region 1] 2 meta corruption detected
  + meta.regionRanges.Delete(&regionItem{region: d.Region()}) == nil
  + 原因：貌似是没有setregion 
  + 解决：hasRaftReady处修改region和regionRanges

### TestConfChangeRecover3B

+ bug1: panic: runtime error: slice bounds out of range [1:0]
+ bug2:panic: unmatched conf version
+ 很奇怪，追踪了confVer的变化，有的时候从5变到7，但是后面的又从5开始
+ 简单来说就是regionsRange这个b加树里存的confVer 和 传过来的region的confVer不一样
+ search的region 是 在confver++的时候直接更新进去的
+ 传过来的region 调用链为 handleHeartbeatConfVersion<-RegionHeartbeat<-onHeartbeat<-SchedulerTaskHandler Handle<-worker Start
+ peer有个 HeartbeatScheduler 里面会clone当前的region 发送请求给 调度器worker
+ 两个时刻会触发 一个是leader并且 onSchedulerHeartbeatTick 一个是AnyNewPeerCatchUp
+ 跟踪一下confVer变化
  + 5 leader heartbeat 5
  + 4 follower confver 5->6
  + 4 follower confver 6->7
  + 1 follower 5->6
  + 1 follower 6->7
  + 5 follower 5->6
  + 5 follower 6->7
  + 1 成为leader heartbeat变为7
+ 问题是这样的的
  + 看日志 leader的applied从1329 退到了 630？
+ 又给ctx加了lock 有点效果 但是总体很迷
+ TODO 很迷 最后还是没能完全解决 有概率会出现

## 可能用到和修改的地方

### pendingConfIndex

> 这个地方有坑，单独拿出来说

+ 它是用来存储没有applied的成员变更的
+ 判断它是否合法的时候要用 pendingConfIndex > applied

### peer destroy流程

+ 给kv和raft写入墓碑
+ clear peerstorage数据
+ 给每个请求发送注意peer被删了

### processConfChange

+ 添加结点
  + 如果已经存在peerStorage的region里面了，就break
  + 否则 confver++
  + 往region的peers里面加peer结点
  + 往kv batch里面写regionstate信息
  + insertPeerCache
+ 删除结点
  + 如果当前=删除结点 自毁 break
  + 如果当前节点不存在在region break
  + 否则confver++
  + 删除peers里面的该结点
  + 往kv batch里面写regionstate信息
  + removePeerCache

## 问题

+ storeID是什么
  + peer里面的两个id 一个是peerid 一个是storeid 按照官方所说
  + storeID：TiKV 节点编号 
  + store_id ： TiKV 存储 ID
  + STORE_ID：热点 Region 所在 TiKV Store 的 ID。
+ 如何添加peer的
  + leader在第一次给他发送消息的时候，会判断maybeCreatePeer
  + 这里面会调用newPeer newRawnode newRaft等
+ 1主 2从 removeNode1会怎么样
  + 收到propose的时候先判断是否是这种情况，如果是，就丢掉这个propose

## TODO

+ confChange没有debug出来

## 参考

+ http://blog.rinchannow.top/tinykv-notes/
+ https://www.inlighting.org/archives/talent-plan-tinykv-white-paper/
+ https://asktug.com/t/topic/274196