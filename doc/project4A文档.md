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

## 要看和修改的函数

+ PutLock：放锁
+ PutWrite：key和ts一起编码 列族是写族，值是write.Tobytes
+ PutValue：放值
+ GetLock:获取锁，通过txn.reader获取Lock列族中的key，把获取的锁parse回对象
+ GetValue：还不能简单的返回get key，其实是要返回快照最新ts的key
  + Getvalue
+ EncodeKey
  + 把key和时间戳级联起来
  + 但是为什么要把ts取反了再存进去
+ CurrentWrite：当前读不是并发读……
  + 从0开始遍历ts直到当前事务ts

## 数据结构

+ WriteKind：写入的操作

## 列族

+ CfDefault：放kv值
+ CfWrite：WAL

## 测试

### TestPutLock4A

#### 流程

+ testTxn：记录开始的时间戳
+ Lock：存主key，时间戳，TTL，锁类型
+ PutLock：给事务加锁
+ 看写操作里面是否和传入的一样

## 问题
+ 为什么iter出来的key value一定要用copy 不用不行吗
+ cfwrite和cfDefault到底谁先谁后 有关系吗？ 
  + 如果prewrite了，那么就会调用往cfwrite里面加东西
  + 此时为commit
  + commit后面apply体现为cfdefault加东西
+ 顺序
  + 用户key升序
  + 同key， ts降序
+ 扫表是按照列族扫过去的，可能最后就不等于本key了， 应该返回
+ 比较疑惑的就是iter到底seek到的什么地方？

## 结果

![](https://raw.githubusercontent.com/TaurusGGBOY/photobed/master/20220330150418.png)