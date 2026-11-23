# GSB Bugfix Task

## 题面

赛季结算前夜，值班同学发现天梯榜上停打了三周的账号分数纹丝不动，而只隔一天没打的账号却被扣了分；同一场比赛的战报被重复投递时，分数直接加了两次；一名被判定开挂的玩家需要回滚最近一场比赛的加分，接口却把三周前那场也退了，衰减扣掉的分还被补了回来。客服翻了近三个月的工单，发现同类投诉在三个区服反复出现，每次都靠人工补分了事。修复范围在模块 gsb/batch46-game-a-01 的 rating_decay_guard.go，类型 RatingDecay，任务类型 Bugfix，接口与语义按下面执行。

NewRatingDecay(cfg Config) *RatingDecay、NewRatingDecayFromSnapshot(cfg Config, data []byte) (*RatingDecay, error)、RegisterPlayer(p PlayerRating) error、ApplyMatch(playerID string, m MatchResult, now time.Time) (int, error)、ApplyDecay(playerID string, now time.Time) (int, error)、PendingDecay(playerID string, now time.Time) int、RevertMatch(playerID, matchID string, now time.Time) (int, error)、Rating(playerID string) (int, error)、History(playerID string) []MatchResult、CloseSeason(seasonID string) error、SeasonClosed(seasonID string) bool、Snapshot() ([]byte, error)。

一、重复的比赛号即使已被回滚也必须返回 ErrDuplicateMatch，分数与历史都不能再变；ApplyMatch 返回结算后的分数。二、空玩家号或空赛季号注册返回 ErrInvalidPlayer，同一玩家重复注册返回 ErrPlayerExists，未知玩家返回 ErrUnknownPlayer，未注册玩家的 PendingDecay 返回 0。三、衰减按 Config.DecayInterval 的整数倍结算，不足一个周期的零头不计，同一时刻第二次调用必须扣 0。四、衰减不得把分数压到 Config.MinRating 以下，返回值是实际扣掉的分数；PendingDecay 只预测不落账，必须与随后 ApplyDecay 的返回值相等。五、单场失败同样受 MinRating 约束，回滚只还原当时真正生效的分值：105 分吃一场 -400 实际只掉 5 分，回滚返还 5 分。六、回滚只针对最近一场未回滚的比赛，越序返回 ErrOutOfOrder，已回滚返回 ErrAlreadyReverted，未知或已被淘汰返回 ErrNothingToRevert，返回值是本次实际改动的分值且取正数。七、已结算的衰减不因回滚退回：先加 100 再衰减 36，回滚后应净得 1464，这笔账要能逐笔对上。八、每个玩家的历史最多保留 Config.HistoryCap 场，超出淘汰最旧，被淘汰的比赛不能再回滚，未知玩家的 History 返回空。九、并发提交同一比赛号只允许一个成功，其余返回 ErrDuplicateMatch，分数只加一次。十、赛季关闭后加赛、衰减、回滚、注册一律返回 ErrSeasonClosed，Rating 仍可读，重复关闭同一赛季不报错。十一、Snapshot 之后从快照恢复的实例，分数、衰减锚点、已用比赛号、历史和关闭状态必须一致，重启后重放旧比赛仍被拒，恢复后同一时刻再次衰减必须为 0。十二、以上读写要能被多个请求同时调用，不得出现数据竞争或丢失更新。上线前用同一批战报做一次全量重放，分数必须与人工核对表完全一致。

验收：go test ./... 的 15 条必须全过；不得改测试、不得跳过、不得按固定输入写死。并发与重启自行验证；只做本地实现，不涉及其他目录。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
