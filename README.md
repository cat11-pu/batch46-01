# GSB Bugfix Task

## 题面

云游戏串流最近出现规律性抖动：解码帧明明按时到达，画面却每十几帧顿一下；网络抖动后连送两帧，播放器把旧帧和新帧一起显示，造成重影；长时间播放后画面比声音慢半秒，字幕和口型完全错位；暂停再恢复时队列里的旧帧还会闪现一次，统计面板上的展示数也跟着对不上。问题集中在 gsb/batch46-game-g-05，主文件 frame_pacing_guard.go，类型 FramePacer，任务类型 Bugfix。

接口方面需要实现：NewFramePacer(streamID string, cfg Config) (*FramePacer, error)、NewFramePacerFromSnapshot(cfg Config, data []byte) (*FramePacer, error)、Submit(f Frame) error、Next(now time.Duration) (Presentation, error)、Pending() int、PresentedCount() int64、DroppedCount() int64、SetAudioClock(d time.Duration) error、AudioClock() time.Duration、Drift() time.Duration、FrameInterval() time.Duration、Reset()、Snapshot() ([]byte, error)。错误值使用 ErrInvalidConfig、ErrInvalidFrame、ErrDuplicateFrame、ErrQueueFull、ErrNotReady、ErrBadSnapshot。

具体故障和边界：1）RefreshHz 必须在 24..240，MaxPending 在 1..64，LateToleranceMS 在 0..100，AudioResyncThresholdMS 在 10..500，越界返回 ErrInvalidConfig；2）FrameInterval 由刷新率算出，60 Hz 约为 16.666 ms，120 Hz 约为 8.333 ms；3）提交时 ID 必须为正、PTS 不得为负、Duration 必须为正，否则返回 ErrInvalidFrame 且不入队；4）重复 ID 返回 ErrDuplicateFrame，队列达到 MaxPending 返回 ErrQueueFull，已入队的帧不能被挤掉；5）展示时间必须向上对齐到最近的 vsync 边界，例如 60 Hz 下 1 ms 的 PTS 对齐到 16.666 ms，不能提前；6）now 尚未到达展示时间时 Next 返回 ErrNotReady，队列保持不变；到达后按 PTS 升序逐帧展示，即使提交顺序是乱的；7）now 超过展示时间加 LateToleranceMS 的帧必须丢弃并累加 DroppedCount，不能补显示；仍在容差内的迟到帧要正常展示；8）PTS 早于上一帧已展示 PTS 的帧属于过期帧，Submit 仍可先入队，下一次 Next 再把它丢弃并计数；9）设置音频主时钟后，若帧 PTS 与音频时钟相差超过 AudioResyncThresholdMS，展示时间改为对齐音频时钟，Presentation.Resynced 置真，PresentAt 落在音频时钟附近的 vsync 上；10）音频时钟未设置、或漂移在阈值内时不得重锚，Resynced 为假；11）快照要保存待展示队列、计数器和音频时钟，恢复后继续按原顺序出帧，损坏数据返回 ErrBadSnapshot；12）Reset 清空队列、展示与丢弃计数以及音频时钟，下一次 Next 返回 ErrNotReady。Next 每次最多展示一帧，Pending 只统计尚未展示的帧；Submit 不得修改调用方传入的 Frame；快照里的队列必须保持 PTS 顺序，恢复后 Pending、PresentedCount、DroppedCount 与上一帧 PTS 都要接续，不能从头开始计；Drift 用上一帧已展示 PTS 减去音频时钟，尚未展示时返回 0；RefreshHz 在整个允许区间内算出的间隔都必须为正。

硬约束：解码线程、合成线程和音频线程会并发调用 Submit、Next、SetAudioClock 与 Snapshot，队列必须串行化访问，不能出现同一帧被展示两次或计数丢失；队列按 PTS 有序，Next 的丢弃与展示要在一次临界区内完成；待展示帧数严格受 MaxPending 限制，不能靠事后裁剪；快照恢复后要与音频时钟继续对齐，不能从头重置节奏。

验收：go test ./... 的 15 条必须全部通过；不得修改测试、跳过或用固定输入写死时间；并发与快照恢复场景自行验证；只做本地实现，不涉及上游提交。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 困难
