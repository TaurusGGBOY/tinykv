# Tinykv 3A

> 实现成员更改

## 题目要求

+ 代码
  + 修改raft.go
  + 修改rawnode.go
  + 看eraft.proto
+ 领导转移
  + 新消息类型`MsgTransferLeader`， `MsgTimeoutNow`
  + 首先在当前leader上调用setup的`MsgTransferLeader`
  + 检查资格，不合格可以终止或者帮助
  + 如果日志不够新，就发送append，停止接收新的proposal
  + 合格了，应该发送`MsgTimeoutNow`给继任者，马上
  + 继任者收到消息马上开始选举，无论是否选举超时
  + 很大概率会成为新的leader
+ 成员变更
  + `ProposeConfChange` `EntryConfChange` `ConfChange`
  + 如果EntryConfChange commited了，应用ApplyConfChange 通过ConfChange
  + 之后才可以addNode和removeNode 到 ConfChange
  + 处理完pendingIndex之后需要置为空
+ `MsgTransferLeader`是本地消息
+ 可以设置Message.from为继任者
+ Hup开始新选举
+ Marshal来获取Data里面的ConfChange


## 领导选举论文

+ 停止新的proposal
+ 保证下一任完全复制，可以sendappend
+ 发送TimeOutNow，开始选举
+ 新的leader选出来之后发送消息给老leader
+ 如果选举超时时间没有完成，会终止

## 修改的和要使用的地方

+ leadTransferee：继任者
+ PendingConfIndex：只能有一个conf change阻塞。设置为最后一个阻塞的conf change？config只允许leader的applied index比这个大才能变更

## TestCommitAfterRemoveNode3A

+ 开raft1，成为leader，有peer2
+ 提一个成员变更，删除2
+ 成员变更阻塞的时候，提一个普通日志hello
+ 收到Append消息的时候，2要回response
+ 删除结点2
+ 阻塞的日志hello可以提交了

## TestTransferNonMember3A

 + 创建1234四个的集群
 + 2给1发timeout
 + 2给1投票
 + 3给1投票
 + 解决：newRaft有问题……不能默认自己就是集群里面的

## 结果

![](https://raw.githubusercontent.com/TaurusGGBOY/photobed/master/20220407165721.png)