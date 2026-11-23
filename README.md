# GSB Bugfix Task

## 题面

活动首日商城对账不平：客户端超时重试导致同一笔兑换扣了两次金币，宝石只到账一次；有玩家先把金币冻结，等汇率上调后再结算，凭空多拿了一截；还有账号把当日额度用满后反复冻结再释放，额度居然回到满值，可以无限兑换。运营手工补了三笔差额，但当晚又出现新的重复扣款。风控要求把兑换、冻结和额度一起修好，范围是 gsb/batch46-game-a-02 的 currency_exchange_guard.go，类型 CurrencyExchange，任务类型 Bugfix。

接口：NewCurrencyExchange(cfg Config) *CurrencyExchange、NewCurrencyExchangeFromSnapshot(cfg Config, data []byte) (*CurrencyExchange, error)、OpenAccount(accountID string) error、Credit(accountID, currency string, amount int64) error、Balance(accountID, currency string) (Balance, error)、SetRate(from, to string, num, den int64, updatedAt time.Time) error、Quote(from, to string, amount int64, now time.Time) (Quote, error)、Exchange(req ExchangeRequest, now time.Time) (ExchangeResult, error)、Freeze(req HoldRequest, now time.Time) (Hold, error)、SettleHold(requestID string, now time.Time) (ExchangeResult, error)、ReleaseHold(requestID string, now time.Time) error、ExpireHolds(now time.Time) int、DailyUsed(accountID, currency string, now time.Time) int64、Snapshot() ([]byte, error)。

契约：一、币种必须是三位大写字母，否则 ErrInvalidCurrency；金额必须为正，否则 ErrInvalidAmount；账号不存在返回 ErrUnknownAccount；OpenAccount 重复调用是幂等的，未开通的币种余额按 0 处理。二、汇率有方向，只登记 GLD 到 GEM 时反向报价返回 ErrRateNotFound，不得自动取倒数。三、汇率发布超过 Config.MaxRateAge 返回 ErrStaleRate，同币种兑换返回 ErrSameCurrency，Quote 不得改动任何余额，对来源币种与目标币种的校验同样生效。四、兑换按 amount*Num/Den 向下取整得到 Gross，再按 Config.FeeBasisPoints 从 Gross 扣手续费，Net 等于 Gross 减 Fee；1000 GLD 按 2:1、250 基点应得 2000、50、1950，费用与净额都要出现在结果里方便对账。五、同一 RequestID 相同载荷重放返回原结果并只扣一次，载荷不同返回 ErrIDConflict，缺 RequestID 返回 ErrMissingRequestID；冻结请求以同一 RequestID 重复提交相同载荷时返回原冻结单，不得再冻一次。六、余额不足返回 ErrInsufficientFunds，余额与额度占用都不得有任何变动。七、冻结把金额从 Available 移入 Frozen，Total 不变；冻结时锁定的 Gross、Fee、Net 用于结算，之后改汇率不影响结算结果。八、日额度按账号、来源币种和 UTC 自然日统计，兑换与冻结共用额度，释放或过期必须退还到冻结当天的额度桶里。九、超过 Config.HoldTTL 未结算的冻结，SettleHold 返回 ErrHoldExpired 且保持冻结，ExpireHolds 统一释放并返回数量，已释放或已结算的请求返回 ErrHoldNotFound；释放或结算已失效的冻结同样返回 ErrHoldNotFound。十、并发兑换不得把余额扣成负数；同一账号的兑换与冻结交替并发时，可用余额、冻结金额与当日额度三者的不变量不能被破坏，对账脚本会以多线程方式抽检上百个账号。十一、Snapshot 恢复的实例必须保住余额、冻结、汇率、额度用量和已结算请求号，重启后重放同一 RequestID 不能再扣一次，冻结仍按原报价结算，恢复后的日额度读数必须与恢复前相同。十二、所有读写必须能被并发调用，不得出现数据竞争。

验收：go test ./... 的 15 条全过；不得改测试、不得跳过、不得按固定输入写死。并发与重启自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
