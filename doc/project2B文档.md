# 项目2B及其天坑

> 实现storage
>
> 难度：hard（主要的问题是调用链路难梳理以及用什么接口无引导）

## 2B

## 术语

+ pd：存储元信息，调度，uuid，授时，继承etcd，rpc
+ region：相邻key聚在一起 多个peer组成
+ peer：手上有一个raft副本
+ 前缀：数据z，元数据0x01
+ 元数据前缀：
  + 0x01 集群，存储id信息
  + 0x02 region信息 log index raft本地状态 raftapply状态
  + 0x03 region id本地状态
+ command
  + 读：leaseread
  + transfer：transfer
  + change：conf_change
  + 其他：propose：Map<uuid, callback> pendingCmd
+ commited
  + 写入writebatch
  + 原子写入到rocksdb
+ Transport：send到server层
+ Store：region 保存 Map<uuid, Peer> peerMeta
+ Tick
  + 100ms一个
  + 回调：writebatch处理所有ready，pos成功，apply

## GenericTest

#### 测试类型

+ 不可靠：RPC可能失败
+ 崩溃：一段时间后重启
+ 分区：重新分区
+ 最大raft日志：不应该超过这么多
+ confchange：随机改变配置文件
+ 分裂：超过1024字节就分区

#### 流程

+ 5个服务器
+ 如果maxraftlog为-1就为无限日志
+ 如果要分区，分区最大数量为300,200大小就分裂（这个不确定）
+ 开启新集群
+ 等待选举完成
+ 3轮
+ 如果完成客户端人数为0 有一半几率Put 有一半几率scan
+ 如果不可靠或者分区 300ms之后制造网络混乱
+ 如果confchang，100ms之后改变配置
+ 5ms之后 告诉客户端退出 分区者退出， 日志改变者退出
+ 如果不可靠或者分区，等待分区，集群清除过滤器 等待选举完成
+ 等待客户端返回
+ 如果崩溃，就停止所有服务器，等待选举时间，重启服务器
+ 对于每一个client，扫描，追加，删除测试
+ 检查最大raftlog数量
+ 如果要分裂，检查EndKey是否为0

## 题目

+ 不考虑region
+ 看RaftStorage
+ raftWorker
+ 流程
  + 工人：raftCh获取信息，包括tick，proposed的cmd
  + 处理ready，发送msg，持久化，apply

## 2BA

> 实现peer的storage

### 题目

+ 看raft_serverpb
+ 看kv/raftstore/meta
+ 元数据 PeerStorage
+ 就实现一个功能 PeerStorage.SaveReadyState
  + 保存ready中的数据，包括entries和硬状态
  + entries直接append到ready
  + 删除过去append的但是不会commit的
  + 更新RaftLocalState并且保存到raftdb
  + 更新`RaftLocalState.HardState`，保存到raftdb
+ 使用writebatch
+ 看peer_storage.go
+ 设置log环境变量LOG_LEVEL=debug

#### raft_server

+ RaftStorage
  + 有raftdb，kvdb，配置文件，结点（这个是啥），快照管理，路由，系统，解析工人，快照工人
+ NewRaftStorage
  + kv，raft，快照分三个文件夹保存
+ Write
  + Put和Delete给pd提交请求就行
+ Reader
  + 给pd提交读请求，有些事务的处理
+ Start
  + 初始化各个角色
+ Stop
  + 停止

#### SaveReadyState

+ 不要编辑ready
+ 后面要利用ready、

### 过程

+ 保存硬状态
  + GetRaftLocalState看看SetMeta放些什么

### 调用关系

+ 谁调用的saveReadyState

### ready结构体

+ 就是rawnode调用的一个函数
+ 准备好的定义
  + hardState有更新
  + softState有更新
  + entries有更新（没有commit的也要存）

### Append

+ 为什么会需要删除
  + 之前持久化的可能不会commit
  + 那么就需要先把entry里面的所有先update了
  + 然后后面多了的只有一种可能，就是被截断了的

