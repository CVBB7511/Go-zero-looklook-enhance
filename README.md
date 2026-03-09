

## I 原版go-zero-looklook说明

English | [简体中文](README-cn.md)

## II 改进思路：多级缓存防击穿与热点数据并发控制

秒杀场景的核心是“读多写少”。简单的 Redis 缓存无法应对瞬间数十万 QPS 的热点数据请求（例如某一款爆款商品的库存查询），这会造成网络带宽打满或 Redis 节点 CPU 飙升。

**具体修改方案：**

1. **引入 Local Cache (L1) + Redis (L2)：**
* 使用 `freecache` 或 `bigcache` 在 Go 进程内存中建立本地缓存。这类库的底层实现避免了大量指针，从而极大降低了 Go 的 GC（垃圾回收）扫描压力。底层内存管理和 Page Fault。
* 查询库存时，先查本地缓存，未命中再查 Redis，最后才查 MySQL。


2. **使用 `singleflight` 解决缓存击穿 (Thundering Herd)：**
* 引入 `golang.org/x/sync/singleflight` 包。
* **代码逻辑：** 当本地缓存和 Redis 同时失效的瞬间，如果有 10000 个并发 Goroutine 试图去查 MySQL，利用 `singleflight` 的 `Do` 方法，将这 10000 个相同的 key 请求合并。只让第 1 个 Goroutine 去查数据库并回填缓存，其余 9999 个 Goroutine 阻塞等待共享这 1 次查询的结果。


3. **基于 `sync.Pool` 的内存对象复用：**
* 在高并发的 HTTP 解析或 RPC 序列化/反序列化（如 Protobuf/JSON）过程中，会产生大量临时字节切片（`[]byte`）。
* 使用 `sync.Pool` 预先分配和池化这些内存对象，减少频繁的内存申请和销毁，显著降低系统层面的系统调用（syscall）开销和进程上下文切换频率。


原版的 `go-zero-looklook` 项目**并没有实现多级缓存（即没有本地缓存 L1）**。它完全依赖于 `go-zero` 框架底层自带的缓存控制机制（基于 Redis 的单级分布式缓存）。

对于**防缓存击穿与热点数据并发控制**，原版项目也没有在 Logic 业务层手写相关代码，而是**隐式地利用了 `go-zero` 框架在 Model 数据层的自动化处理**。

原版的处理方式如下：

### 1. 依靠底层 `SharedCalls` 防缓存击穿（自带的 singleflight）

原版项目中由 `goctl` 生成的模型代码（例如 `app/travel/model/homestayModel_gen.go` 中的 `FindOne` 方法），它底层调用了 `go-zero` 的 `sqlc.CachedConn.QueryRow()`。

* **底层机制**：在 `go-zero` 框架内部，`QueryRow` 方法包装了 `syncx.SharedCalls` 组件（这是 `go-zero` 自己实现的一个 `singleflight`）。
* **并发控制效果**：当某一个热点民宿的 Redis 缓存失效时，如果有 10 万个并发请求同时涌入试图查询这个 ID，底层的 `SharedCalls` 会以对应的 Cache Key 为锁，**合并这些并发请求**。最终只会有 1 个 Goroutine 穿透去查询 MySQL 并回填 Redis，另外 99999 个请求会在原地阻塞，等待这 1 次查询的结果并直接共享，从而保护了 MySQL 不被打挂。

### 2. 依靠“空值缓存”防缓存穿透

在原版项目中，如果黑客使用大量数据库中不存在的无效 ID（如负数 ID）进行恶意请求，Redis 未命中后会打到 MySQL，MySQL 也查不到，通常这会导致缓存永远无法建立，请求一直打爆数据库。

* **底层机制**：原版依赖的 `go-zero` `sqlc` 缓存组件遇到 `ErrNotFound`（查无此记录）时，**会自动在 Redis 中存入一个极短过期时间（如 1 分钟）的特殊占位符（通常是一颗星号 `*`）**。
* **效果**：后续针对同一个无效 ID 的请求，在短时间内会被 Redis 中的 `*` 直接拦截并返回“未找到”，有效防止了缓存穿透。

### 3. 依靠“随机过期时间”防缓存雪崩

* **底层机制**：虽然原版配置里可以指定缓存的过期时间，但 `go-zero` 在真正写入 Redis 时，会在设定的基础时间上**自动增加一个 5% 左右的随机偏移量（Jitter）**。
* **效果**：这使得同一批写入的缓存不会在完全相同的秒数瞬间集体失效，避免了由于缓存集体过期而引发的缓存雪崩问题。

---

## III 与原版的对比

