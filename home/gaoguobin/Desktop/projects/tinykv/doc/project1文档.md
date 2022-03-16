# 项目1

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

## 想法

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

尝试一下在StandAloneStorage创建engine

+ 看了下Config