# 项目2A

>  实现基础Raft算法

## 题目

+ 地址：`raft/`下
+ 用的是逻辑时钟，不要使用定时器
+ 逻辑时钟用的`RawNode.Tick()`
+ Raft不会阻塞等待请求消息
+ 看`proto/proto/eraftpb.proto`
+ 心跳的AppendEntries分开
+ 实现，三步走：选举，日志复制，原始结点接口

### 选举

+ `raft.Raft.tick()`实现超时
+ 发消息：压到`raft.Raft.msgs`
+ 接收到的在`raft.Raft.Step()`
+ 处理`MsgRequestVote`, `MsgHeartbeat`
+ 实现测试stub functions？
+ `raft.Raft.becomeXXX` 更新中间状态（follower candidate leader）
+ make project2aa

### 日志复制

+ 处理 `MsgAppend` and `MsgAppendResponse`
+ 看`raft.RaftLog` in `raft/log.go`
+ 和`Storage` interface defined in `raft/storage.go`对接
+ `make project2ab`

### 实现原始结点接口

+ `raft.RawNode` in `raft/rawnode.go`提供了一些包装的函数
+ `RawNode.Propose()` propose？
+  `Ready` is also defined here
+ 发送消息给其他人
+ 保存日志到固定存储
+ 保存状态比如term，commitIndex，voteto
+ 应用提交过的日志给状态机
+ 封装在ready然后 `RawNode.Ready()` 返回
+ `RawNode.Advance()` 更新applied index, stabled log index, etc.
+ `make project2ac`

### HINT

+ `raft.Raft`, `raft.RaftLog`, `raft.RawNode` and message on `eraftpb.proto`添加状态
+ 第一个消息term是0
+ 新被选上的leader应该添加一个空日志
+ leader一旦提交 更新commitIndex，通过`MessageType_MsgAppend`广播消息
+ 本地消息没有设置term？`MessageType_MsgHup`, `MessageType_MsgBeat` and `MessageType_MsgPropose`.
+ leader和非leader添加日志很不一样
+ 选举时间要加随机因子
+ 一些包装好的函数 `rawnode.go` can implement with `raft.Step(local message)`
+ 新开的raft从 `Storage` to initialize `raft.Raft` and `raft.RaftLog`


## 2AA

### 看测试脚本

+ project2aa:   $(GOTEST) ./raft -run 2AA
+ TestLeaderElection2AA 等10个测试脚本
+ 有NopStepper 自称black hole 可以看做宕机的？
+ 通过newNetworkWithConfig创建一些网络环境，比如一共三个人啥的，最后收敛的term是多少
+ newNetworkWithConfig会调用newTestConfig，包括一些peers啊，选举计时器啊，心跳计时器，存储一类的东西
+ 最后会调用一个newRaft跑起来，然后会返回一个raft回去
+ pb是从github上拉的一个类……
+ 那么问题来了，怎么进行的选举
+ 看了一下测试脚本 是通过自己给自己发一个Hup消息开始进行的选举
+ 也就是说要写一下处理cup消息的部分
+ 看了下不需要考虑消息的收发，收发的时候推送到msgs队列里就可以了

### 要修改的部分

+ newRaft
+ Step
+ 新增followerHandler
+ becomeCandidate
+ becomeLeader
+ 新增sendAllRequestVote
+ 新增sendRequestToPeer
+ 等等

### 测试脚本分析

+ 五个raft，五种分区情况，最后leader1当选就行
+ 三个raft，1,2,3依次开始选举，最后要轮番当选
+ 任何状态的投票
+ 单节点leader
+ 测试候选人重置Term
+ 选主之后隔离一个从，重新选主，
+ 等等

### 结果