原版的 `go-zero-looklook` 做到了 **“框架兜底的标准化高并发保护”**，应付一般的互联网流量绰绰有余。

1. **突破 Redis 的网络与单点瓶颈**：原版即使有底层的防击穿保护，几十万 QPS 的读请求依然全量打到了 Redis。对于字节跳动级别的大促或秒杀，这会瞬间打满 Redis 集群的网卡带宽。你引入 `freecache` (L1) 后，99% 的热点读流量被挡在了 Go 进程内存里，省去了网络 I/O。
2. **保护 Redis 本身**：原版底层的 `SharedCalls` 是在 Redis 未命中、**去查 MySQL 时**才触发合并的，它保护的是 MySQL。而你在 Logic 层自己套一层 `singleflight`，是在 L1 缓存未命中、**去查 Redis 时**合并请求，这进一步保护了 Redis 免受瞬间高并发的冲击。

“**原版项目依赖 go-zero 底层保护了 DB，但我在此基础上，通过 L1 缓存和外层 singleflight 保护了 Redis 并消除了网络开销，完成了从‘一般高并发’到‘极致高并发’的架构演进。**”
针对多级缓存、防止缓存击穿以及基于 `sync.Pool` 的内存优化，在 `go-zero-looklook` 项目中，你可以重点修改以下几个模块的文件。这些修改不仅契合高并发场景，也非常贴合字节跳动支付等核心交易链路对极致性能（尤其是 GC 和内存控制）的要求。

## IV 修改的具体方案
### 改进一具体方案、 引入 Local Cache (L1) + Redis (L2) 与 Singleflight

在秒杀或爆款场景中，读请求通常集中在商品或服务详情上。在这个项目中，“民宿详情（Homestay Detail）”是最典型的“读多写少”且易产生热点数据的业务。

建议修改以下文件：

**1. 配置文件与结构体**

* **`app/travel/cmd/rpc/etc/travel.yaml`**
* **`app/travel/cmd/rpc/internal/config/config.go`**
* **修改内容**：在配置中增加本地缓存相关的参数，例如 L1 缓存的容量限制（如 `LocalCacheSize`）和过期时间（如 `LocalCacheExpire`），以便于后续通过配置中心动态调整。



**2. 模型层封装多级缓存**

* **`app/travel/model/homestayModel.go`**
* **修改内容**：`go-zero` 生成的 `homestayModel_gen.go` 已经通过 `sqlc` 包自带了 Redis 缓存（L2）和基于 DB 维度的 `singleflight`。你应当在自定义的 `homestayModel.go` 中进行扩展，拦截 `FindOne` 等高频查询方法。
* **实现逻辑**：在此处引入 `freecache` 或 `bigcache` 实例作为包级变量或注入到 Model 结构体中。查询时先走 Local Cache，若未命中，再调用原有的 `sqlc` Redis 缓存层。



**3. 业务逻辑层引入 Singleflight 防击穿**

* **`app/travel/cmd/rpc/internal/logic/homestayDetailLogic.go`**
* **修改内容**：在 `HomestayDetail` 方法中，当 L1 缓存未命中，需要穿透到 Redis 甚至 MySQL 时，引入 `golang.org/x/sync/singleflight`。
* **实现逻辑**：实例化一个全局的 `singleflight.Group`。当大量请求同时发现 L1 缓存失效，针对同一个 `id`，使用 `group.Do(id, func() (interface{}, error) { ... })` 将并发请求合并。只有第一个 Goroutine 会去请求底层数据并回填 L1 缓存，其余 Goroutine 直接共享结果。

