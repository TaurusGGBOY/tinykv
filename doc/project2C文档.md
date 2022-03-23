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

+ 收到gc消息
+ 压缩 

## 需要看和修改的函数

+ maybeCompact
+ sendAppend():发现已经不能发Entries只能发快照了
+ sendSnapshot：leader调用这个 发送快照给follower
+ Storage.Snapshot():调用这个函数会返回快照，也就是
+ handleSnapshot：follower调用这个来存储快照，基本走了一遍newRaft，会保存快照
+ ApplySnapshot
+ hasReady：判断一下是否有新的快照
+ raedy()：取新的快照给raedy
+ SaveReadyState：要apply snapshot
+ Advanced()：会修改


