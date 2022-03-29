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
