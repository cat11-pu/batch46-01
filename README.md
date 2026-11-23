# GSB Bugfix Task

## 题面

拍卖行结算连续两天少给卖家钱：5% 的交易税按整数截断，零头全部抹掉，卖家集体投诉；成交额很小时一分税都不收，成交额很大时又没有上限，单笔上万的交易税把大卖家劝退；同一笔成交被回调重放两次，卖家收到两笔打款；撤销的成交只退了货款没退税款，日结报表和实际税金对不上；服务发布重启后当天累计税金归零，财务报表差了一大截。结算侧已经暂停自动打款，改为人工核对，但代码问题需要当天修完。卖家已经联名要求补偿，运营急需一份能对得上的税金口径，每一笔成交的税和打款都要能追溯到原始订单。问题都集中在模块 gsb/batch46-game-a-09 的 tax_ledger_guard.go，类型 TaxLedger，任务类型 Bugfix。

保留以下接口：NewTaxLedger(rule TaxRule) (*TaxLedger, error)；PreviewTax(gross int64) (int64, error)；Record(t Trade) (Settlement, error)；Payout(seller string) int64；TaxCollected() int64；SellerSettlement(seller string) Settlement；Reverse(tradeID, reason string) (Trade, error)；Trade(tradeID string) (Trade, error)；Trades(seller string) []string；DailyTotals(day string) (int64, int64)；Snapshot() ([]byte, error)；NewTaxLedgerFromSnapshot(data []byte) (*TaxLedger, error)。

要修的行为：
1. 税率按万分比计算，结果四舍五入到整数（half up），不能直接截断，也不能用浮点累计误差。
2. 计算后低于 MinTax 按 MinTax 收，高于 CapTax 按 CapTax 收；RateBps 为 0 时仍收 MinTax，不能出现零税。
3. 构造时校验规则：RateBps 超出 [0,10000]、MinTax 为负、CapTax 非正或小于 MinTax，返回 ErrBadRule。
4. Record 校验 TradeID、Seller、Buyer、ItemID 非空，买卖双方不能同一人，Gross 必须为正，否则 ErrBadTrade。
5. 成交结算 Payout 等于 Gross 减 Tax，并累加到卖家；Record 返回单笔 Settlement（Trades 为 1），不能返回卖家历史汇总。税额与打款必须在同一次事务里写清，不能先给卖家加钱再补税。
6. TradeID 幂等：相同内容重放返回同样的 Settlement，不重复计税金和打款；同 ID 不同内容返回 ErrTradeIDConflict。
7. Reverse 撤销成交，扣回卖家打款和税金；同一笔重复撤销返回 ErrAlreadyReversed，未知成交返回 ErrUnknownTrade。
8. Trades 按入账顺序返回未撤销成交的 ID，SellerSettlement 给出该卖家的 Gross、Tax、Payout、Trades 汇总。
9. DailyTotals 按成交时间的 UTC 日期 YYYY-MM-DD 统计，撤销后当天数字同步减少，空日期返回零。
10. 快照保存规则、成交、撤销标记、卖家汇总和日汇总；恢复后 Payout、TaxCollected、DailyTotals、幂等与撤销状态一致；坏快照返回 ErrSnapshotInvalid。

并发与上限：同一卖家的多笔成交并发入账时，Payout、TaxCollected 和日汇总必须精确等于逐笔之和，不能丢更新；已撤销的成交不得再次计入任何汇总，内存中的成交与税金记录不能因为重放无限增长。金额计算全程使用整数，任何一步都不能因为四舍五入产生负数税金或超过 CapTax。日汇总只统计未撤销成交，撤销动作要同步扣减当天税金与卖家汇总，不能只改卖家余额。

交付：修改后的源码、go test 通过记录和一段简要说明，写清税额取整、上下限与撤销的判定顺序，并说明并发下如何保证合计不丢。

验收：go test ./... 15 条全部通过，不得改测试、跳过或按固定金额写死；并发入账与重启恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
