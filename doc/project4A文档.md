# Tinykv 4A

> 实现MVCC

## 题目要求

+ 实现MVCC

## 快照隔离

+ 读取lastCommit（这个还是要加锁？）
+ 没有冲突才能合并
+ 比serializable更好性能
+ 写偏斜：同一个数据集不相交的地方更新（感觉没有问题啊？） 问题在于可能有查询约束 when v1>0 and v2>0这种情况
+ 解决：
  + 全局冲突表
  + 表写锁

## 4题目

+ 实现SI需要存储key+时间戳
+ 写冲突应该取消整个事务
+ TinyScheduler获取start的时间戳
+ 用kvget和kvscan
+ 事务建立，会选择一个主key
+ 客户端先发送kvprewrite给tinykv
+ 包含事务中所有写操作
+ 加锁失败就返回，可以充实
+ 所有key成功 prewrite九成宫了
+ 加锁存储主key和TTL
+ prewrite成功要发送commit请求
+ commit请求会有时间戳
+ prewrite失败，KvBatchRollback
+ 主key成功了就commit其他的key
+ 其他key commit应该成功
+ 一旦prewrite成功，只能超时导致commit失败
+ commit失败回滚

## 4A题目

+ default列用来存值
+ lock存锁 存lock结构体
+ wrtie存变化
+ default 存的有开始的时间戳
+ write key+commit时间戳 存write结构体
+ 编码和解码key看transaction.go
+ mvccTxn提供读和写
+ mvccTxn应该知道开始时间戳
+ GetValue使用StorageReader进行列族迭代 迭代主要看commit时间戳而不是start时间戳