## 2BB

> 实现ready过程

### 运行流程

+ raftWorker会起一个run，run里面是个死循环，select消息
+ 处理几种消息：关闭消息，正常消息
+ 正常消息处理完之后还要处理ready
+ 处理正常消息有几种消息：raft消息，raft命令，时钟消息，分裂，region大致大小，垃圾回收，开始

**根据官方文档梳理一下运行流程**

+ 客户端调用RawGet这些
+ rpc handler分流
+ RaftStorage 先处理 WAL（？）
+ propose
+ 持久化
+ commit
+ 执行commit的命令，返回回调
+ 接收回调 返回RPC
+ RPC handler返回给客户端

### onRaftMsg

> 参考代码

#### 流程

+ 是否是合法raft消息
+ 是否已经停止
+ 看下是不是墓碑（？）
+ 检查消息（？）
+ 检查是否要加载镜像
+ 将peer加入缓存
+ 给group step这个msg
+ 检查是否有peer追上进度了，如果有就加上进度

### proposeRaftCommand

> 属于是上面处理正常消息里面的raft命令那一部分
>
> 可以参考onRaftMsg

#### 过程

+ 感觉意思是，根据CRUD，用过callback返回结果
+ 感觉这个地方只用处理propose就行
+ 要处理两个错误
  + ErrNotLeader不是leader
  + ErrStaleCommand 没有commit的日志被重写了（这会发生吗？不应该commit才返回？）
+ admin的先不管
+ 集中处理非admin的命令
+

### HandleRaftReady

> 属于上面处理完消息之后处理ready这一部分
>
> 参考HandleMsg

#### 过程

+ 看了下其他地方有ready advanced的地方
+ 就直接ready->Append->Advance就完事儿了

### proposal的处理

+ 很坑，中途基本上没有提到proposal的处理，找了半天，以及参考了代码，找到了描述信息
+ In this stage, you may consider these errors, and others will be processed in project3: ErrNotLeader，ErrStaleCommand 
+ 就是上面这一句提到了怎么处理proposal
+ 要处理proposal的原因：
  + proposal保存的时候就是直接追加到尾部，所以proposal本身是有线性性质的
  + 但是proposal和entry是不是一一对应的状态呢？不好意思，不是
  + 为什么，因为entry不一定会全部被commit，只有commit之后，准备apply了，才会调用proposal里面的callback
  + 所以说，如果对于所有的commitLog Index，一个proposal我们有以下处理方式
    + entry.index < proposal.Index：不可能出现，弹出一个entry的时候，至少弹出一个proposal
    + entry.index > proposal.Index：之前的proposal都过期了，可能已经被覆盖掉了，这些都可以返回ErrStaleCommand
    + entry.index = proposal.Index：分情况考虑
      + entry.Term = proposal.Term：就是这个回调，用就可以了
      + entry.Term < proposal.Term：同一个index可能在多个term被propose了，但是前面的没被返回，返回ErrStaleCommand
      + entry.Term > proposal.Term：乱序rpc导致后面term的proposal提前传过来了，返回ErrStaleCommand


## 测试脚本

### TestBasic2B

+ bug1：panic: request timeout

  + 看了下是mustPut函数超时
  + 最终调用的是Request函数
  + 请求5s就算超时
  + 会走到router发送一个raftMsg给peer
  + 超时是在WaitRespWithTimeout
  + 接收cb.done里面的消息，然后done迟迟没有消息来

+ bug2：panic: remove /tmp/test-raftstore1638636090/snap/gen_1_5_5_lock.sst.tmp: no such file or directory [recovered]                     panic: remove /tmp/test-raftstore1638636090/snap/gen_1_5_5_lock.sst.tmp: no such file or directory

  + 追溯一下
  + err := c.simulator.RunStore(c.cfg, engine, context.TODO())
  + err := node.Start(ctx, engine, c.trans, snapManager)
  + 追不上去了 找不到哪的问题

