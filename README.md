# GSB Bugfix Task

## 题面

版本 1.4 上线后，开放世界换区时的卡顿投诉集中爆发。排查发现着色器变体缓存根本没有按预算淘汰：材质包的 300 多个变体一直堆在内存里，越过 256 MB 上限后还在增长；街区附近的高频变体被一次冷启动编译挤掉，玩家每次路过同一片区域都要重新编译；被剧情流式加载钉住的变体照样会被淘汰，过场动画首帧直接黑屏；美术热重载换了宏定义之后，旧变体没有当成新条目，反而覆盖了原记录；进程重启后缓存全丢，命中率统计一起归零。请修 `gsb/batch46-game-e-02` 模块 `shader_cache_guard.go` 里的 `ShaderCache`，任务类型 Bugfix，测试文件保持原样。

需要实现的对外方法如下，参数、返回值和错误码都要与骨架一致：

```go
func NewShaderCache(capacityBytes int) (*ShaderCache, error)
func NewShaderCacheFromSnapshot(capacityBytes int, data []byte) (*ShaderCache, error)
func (c *ShaderCache) MakeKey(stage, source, defines, target string) ShaderKey
func (c *ShaderCache) Put(key ShaderKey, byteSize int, compileMs float64) error
func (c *ShaderCache) Get(key ShaderKey) (Entry, bool)
func (c *ShaderCache) Peek(key ShaderKey) (Entry, bool)
func (c *ShaderCache) Pin(key ShaderKey) error
func (c *ShaderCache) Unpin(key ShaderKey) error
func (c *ShaderCache) Evict(n int) ([]ShaderKey, error)
func (c *ShaderCache) EvictionOrder() []ShaderKey
func (c *ShaderCache) Len() int
func (c *ShaderCache) SizeBytes() int
func (c *ShaderCache) HitRate() float64
func (c *ShaderCache) TotalCompileMs() float64
func (c *ShaderCache) Snapshot() ([]byte, error)
```

具体的故障点和边界：

1. 容量小于等于 0 时构造返回 `ErrBadCapacity`；单条大小小于等于 0 返回 `ErrSizeInvalid`；单条大于总容量返回 `ErrTooLarge`，这三种失败都不许改动缓存。
2. 相同 key 重复写入返回 `ErrDuplicateKey`，原有条目的大小、编译耗时和命中数一律保留。
3. 写入后字节数会超过容量时，自动淘汰最久未使用的未钉住条目直到放得下；Get 会刷新使用顺序，Peek 既不刷新顺序也不增加命中数。
4. 被 Pin 的条目永不被自动淘汰。如果剩余空间只差一点而可淘汰的条目已经为零，写入返回 `ErrCacheFull`，并且不能插入半条记录。
5. Pin、Unpin 不存在的 key 返回 `ErrKeyNotFound`；两者都不改变条目的使用顺序，Unpin 之后该条目重新成为淘汰候选。
6. 显式 Evict(n)：n 小于 1 或大于当前条目数返回 `ErrEvictRange`；可淘汰的未钉住条目不足 n 条时返回 `ErrPinnedBudget`，并且一条都不删；成功时按最久未使用到最近使用的顺序返回被删 key。
7. EvictionOrder 返回全量顺序，最久未使用的在最前；空缓存返回空切片。
8. Len 和 SizeBytes 必须是当前真实条目数与字节数，淘汰、失败写入之后都要立刻正确。
9. Get 返回的 Entry 里 Hits 要包含本次命中；HitRate 等于命中数除以命中数加未命中数，尚未发生任何查询时为 0。
10. MakeKey 把 stage 统一成小写，四个字段共同决定身份；同一个 source 只改 defines 就是另一个变体。
11. TotalCompileMs 汇总当前仍在缓存中的条目的编译耗时，被淘汰的条目不再计入。
12. Snapshot 要保存条目、命中计数和 LRU 顺序；恢复时容量与快照不一致返回 `ErrCapacityMismatch`；恢复后的 EvictionOrder、HitRate、TotalCompileMs 必须和快照前一致。

硬约束有三条。第一是资源上限：任何写入路径都不能让 SizeBytes 超过容量，也不能靠临时超卖再补救。第二是并发与原子性：多个加载线程会同时 Put 和 Get，缓存内部状态必须一致，失败路径不能留下删了一半的淘汰结果。第三是重启恢复：进程重启要能从快照接上命中率和淘汰顺序，否则下次筛选会重新踩坑。

验收：在模块根目录跑 `go test ./...`，15 条用例全过；不许改测试、不许跳过、不许针对固定输入写死返回值；并发写入和快照恢复要自己构造场景验证。只做本地实现，不需要提交到上游仓库。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
