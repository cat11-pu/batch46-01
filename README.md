# GSB Bugfix Task

## 题面

排位赛上线当晚，客服转来一批射击对局申诉：同一发子弹在死亡回放里结算了三次伤害；有人隔着半张地图用手枪点掉了满血对手；同一把步枪在冷却时间内连续打出两枪；队友误伤把自家前排打残；还有玩家被击杀后继续掉血、申诉回滚后血量和击杀数又对不上。联赛裁判组要求每一次击杀都能追溯到具体一枪，申诉必须在十秒内给出可复算的血量。服务端目前的命中模块基本是收到就扣血，没有事务边界。请修复模块 `gsb/batch46-game-c-02` 中文件 `hit_registration_guard.go` 的类型 `HitRegistration`，任务类型 Bugfix。

武器伤害按 `BaseDamage` 乘以部位系数后四舍五入到整数：头部用 `HeadMultiplier`，躯干固定 1 倍，四肢用 `LimbMultiplier`，其他部位视为非法事件。命中结果要带回 `AppliedDamage`、`RemainingHP`、`Killed` 和 `Absorbed`，回滚结果要带回 `RestoredHP` 和 `Revived`。需要实现并保持下列接口：
- `NewHitRegistration(cfg Config) *HitRegistration`、`NewHitRegistrationFromSnapshot(data []byte) (*HitRegistration, error)`
- `RegisterWeapon(p WeaponProfile) error`、`SpawnPlayer(id, team string, hp int) error`、`SetInvulnerable(id string, untilTick uint32) error`
- `Submit(ev DamageEvent) (HitResult, error)`、`SubmitBatch(events []DamageEvent) ([]HitResult, error)`
- `RollbackShot(shotID string) (RollbackResult, error)`
- `Health(id string) (int, error)`、`Player(id string) (PlayerState, error)`
- `Snapshot() ([]byte, error)`

一局比赛里同一名玩家可能同时被多个攻击者命中，回滚又可能在结算后几秒才到达，模块必须在并发下保证血量不出现丢失更新。必须处理的具体问题：1）距离超过武器 `MaxRange` 返回 `ErrOutOfRange`，单次伤害超过 `Config.MaxDamagePerEvent` 返回 `ErrDamageCap`，武器未注册返回 `ErrUnknownWeapon`，攻击者或受害者不存在返回 `ErrUnknownPlayer`，自伤和非法部位返回 `ErrInvalidEvent`；2）同一 `ShotID` 只能生效一次，重复提交返回 `ErrShotReplayed`，且不得扣血、不得推进任何状态；3）每个攻击者维护时间线水位，事件 `Tick` 落后水位超过 `Config.MaxTickRewind` 返回 `ErrStaleTick`，水位校验要排在冷却校验之前；4）每个攻击者加武器独立冷却，`Tick` 小于上次生效 Tick 加 `CooldownTicks` 返回 `ErrCooldown`；5）同队伤害在 `Config.FriendlyFire` 关闭时返回 `ErrFriendlyFire`，开启时正常结算；6）受害者 `Tick < InvulnerableUntil` 时本次命中记为 `Absorbed`、伤害为 0、血量不变，但 `ShotID` 仍被占用；7）受害者血量归零后继续被命中返回 `ErrVictimDead`；8）`SubmitBatch` 要么整批生效，要么整批回滚：任一条非法就返回 `ErrBatchRejected`，血量、冷却、时间水位和已用 `ShotID` 都不能留下中间状态；9）`RollbackShot` 要按该发子弹实际造成的伤害回补血量、必要时复活受害者，并把 `ShotID` 释放、按剩余命中重算该攻击者的时间水位和武器冷却，找不到该发返回 `ErrShotNotFound`；10）同一个 `ShotID` 被并发提交时只能有一次成功，其余必须返回 `ErrShotReplayed`；11）`Snapshot` 保存玩家、武器、已用 `ShotID`、冷却和水位，重启后重放同一发仍要被拒绝，血量保持原值；12）断线重连后客户端可能把同一批事件再上报一次，重复的 `ShotID` 和已回滚的 `ShotID` 都要按当前日志重新判定，不能把事件算两遍。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过或者按固定输入写死结果；并发与重启场景自行构造验证；只做本地实现；回滚后的状态要能再次接受同一 `ShotID` 的合法重提，方法签名不得更改。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