+ bug3：panic: len(resp.Responses) != 1

  + 调用Scan的时候，起手会有个Snap的请求
  + 如果这个请求返回不为1则报错，那么只需要收到Snap的时候返回一个response就行

+ bug4：[error] failed to generate snapshot!!!, [regionId: 1, err : stat /tmp/test-raftstore2587279877/snap/gen_1_5_5_default.sst.tmp: no such file or directory]

  + 	暂时没管
  + 	file, err = os.OpenFile(cfFile.TmpPath, os.O_CREATE|os.O_WRONLY, 0600)一路追到这 不能open 是权限问题吗？

+ bug5：test_test.go:44: failure                                                                                                      panic: runtime error: invalid memory address or nil pointer dereference

  + 	snap命令没有返回txn 导致txn是个空指针

+ bug6：[fatal] get wrong value, client 0 want:x 0 0 y                                                                                                                      got:
  + 感觉是幂等性的问题
  + 底层存储kv的格式是：就是kv
  + 很奇怪的是 后面的测试有些能够正确输出一部分结果，然后有一部分突然被截断，之前的丢失，只有之后的
  + 现在有两个可能
    + 第一个put没有写入到db中
    + 第二个snap没有正确的获取快照
      + 跟踪一下 感觉和失败生成快照错误有有关
+ bug7：find no region for xxxx
  + see the blog of pingcap
  + because the leader election fail
  + 看了下r.msg，里面的消息根本没有被消费，这怎么可能好使？
  + 拷贝一个正确答案看一下
+ bug8：want:x 0 0 yx 0 1 yx 0 2 y
  got: x 0 0 yx 0 1 yx 0 1 yx 0 1 yx 0 2 yx 0 2 y
  + 没考虑幂等性
  + 看了下日志 很奇怪，感觉是leader在收到了put消息之后马上就commit了，但是其他1/2的成员根本还没有commit，所以感觉是commit了两次，这样就apply了两次
  + 不对 本来就是put之后马上response，所以不是这个问题
  + 哦 再次反转 process是要在commit之后再调用，说明了是先commit再process，所以上面的顺序是对的，但是为什么会有重复日志还是不清楚
  + 弯路原因：貌似找到原因了 put之后还没有写入到db中，就来了个snapshot，结果先跑去执行snapshot（或者说并行执行），这个时候hasReady依然为true，导致ready重复消费
  + 弯路深究：在调用snapshot的时候没有更新applyIndex，跟这个有关？
  + 弯路解决：在写入wb之前都更新了applyIndex
  + 好象是propose的问题，查看前一章proposal的部分
    + 处理了，发现不是，因为proposal callback只影响response，不应该影响kvdb里面的值
  + 又改了下Append里面的ps.raftState
  + 定位到了是wb在snapshot的时候没有清空

### TestUnreliable2B
+ bug1:panic: resp.Responses[0].CmdType != raft_cmdpb.CmdType_Put
  + 输出了一下，发现proposal有问题……经常有错误的index和term的日志的proposal
  + 参考了一下个各位的代码 发现他们的做法是直接不处理Index相同但是term不相同的（？等待超时？）
+ bug2:panic: remove /tmp/test-raftstore194857741/snap/gen_1_5_5.meta.tmp: no such file or directory
  + 尝试直接改代码跳过这个错误
  + 放弃了 跑单测吧
  + --- FAIL: TestOnePartition2B (0.18s) 通过
    --- FAIL: TestPersistConcurrent2B (0.14s) 通过
    --- FAIL: TestPersistPartition2B (0.23s)
  + 还是会经常有tmp无法remove的问题，怀疑是权限问题？应该用sudo去跑
  
