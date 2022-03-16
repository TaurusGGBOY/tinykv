# 项目1

> 实现思路后面会有上手想法分享，看看从零上手的时候有什么问题，我自己上手的时候就遇到很多无从下手的情况

## 题目

建立这样一个服务

+ 独立的：单个结点，不是个分布式系统
+ 键值存储
+ grpc
+ 支持列族：键命名空间，相当于field？比如key为id，每个id对应的属性有不同的值

多列族可以视为分开的小数据库

列族后面project4支持事务的时候有用

服务要支持四种操作

+ Put：没有就加，有就改
+ Delete：删
+ Get：获取当前的值
+ Scan：获取多个key的值

分成两步

+ 实现一个独立的存储引擎
+ 实现键值服务句柄

### 代码

+ 不用考虑`kvrpcpb.Context`
+ write 是一个badger实例
+ reader 在快照上
+ badger：一个存储引擎
+  [badger.Txn]( https://godoc.org/github.com/dgraph-io/badger#Txn ) 实现reader
+ 通过添加前缀来模拟列族 用的是engine_util
+ 读`util/engine_util/doc.go`
+ 用`github.com/Connor1996/badger`
+ 丢弃之前要调用`Discard()` for badger.Txn 

### 实现服务句柄

实现

+ RawGet
+ RawScan
+ RawPut
+ RawDelete
+ `kv/server/raw_api.go`
+ `make project1`

## 读代码

搜索`// Your Data Here (1).`，一共有九处要添加的分别是

+ standalone_storage.go

  + type StandAloneStorage struct
  + func NewStandAloneStorage(conf *config.Config) *StandAloneStorage
  + func (s *StandAloneStorage) Start() error
  + func (s *StandAloneStorage) Stop() error 

  + func (s *StandAloneStorage) Reader(ctx *kvrpcpb.Context) (storag	e.StorageReader, error)
  + func (s *StandAloneStorage) Write(ctx *kvrpcpb.Context, batch []storage.Modify) error

+ raw_api

  + func (server *Server) RawGe
  + func (server *Server) RawPu
  + func (server *Server) RawDelete
  + func (server *Server) RawScan

### 测试脚本

搜索`NewStandAloneStorage`

阅读`func TestRawGet1(t *testing.T)`部分的代码

测试流程：

+ 创建配置文件
+ 通过配置文件创建新的存储结点实例
+ 运行存储结点实例
+ 通过存储结点，创建一个新的服务器
+ 创建默认列族
+ 设置为服务器默认列族
+ 创建rpc Get请求 存kv：{42, 99}
+ 执行rpcGet请求
+ 断言错误是否是nil
+ 断言返回的值是否是42

### Engines.go

这个项目不用自己写底层，有一个badger的db可以用

Engine里面创建了一个Kv数据库，一个Raft数据库

里面有几个接口

+ WriteKV
+ WriteRaft
+ Close
+ Destroy
+ CreateDB

看起来意思是创建的话，就创建一个WriteBatch，然后调用WriteKV

WriteBatch里面有日志啊，大小什么的

### server.go

```go
type Server struct {
	storage storage.Storage
	// (Used in 4A/4B)
	Latches *latches.Latches
	// coprocessor API handler, out of course scope
	copHandler *coprocessor.CopHandler
}
```

只用实现这个storage，但是看了一下，我们要实现的是stanalone_storage

然后完全没有看到implement关键字，然后再看了一下go的实现接口是不需要关键字的-_-||

实现同名方法就算是实现接口了，行吧

## 实现思路

### 大体思路

+ raw_api调用standalone_storage的接口
+ standalone_storage调用KvReader（实现了StorageReader接口的类）以及engine_util代码
+ KvReader调用了engine_util，badger代码

### type StandAloneStorage struct

+ 只用存一个engine_util.Engines

### func NewStandAloneStorage(conf *config.Config) *StandAloneStorage

+ 初始化上述engine

### func (s *StandAloneStorage) Start() error

+ 不用改

### func (s *StandAloneStorage) Stop() error

+ 关闭engine

### type StorageReader interface

+ 创建一个类实现他的所有接口
+ 使用`txn := r.db.NewTransaction(false)`创建事务对象，这个r.db从哪来呢？可以想想
+ 有些KeyNotFound情况需要特判，这个测试样例一看便知
+ iter的close需要手动调用，不要直接close了，肯定报错

### func (s *StandAloneStorage) Reader

+ 返回一个上面的实例就可以了

### func (s *StandAloneStorage) Write

+ 遍历Modify，针对不同的请求，进行不同的处理
+ 因为单机，不用对raft进行更改，更改kv即可

### func (server *Server) RawGet

+ 获取reader
+ 调用reader的getAPI
+ 特判notfound

### func (server *Server) RawPut

+ 新建batch
+ 调写好的storage.Write接口即可

### func (server *Server) RawDelete

+ 同上

### func (server *Server) RawScan

+ 获取iter
+ 按照req要求进行迭代（开始地址，limit等）
+ 

## 想法

> 这一段很有借鉴意义老哥们……可以看看怎么个思路 刚上手代码着实有问题……

### 任务一

调用栈应该是这样的

+ func (server *Server) RawGet
+ func (s *StandAloneStorage) Reader
+ StorageReader

standalone_storage.go

  + type StandAloneStorage struct : 保存一个map即可 但是官方是让用engine.util
  + func NewStandAloneStorage： new 一个StanAlone返回即可
  + func (s *StandAloneStorage) Start StandAlone里面加一个State
  + func (s *StandAloneStorage) Stop() StandAlone里面加一个State

  + func (s *StandAloneStorage) Reader 读是给一个快照的话 直接深拷贝？
  + func (s *StandAloneStorage) Write 写是一个实例就是给个引用出去？


+ raw_api

  + func (server *Server) RawGet：reader里面读
  + func (server *Server) RawPut：write里面写
  + func (server *Server) RawDelete：write里面写
  + func (server *Server) RawScan：reader里面读

然后就没思路了，主要的问题是，来数据了，怎么说，用Engine去构造log，然后发给kv和raft？

+ 尝试一下在StandAloneStorage创建engine
+ 看了下Config
+ Start又不会写了……
+ 看一眼engine_util的测试 怎么用的事务的

感觉Start没啥写的？因为New的时候CreateDB里面就Open了

stop也没啥写的？调用Close就行？

Reader就有说法了，这个Reader是什么Reader？返回的storage.StorageReader又是什么？不是个接口吗？没有地方实现这个啊

+ 看了眼官方文档，仔细看了下这部分说明
+ Reader应该返回一个`StorageReader`要支持在快照上kv单点get和扫描操作
+ Write应该提供一种方式提供一系列在badger实例上修改内部状态

现在的思路是，直接Reader返回一个kvReader

write直接将modify写入

+ 问题是Engines的WriteKV的参数是WriteBatch，里面是`entries       []*badger.Entry` 

又发现出路，TestRawPut1中有一个Get函数最后是`return reader.GetCF(cf, key)`。

想了一下 创建了一个KvReader继承Reader的接口，但是后面又不知道怎么写了，算了看老哥们思路把……

想法部分结束