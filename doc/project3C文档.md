# Tinykv 3C

> 实现调度器

## 题目要求 

+ 收集心跳
  + 但是每个心跳都更新吗？
    + 如果没有更新的话跳过
    + 不能相信每一个心跳 （分区）
  + 用confVer和version判断
    + 首先比较region version
    + 相同 比较 confVer 越大越好
  + 判断
    + region是否存有相同ID 如果有 并且confver version 中一个太低了就丢掉
    + 如果没有 就扫描所有区域 如果两个version不是比所有的都大于等于 那么这个分区就过时了
  + 跳过
    + 两个ver大 不能跳
    + leaderchange了 不能跳
    + new或者老 有pendingPeer不能跳
    + ApproximateSize变了 不能跳
    + ……
    + 重复update不影响正确性
  + 要更新 用 RaftCluster.core.PutRegion更新store
  + 更新 region tree
+ 实现负载均衡
  + 实现Schedule方法，避免store有太多region
    + 选择所有适合的store
    + 按照区域大小排序
    + 放最大区域大小store
  + 移动
    + 找一个pending的region
    + 没有就找一个follower
    + 如果不行 就选leader region
    + 选择region来move
    + 或者尝试下一个小一点的region
    + 直到所有都尝试
  + 选好
    + 选一个store的最小region
    + 检查原本store和现在store的size
    + 如果够大 就move peer
  + 哪些store适合移动
    + down的时间<=cluster.GetMaxStoreDownTime()
  + 选region
    + GetPendingRegionsWithLock
    + GetFollowersWithLock
    + GetLeadersWithLock
  + 价值判断
    + 目标和原始区域大小差值 > region.size *2 才移动回原region

## 测试

### TestReplicas13C

#### 流程

+ 创建集群 调度器
+ replica初始化1
+ 添加(store, regionCount) (1, 5) (2, 8) (3, 8) (4, 16) 
+ 给region1添加leader在store 4 那么4就有leader Region了 
+ (4,4) leader转给(1,1)
+ 1离线
+ 更新(2, 8)为 (2, 6)
+ (4,4)leader转给(4,2)
+ 设置replicas为3
+ schedule为NIL
+ 设置replicas为1
+ schedule不为NIL

## 注意的地方

+ EmptyRegionApproximateSize 設置的1MB 也就是1
+ 坑：如果当前的ids.len 小于最大replica 就继续move了

## 结果
![](https://raw.githubusercontent.com/TaurusGGBOY/photobed/master/20220419231830.png)