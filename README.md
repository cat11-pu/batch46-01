# GSB Bugfix Task

## 题面

昨晚八点 DASH 源站换了一次编码配置，随后客服收到集中投诉：同一部剧在 720p 和 1080p 之间来回切换时，画面会先黑一下再卡两秒，进度条回到切点之前；下载器按时间段拉片，拉回来的分片时间戳对不上，最后两分钟永远播不到。我们核对 MPD 后发现，两条 representation 的时间轴参数（timescale、分段时长、StartNumber、presentationTimeOffset）完全不同，播放端却按“段号相同就是同一时刻”来对齐，问题就出在这里。

请在模块 `gsb/batch46-game-h-02` 的 `dash_segment_guard.go` 中补全 `DashSegment` 类型，任务类型为 Bugfix，包名保持 task，测试文件不要修改。需要保持的公开签名包括：

- `NewDashSegment(cfg Config) *DashSegment`
- `AddRepresentation(rep Representation) error`
- `SegmentAt(repID string, number uint64) (SegmentRef, error)`
- `SegmentForTime(repID string, t time.Duration) (SegmentRef, error)`
- `SegmentNumberAt(repID string, t time.Duration) (uint64, error)`
- `SegmentDuration(repID string) (time.Duration, error)`
- `AlignFrom(repID string, from time.Duration, count int) ([]SegmentRef, error)`
- `SwitchRepresentation(fromID, toID string, at time.Duration) (SegmentRef, error)`
- `Stats() Stats` / `Snapshot() ([]byte, error)` / `NewDashSegmentFromSnapshot(cfg Config, data []byte) (*DashSegment, error)`

需要修正的具体现象与边界：

1. timescale 为 90000、分段时长 360000 的 720p 时间轴，段号 1 对应的起止时间被算成了 0 秒到 4 秒，实际应从 presentationTimeOffset 的 10 秒开始，到 14 秒结束。
2. 配了 1000 timescale、1500 分段时长的音频轨，`SegmentDuration` 返回 0，`SegmentAt` 的边界也全是整数秒；必须按 tick 换算成 1.5 秒。
3. 段号小于 StartNumber 或超出声明的段数仍然被解析出来，URL 拼成了不存在的分片；这两种情况都返回 `ErrSegmentOutOfRange`。
4. 请求时刻早于 presentation 起点时被当成第 0 段处理；应返回 `ErrBeforeStart`，与越界区分开。
5. 请求时刻正好等于最后一段的结束时刻时返回了最后一段，实际这个时刻已经没有分片；应按 `[start, end)` 左闭右开处理并返回 `ErrSegmentOutOfRange`。
6. 时间段读取一次返回 5 段，直接把整个 20 分钟的时间轴拉进内存；单次查询段数必须在配置的预算内，超过预算返回 `ErrQueryTooLarge`，预算为 0 或负数同样拒绝。
7. 时间段读取的起点落在某个分片中间时，返回的段列表从下一段开始，漏掉了包含起点的这一段。
8. 分片数声明为 11 的 representation 被注册成功，实际配置上限是 10，后面的段号解析全部漂移；超过上限、timescale 或分段时长为 0、StartNumber 为 0 都要返回 `ErrInvalidRepresentation`。
9. 同一 ID 注册两次没有报错，第二次的参数悄悄覆盖了第一次，已经缓存的段号全部错位；应返回 `ErrDuplicateRepresentation` 且保留第一次的注册。
10. 从 720p 切到 1080p 时直接沿用同一个段号，25 秒的位置切到了 1080p 第 25 段（实际该轨第 25 段在 154 秒），必须对齐到目标轨道在切点之后最近的段边界。
11. 切点落在源轨道时间轴之外（比如起点之前或最后一段之后）时仍然完成了切换，当前档位被改成了一个还没开始播放的表示；应返回 `ErrBeforeStart` 或 `ErrSegmentOutOfRange`，选中状态不变。源和目标是同一路时返回 `ErrSameRepresentation`。
12. 未注册的 representation 被查询或作为切换目标时直接返回空结构，调用方无从判断；应返回 `ErrUnknownRepresentation`，并且失败的查询不能计入统计。

硬性要求：切换与查询并发进行时，任一查询返回的分片都必须真的覆盖请求时刻，不能读到切换过程中的半更新状态；进程重启后从快照恢复，注册的轨道、时间轴参数、当前选中档位和统计计数必须延续，恢复后同一时刻仍解析出同一段号；单次查询的段数是硬预算，不能靠调用方自己截断。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过用例或按固定输入写死结果；并发与重启场景自行验证；只做本地实现，不涉及上游提交。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 困难
