# tinykv笔记

## 课程

### 第一讲

### 第二讲

+ KV：高性能 高拓展性 高可用性
+ LSM优势：高写性能 高空间利用率 简化并发控制
+ 1976 异地更新 串行化
+ 1987 Postgres LS存储
+ 1996 LSM
+ 索引更新策略：就地更新 异地更新
+ 就地更新：读友好 更新的时候会频繁随机IO
+ 异地更新：串行IO写 读不友好 后台需要有个线程跑compaction
+ memtable：b+/跳表 对并发比较友好的数据结构
+ 写入放大比例：实际写入量/数据请求写入量 合并策略主要考虑这个
+ merge
    + leveling 一个组件一层 下一层比上一层大T倍
    + tiering merge的时候 不修改下一层的组件 包含多个可重叠的组件 写放大不明显 读的时候都要遍历一遍
+ 分割
    + 比如把0-100分为 0-30 34-70 71-99 等
    + 通常是固定大小的（固定大小的分割？）
    + level 0是不分的
    + 下一层的分割要比上一层多
    + sstable
    + 优势：merge 的时候，只需要把重叠的分割给拿出来merge 并发好
+ 布隆过滤器 1970年
+ 获取流
    + memtable
    + immutable memtable 内存上
    + 查TableCache 索引过滤器什么的
    + 缓存不命中 sstable 磁盘里 查MetaBlock
    + 把查到的MetaBlock插入到TableCache中
    + 按照Meta读data Block 
    + 把查到的Block插入到BlockCache中
+ 核心结构
    + memtable 最近最新 leveldb：跳表 rocksdb：跳表 哈希跳表：桶中是跳表 哈希链表
    + immutable memtable 只可读 不可改
    + sstable 粒度是块 key是按顺序存储，有数据块和元数据块
+ Table Cache
    + LRU
    + 缓存所有已打开的SSTable的Meta
    + 上限是max_open_files
+ Block Cache
    + LRU
    + 从SSTable读的 DataBlock放入BlockCache
    + 可缓存 IndexBlock和FilterBlock （优先级更高？） Cache慢的时候不容易被换出
    + 可选择开启
+ RocksDB优化
    + 上一层存两个指针指向下一层的左边界和右边界
+ merge的时候读 会加锁
+ compact的时候cache会刷掉
+ NVem情况下可以不用维持每一行的有序性

### 第三讲

+ TinyKV基本框架
    + TinySQL
    + TinyScheduler 调度
    + TinyKV
+ 基本组件
    + 集群
        + 存储结点
        + 调度
    + 存储结点
        + 路由
        + 调度插件
        + 传输
        + 计时器驱动
    + 工人
        + raft工人
        + 存储工人
        + raft log gc
        + 区域任务
        + 分裂确认
          
        + 调度任务
    + 引擎
        + raftDB 日志
        + kvDB 数据 
+ 新加入结点
    + 不参与选举 类似 etcd的learner
    + 不能发起选举 但是能够正常投票 推荐 不用特殊处理
+ 删除结点
    + 会不停的选举
    + map存白名单 不在白名单的 接受到这个人的信息就直接排除掉
+ log：
    + stabled持久化到rocksdb里面
    + 初始化：要调用已有函数进行初始化
    + 初始化需要applied进行修正？FirstIndex开头可能会有一个空日志？需要--？
+ raft
    + 心跳用来判断自己还是否是leader？ 