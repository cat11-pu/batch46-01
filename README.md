# GSB Bugfix Task

## 题面

直播推流的音频链在鼓点密集段落持续削波，导出文件里能听到连续的爆音；切换到 96 kHz 素材后输出延迟从 1.5 ms 涨到几十毫秒，主播口型明显对不上；立体声节目里右声道的人声被压得很小，左声道却还在过载；现场调音台改释放时间时，音频线程偶发卡死，画面跟着丢帧。故障模块是 gsb/batch46-game-g-02，主文件 dsp_limiter_guard.go，类型 DspLimiter，任务类型 Bugfix。

需要实现的接口：NewDspLimiter(cfg Config) (*DspLimiter, error)、NewDspLimiterFromSnapshot(cfg Config, data []byte) (*DspLimiter, error)、ProcessBlock(in, out [][]float64) error、SetCeiling(dbfs float64) error、SetRelease(ms float64) error、LatencySamples() int、GainReductionDB() float64、MaxGainReductionDB() float64、Reset()、Snapshot() ([]byte, error)。错误值使用 ErrInvalidConfig、ErrChannelMismatch、ErrBlockTooLarge、ErrInvalidParameter、ErrBadSnapshot。

具体故障和边界：1）SampleRate、Channels、ReleaseMS、CeilingDBFS 越界必须返回 ErrInvalidConfig，不得带着非法参数启动；2）LookaheadMS 超过 5 ms 时按 5 ms 计算，48 kHz 下延迟固定为 240 帧，1.5 ms 对应 72 帧；3）输入输出声道数必须等于 Channels、块长一致且不超过 MaxBlockSamples，否则分别返回 ErrChannelMismatch、ErrBlockTooLarge 或 ErrInvalidParameter，空块同样拒绝，零长度输入不得进入限幅状态；4）由于前瞻，前 LatencySamples 个输出帧必须为 0，之后低于 ceiling 的信号原样通过；5）任何输出样本的绝对值都不得超过线性 ceiling，包括孤立的 4.0 冲激；6）持续过载必须被压到 ceiling 附近，不能只限一部分；7）GainReductionDB 返回当前平滑增益的 dB 值，过载 4.0、ceiling -0.3 dBFS 时应约为 -12.3 dB；8）MaxGainReductionDB 记住自 Reset 以来最深的一次压缩，后续安静素材不得把它抬高；9）ReleaseMS 控制恢复速度，50 ms 释放下经过 1 秒静音，压缩量要回到 -0.5 dB 以内；10）Reset 清空延迟线、当前增益和最坏值，但保留采样率与延迟设置；11）StereoLink 为真时两个声道共用同一个由最大峰值算出的增益，安静声道也要按比例衰减；关闭时各声道独立，安静声道必须保持原样；12）快照要保存每个声道的延迟线和共享增益包络，恢复后继续处理同一段素材必须逐样本一致，损坏数据返回 ErrBadSnapshot。SetCeiling 和 SetRelease 同样要校验范围，越界返回 ErrInvalidParameter 且保留原参数；ProcessBlock 不得原地改写输入切片，输出切片长度不足时返回错误而不是越界写入；延迟线与峰值窗口按块复用，连续处理 4096 帧的块不应产生按样本的新分配。

硬约束：控制线程会在音频线程处理块的同时调用 SetRelease、SetCeiling 和 Snapshot，所有公开方法必须并发安全，不能出现半更新参数、撕裂的增益或数据竞争；延迟线、滑动峰值窗口和输出缓冲必须复用，处理 4096 帧的块时不得按样本分配内存；快照恢复后要能无缝接续直播，不能从头重新起播。

验收：go test ./... 的 15 条必须全部通过；不得修改测试、跳过或用固定输入写死输出；并发与快照恢复场景自行验证；只做本地实现，不涉及上游提交。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 地狱
