# GSB Bugfix Task

## 题面

总决赛第三局，官方直播间比现场晚了半分钟，可弹幕里已经有人把最后一波团战的结果打了出来；同一天还有观众反馈，暂停后重连会被重复推送十几条旧击杀，另一批人则整段错过了赛点。复盘发现观战服把事件直接广播给所有连接，既没有延迟窗口，也没有游标和缓冲上限。这单挂在 `gsb/batch46-game-b-03` 模块，逻辑在 `spectator_delay_guard.go` 的 `SpectatorDelay` 上，任务类型 Bugfix：把按场次的延迟队列、游标和重新同步补起来。

需要保持以下签名不变：`NewSpectatorDelay(cfg Config) *SpectatorDelay`、`NewSpectatorDelayFromSnapshot(cfg Config, data []byte) (*SpectatorDelay, error)`、`RegisterSpectator(matchID, spectatorID string, now time.Time) error`、`Unregister(spectatorID string) error`、`Publish(ev Event) error`、`Next(spectatorID string, now time.Time, max int) ([]Packet, error)`、`Resync(spectatorID string, now time.Time) error`、`Pending(spectatorID string) (int, error)`、`SetDelay(spectatorID string, d time.Duration) error`、`Snapshot() ([]byte, error)`。错误值使用包里已声明的 ErrInvalidSeq、ErrConflictingEvent、ErrSpectatorExists、ErrSpectatorNotFound、ErrTooManySpectators、ErrResyncRequired、ErrDelayRange。

队列规则：事件序号必须为正，且随 ServerTime 单调不减；同一个序号重复发布且内容完全一致算幂等，内容不同则拒绝；每个事件的放行时间是 ServerTime 加上该观众当前的延迟，放行时间小于等于 now 才能下发；Next 只从游标之后按序号升序连续取，遇到第一条还没到放行时间的事件就停，max 小于等于 0 表示不限量，游标只跟着已下发的包前进；新观众从注册时刻缓冲区里最大的序号开始，入房之前的事件永远不再补发；注册时先判同一 ID 是否已存在，再判该场人数是否达到上限。

要修的具体问题：

1. 延迟参数完全没生效，观众在结果公布前就收到了击杀事件。
2. 同一事件被重复发布两次会推送两遍，客户端列表出现重影。
3. 同一序号被换成另一条内容重新发布也照样进入队列。
4. 事件乱序到达时按到达顺序下发，击杀排在了助攻后面。
5. 一次拉取把整个缓冲区全返回，前端一次渲染上百条。
6. 游标在拉取失败时也前进，丢掉的包再也补不回来。
7. 中途进房的观众被补发了开赛以来的全部历史事件。
8. 缓冲区满了以后静默丢掉最旧的事件，观众毫无察觉地少看一段。
9. 单场观战人数没有上限，热门赛事把进程内存打满。
10. 同一观众重复注册会生成两条游标，拉取时互相抢事件。
11. 延迟被改成负数或超过 MaxDelay 之后仍然生效。
12. 重启后缓冲区、游标和每个观众的延迟全部清零，重连即重复。

硬约束：观战者的注册、拉取、重新同步和事件发布必须共用同一把锁，10 个并发拉取同一个观众时最终拿到的序号集合必须正好是 1 到 10 且不重复；每个赛事缓冲区最多保留 MaxPending 条，溢出时丢弃最旧一条并把该场所有观众标记为需要重新同步，此时 Next 返回 ErrResyncRequired，只有 Resync 丢弃该场未消费的积压、把游标移到缓冲区最新序号并清掉标记后才能继续；重启恢复后游标、缓冲区和延迟表必须与重启前一致，恢复后新发布的事件照常下发；单场观众上限为 MaxSpectators，超过返回 ErrTooManySpectators，不允许按需扩容。

验收：`go test ./...` 全部通过，现有测试不许改、不许跳过、不许按固定输入写死；并发拉取和重启恢复要自己跑；只做本地实现，不接真实推流服务。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
