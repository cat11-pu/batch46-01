# GSB Bugfix Task

## 题面

昼夜切换之后，室内体素光照出现残影，重启进程后已经烘焙的辐照度全部归零，个别体素在两次提交之间闪烁，探针边界处还能看到一格亮的方块。我们把混合系数从 0.2 调到 0.5 做实验后残影更重，怀疑更新顺序也有问题。需要修复辐照度体素更新：模块路径 gsb/batch46-game-d-05，文件名 irradiance_volume_guard.go，主类型 IrradianceVolume，任务类型 Bugfix。

更新包用 VoxelUpdate{Coord, Irradiance, Samples}，统计用 FrameStats{Frame, Staged, Applied, Skipped}，网格用 VoxelGrid{X, Y, Z}。关键接口：
NewIrradianceVolume(grid VoxelGrid, spacing float64) (*IrradianceVolume, error)
VoxelCount() int
VoxelIndex(coord [3]int) (int, error)
VoxelCoord(index int) ([3]int, error)
WorldToVoxel(p Vec3) ([3]int, Vec3, error)
BeginFrame() (uint64, error)
Stage(u VoxelUpdate) error
CommitFrame() (FrameStats, error)
AbortFrame() error
StagedCount() int
Irradiance(coord [3]int) (Vec3, error)
Samples(coord [3]int) (int, error)
SetBlendAlpha(alpha float64) error
BlendAlpha() float64
WrapIndex(i, n int) int
Snapshot() ([]byte, error)
Restore(data []byte) error

具体问题：
1. 间距必须正且有限，否则 ErrBadSpacing；每个轴 1 到 64 且总数为 262144 以内，否则 ErrBadGrid；混合系数默认 0.2，供昼夜过渡使用。
2. 索引按 x + grid.X*(y+grid.Y*z) 计算，越界返回 ErrIndexRange；世界坐标到体素采用半开区间，越界或 NaN 返回 ErrOutsideVolume。
3. BeginFrame 返回从 1 开始递增的帧号，重入返回 ErrFrameActive，帧号用于对齐动画时间轴，渲染线程不能在一个未提交的帧上继续叠数据。
4. Stage 在没有打开帧时返回 ErrNoFrame，坐标越界返回 ErrIndexRange，样本数为负返回 ErrBadUpdate，同一帧重复暂存返回 ErrDuplicate 且保留第一笔。
5. CommitFrame 在没有帧时返回 ErrNoFrame；样本数为 0 的更新计入 Skipped 且不落盘；从未烘焙的体素直接采用新值，已烘焙体素按 新值*alpha 加 旧值*(1-alpha) 混合，样本数累加，同一帧内所有更新按暂存顺序结算。
6. AbortFrame 丢弃全部暂存并关闭帧，没有帧时返回 ErrNoFrame，关卡切换时暂存区必须整体丢弃，不能把上一关的光照带进新场景。
7. 混合系数必须在 (0,1] 内，否则 ErrBadAlpha，NaN 同样拒绝；设为 1 表示完全信任本次烘焙结果。
8. WrapIndex 对负数回绕到区间内，n 小于等于 0 时返回 0，边界体素复制时依赖这个行为避免越界。
9. 快照保存体素、样本数、混合系数和帧号；连续两帧后恢复，下一次 BeginFrame 必须返回 3，帧号不能倒退。
10. 网格或间距不一致、数据损坏返回 ErrSnapshot，且不能改动现有体素，恢复失败时渲染端继续使用旧数据。
11. 64 个协程同时暂存不同体素后提交，Applied 必须为 64，每个体素样本数为 1，不能丢更新或覆盖别人的写入。
12. 26 万体素是硬上限，暂存与提交应为常数级开销，不得每次提交重建整张体素表，也不能让单帧耗时随帧数增长，渲染线程读取时不能看到半提交状态。

验收要求 go test ./... 的 15 条全部通过，不能改测试、跳过或按固定数值写死；并发暂存和重启恢复自行验证；帧号用于日志对账，提交后的数据要立即可读，不允许后台线程偷偷写体素表；只做本地实现，不涉及烘焙管线其他阶段。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
