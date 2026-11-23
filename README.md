# GSB Bugfix Task

## 题面

充值退款接入自动审核后，财务发现部分账号的宝石余额被反复加回：同一个退款单号提交两次，两次都入账；已经全额退过的订单还能再退一次，退款金额超过了订单本身；有客服把 A 玩家的订单退到了 B 玩家账上；账户被扣成负数后仍然能继续消费，欠款越滚越大；更严重的是退款把余额打到负五千以下，超过了公司允许的垫付上限，财务无法对账。现在自动审核已经暂停，只保留人工退款，但代码里的问题必须一次修清。退款流水会实时同步到财务系统，任何一次重复入账都是真实资损，修复前需要先把判定顺序写清。修复范围在模块 gsb/batch46-game-a-08 的 refund_ledger_guard.go，类型 RefundLedger，任务类型 Bugfix。

需要保留的公开接口：NewRefundLedger(debtFloor int64) (*RefundLedger, error)；Grant(o Order) error；Spend(player string, amount int64) error；Refund(req RefundRequest) (int64, error)；Repay(player string, amount int64) error；Balance(player string) int64；Order(orderID string) (Order, error)；Refunds(orderID string) []Refund；RefundByID(refundID string) (Refund, bool)；DebtLocked(player string) bool；Snapshot() ([]byte, error) 及 NewRefundLedgerFromSnapshot(data []byte) (*RefundLedger, error)。

修复清单：
1. Grant 按订单给玩家入账，重复订单号返回 ErrDuplicateOrder，金额非正或玩家为空返回 ErrBadAmount。
2. Spend 只能在余额充足时扣款，余额不足返回 ErrInsufficientCredits，扣款失败不能改动余额。
3. 退款按订单累计，超过订单金额未退部分返回 ErrRefundExceeds；部分退款多次累计，Order 返回的 Refunded 要同步。
4. 退款可把余额打成负数，但不得低于 -debtFloor；越界返回 ErrFloorBreach，且余额、订单已退额、退款记录都不能变；正好等于下限要允许。
5. 退款必须校验订单归属，Request 的 Player 与订单不符返回 ErrOrderOwner；订单不存在返回 ErrUnknownOrder。
6. RefundID 幂等：相同订单、玩家、金额重放返回当前余额且不重复入账；同 ID 换金额或换订单返回 ErrRefundIDConflict。
7. 余额为负时 DebtLocked 为真，Spend 返回 ErrDebtLocked；Repay 入账后余额归零即解锁，之后按余额判断。
8. Refunds 按发生顺序返回该订单的退款记录，RefundByID 能按单号查回，退款记录要带 Reason 与 At。
9. 快照要包含下限、余额、订单及已退额、退款记录和幂等键；恢复后重复提交同一个 RefundRequest 仍不得重复入账；坏快照返回 ErrSnapshotInvalid。

并发与上限：同一订单的退款在并发下不能超过订单金额，20 笔 100 的退款打向 500 的订单只能成功 5 笔；余额与已退额必须原子更新，不能出现负数超限或记录与余额不一致。每个订单的退款记录随退款次数增长，已全额退款的订单不得再产生新记录，重复提交也不能把记录数刷大。退款与扣款共用同一本账，读到的余额必须是最新值，金额统一按整数处理，不做浮点换算。

交付：修改后的源码、go test 通过记录和一段简要说明，讲清退款幂等与下限判断的先后顺序。

验收：go test ./... 15 条用例全过，不得修改测试、跳过或写死输入；并发退款与重启恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
