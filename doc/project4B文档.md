# Tinykv 4B

> 实现快照读等

## 题目要求

+ 实现KvGet KvPrewrite 和 KvCommit
+ kvget读给定时间戳的数据，如果锁住就失败
+ kvprewrite是要写入到数据库的 没有锁的话
+ kvcommit是不改变的 只是记录
+ latch key类似key锁
+ 事务使用starttimestamp标志的
+ 处理error

## 思想

+ lock：操作某个key的时候必须加锁（包括commit和default），写default之前加锁，commit之后解锁
+ default：加锁，写default列族，返回
+ write：如果commit了，write写入这个key和commitTS，释放锁，注意要加latch进行并发控制

## 函数

+ KvGet 
+ KvPrewrite
  + lock的kind是要根据m.Op进行改变的
## 测试

### TestGetLocked4B

+ Expected :[]byte{0x2a}  Actual   :[]byte(nil)
  + 我的逻辑是如果有锁就放弃，返回nil
  + 看结果的意思是，加锁了，应该可以读之前的版本？
  + 改成不判断锁之后，240行会调用一些锁的属性出 问题
  + 解决：改成只有请求大于等于锁ts的才报错
  + 看一下KeyError
#### 想法

+ 想要获取lock GetLock，就要首先创建一个txn
+ NewMvccTxn方法创建txn，需要传入reader
+ reader可以用server.storage.reader
+ reader需要一个上下文，用req里面的上下文……

### TestSinglePrewrite4B
+ bug1:Expected :[]byte{0x1, 0x1,}  Actual   :[]byte(nil)
  + 还没有实现lock
  + 事务设置好了要用 server.storage.Write写入

### TestPrewriteLocked4B

+ bug1 : Error:      	Not equal: expected: 1  actual  : 0
  + 没有考虑加锁冲突
  + 在加锁之前先获取锁

### TestCommitConflictRollback4B

+ bug1:assert.False(builder.t, builder.mem.HasChanged(kv.cf, key)) 
  + HasChanged我返回是true
  + 要么是result = s.CfWrite.Get(item)==nil
  + 要么是!result.fresh==true
  + 解决：在commit的时候要遍历一下锁，如果没有锁，就说明prewrite有问题，直接返回

## 结果

![](https://raw.githubusercontent.com/TaurusGGBOY/photobed/master/20220401230557.png)