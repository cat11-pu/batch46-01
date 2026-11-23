# GSB Bugfix Task

## 题面

公会开荒前一晚，组队频道被同一条邀请刷屏：同一个人对同一目标连点邀请，目标看到十几条一模一样的弹窗；有人已经被拉进队伍，邀请还能继续点；拒绝之后立刻又能收到新邀请；队长掉线换人之后邀请权限还挂在旧队长身上；跨服的好友也被拉进本服队伍，进本后直接报错。团长要求邀请面板必须干净、可撤销、能追溯，每个目标同一时间只看到该看到的几条，接受和拒绝都要立刻生效。模块路径 `gsb/batch46-game-b-09`，根目录文件 `party_invite_guard.go`，类型 `PartyInviteGuard`，任务类型 Bugfix。骨架里的 `Config`、`Invite`、`Party` 与错误值已给出，方法体都是桩。

要实现的公开接口：

```go
func NewPartyInviteGuard(cfg Config) *PartyInviteGuard
func NewPartyInviteGuardFromSnapshot(cfg Config, data []byte) (*PartyInviteGuard, error)
func (g *PartyInviteGuard) CreateParty(partyID, leader, realm string, now time.Time) error
func (g *PartyInviteGuard) Invite(partyID, inviter, invitee, realm string, now time.Time) (Invite, error)
func (g *PartyInviteGuard) Accept(inviteID string, now time.Time) (Party, error)
func (g *PartyInviteGuard) Decline(inviteID string, now time.Time) error
func (g *PartyInviteGuard) Cancel(partyID, invitee string, now time.Time) error
func (g *PartyInviteGuard) PendingFor(invitee string) []Invite
func (g *PartyInviteGuard) PartyOf(player string) (Party, bool)
func (g *PartyInviteGuard) Leader(partyID string) (string, error)
func (g *PartyInviteGuard) Promote(partyID, newLeader string) error
func (g *PartyInviteGuard) Leave(player string, now time.Time) error
func (g *PartyInviteGuard) Expire(now time.Time) int
func (g *PartyInviteGuard) Snapshot() ([]byte, error)
```

规则与边界：

- `CreateParty` 重复队伍号返回 `ErrDuplicateParty`；邀请只能由队长发出，否则返回 `ErrNotLeader`；邀请自己返回 `ErrSamePlayer`；邀请目标的 `Realm` 与队伍 `Realm` 不同返回 `ErrCrossRealm`。
- 队伍人数达到 `MaxSize` 返回 `ErrPartyFull`；目标已在其他队伍返回 `ErrPlayerBusy`。
- 同一队伍对同一目标的待处理邀请只允许一条，重复返回 `ErrInvitePending`；目标待处理邀请达到 `MaxPendingPerInvitee` 返回 `ErrInboxFull`。
- 邀请有效期是 `InviteTTL`；`Accept` 过期邀请返回 `ErrInviteExpired` 并移除；未知邀请号返回 `ErrInviteNotFound`。接受成功后加入队伍，并清掉该目标其它待处理邀请。
- `Decline` 移除邀请并对该发件人到该目标开启 `Cooldown`，冷却内再邀请返回 `ErrCooldown`；过期与主动取消不算拒绝，不产生冷却。`Expire(now)` 清理所有过期邀请并返回条数。
- `PendingFor` 按创建时间升序返回该目标的待处理邀请副本。
- `Leave` 对不在队伍的人返回 `ErrNotInParty`；队长离开时，队长位置交给剩余成员中最早加入的一位，成员顺序保持加入顺序；空队伍解散。
- `Snapshot` 要恢复队伍、成员、待处理邀请与冷却；恢复后重复邀请仍返回 `ErrInvitePending`，接受仍可用。
- 并发接受同一目标的两条邀请只能成功一条，另一条返回错误，最终只进入一个队伍，该目标的待处理列表清空。
- 邀请号由实现生成且全局唯一；入队判断以接受时的实际人数和队伍状态为准，不能沿用发邀请时的旧快照。
- `Cancel` 撤回本队对该目标的一条待处理邀请，没有可撤回的邀请时返回 `ErrInviteNotFound`；撤回不产生冷却。

硬约束：邀请去重与入队必须原子；进程重启后去重集合与冷却状态不能丢；待处理数量有硬上限；并发接受只能有一个赢家。

验收：`go test ./...` 的 15 条测试全部通过，不得修改测试、跳过或用固定输入写死结果；并发和重启场景自行验证；只做本地实现，不涉及上游提交与平台操作。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
