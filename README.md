# GSB Bugfix Task

## 题面

拍卖行上周三凌晨出现撮合错乱：同一件装备的卖单被跳过，价格更差的挂单先成交；两笔同价卖单的成交顺序每天不一样，玩家截图对不上账；大单扫货只吃到第一档，剩下的量凭空消失；撤单返回的数量比实际剩余少，撤销后订单还挂在深度里；有玩家用别人的订单号撤单成功；重复提交同一个订单号，两笔都进了簿子；价格不是最小报价单位整数倍的订单也被接收；重启后成交序号从 1 重新开始，深度和成交记录全部丢失。值班同学先冻结了拍卖行入口，但根因还在，任何一笔新挂单都可能继续错配。修改范围在模块 gsb/batch46-game-a-07 的 market_orderbook_guard.go，类型 OrderBook，任务类型 Bugfix。

保持以下公开签名：NewOrderBook(itemID string, tick int64) *OrderBook；Submit(o Order) ([]Fill, error)；Cancel(orderID, player string) (int64, error)；BestBid() (int64, int64, bool)；BestAsk() (int64, int64, bool)；Depth(side Side, levels int) []Level；Remaining(orderID string) (int64, error)；Trades() []Fill；OpenOrders(player string) []string；Snapshot() ([]byte, error) 和 NewOrderBookFromSnapshot(data []byte) (*OrderBook, error)。

具体要求：
1. 买价大于等于卖价才成交，成交价取挂单方价格；卖单同理，不能让吃单方决定成交价。
2. 同价按进入簿子的先后成交，先挂先成交，不能用随机或 map 遍历顺序。
3. 部分成交后剩余量继续挂单，Remaining 返回剩余量；全部成交的订单从 OpenOrders 消失。
4. 不交叉的限价单直接入簿，不产生成交，也不能改动对手方盘口；空盘口时 BestBid 与 BestAsk 的 ok 必须是 false。
5. 一次提交跨越多个价位时按价格优先逐档吃单，Fill 的 Seq 从 1 单调递增，TradeID 不得重复；每笔 Fill 的 TradeID、BuyOrderID、SellOrderID、ItemID、Price、Qty 都要填全。
6. 撤单只能由下单玩家发起，否则 ErrNotOwner；成功撤单返回剩余量并移出深度，再次操作返回 ErrUnknownOrder。
7. 重复订单号返回 ErrDuplicateOrder，且不得改变盘口与成交记录。
8. 数量非正、价格为非正或非 tick 整数倍、ItemID 与簿子不符、ID 或玩家为空的订单返回 ErrBadOrder。
9. Depth 按价格聚合，买盘价格降序、卖盘升序，Level 的 Orders 是价位上的订单数；levels 小于等于 0 时返回全部。
10. Trades 保留全部成交，TradeID 唯一；快照恢复后旧成交、未成交挂单、盘口和序号都要延续，新成交的 Seq 不能回退；坏快照返回 ErrSnapshotInvalid。

并发与资源：同一卖单同时被多个买家吃单时，累计成交量必须等于卖单数量，不能超卖或出现负剩余；同一订单号并发提交只能有一笔成功，其余必须拿到 ErrDuplicateOrder。盘口只保留未成交订单，已成交和已撤销订单不得常驻内存，订单号表也不能随重放无限增长。撤单与撮合是同一个事务，撤到一半失败不能留下半条订单。

交付：修改后的源码、go test 通过记录和一段简要说明，写清价格优先、时间优先与快照续号的实现顺序。

验收：go test ./... 15 条全过，不改测试、不跳过、不按固定顺序写死；并发撮合和重启恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
