# GSB Bugfix Task

## 题面

竞技射击的延迟补偿上线后，论坛出现两类帖子：一类说准星明明压在头上却判定未命中，另一类是“我已经躲到墙后两秒了还被一枪带走”。抓包显示客户端和服务端的 tick 相差几十到几百毫秒，服务端只拿最新世界状态做射线检测；历史帧保存过久导致内存持续上涨；断线重连后回溯到已被淘汰的帧，命中结果直接丢失。值班同学对照比赛日志发现，射手端显示命中而服务端判空的比例在跨区对局里超过三成，延迟补偿窗口由策划配置，超出窗口的申诉一律按服务端当前帧裁决。请修复模块 `gsb/batch46-game-c-03` 中文件 `lag_compensation_guard.go` 的类型 `LagCompensation`，任务类型 Bugfix。

服务端按 tick 保存世界快照，校验射击时回退到射手当时看到的帧做射线检测。需要实现并保持下列接口：
- `NewLagCompensation(cfg Config) *LagCompensation`、`NewLagCompensationFromSnapshot(data []byte) (*LagCompensation, error)`
- `RecordFrame(tick uint32, entities []EntityState) error`、`Rewind(tick uint32) (WorldFrame, error)`
- `ClampClientTick(clientTick uint32) (uint32, error)`
- `EntityAt(tick uint32, id string) (EntityState, error)`、`Raycast(tick uint32, origin, dir Vec3, maxDistance float64) (RaycastHit, error)`
- `FrameCount() int`、`OldestTick() (uint32, bool)`、`NewestTick() (uint32, bool)`
- `MemoryBytes() int`、`Prune(beforeTick uint32) int`、`Snapshot() ([]byte, error)`

射线检测的输入是射手上报的起点、方向和最大距离，方向允许不是单位向量，命中结果要能还原到当时的帧坐标。必须处理的具体问题：1）帧 tick 必须严格大于上一帧，重复或回退返回 `ErrTickOutOfOrder`；同一帧里实体 ID 重复返回 `ErrDuplicateEntity`，实体数超过 `MaxEntitiesPerFrame` 返回 `ErrFrameTooLarge`，空 ID、非正半径或非有限坐标返回 `ErrInvalidEntity`，被拒绝的帧不能改变历史；2）历史只保留最近 `HistoryFrames` 帧，超出的最旧帧被淘汰，`OldestTick` 随之前移，`Rewind` 被淘汰的 tick 返回 `ErrTickExpired`，比最新帧还新返回 `ErrTickInFuture`；3）`Rewind` 返回的是冻结副本，调用方改动返回的实体不能污染历史；4）`EntityAt` 读指定历史帧，不存在的实体返回 `ErrEntityNotFound`；5）`Raycast` 只与存活实体求交，返回命中实体的 `EntityID`、`Distance` 和 `Point`，全为尸体时返回 `ErrNoHit`，方向为零或 `maxDistance` 非正返回 `ErrInvalidRay`，距离相同时按实体 ID 字典序取最小；6）`ClampClientTick` 把客户端 tick 夹到 [最新 tick 减 `MaxRewindTicks`, 最新 tick] 区间，窗口内原样返回，没有任何帧时返回 `ErrNoFrames`；7）`MemoryBytes` 必须随保留帧数有界，历史满后不能继续增长（每条实体按 64 字节加 ID 长度计量即可）；8）`Prune` 删除早于给定 tick 的帧并返回删除数量，之后 `OldestTick` 必须反映剩余历史，不能留下空壳帧；9）记录与回溯可能来自不同协程，必须并发安全，读到的帧要么完整要么不存在；10）`Snapshot` 保存配置和保留的历史，重启后 `FrameCount`、`OldestTick`、`Rewind` 的结果与重启前一致；11）同一帧内的实体顺序不做保证，但每次 `Rewind` 返回的实体集合和位置必须与该 tick 记录时完全相同。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过或者按固定输入写死结果；并发与重启场景自行构造验证；只做本地实现；内存计量和射线精度要能被重复复算，方法签名不得更改。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
