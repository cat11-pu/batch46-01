# GSB Bugfix Task

## 题面

《深空遗迹》在 40 分钟的长时间试玩里必崩一次，验证层报描述符池耗尽：同一个池子被所有帧共用，上一帧释放的 set 描述符要等到下一帧才真正还回来，uniform buffer 预算一直被占着；更糟的是有帧在 GPU 还没跑完时就执行了重置，把正在采样的 sampler 描述符复用了，画面出现随机黑块；战斗特效频繁申请和释放 set 之后，日志里同一个 id 指向了两条记录；进程重启后帧内分配状态全部清零，正在读的贴图句柄也跟着丢了。请修 `gsb/batch46-game-e-04` 模块 `descriptor_pool_guard.go` 里的 `DescriptorPool`，任务类型 Bugfix，测试文件保持原样。

要实现的方法如下，参数、返回值和错误码都要和骨架一致：

```go
func NewDescriptorPool(frames, setsPerFrame int, perFrame DescCounts) (*DescriptorPool, error)
func NewDescriptorPoolFromSnapshot(frames, setsPerFrame int, perFrame DescCounts, data []byte) (*DescriptorPool, error)
func (p *DescriptorPool) AllocSet(frame int, need DescCounts) (string, error)
func (p *DescriptorPool) FreeSet(id string) error
func (p *DescriptorPool) ResetFrame(frame int) error
func (p *DescriptorPool) Usage(frame int) (DescCounts, error)
func (p *DescriptorPool) FreedPending(frame int) (int, error)
func (p *DescriptorPool) ActiveSets() int
func (p *DescriptorPool) Faults() int
func (p *DescriptorPool) Describe(id string) (SetInfo, error)
func (p *DescriptorPool) SetCount(frame int) (int, error)
func (p *DescriptorPool) Snapshot() ([]byte, error)
```

具体故障点与边界：

1. 分配出的 id 形如 `f<帧号>-<槽位>`，槽位从 1 开始并取当前最小空闲值；同一帧连续分配应得到 f0-1、f0-2。
2. 每帧 set 数量达到 setsPerFrame 后返回 `ErrOutOfSets`；四类描述符超出各自预算时分别返回 `ErrOutOfUniforms`、`ErrOutOfStorages`、`ErrOutOfSamplers`、`ErrOutOfImages`。任何失败都不能改动 Usage、SetCount、ActiveSets。
3. 需求里出现负数返回 `ErrNegativeCount`；帧号越界（AllocSet、Usage、SetCount、FreedPending 都一样）返回 `ErrBadFrame`。
4. 只有预算类失败才让 Faults 加一，非法帧号和负数需求不算故障。
5. 释放未知 id 返回 `ErrUnknownSet`，重复释放同一个 id 返回 `ErrDoubleFree`，释放成功后该帧的 FreedPending 增加。
6. 释放本身不归还描述符预算，只有 ResetFrame 回收该帧之后 Usage 才归零；回收后 SetCount 为 0，槽位从 f0-1 重新可用，FreedPending 归零。
7. ResetFrame 遇到该帧还有未释放的 set 时返回 `ErrFrameBusy`，整帧的分配和计数一条都不许变。
8. ResetFrame 必须按帧的环形顺序推进（0、1、0、1……），跳帧或重复重置返回 `ErrFrameOrder`。
9. 各帧预算互相独立，帧 0 用满 set 配额不影响帧 1 分配。
10. Describe 返回该 set 的 Frame、Need 和 Freed 标记；未知 id 返回 `ErrUnknownSet`。
11. Snapshot 保存分配表、Freed 标记、重置顺序和 Faults；用不同的帧数、set 数或描述符预算恢复返回 `ErrConfigMismatch`；恢复后 ActiveSets、SetCount、FreedPending、Usage 与快照前一致，且下一次 ResetFrame 从正确的帧继续。
12. 同一个 set 槽位释放并回收后必须能被重新分配，id 复用不能产生两条并存记录。

硬约束有三个。第一是资源上限：setsPerFrame 和四类描述符预算都是硬顶，任何路径都不得超卖。第二是并发与原子性：多个渲染线程会同时申请 set，30 个并发分配必须拿到 30 个互不相同的 id，预算用量精确等于需求之和。第三是顺序与恢复：重置必须按帧序推进，freed 状态要能跨进程从快照接上，否则重启后会把仍在使用的描述符当成空闲。

验收：在模块根目录执行 `go test ./...`，15 条用例必须全过；不得修改测试、跳过测试或针对固定输入写死返回值；并发分配和快照恢复场景自行验证。只做本地实现，不涉及上游仓库。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
