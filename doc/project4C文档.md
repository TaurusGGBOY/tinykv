# Tinykv 4B

> 实现scan等

## 题目要求

+ 实现`KvScan`, `KvCheckTxnStatus`, `KvBatchRollback`, and `KvResolveLock`
+ `KvCheckTxnStatus`, `KvBatchRollback`, and `KvResolveLock` 遇到冲突
+ `KvCheckTxnStatus`检查超时 删除过时锁 返回锁状态
+ `KvBatchRollback` 检查key锁了，删除value 回滚
+ `KvResolveLock` 检查批量锁的key
+ 自己实现迭代器 `kv/transaction/mvcc/scanner.go`
+ 错误不应该停止
+ `KvBatchRollback` and `KvCommit` 代码差不多
+ 只用 物理时间`PhysicalTime` function in [transaction.go](/kv/transaction/mvcc/transaction.go)

### TestCheckTxnStatusNoLockNoWrite4C

+ 为什么检查的时候，没有锁就要返回Action_LockNotExistRollback，并且要write？

### TestResolveCommit4C

+ KvResolveLock直接是扫锁列族……效率太低了吧

## 函数
 
+ scanner.Next()
  + 先通过ts和nextkey seek到一个item
  + 这个item就是要返回的item
  + 但是要准备好nextkey
  + 就是从当前地方一直迭代到下一个不是自己这个key的地方

## 结果

![](https://raw.githubusercontent.com/TaurusGGBOY/photobed/master/20220402161340.png)

## 技巧

+ 做到这才发现这个项目是按照脚本编程
+ 要做的应该是读清楚脚本，然后按照脚本顺序一个个完成需求……