### TestPersistPartition2B
+ panic: runtime error: index out of range [18446744073709551611] with length 461
  + handleAppendEntriesResponse都发生在这
  + 没有处理快回退，而是直接回退到match+1上
  + sendAppend的时候 有时候没带Index带了个None
  + r.sendAppendResponse(m.GetFrom(), m.GetIndex()-1, true)这一句的问题吧 不能这样回退 要回退到相同term的第一个Index
  + 特判appendResponse的index为None时直接return

### TestLeaderIncreaseNext2ABTestLeaderIncreaseNext2AB
+ 中途2a出现bug1 raft_test.go:995: next = 4, want 6
+ 看日志的意思是 已经有3个日志，然后加上一个空日志，加上一个propose的日志，下一次的next应该为6
+ 但是我是4 说明只发完了第三个
+ 解决：r.sendAppendResponse(m.GetFrom(), r.RaftLog.LastIndex() + 1, None, true)
+ 上一句必须是返回lastIndex+1 而不是lastIndex

### version "go1.16.3" does not match go tool version "go1.13.8"
  + 
## 结果
![](https://gitee.com/agaogao/photobed/raw/master/img/20220322152330.png)

## 过程中遇到的问题

+ msg proto.Message到底接的是啥参数啊……
+ setMeta第二个参数到底传的什么
+ entry里面是怎么存储的
  + 不用管吗？只用看最后msg读出来的
+ 怎么读取entry里面的数据
  + msg.unMarshal
+ msg的req里面怎么存的
  + 只用管第一个？
  + 根据MustPutCF的代码，可以看到只放了一个req进去
  + 并且只查了一个response[0]
+ 如何保证d.proposal[0]就是对应的entry的callback？
  + 需要自己却确保
+ cluster.Scan是取了个快照回来进行的扫描
+ snap命令到底做了什么


## 小技巧

### 如何区别writeBatch中的leader的日志
+ 很坑 因为id是private，所以无法在rawnode层打印id信息
+ 可以把writeBatch里面的日志data打印出来
  +  leader的：key:default_2 00000000�������� value:x 2 0 y
  +  follower的：key:default_2 00000000 value:x 2 0 y
+ 目测原因是leader的key后面带了txn相关的东西

## 天坑

### 天坑1

+ 事情是这样子的，就是发现2B的所有Test经常会出现丢日志的情况
+ 具体表现就是日志的value可能第一个就添加到kvdb中失败或者出现中途突然之前的日志被截断
+ 但是通过WAL发现每次请求是写入到了WriteBatch里面了的，并且最后也执行过了
+ 那么问题就很奇怪了就只有两个原因
  + writeBatch写入有问题
  + 每次scan读取的snap有问题
+ 但是这俩都很难排查，感觉跟本不是我这一层的问题
+ 知道看了 [project2b中，使用engine_util.PutCF接口添加的数据，经常获取失败导致 - 学习与认证 - AskTUG](https://asktug.com/t/topic/273613/13)
+ 老哥提到，是没有在初始化的时候，初始化peer的配置文件，导致一直是单机状态运行
+ 我一看！情况一模一样！无语
+ 原来是2A没有测试多服务器的情况，但是2B上来就是3-5台，而且关于这个没有任何报错……
+ 各位遇到这种情况可以看看newRaft函数

### 天坑2

+ 选举一直失败
+ 发现是send了vote消息
+ 但是一直没有人收到
+ 跟踪了消息队列msgs，发现消息根本没有发送出去
+ 看了下别人的代码 才发现在获取ready的时候需要把msgs全取出来
+ 在advance之前发送出去
+ 啊对对对对对

### 天坑3

+ 日志等级怎么调整？
+ 搜了半天LOG_LEVEL发现无用
+ 看了代码发现是要设置环境变量为error（比如才有用）
+ 如 export LOG_LEVEL=error 无语
+ 就算是该了代码里面的else都没用 一定要这么搞

### 天坑4

+ 运行空间不足
+ 一看tmp文件夹 创建了几十个临时文件夹 一个500MB
+ 要经常清理 很麻烦