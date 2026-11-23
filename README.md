# GSB Bugfix Task

## 题面

发布当天的开放世界关卡里，玩家快速转身时脚步声会从左边突然跳到右边，背对声源时左右声道几乎消失；远处瀑布的衰减在跨过某个距离时直接静音，产生明显的开关声；载具经过时引擎音高跳变超过一个八度，听起来像磁带被拉断。值班同学还发现，同一关卡重进后声源位置会回退几个身位，像是混音器一直在用上一帧的缓存。这些问题都集中在场景实例 level-04 的空间音频混音器上，模块路径 gsb/batch46-game-g-01，主文件 spatial_audio_guard.go，类型 SpatialAudio，任务类型 Bugfix。

需要补齐的公开接口是：NewSpatialAudio(sceneID string, cfg Config) *SpatialAudio、NewSpatialAudioFromSnapshot(cfg Config, data []byte) (*SpatialAudio, error)、SetListener(l AudienceListener) error、AddSource(src Source) error、UpdateSource(src Source) error、RemoveSource(id string) bool、Pan(id string) (PanGains, error)、Attenuation(id string) (float64, error)、Doppler(id string) (float64, error)、Mix() (FrameMix, error)、MixOrder() []string、Snapshot() ([]byte, error)。错误必须使用包里已有的 ErrUnknownSource、ErrDuplicateSource、ErrSourceTableFull、ErrInvalidSource、ErrInvalidBasis、ErrNoListener、ErrBadSnapshot。

具体要修的点：1）听者基向量 Forward 为零或与 Up 平行时 SetListener 返回 ErrInvalidBasis，并保留旧姿态；2）声源在听者右侧时全右、左侧时全左，正前方左右各 1/√2；3）方位角以 Forward 为 0°、向右为正，范围 -180..180，仰角以 Up 为 90°；4）背后声源必须标记 Rear，左右增益都收敛到 1/√2，不能整体压到一侧；5）距离小于 MinDistance 时衰减为 1，之后衰减值等于 MinDistance 除以实际距离，达到 MaxDistance 时为 0 并从混音剔除，同时累加 FrameMix.Culled；6）遮挡声源在 FrameMix.Voices 里的 Gain 必须是源增益、距离衰减和 OcclusionGain 三者乘积；7）多普勒因子取 343/(343+v)，v 是声源沿视线远离听者的径向速度，结果钳制在 [0.5, 2.0]，快速掠过时不能出现负值、零或无穷大；8）每帧可听声部受 MaxVoices 限制，按 Priority 降序保留，优先级相同按 ID 升序，落选者按同样顺序写入 Dropped；9）MixOrder 必须与插入顺序无关，连续调用结果一致；10）重复 ID 的 AddSource 返回 ErrDuplicateSource 且不得覆盖旧声源，注册数超过 MaxSources 返回 ErrSourceTableFull；11）Mix 只读取当前状态，连续调用两次结果相同，不得改写声源表；12）快照要保存听者姿态和全部声源，恢复后的 Pan、Attenuation、Mix 与快照前一致，损坏的数据返回 ErrBadSnapshot，恢复失败时不能返回半成品对象。另外，UpdateSource 对不存在的 ID 返回 ErrUnknownSource，RemoveSource 只在真正删除时返回 true；空 ID、NaN 坐标或负增益的 Source 一律返回 ErrInvalidSource；返回的 Voices 与 Dropped 不能引用内部切片，调用方改动后不得影响后续帧。

硬约束有三类：音频线程、网络同步线程和场景加载线程会同时操作同一个 SpatialAudio，AddSource、UpdateSource、Mix、Snapshot 必须可并发调用且不丢更新；场景切换要从快照恢复，不能残留旧场景的声源或姿态；单场景声源上限和每帧声部上限必须严格执行，不能先全量混音再事后裁剪。

验收：go test ./... 的 15 条必须全部通过；不得修改测试、跳过或用固定输入写死结果；并发与快照恢复场景自行验证；只做本地实现，不涉及上游提交。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 困难
