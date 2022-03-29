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
