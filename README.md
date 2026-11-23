# GSB Bugfix Task

## 题面

赛季更新后，PC 端的窗口拖拽体验急剧恶化：窗口尺寸每变一像素就重建一次交换链，三秒内重建了 200 多次，驱动直接把 surface 标记成 suboptimal；显示器旋转成竖屏后画面被横向拉伸，宽高根本没有交换；4K 屏把系统缩放调到 150%，重建出来的链还是 1280×720；窗口缩到 100×50 这种极小尺寸时重建直接失败黑屏；崩溃恢复后交换链代数从 0 重新开始，旧代数被当成新链复用，正在显示的帧出现撕裂；退役的旧链一直没人释放，显存统计一路上涨。请修 `gsb/batch46-game-e-05` 模块 `swapchain_resize_guard.go` 里的 `SwapchainResize`，任务类型 Bugfix，测试文件保持原样。

对外方法按下述契约实现，参数、返回值和错误码要对得上：

```go
func NewSwapchainResize(caps SurfaceCaps) (*SwapchainResize, error)
func NewSwapchainResizeFromSnapshot(caps SurfaceCaps, data []byte) (*SwapchainResize, error)
func (s *SwapchainResize) RequestResize(width, height int) error
func (s *SwapchainResize) SetScale(scale float64) error
func (s *SwapchainResize) SetOrientation(degrees int) error
func (s *SwapchainResize) PendingResize() (Extent, bool)
func (s *SwapchainResize) MarkOutOfDate() error
func (s *SwapchainResize) Rebuild(desiredImages int, frame uint64) (SwapchainInfo, error)
func (s *SwapchainResize) Current() (SwapchainInfo, bool)
func (s *SwapchainResize) Generation() uint64
func (s *SwapchainResize) Retired() int
func (s *SwapchainResize) Release(generation uint64) error
func (s *SwapchainResize) Snapshot() ([]byte, error)
```

故障点与边界：

1. caps 必须满足 MinImages 至少为 1、MaxImages 不小于 MinImages、MinExtent 两轴至少为 1、MaxExtent 两轴不小于 MinExtent，否则构造返回 `ErrBadCaps`。
2. 新建的守卫没有当前链，Generation 为 0、Retired 为 0，PendingResize 报告没有待处理重建；第一次 RequestResize 之后必须报告有待处理。
3. RequestResize 宽或高小于等于 0 返回 `ErrBadExtent`，且不能产生待处理状态；连续多次请求以最后一次的尺寸为准。
4. 有效尺寸按逻辑宽高乘 scale 四舍五入，方向为 90 或 270 时交换宽高，最后按 caps 的 MinExtent 与 MaxExtent 双轴裁剪。
5. SetScale 只接受大于 0 且不超过 4 的值，否则返回 `ErrBadScale`；SetOrientation 只接受 0、90、180、270，其他角度返回 `ErrBadOrientation`。
6. 没有待处理变更时 Rebuild 返回 `ErrNoPendingResize`；MarkOutOfDate 必须能强制一次重建，重建沿用当前尺寸。
7. Rebuild 的帧号必须严格递增，同一帧只能重建一次；只要帧号没有比上一次大就一律返回 `ErrFrameOrder`（即使当前也没有待处理变更），而且这次失败不能改动当前链和代数。
8. 请求的图像数要裁剪到 MinImages 与 MaxImages 之间，偏小取最小值，偏大取最大值。
9. 每次成功重建代数加一，旧链进入退役集合；Current 返回新链，Generation 返回新代数，Retired 返回尚未释放的退役代数数量。
10. Release 只接受尚未释放的退役代数；当前代数、未知代数、重复释放都返回 `ErrStaleGeneration`，Retired 计数要同步变化。
11. Snapshot 保存逻辑尺寸、缩放、方向、待处理标记、当前链和退役链；用不同 caps 恢复返回 `ErrConfigMismatch`；恢复后代数只能继续递增，绝不能回到 0 重来。
12. 并发请求必须合并：20 个并发 resize 事件之后只能有一个待处理尺寸，紧接着的一次 Rebuild 只产生一代。

硬约束：一是顺序与幂等，同帧只允许一次重建、帧号严格递增，重复 resize 事件合并而不是逐个处理；二是资源上限，尺寸和图像数都不得越过 surface caps；三是并发与原子性，多线程灌 resize 事件时待处理状态必须一致；四是重启恢复，代数与退役链要能跨进程接上，否则会复用仍在显示的旧链。

验收：在模块根目录执行 `go test ./...`，15 条用例必须全过；不得修改测试、跳过或针对固定输入写死返回值；并发 resize 和快照恢复场景自行构造验证。只做本地实现，不涉及上游仓库。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
