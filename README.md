# GSB Bugfix Task

## 题面

上线前的性能验收发现，《星港》主城场景的帧分析面板数据全乱了：同一帧里 GBuffer、光照、后处理三个 pass 的耗时加起来比整帧时间还长，阴影 pass 常常显示 0 ms，偶尔还会出现负的耗时。值班同学捞日志看到，时间戳查询池连续跑满三帧后再也拿不到槽位，可面板仍展示上一帧的旧数据；把窗口拖到另一块屏触发一次设备丢失重连后，历史耗时和预算告警一起消失。请补完 `gsb/batch46-game-e-01` 模块 `gpu_timestamp_guard.go` 里的 `GpuTimestamp`，任务类型 Bugfix，测试文件不要改动。

对外接口按下面这份契约实现，参数和返回错误码都要对得上：

```go
func NewGpuTimestamp(poolSize int, tickPeriodNs float64) (*GpuTimestamp, error)
func NewGpuTimestampFromSnapshot(poolSize int, tickPeriodNs float64, data []byte) (*GpuTimestamp, error)
func (g *GpuTimestamp) AllocQuery(label string) (int, error)
func (g *GpuTimestamp) BeginQuery(index int, frame, ts uint64) error
func (g *GpuTimestamp) EndQuery(index int, frame, ts uint64) error
func (g *GpuTimestamp) DurationMs(index int) (float64, error)
func (g *GpuTimestamp) MarkUnavailable(index int, reason string) error
func (g *GpuTimestamp) Release(index int) error
func (g *GpuTimestamp) ResolvedQueries(frame uint64) []QueryInfo
func (g *GpuTimestamp) PassMs(frame uint64, pass string) float64
func (g *GpuTimestamp) FrameMs(frame uint64) float64
func (g *GpuTimestamp) SetBudget(pass string, budgetMs float64)
func (g *GpuTimestamp) OverBudget(frame uint64) []string
func (g *GpuTimestamp) Snapshot() ([]byte, error)
```

故障与边界一共这些，逐条都要落到行为上：

1. poolSize 小于等于 0 返回 `ErrBadPoolSize`，tickPeriodNs 小于等于 0 返回 `ErrBadTickPeriod`，只在构造时判断。
2. 分配永远取当前最小空闲下标；释放掉的低下标要先于从未用过的更高下标被复用；池满返回 `ErrPoolExhausted`，不许偷偷扩容。
3. 对没分配的索引调用 BeginQuery 返回 `ErrUnknownQuery`；同一槽位连续两次 begin 返回 `ErrQueryInFlight`。
4. EndQuery 遇到没 begin 的槽返回 `ErrNoBegin`；结束帧号与开始帧号不一致返回 `ErrFrameMismatch`，并且这次失败不能让槽位变成可用结果。
5. 结束时间戳小于开始时间戳返回 `ErrBadRange`，同样不得产生时长；修正顺序后重新 end 必须能成功。
6. 时长按 (end-begin)×tickPeriodNs/1e6 换算成毫秒；begin=1000000、end=3500000、tick=1 时必须得到 2.5。
7. DurationMs 在结果就绪前返回 `ErrNotReady`，Ready 只对已 resolve 的槽返回 true。
8. MarkUnavailable 之后该槽 Ready 为 false，且它的时长不能进入 PassMs、FrameMs 和 ResolvedQueries；ResolvedQueries 只列已 resolve 的槽并按索引升序。
9. Release 一个还处于 begun 状态的槽返回 `ErrReleaseBusy`；正常释放后 ActiveCount 减一，下一次分配应拿回同一个索引。
10. PassMs 汇总同一帧内同标签的所有已 resolve 结果，FrameMs 汇总整帧，空帧返回 0，未知 pass 返回 0。
11. SetBudget 设置的是单帧预算，OverBudget 按标签升序返回超预算的 pass，没有数据的帧返回空。
12. Snapshot 必须保住各帧各 pass 的累计耗时和预算；用 NewGpuTimestampFromSnapshot 恢复后 ActiveCount 为 0，但同一帧的 PassMs、FrameMs、OverBudget 与快照前完全一致。

并发和一致性是硬要求：八个协程同时 AllocQuery 必须拿到八个互不相同的下标，期间不允许重复发号或丢号；Begin、End、DurationMs 也要能被多个渲染线程同时调用。另一个硬要求是设备丢失后的恢复，快照里的统计不能因为重启而丢，否则预算告警会静默失效。槽位总数是硬上限，任何路径都不得突破 poolSize。

验收方式：在模块根目录执行 `go test ./...`，15 条用例必须全过；测试文件不得修改、不得跳过、不得按固定输入写死返回值；并发和快照恢复场景自己构造验证。只做本地实现，不涉及上游仓库。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 地狱
