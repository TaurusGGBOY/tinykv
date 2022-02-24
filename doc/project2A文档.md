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

+ + 

## 2AA

### 测试脚本

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

### 测试脚本分析

+ 五个raft，五种分区情况，最后leader1当选就行
+ 三个raft，1,2,3依次开始选举，最后要轮番当选
+ 任何状态的投票
+ 单节点leader
+ 测试候选人重置Term
+ 选主之后隔离一个从，重新选主，

## 2AB

## 2AC

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