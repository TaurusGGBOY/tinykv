# Tinykv 2C

> 实现快照

## 题目要求

+ 服务器会丢掉超过阈值的日志
+ 快照是一个类似Append的消息
+ 为了不阻塞网络用的是独立的链接
+ 分块传的
+ leader通过storage.snapshot生成快照
+ follower通过handleSnapshot处理
+ 要处理gc
+ gc在AdminRequest 
+ 更新元数据，安排gc工人
+ snapshot是调用region工人来打快照
+ snap_runner 快照发送和接受
+ ready的时候可以修改snapshot
+ 修改kvdb和raftdb的陈旧状态