![](https://gitee.com/agaogao/photobed/raw/master/img/20220224101723.png)

## 2AB

### 测试脚本

+ TestProgressLeader2AB感 觉有点问题，成为leader会有一个空日志然后自己match变成0，next变成1，但是程序要求空日志提交之后match就是1
  + 解决：第一个log的index是1，term也是1，但是其实他的下标是0……
+ TestLeaderElectionOverwriteNewerLogs2AB
  + 创建5个节点
  + 在此之前，1赢1宕3赢3宕
  + 1选举失败，拉高term到3
  + 1选举成功 成为leader 拉高term到到3
  + 检查是否日志都为term 1 3的
+ TestLeaderCommitPrecedingEntries2AB：nextIndex回退有问题，理论上是不可能一回合回退的，如果第一轮next设置为lastIndex不能match，理论需要一轮回退但是这里面没有用这个，就直接回退成功了？
  + 只看第二轮 序号为1的这一轮
  + 首先 存储中是有一条 index=1 term=2的消息
  + 成为leader 有一条 index = 2 term = 3的消息
  + 会提议一条 index = 3 term = 3的消息
+ TestLogReplication2AB：
  + 1号选举成为leader，提议一个空日志 index：1
  + 1号propose一个日志 index：2
    + 
  + 2号开始选举 index：3
  + 2号提议一个日志 index：3
+ TestFollowerAppendEntries2AB

### RaftLog

> 被RaftLog和Storage整晕了……

+ 日志存储方式snapshot/first.....applied....committed....stabled.....last
+ snapshot快照，这个就不用管了
+ first：第一个不是快照的日志
+ applied：应用到状态机的日志
+ commited：已是提交状态的日志
+ stabled：已经持久化的日志（？不应该所有的都持久化？）
+ last：最后一个日志
+ 两个地方要更新first
  + 加载日志进raftlog
  + 更新快照

### Storage

> 存的到底是什么：所有持久化了的日志

+ InitialState：
  + HardState：存日志以外的一些信息
  + ConfState：group的群组关系
+ Entries：打快照了的不能返回
+ Term：
+ LastIndex：持久化了的最后一个Index
+ FirstIndex：Entries的第一个，快照的后一个
+ Snapshot：打快照了

### stabled

+ 什么时候会stable
	+ commitNoopEntry：r.RaftLog.stabled = r.RaftLog.LastIndex()
+ 什么时候需要回退

### 结果

![](https://gitee.com/agaogao/photobed/raw/master/img/20220307233553.png)

## 2AC

### RawNode

+ RawNode是什么：是一个raft的封装？就这么简单？
+ 应用层是通过RawNode来接触Raft的
+ Raft和应用层通过Ready通信
  + 发送信息给其他用户
  + 保存日志到stable
  + 保存硬状态
  + apply提交日志
  + 等
+ 应用层调Ready（）决定什么时候处理
+ 调了ready过后，应用层调起其他函数

### 测试脚本
**TestRawNodeStart2AC**

+ 创建RawNode
+ 竞选
+ 查询ready
+ 往storage里面追加日志
+ 告知raft ready已经用完了
+ 提议一个日志
+ 查询ready
+ 此时rawNode上日志长度应该为1，commitEntries也应该为1
+ 往storage里面追加日志
+ 告知raft ready已经用完了
+ 如果rawNode还说有ready就是错的

**TestFollowerAppendEntries2AB**

+ 说在获取unstableEntries的时候超了
+ 看了下在只有1日志的时候，说已经stable到2了
+ 有点问题
  + 感觉意思是如果心跳中没有包含所有新的日志就要继续要新日志？

![](https://gitee.com/agaogao/photobed/raw/master/img/20220307233359.png)

## 2A

### 结果

![](https://gitee.com/agaogao/photobed/raw/master/img/20220307233512.png)

## 常见错误

+ 把没带from和to的消息传给消息队列了，会在获取peers[to]的时候报空指针
+ 把hup/beat等消息传给消息队列了。这种直接调step

## Q&A

+ RawNode是什么东西

## 参考

 [TiKV 源码解析系列文章（二）raft-rs proposal 示例情景分析 - 知乎 (zhihu.com)](https://zhuanlan.zhihu.com/p/56820135) 

## 想法

+ propose是干什么的？看原本的论文的地方没有地方提到
  + propose里面会调用**`RawNode::step`**，