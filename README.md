# GSB Bugfix Task

## 题面

客服录音质检平台最近把大量静音段标成“客户在说话”，质检员被迫逐条回听；实时字幕则相反，一句话说完后的尾音被立刻切断，导致“好的”只剩下“好”；同一段 30 ms 帧长的话务，改采样率后帧计数和占比全部错位，夜班监控看到的曲线每隔几秒就跳一次，质检规则更新后历史统计也跟着漂移。问题定位在 gsb/batch46-game-g-03，主文件 voice_activity_guard.go，类型 VoiceActivity，任务类型 Bugfix。

需要补齐的接口：NewVoiceActivity(cfg Config) (*VoiceActivity, error)、NewVoiceActivityFromSnapshot(cfg Config, data []byte) (*VoiceActivity, error)、FrameSamples() int、Push(samples []float64) (FrameInfo, error)、PushFrames(samples []float64) ([]FrameInfo, error)、IsSpeech() bool、NoiseFloorDB() float64、TotalFrames() int、SpeechFrames() int、SpeechRatio() float64、SetEnergyThreshold(dbfs float64) error、Reset()、Snapshot() ([]byte, error)。错误值使用 ErrInvalidConfig、ErrFrameSize、ErrInvalidThreshold、ErrBadSnapshot。

具体故障与边界：1）SampleRate 必须在 8000..192000、FrameMS 在 10..40、阈值在 -90..0、MinSpeechFrames 至少为 1、HangoverFrames 在 0..100，越界返回 ErrInvalidConfig；2）16 kHz、30 ms 的帧必须正好 480 个样本，FrameSamples 与 Push 的长度校验要一致，长度不符返回 ErrFrameSize 且所有计数保持不变；3）帧能量按 20log10(RMS) 计算，0.5 幅度约为 -6 dBFS，0.001 约为 -60 dBFS，跨阈值才标记 Voice；4）MinSpeechFrames 要求连续多个 Voice 帧才能把 Speech 置真，前两帧只能报告 Voice，第三帧才报告 Speech；5）最后一个 Voice 帧之后，Speech 必须恰好再保持 HangoverFrames 帧，随后一帧立即转假，IsSpeech 同步变化；6）底噪以配置阈值为初值，只跟随非语音帧按 0.9/0.1 的指数滑动更新，100 帧 1e-4 幅度后 NoiseFloorDB 应接近 -80；7）PushFrames 必须一次校验整批长度是帧长的整数倍，不合格时整批拒绝，不能先吃掉前面几帧；底噪只用于诊断展示，不参与门限判定；8）帧序号从 0 连续递增，TotalFrames 只统计成功分析的帧，帧序号在整条流内单调递增，不随批次重置，帧计数不得回绕；9）SpeechRatio 等于 SpeechFrames/TotalFrames，没有帧时返回 0，不能除零；10）SetEnergyThreshold 只接受 -90..0，调整后从下一帧生效，不影响已有计数；11）快照要保存帧序号、连续语音计数、剩余挂起帧数和底噪，恢复后挂起状态不能丢，损坏数据返回 ErrBadSnapshot；12）Reset 清空帧计数、语音状态和占比，并把底噪恢复到配置阈值。Push 与 PushFrames 都不得保留调用方的样本切片；能量恒为 0 的整帧按极低 dBFS 处理，不能产生 NaN；SpeechFrames 只累计 Speech 为真的帧，Voice 为真但还没通过连续帧确认的帧不计入占比；整批被拒绝时 TotalFrames、SpeechFrames 与 SpeechRatio 必须保持原值。

硬约束：同一路流会被实时音频线程和监控线程同时驱动，Push、PushFrames、Snapshot 可能并发进入，必须串行化状态更新且不丢帧；批量接口要在消费任何帧之前完成校验，避免半批提交；单次调用不得保留调用方样本切片，防止上层复用缓冲区后状态被污染。

验收：go test ./... 的 15 条必须全部通过；不得修改测试、跳过或用固定输入写死判定；并发与快照恢复场景自行验证；只做本地实现，不涉及上游提交。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 困难
