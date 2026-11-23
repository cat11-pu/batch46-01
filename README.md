# GSB Bugfix Task

## 题面

直播频道上周末切到 1080p 之后，香港节点的观众反复报同一个故障：播到第三分钟卡一次，恢复后音画不同步，客户端日志里能看到分片序号跳号、节目时间戳回退，同一片还被重复拉起了两次。值班同学把播放列表拉下来对照，发现码率阶梯、分片入窗、不连续点计数和直播延迟各算各的，切换档位时播放器根本收不到不连续信号。

请在模块 `gsb/batch46-game-h-01` 的 `hls_playlist_guard.go` 中补全 `HlsPlaylist` 类型，任务类型为 Bugfix，包名 task，测试文件不要改。对外签名保持骨架现状，至少包括：

- `NewHlsPlaylist(cfg Config) *HlsPlaylist`
- `AddVariant(v Variant) error`
- `SelectVariant(bandwidth, heightCap int) (Variant, error)`
- `SetVariant(uri string, now time.Time) error`
- `AppendSegment(seg Segment) error`
- `MediaSequence() (uint64, error)`
- `DiscontinuitySequence() uint64`
- `Stats() WindowStats`
- `TrimWindow(max int) (int, error)`
- `Snapshot() ([]byte, error)` / `NewHlsPlaylistFromSnapshot(cfg Config, data []byte) (*HlsPlaylist, error)`

需要修掉的现象与边界：

1. 4Mbps 链路上选中了 5.2Mbps 的 1080p，起播就卡；选档必须取带宽上限以内的最高档，不许越界。
2. 带了 720p 的高度上限仍然选到 1080p，高度约束要同时生效。
3. 所有档位都超过实测带宽时静默回落到最低档，实际应返回 `ErrNoVariantFit`，由上层决定是否停播。
4. 同一 URI 注册两次没有被拒绝，阶梯里出现重复项，档位计数虚高；应返回 `ErrDuplicateVariant`，空 URI 或带宽为 0 返回 `ErrInvalidVariant`。
5. 配置 4 秒目标时长时，4.5 秒的分片照样入窗，播放器缓冲被拖垮；超过目标时长或时长非正一律 `ErrSegmentDuration`，且不入窗。
6. 媒体序号从 100 直接跳到 102 被接受，缺失的 101 再也补不回来；必须严格等于上一片加一，否则 `ErrSegmentOrder`。
7. 同序号同 URI 的分片重复 reload 被当作新片追加，窗口出现重复；应返回 `ErrDuplicateSegment`，窗口长度不变。
8. 直播窗口不按配置上限裁剪，频道的长跑进程内存一路上涨；入窗后只保留最新 MaxWindow 片，`TrimWindow` 也只能在配置上限内收窗，传 0 或超过上限返回 `ErrWindowTooLarge`。
9. 切换档位不递增不连续序号，后续分片沿用旧值，播放器不重置解码器；切换成功后序号加一，新入窗分片带上新值。
10. 切到未注册的 URI 时当前档位和不连续序号都被改了；应返回 `ErrUnknownVariant` 且状态完全不动。重复选中当前档位是幂等操作，不得再加序号。
11. 新片节目时间戳比上一片结束时刻还早 500 毫秒也被接受；超过容差要返回 `ErrPDTRegression`，容差以内的重叠允许通过。
12. `LiveEdgeDelay` 在本地时钟超前直播边缘时返回负数，监控因此告警；负值一律归零。

硬性要求：窗口、统计和序号在并发读取下必须看到同一个完整快照，不能读到裁剪到一半的数据；进程重启后从快照恢复，媒体序号、不连续序号、窗口内容和重复判定都要延续，重启后重复分片仍要被拒绝；窗口上限是硬资源约束，不能靠调用方自觉。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过用例或按固定输入写死结果；并发和重启场景自行验证；只做本地实现，不涉及上游提交。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 地狱