### 压力测试
压力测试方案：
```
Summary:
  Total:        1.3456 secs
  Slowest:      0.1415 secs
  Fastest:      0.0010 secs
  Average:      0.0311 secs
  Requests/sec: 14863.1963

  Total data:   9860000 bytes
  Size/request: 493 bytes

Response time histogram:
  0.001 [1]     |
  0.015 [2861]  |■■■■■■■■■■■■■■
  0.029 [8035]  |■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.043 [5079]  |■■■■■■■■■■■■■■■■■■■■■■■■■
  0.057 [2421]  |■■■■■■■■■■■■
  0.071 [899]   |■■■■
  0.085 [427]   |■■
  0.099 [188]   |■
  0.113 [73]    |
  0.127 [13]    |
  0.142 [3]     |


Latency distribution:
  10% in 0.0125 secs
  25% in 0.0189 secs
  50% in 0.0273 secs
  75% in 0.0394 secs
  90% in 0.0539 secs
  95% in 0.0653 secs
  99% in 0.0928 secs

Details (average, fastest, slowest):
  DNS+dialup:   0.0000 secs, 0.0010 secs, 0.1415 secs
  DNS-lookup:   0.0000 secs, 0.0000 secs, 0.0000 secs
  req write:    0.0001 secs, 0.0000 secs, 0.0509 secs
  resp wait:    0.0294 secs, 0.0009 secs, 0.1181 secs
  resp read:    0.0008 secs, 0.0000 secs, 0.0513 secs

Status code distribution:
  [200] 20000 responses
```
```
hey -n 20000 -c 500 -m POST -T "application/json" -D req.json http://127.0.0.1:1003/travel/v1/homestay/homestayDetail
```
压力测试结果
```
Summary:
  Total:        1.3456 secs
  Slowest:      0.1415 secs
  Fastest:      0.0010 secs
  Average:      0.0311 secs
  Requests/sec: 14863.1963

  Total data:   9860000 bytes
  Size/request: 493 bytes

Response time histogram:
  0.001 [1]     |
  0.015 [2861]  |■■■■■■■■■■■■■■
  0.029 [8035]  |■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■■
  0.043 [5079]  |■■■■■■■■■■■■■■■■■■■■■■■■■
  0.057 [2421]  |■■■■■■■■■■■■
  0.071 [899]   |■■■■
  0.085 [427]   |■■
  0.099 [188]   |■
  0.113 [73]    |
  0.127 [13]    |
  0.142 [3]     |


Latency distribution:
  10% in 0.0125 secs
  25% in 0.0189 secs
  50% in 0.0273 secs
  75% in 0.0394 secs
  90% in 0.0539 secs
  95% in 0.0653 secs
  99% in 0.0928 secs

Details (average, fastest, slowest):
  DNS+dialup:   0.0000 secs, 0.0010 secs, 0.1415 secs
  DNS-lookup:   0.0000 secs, 0.0000 secs, 0.0000 secs
  req write:    0.0001 secs, 0.0000 secs, 0.0509 secs
  resp wait:    0.0294 secs, 0.0009 secs, 0.1181 secs
  resp read:    0.0008 secs, 0.0000 secs, 0.0513 secs

Status code distribution:
  [200] 20000 responses
```

### 二、 使用 `sync.Pool` 进行内存对象复用

在高并发支付链路中，支付网关的回调、消息队列的积压处理以及统一的 HTTP 返回封装是产生大量临时对象（导致 GC 停顿）的重灾区。

我修改了以下文件：

**1. 支付回调网关解析（支付岗核心关注点）**

* **`app/payment/cmd/api/internal/handler/thirdPayment/thirdPaymentWxPayCallbackHandler.go`**
* **修改内容**：支付系统在业务高峰期会瞬间收到海量的微信/支付宝回调通知，这些 HTTP 请求包含大量的 XML/JSON payload。
* **实现逻辑**：在读取 `r.Body` 以及反序列化时，使用 `sync.Pool` 预先分配 `[]byte` 缓冲区（如池化 `bytes.Buffer`）。避免每次接收回调都动态分配内存，显著降低 Page Fault 和 GC 压力。



**2. 消息队列的异步处理**

* **`app/mqueue/cmd/job/internal/logic/paySuccessNotifyUser.go`**
* **`app/mqueue/cmd/job/jobtype/jobpayload.go`**
* **修改内容**：在支付成功后，系统会通过 Asynq 投递和消费异步任务（如发送通知、结算记录）。
* **实现逻辑**：在解析 Asynq 的 `Payload` 或构建通知消息时，复用序列化过程中产生的临时结构体或字节切片。你可以创建一个专门的 payload 对象池，每次消费完消息后将其 `Reset` 并放回池中。



**3. API 层的统一响应处理**

* **`pkg/result/httpResult.go`**
* **`pkg/result/responseBean.go`**
* **修改内容**：所有对外暴露的 API 接口最终都会调用这里的 `HttpResult` 进行 JSON 序列化输出。
* **实现逻辑**：如果项目使用了标准的 `json.Marshal`，可以考虑引入池化的 `bytes.Buffer` 或替换为带对象池的 JSON 库（如 `jsoniter`），从而在统一出口处收敛内存开销。



### 总结实施路径
1. 在 `travel` 模块中实现并测试 **L1 + L2 + Singleflight** 的查询链路，压测对比 QPS 和 Redis 命中率。
2. 在 `payment` 模块的微信回调 Handler 中引入 **`sync.Pool`**，通过 `pprof` 抓取并对比修改前后的 heap 对象分配数量和 GC 耗时。
## Thanks

go-zero: https://github.com/zeromicro/go-zero

dtm: https://github.com/dtm-labs/dtm

jetbrains: https://www.jetbrains.com/



