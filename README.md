# GSB Bugfix Task

## 题面

限时卡池上线当天，运营发现宝石账目对不上：同一笔扣费回调重复到达，玩家被扣了两次宝石；余额不足时扣费仍然成功，余额直接变成负数；有玩家用脚本在一天内刷了上百次十连，日限额配置形同虚设；客服给玩家补发的退款把已经退过的订单又退了一遍；赠送宝石买的礼包也走了退款通道；服务重启后余额、当天消耗和限次全部回到初始值。卡池已经紧急下线，运营手工补发受影响的账号，客服只能按订单号逐笔核对，效率极低。代码必须一次修完，修复范围在模块 gsb/batch46-game-a-10 的 gem_sink_guard.go，类型 GemSink，任务类型 Bugfix。

公开方法签名保持不变：NewGemSink(refundWindow time.Duration) (*GemSink, error)；TopUp(player string, gems int64, ref string) error；Balance(player string) int64；Spend(req SpendRequest) (int64, error)；Refund(entryID, player string, now time.Time) (int64, error)；Entry(entryID string) (Entry, error)；Entries(player string) []string；DailySpent(player, day string) int64；SetDailyCap(player string, cap int64) error；Snapshot() ([]byte, error) 与 NewGemSinkFromSnapshot(data []byte) (*GemSink, error)。

需要满足的细节：
1. TopUp 按 ref 幂等，相同玩家同一 ref 同一金额重复提交只入账一次；同 ref 不同金额或玩家返回 ErrTopUpRefConflict；金额非正返回 ErrBadAmount。
2. Spend 校验 EntryID、Player、Feature 非空且金额为正，否则 ErrBadSpend；余额不足返回 ErrInsufficientGems，余额与当日消耗都不变。
3. Spend 按 EntryID 幂等：同内容重放返回当前余额，不重复扣款、不新增 Entry；同 ID 不同内容返回 ErrEntryIDConflict。
4. 成功扣费要写入 Entry（EntryID、Player、Amount、Feature、Refundable、At、Refunded），Entries 按扣费顺序返回。
5. SetDailyCap 设置每日上限，0 表示不限制；当日累计消耗加上本次金额超过上限返回 ErrDailyCap，被拒的扣费不计入当日消耗。
6. 日限额按 UTC 日切，DailySpent 用 YYYY-MM-DD 查询，跨天自动重置，不能沿用前一天的累计值。
7. Refund 只允许原玩家操作，订单不存在返回 ErrUnknownEntry，他人订单返回 ErrEntryOwner，被拒退款不能改动余额。
8. 只有 Refundable 的 Entry 能退；赠送宝石产生的扣费退款返回 ErrNotRefundable，不能只看余额是否足够。
9. 退款必须在 refundWindow 内，超时返回 ErrRefundWindowClosed；同一条 Entry 只能退一次，重复退返回 ErrAlreadyRefunded；退款成功后余额加回、Entry 标记 Refunded、当天消耗相应减少。
10. 快照保存余额、Entry、退款标记、每日消耗和限额；恢复后重复提交同一 EntryID 仍不重复扣费；坏快照返回 ErrSnapshotInvalid。

并发与资源：余额 100 时 30 个并发扣费各 10 宝石，只能成功 10 个，余额恰好为 0，Entry 数量与当日消耗必须一致；并发退款同一 Entry 只能成功一次，不能出现负余额或重复入账。Entry 与每日统计只随真实扣费增长，重放请求不得留下任何新记录，内存不能随重试次数无限膨胀。每次扣费、退款与限额判断都要在同一临界区内完成读改写，不能先查余额再扣款。

交付：修改后的源码、go test 通过记录和一段简要说明，讲清幂等键、日限额与退款窗口的判定顺序。

验收：go test ./... 的 15 条用例必须全过，不得修改测试、跳过用例或按固定输入写死；并发扣费与重启恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
