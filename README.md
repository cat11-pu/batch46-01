# GSB Bugfix Task

## 题面

跨平台版本发布前一天，渲染组发现 PC 端每次切换画质档位都会卡 400 ms 以上：管线状态缓存的条目数从 200 一路涨到 900 多，完全没有上限；阴影 pass 的附件数从 1 改成 2 之后，旧管线仍在被复用，画面直接花屏；美术热重载了一个顶点着色器，缓存里引用它的图形管线没有失效，继续返回旧句柄；同一个 render pass 被重复注册时，整批条目被误判成兼容性变化而清空；升级驱动后缓存文件版本已经变了，程序没有拒绝，反而拿旧 blob 建出一个空缓存。请修 `gsb/batch46-game-e-03` 模块 `pipeline_cache_guard.go` 里的 `PipelineCache`，任务类型 Bugfix，测试文件不要改动。

对外方法按下面的契约实现，参数、返回值和错误码要一致：

```go
func NewPipelineCache(version string, maxEntries int) (*PipelineCache, error)
func NewPipelineCacheFromBlob(version string, maxEntries int, blob []byte) (*PipelineCache, error)
func (c *PipelineCache) RegisterRenderPass(name string, attachmentCount uint32) (int, error)
func (c *PipelineCache) Insert(desc PipelineDesc, handle uint64) error
func (c *PipelineCache) Lookup(desc PipelineDesc) (uint64, bool)
func (c *PipelineCache) Evict(n int) ([]PipelineDesc, error)
func (c *PipelineCache) InvalidateShader(shaderHash string) int
func (c *PipelineCache) Len() int
func (c *PipelineCache) Hits() int
func (c *PipelineCache) Misses() int
func (c *PipelineCache) Blob() ([]byte, error)
```

故障点和边界：

1. Insert 前 desc 的每个字符串字段都必须非空，缺一个就返回 `ErrInvalidDesc`；RegisterRenderPass 名字为空或附件数为 0 同样返回 `ErrInvalidDesc`。失败时 Len、Hits、Misses 都不能变。
2. 往没注册过的 render pass 插条目返回 `ErrUnknownRenderPass`。
3. Subpass 必须严格小于该 pass 的附件数，等于附件数返回 `ErrSubpassOutOfRange`，等于附件数减一时合法。
4. 同一个 desc 重复插入返回 `ErrDuplicateEntry`，保留第一次写入的 handle，不覆盖编译耗时等原有信息。
5. 两个不同 desc 使用同一个 handle 返回 `ErrDuplicateHandle`，后一条不能进入缓存，Lookup 它必须未命中。
6. 条目数达到 maxEntries 后再插返回 `ErrCacheFull`，不做隐式淘汰，也不许顶掉旧条目。
7. RegisterRenderPass 第一次注册返回 0；用相同附件数重复注册返回 0 且已有条目一条不动；附件数变化时删除该 pass 的全部条目并返回删除数量，其他 pass 的条目不受影响。
8. InvalidateShader 删除顶点着色器或片元着色器等于该 hash 的所有条目，返回删除数；不存在的 hash 返回 0。删除后 Len 和 Lookup 必须同步反映。
9. Lookup 命中时 Hits 加一并把该条目刷成最近使用；未命中时 Misses 加一。
10. Evict(n)：n 小于 1 或大于 Len 返回 `ErrEvictRange`；成功时按最久未使用到最近使用的顺序返回被删除的 desc。
11. 只有 VertexLayout 或 FragmentShader 不同的 desc 属于不同 key，各自占用一个 handle。
12. Blob 保存版本、条目、handle、命中计数和 LRU 顺序；用不同版本恢复返回 `ErrVersionMismatch`；恢复后的 Hits、Misses、Len 与淘汰顺序必须和快照前一致。

硬约束：一是顺序与幂等，重复注册同附件数不能被视为兼容性变化，兼容性变化必须先让旧条目失效再接受新插入；二是并发与原子性，多个渲染线程会同时 Lookup 和 Insert，校验失败不能留下半条记录，计数也不能丢；三是资源上限，maxEntries 是硬顶，任何路径都不得突破；四是重启恢复，缓存 blob 跨进程还原后淘汰顺序不能乱。

验收：在模块根目录执行 `go test ./...`，15 条用例必须全过；不得修改或跳过测试，不得针对固定输入写死返回值；并发和 blob 恢复场景自行构造验证。只做本地实现，不涉及上游仓库。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 地狱
