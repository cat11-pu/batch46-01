# GSB Bugfix Task

## 题面

帧同步对战房间上线后接连收到回放对不上的投诉：同一份录像在两台机器上复盘，从第 37 帧开始双方看到的暴击结果不同，客服换台机器复现又变成第三种结果。运营已经把回放一致性列为赛季验收的硬指标，再出一次就要回滚版本。排查后确认随机数服务只有一条全局递增序列，战斗、掉落、刷怪共用，某个模块多抽一次，后面所有模块的结果就整体错位；热更新重启后序列从头开始，录像校验直接判失败；断线重连用旧存档回滚，回滚后抽到的值和原局不一样；还有一局里某条流抽了上千万次，把进程内存吃满。请修复模块 `gsb/batch46-game-c-01` 中文件 `deterministic_rng_guard.go` 的类型 `DeterministicRng`，任务类型 Bugfix。

每条流按名字独立维护游标、内部状态和摘要，流的初始状态只能由配置种子和流名决定：同一份种子、同一串流名，无论在两个进程里以什么顺序抽、中间插了多少别的流，同一条流的第 N 次结果必须完全一致；不同流互不影响。需要实现并保持下列公开接口：
- `NewDeterministicRng(cfg Config) *DeterministicRng`、`NewDeterministicRngFromSnapshot(data []byte) (*DeterministicRng, error)`
- `AddStream(name string) error`：空名字返回 `ErrInvalidStream`，重名返回 `ErrStreamExists`，超过 `Config.MaxStreams`（0 按 32）返回 `ErrStreamLimit`
- `Next(stream string) (uint32, error)`、`DrawRange(stream string, low, high int32) (int32, error)`：未知流返回 `ErrUnknownStream`；区间反向返回 `ErrRangeInvalid`；结果必须落在闭区间 [low, high]，low 等于 high 时直接返回该值且不消耗抽取
- `Consumed(stream string) (uint64, error)`、`StreamDigest(stream string) (uint64, error)`、`ResetStream(stream string) error`
- `Checkpoint() (Checkpoint, error)`、`Restore(cp Checkpoint) error`、`Fork(src, dst string) error`
- `Snapshot() ([]byte, error)`

线上一个房间会同时跑几十条流，抽取频率很高，单次抽取不能随着流数量线性变慢。具体故障和边界：1）每流抽取达到 `Config.MaxDrawsPerStream` 后，下一次必须返回 `ErrDrawBudget`，游标停在限额上不再前进；2）`Checkpoint` 要同时记录每条流的游标、内部状态和摘要，`Restore` 后按原序列继续抽必须逐值复现，`Consumed` 和 `StreamDigest` 也要回到快照时的值；3）同一份 `Checkpoint` 只能成功恢复一次，第二次必须返回 `ErrStaleCheckpoint`；4）`Fork` 以源流当前状态派生新流，目标名已存在返回 `ErrStreamExists`，源流不存在返回 `ErrUnknownStream`，派生不得推进源流游标，目标从未抽取状态开始，相同父状态下派生结果必须确定；5）`ResetStream` 后该流的游标为 0、摘要和后续序列都要与新建的同名流一致；6）不存在的流在 `Next`、`DrawRange`、`Consumed` 上都要报 `ErrUnknownStream`；7）八个协程各自抽自己负责的流时不得互相污染，也不得因为共享状态出现死锁，每条流最终的摘要必须和单线程顺序抽取一致；8）`Snapshot` 落盘后再恢复，重启后的游标和后续取值必须与未重启时完全一致；9）单条流被重置时，其他流的状态、游标和摘要一律不能变化。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过或者按固定输入写死结果；并发与重启场景自行构造验证；只做本地实现，不涉及上游提交；改动只限该文件，任务里列出的方法签名不得更改。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
