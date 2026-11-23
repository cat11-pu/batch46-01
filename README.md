# GSB Bugfix Task

## 题面

日常任务板的重随功能上线三天，客服工单堆了四十多张：有玩家一天重随了七八次，而配置里每人每天只有三次；有人重随后两个槽位出现同一个任务 ID；被锁住的任务照样被换掉；凌晨过后额度没有恢复，晚班玩家还在报“次数没到账”；更麻烦的是玩家长按按钮发出的重复请求，每次都被当成新重随扣了次数，刷新页面后任务又变了一次。运营已经临时关闭重随入口，但根因还在，任何一次点击都可能继续放大损失。修复范围锁定在模块 gsb/batch46-game-a-06 的 mission_reroll_guard.go，类型 MissionReroll，任务类型 Bugfix，不要改动包外调用方。

公开接口保持下列签名：NewMissionReroll(playerID string, policy Policy) *MissionReroll；PlayerID() string；SeedSlot(slot int, mission Mission) error；Slot(slot int) (Mission, error)；AddToPool(missions ...Mission) (int, error)；PoolSize() int；Lock(slot int) error；RerollsLeft(now time.Time) int；Reroll(slot int, requestID string, now time.Time) (Mission, error)；History(slot int) ([]RerollRecord, error)；RequestResult(requestID string) (Mission, bool)；Snapshot() ([]byte, error) 与 NewMissionRerollFromSnapshot(data []byte) (*MissionReroll, error)。

需要修的点：
1. Policy 的 Slots、DailyBudget、MinTier、MaxTier、PoolCap 必须全部生效，槽位下标小于 0 或不小于 Slots 时返回 ErrBadSlot；Slot 要按播种值返回 ID、Kind、Tier、Reward。
2. 重随成功只扣当天一次额度；额度用尽返回 ErrNoReroll，失败时槽位、历史与当天计数都保持原样。
3. 锁定槽位重随返回 ErrLocked，同样不能扣次数，也不能把锁定状态写丢。
4. 候选任务缺 ID、Kind 为空或 Tier 不在 [MinTier, MaxTier] 返回 ErrBadMission，校验不通过的候选不入池。
5. 入池按 PoolCap 截断，返回本次已接受数量并带 ErrPoolFull；重复 ID 静默跳过，多次调用也只算一条。
6. 重随必须跳过当前任何槽位已在用的任务；没有可用候选返回 ErrPoolExhausted，不扣额度也不写历史。
7. 同一玩家、同一天、同一候选池的重随序列必须可复现，客服才能按日期和请求号复核对账。
8. requestID 幂等：同槽重放返回原任务且不扣次数、不写历史；同一 requestID 出现在别的槽位返回 ErrRequestConflict。
9. 额度按 UTC 日切，跨零点后重置；RerollRecord 要记录 Slot、FromID、ToID、RequestID、Day、At，Day 使用 YYYY-MM-DD。
10. Snapshot 必须保存玩家、策略、当天已用额度、槽位、历史与 requestID 结果；恢复后 RerollsLeft、History、RequestResult 全部一致；坏快照返回 ErrSnapshotInvalid。

并发与边界：多个请求同时重随同一槽位时只能有一个成功，不能出现额度扣成负数或历史重复；不同槽位并发也要共享同一份每日额度。单板状态只随成功重随增长，重放请求不得留下任何新记录，候选池与额度都不允许无限扩容。任务 ID 已经发到客户端，重随返回的空任务会被前端当成“无任务”展示，因此失败路径必须返回明确错误而不是零值。

交付：修改后的源码、go test 通过记录和一段简要说明，讲清额度、幂等与日切的判定顺序。

验收：go test ./... 的 15 条用例全部通过，不得修改测试、跳过用例或按固定输入写死；并发与快照恢复自行补验证；只做本地实现，不涉及上游提交。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
