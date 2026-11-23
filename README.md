# GSB Bugfix Task

## 题面

专用服务器在周末活动期间出现明显的时间线问题：一波团战卡了 400 毫秒后，主循环连续补跑十几帧，紧接着又卡死；低优先级的网络同步任务连续几帧被跳过却没有任何告警；暂停比赛后恢复，计时器一次性补出暂停期间的所有 tick；进程重启后 tick 计数归零，客户端预测全部作废。监控显示 tick 延迟的 P99 在活动开始后从 18 毫秒涨到 90 毫秒，技能释放明显发粘；任务数量在活动期还会动态增减，注册与注销不能破坏正在执行的 tick 计划。请修复模块 `gsb/batch46-game-c-04` 中文件 `tick_scheduler_guard.go` 的类型 `TickScheduler`，任务类型 Bugfix。

调度器把真实经过的时间换算成固定频率的 tick，并按优先级执行到期任务。需要实现并保持下列接口：
- `NewTickScheduler(cfg Config) *TickScheduler`、`NewTickSchedulerFromSnapshot(data []byte) (*TickScheduler, error)`
- `Register(id string, priority int, interval uint32) error`、`Unregister(id string) error`
- `Advance(elapsedMicros int64) (TickPlan, error)`
- `RunTick(tick uint32, exec func(id string) int64) (TickResult, error)`
- `DueTasks(tick uint32) []string`、`TickCount() uint32`
- `Pause() error`、`Resume() error`、`Starved() []string`
- `Snapshot() ([]byte, error)`

预算单位是微秒，`exec` 返回该任务本 tick 的实际耗时；调度结果必须可复算，不能依赖 map 遍历顺序。必须处理的具体问题：1）`Advance` 按 `TickRate` 换算 tick，不足一个 tick 的余数结转到下一次，连续两次 150 毫秒和 60 毫秒在 10Hz 下应得到两个 tick 和 10 毫秒余数；2）单次补帧最多执行 `MaxCatchUpTicks` 个 tick，多出的计入 `DroppedTicks`，`TickPlan` 的 `Ticks` 只列出实际要执行的最后几帧、`RemainderMicros` 记录结转余数，`TickCount` 仍要推进到真实经过的 tick 数；3）负的 `elapsedMicros` 返回 `ErrClockRewound` 且不改变状态；4）任务 ID 重复返回 `ErrTaskExists`，间隔为 0 返回 `ErrInvalidInterval`，任务数超过 `MaxTasks` 返回 `ErrTaskLimit`，空 ID 返回 `ErrInvalidTask`；5）到期判定为 `tick >= NextTick` 且 `(tick - NextTick) % Interval == 0`，任务注册后从当前 tick 起算；6）`RunTick` 按优先级从高到低执行，同优先级按注册顺序，`Ran` 记录实际执行顺序，`exec` 返回该任务消耗的微秒数；预算 `BudgetMicros` 用尽时，当前任务和其后的任务都记入 `Skipped`，`OverBudget` 置真，本次已执行任务的耗时汇总到 `CostMicros`；7）被跳过的任务 `SkipStreak` 加一，连续跳过达到 `StarveAfterTicks` 后出现在 `Starved`（按 ID 字典序），成功执行后清零；8）`Pause` 后 `Advance` 不产生 tick 也不累积时间，`Resume` 从当前余数继续；重复暂停返回 `ErrAlreadyPaused`，未暂停时恢复返回 `ErrNotPaused`；9）`Unregister` 未知任务返回 `ErrUnknownTask`，注销后不再出现在到期列表；10）`Snapshot` 要保存 tick 计数、余数、暂停标志和每个任务的 `NextTick` 与跳过计数，重启后 `DueTasks` 和后续 `Advance` 结果与重启前一致；11）注册、注销、推进和读取可能并发发生，必须保证任务表与 tick 计数一致，不能出现同一任务被执行两次或整表丢失；12）暂停期间到达的 `Advance` 不改变余数，恢复后的第一帧必须从暂停时的余数继续，不能把暂停时长折算成补帧。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过或者按固定输入写死结果；并发与重启场景自行构造验证；只做本地实现；调度顺序、跳过数量和重启后的计划要能复算，方法签名不得更改。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
