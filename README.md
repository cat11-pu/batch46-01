# GSB Bugfix Task

## 题面

版本更新后体力投诉集中爆发：离线十小时回来只补了一小段，公告写的是 8 小时封顶；充值购买的体力在满体力时直接蒸发；扣体力请求超时后重试，体力被扣了两次；客服把上限从 120 调到 130，当前体力被顺手补满，玩家认定是漏洞。客服先按 120 的上限手工补发，第二天同类投诉又堆了几十单。范围是 gsb/batch46-game-a-05 的 energy_regen_guard.go，类型 EnergyRegen，任务类型 Bugfix。

接口：NewEnergyRegen(cfg Config) *EnergyRegen、NewEnergyRegenFromSnapshot(cfg Config, data []byte) (*EnergyRegen, error)、RegisterPlayer(p Player) error、Regen(playerID string, now time.Time) (Grant, error)、PendingPoints(playerID string, now time.Time) int、NextPointIn(playerID string, now time.Time) time.Duration、State(playerID string, now time.Time) (State, error)、Spend(req SpendRequest, now time.Time) (SpendResult, error)、Refill(req RefillRequest, now time.Time) (RefillResult, error)、RaiseCap(playerID string, newMax int) error、Snapshot() ([]byte, error)。

契约：一、注册时玩家号为空、体力为负、体力高于上限或储备越界返回 ErrInvalidPlayer，重复注册返回 ErrPlayerExists，未知玩家返回 ErrUnknownPlayer；State 是只读视图，调用它不能改变任何玩家数据。二、每满一个 Config.RegenInterval 只涨一点，不足一个周期的零头保留到下次，结算后锚点必须推进且剩余时间不能丢。三、离线补偿上限是 Config.MaxOfflineSpan，超出部分的点数直接作废，同一时刻连续结算第二次必须为 0，公告口径与配置必须一致。四、恢复先补体力条，满了进储备桶，储备桶按 Config.ReserveCap 封顶，再多的丢弃并在 Grant 的 Dropped 里报告。五、PendingPoints 只预测不落账，必须等于紧接着 Regen 的 ToEnergy、ToReserve 与 Dropped 之和；NextPointIn 在整点时应等于一个完整周期，未知玩家的这两个读数都返回 0。六、时钟回拨时 Regen、Spend、Refill 一律返回 ErrClockSkew 且不做任何改动，客服的补发记录也不能被回拨污染。七、Spend 先结算恢复再扣费，优先扣体力条其次扣储备；金额为零或负数返回 ErrInvalidAmount 且不得触发恢复结算，缺 RequestID 返回 ErrMissingRequestID，总量不足返回 ErrInsufficientEnergy，但此前结算出的恢复仍然生效。八、同一 RequestID 相同载荷重放返回原结果且不重复扣费，也不重新结算恢复，结果里的剩余体力要与原结果一致；载荷不同返回 ErrIDConflict。九、Refill 先结算恢复，再按体力条、储备桶、丢弃的顺序分配，ToEnergy、ToReserve、Dropped 之和必须等于申请数量，满体力时购买的体力必须进储备或明确计入 Dropped，不得静默蒸发。十、RaiseCap 只能上调且不得补满当前体力，非法上限返回 ErrInvalidCap。十一、并发扣费不得把体力扣成负数，同一账号的多次并发请求也不得丢失恢复点数。十二、Snapshot 恢复后体力、储备、恢复锚点和已结算请求号必须一致，继续按原锚点计时，重启后重放旧请求不能二次扣费、也不能凭空多补一次，审计记录要能逐一对上。

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
