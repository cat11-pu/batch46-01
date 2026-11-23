# GSB Bugfix Task

## 题面

镜面地板上线后反射整体偏移半米，金属墙面反射发黑，部分探针的捕获任务在重启后消失，导致反射永远停在旧帧，玩家走进新房间时还能看到上一张场景的倒影。更麻烦的是，删掉房间后旧探针的捕获任务还在队列里，继续占用渲染线程，线上已经先回滚到关闭反射的版本。需要重做反射探针调度：模块路径 gsb/batch46-game-d-04，文件名 reflection_probe_guard.go，主类型 ReflectionProbe，任务类型 Bugfix。

任务用 CaptureJob{ProbeID, Face, Mip}，面顺序为 FacePosX 到 FaceNegZ；探针信息用 ProbeInfo{ID, Bounds, Priority, Captured, CapturedAt}。关键接口：
NewReflectionProbe(maxProbes, faceSize int) (*ReflectionProbe, error)
AddProbe(id string, bounds BoxBounds, priority int) error
RemoveProbe(id string) error
ProbeIDs() []string
Probe(id string) (ProbeInfo, error)
ScheduleCapture(now int64) ([]CaptureJob, error)
Complete(job CaptureJob) error
Fail(job CaptureJob) error
PendingCount() int
MipCount(faceSize int) int
BoxProject(p, dir Vec3, bounds BoxBounds) (Vec3, error)
BlendWeights(p Vec3) (map[string]float64, error)
Snapshot() ([]byte, error)
Restore(data []byte) error

要修的点：
1. 构造时探针上限必须在 [1,64]，面尺寸必须是 [4,2048] 的 2 的幂，否则 ErrBadConfig，防止显存预算被一个错误配置打穿。
2. 探针 id 非空、包围盒三个轴 Min 严格小于 Max 且数值有限，否则 ErrBadProbe；重复 id 返回 ErrDuplicateProbe；超预算返回 ErrTooManyProbes。
3. ProbeIDs 按优先级降序，优先级相同按 id 升序，保证每次启动后探针顺序完全一致，方便对照日志排查。
4. MipCount 对正整数返回 floor(log2)+1，非正数返回 0，例如 1 得 1、2 得 2、256 得 9，mip 数量直接决定捕获任务总数。
5. 每帧最多派发 8 个新任务，顺序为探针优先级、id、面 PosX 到 NegZ、mip 升序；已派发未完成的任务不能再次派发，全部派完返回空。
6. Complete 只接受挂起任务，未知或已完成返回 ErrUnknownJob；探针最后一面完成后标记 Captured，CapturedAt 取该探针任务的派发时间。
7. Fail 前两次保持挂起，第三次返回 ErrProbeFailed 并丢弃该任务，此后不得再派发同一任务，避免坏探针无限占用渲染线程。
8. PendingCount 统计所有挂起任务；RemoveProbe 要连同它的挂起和已完成任务一起清掉，不能留下指向已删除探针的僵尸任务。
9. BoxProject 从盒内点沿方向求出口点，点越界返回 ErrOutsideBox，方向为零或含 NaN 返回 ErrBadDirection，方向长度不影响结果，出口点必须落在包围盒表面。
10. BlendWeights 只统计包围盒包含该点的探针，权重与到中心距离成反比，正好落在中心时该探针权重为 1，没有探针返回 ErrOutsideProbes。
11. 快照保存探针、挂起任务与派发时间，配置不一致或数据损坏返回 ErrSnapshot，恢复后不能重复派发已经完成的任务。
12. 8 个协程同时调用调度时，每个待捕获任务必须恰好派发一次，不能重复或漏发；以面尺寸 8、两个探针为例应恰好得到 48 个任务，PendingCount 稳定为 48，单帧新增不得超过 8，返回的批次顺序要稳定可复现。

验收要求 go test ./... 的 15 条全部通过，不能改测试、跳过或按固定时间戳写死；并发派发和快照恢复自行验证；探针增删只通过公开接口完成，每次跑批结果一致；只做本地实现，不涉及引擎其他调度器。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
