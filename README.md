# GSB Bugfix Task

## 题面

视频会议室的免提模式上线后，对方总是听到自己的声音以约 150 ms 的延迟返回；双方同时说话时，近端人声被一起压掉，听起来像断线；连续开两小时后进程内存持续上涨；同一台机器上两场会议还会互相串参考信号，后建立的会议甚至复用了前一场的延迟估计。问题集中在 gsb/batch46-game-g-04，主文件 echo_cancel_guard.go，类型 EchoCancel，任务类型 Bugfix。

需要实现的接口：NewEchoCancel(sessionID string, cfg Config) (*EchoCancel, error)、NewEchoCancelFromSnapshot(cfg Config, data []byte) (*EchoCancel, error)、PushReference(samples []float64) error、Process(mic []float64, farEndActive bool) (Result, error)、EstimateDelay() int、Gain() float64、ERLE() float64、ReferenceLen() int、SetDoubleTalkMargin(db float64) error、DoubleTalkFrames() int64、Reset()、Snapshot() ([]byte, error)。错误值使用 ErrInvalidConfig、ErrBlockTooLarge、ErrEmptyBlock、ErrInvalidParameter、ErrBadSnapshot。

具体故障和边界：1）SampleRate 必须在 8000..192000、MaxDelayMS 在 1..500、双讲余量在 1..30 dB、MaxBlockSamples 在 256..8192，越界返回 ErrInvalidConfig；2）PushReference 和 Process 的块长不得超过 MaxBlockSamples，超限返回 ErrBlockTooLarge，块长为零返回 ErrEmptyBlock；3）参考历史必须有上限，最多保留 MaxDelaySamples 加一个最大块的样本，长时间运行不能无限增长；4）纯回声素材中，延迟必须锁定到真实的 150 帧，EstimateDelay 返回 150，Gain 接近回声路径的 0.5；5）回声消除后的 ERLE 至少 30 dB，残余 RMS 要低于麦克风 RMS 的 5%；6）麦克风能量超过预测回声能量加 DoubleTalkMarginDB 时判定双讲，Result.DoubleTalk 置真并累加 DoubleTalkFrames；7）双讲期间延迟和增益必须冻结，不能把近端人声学进回声路径，输出要保留近端信号，与原始近端语音的相关系数不低于 0.95；8）farEndActive 为假时直接透传麦克风块，不估计延迟、不改变增益，ERLE 返回 0；9）没有参考信号时不得产生虚假锁定，EstimateDelay 为 -1、Gain 为 0、输出等于输入；10）每个 session 的状态必须完全隔离，另一场会议推入的参考不能影响本场；11）快照要保存参考历史、锁定延迟和增益，恢复后对同一块输入产生逐样本一致的输出，损坏数据返回 ErrBadSnapshot，损坏快照不得部分加载；12）Reset 清空参考、延迟锁和增益，双讲计数一并归零。SetDoubleTalkMargin 越界返回 ErrInvalidParameter 并保留原值；Process 不得原地修改麦克风切片，输出样本数必须与输入一致，结果切片不能与调用方缓冲共享底层数组；参考历史按时间顺序保存，超出上限时只淘汰最旧样本，不能整段清空或让下标错位。

硬约束：控制线程会在音频线程处理麦克风块的同时调用 Snapshot 和 SetDoubleTalkMargin，两者必须与 Process 并发安全，不能读到半更新的参考历史；参考缓冲必须定长复用，禁止按样本追加导致内存增长；延迟搜索和增益估计必须在单块内完成，不能依赖跨块累积或离线重算。

验收：go test ./... 的 15 条必须全部通过；不得修改测试、跳过或用固定输入写死结果；并发与快照恢复场景自行验证；只做本地实现，不涉及上游提交。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 地狱
