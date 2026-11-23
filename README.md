# GSB Bugfix Task

## 题面

上周五的排位赛里，一支队伍同时上了两个影魔，赛后结算发了双倍金币，客服一晚上接了十几条投诉。复盘发现选人阶段的校验只活在前端，服务端从来没有拦过重复英雄、超编和禁用名单。这单挂在 `gsb/batch46-game-b-01` 模块，逻辑都在 `team_builder_guard.go` 的 `TeamBuilder` 上，任务类型 Bugfix：把选人阶段的整套校验补齐，让非法阵容根本进不了匹配队列。

需要保持以下签名不变：`NewTeamBuilder(rules Rules) *TeamBuilder`、`AddMember(m Member) error`、`RemoveMember(playerID string) (Member, error)`、`SetHero(playerID, heroID string) error`、`SetReady(playerID string, ready bool) error`、`SetCaptain(playerID string) (string, error)`、`Roster() []Member`、`Validate() error`、`Revision() int64`、`ReplaceAll(members []Member, expectedRevision int64) error`、`Snapshot() ([]byte, error)`、`NewTeamBuilderFromSnapshot(rules Rules, data []byte) (*TeamBuilder, error)`。错误值直接用包里声明好的 ErrRosterInvalid、ErrDuplicateMember、ErrDuplicateHero、ErrHeroBanned、ErrRoleLimit、ErrRosterFull、ErrMemberNotFound、ErrStaleRevision、ErrNotReady，调用方要靠它们区分“重复英雄”和“禁选英雄”。

选人规则：AddMember 只负责入队，传入的 Captain 标记一律忽略，队长只能通过 SetCaptain 指定；SetHero 设成同一个英雄、SetReady 设成同一个值都算幂等，不推进 Revision；Validate 依次检查空阵容、重复英雄、职业上限、RequireCaptain 下缺少队长、有人未准备好，命中哪条就返回对应的错误值；ReplaceAll 保留调用方传入的 Captain 标记，但整表最多允许一个队长，多出来的直接拒绝。

要修的具体问题：

1. 同一个 PlayerID 重复入队没被拦住，第二次提交会把第一个人覆盖掉。
2. 两名成员能锁同一个英雄，前端只弹了一个倒计时提示。
3. BannedHeroes 里的英雄照样能选进来。
4. 人数到 Rules.Size 之后还能继续入队，结算按人头发奖励。
5. MaxPerRole 只在前端算，服务端不看，三个辅助也能进阵容。
6. SetHero 换英雄被拒时把旧英雄一起清了，玩家重连发现英雄没了。
7. RemoveMember 对不存在的玩家不报错，还把空成员塞回列表。
8. 队长可以同时存在多个，SetCaptain 也不告诉调用方上一任是谁。
9. 被拒绝的入队、重复设置同一个值也会让 Revision 自增，乐观锁失效。
10. ReplaceAll 在 revision 不匹配时仍然改阵容；中途发现重复英雄会留下半套新成员。
11. 快照恢复后队长标记和 revision 丢失，重启后旧客户端能拿过期 revision 覆盖阵容。
12. 快照超过 256 KiB 或内容损坏时，恢复函数返回一个半成品对象而不是错误。

硬约束：所有写操作在同一把锁内完成，64 名玩家并发入队时每人只占一个位置、Revision 必须恰好等于成功次数；重复设置同一个英雄、同一个 ready 值属于幂等操作，不涨 Revision；ReplaceAll 先整表校验再一次性替换，失败时阵容和 Revision 保持原样，成功只加一；Roster 返回按 PlayerID 升序的副本，调用方改返回值不能影响内部状态；快照上限 256 KiB，超限或解析失败一律 ErrRosterInvalid。

验收：`go test ./...` 全部通过，现有测试不许改、不许跳过、不许按固定输入写死；并发和重启恢复要自己跑过；只做本地实现，不碰上游队列服务。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
