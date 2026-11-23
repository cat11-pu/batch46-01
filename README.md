# GSB Bugfix Task

## 题面

竞技场新赛季的 BP 阶段连续出现三次申诉：同一名英雄被两边先后 ban 了两次，红方在蓝方回合就提交了选择，客户端断线重连后旧动作又被重放了一遍，最后两边各拿到四个英雄，观战端看到的时间线和服务器记录也对不上。赛事组要求把征召流程做成强一致的状态机，所有提交都以服务端回合为准，重连、重放和超时都不能改动已经落定的结果。模块路径 `gsb/batch46-game-b-07`，根目录文件 `pvp_draft_guard.go`，类型 `PvpDraft`，任务类型 Bugfix。骨架已有 `Config`、`Champion`、`Action`、`DraftState` 等类型和错误值，方法体都是桩。

要实现的公开接口：

```go
func NewPvpDraft(cfg Config, champions []Champion, now time.Time) (*PvpDraft, error)
func NewPvpDraftFromSnapshot(cfg Config, champions []Champion, data []byte) (*PvpDraft, error)
func (d *PvpDraft) Submit(a Action, now time.Time) (DraftState, error)
func (d *PvpDraft) State() DraftState
func (d *PvpDraft) Timeout(now time.Time) (DraftState, error)
func (d *PvpDraft) Deadline() time.Time
func (d *PvpDraft) Available(role string) []string
func (d *PvpDraft) Champions() []Champion
func (d *PvpDraft) History() []Action
func (d *PvpDraft) Snapshot() ([]byte, error)
```

规则与边界：

- 征召顺序是先 ban 后 pick，双方轮流，蓝方先手；`BansPerTeam` 与 `PicksPerTeam` 来自 `Config`。当前阶段与当前行动方要能从 `DraftState` 读到（`Phase`、`Turn`、`Team`、`Bans`、`BluePicks`、`RedPicks`），`Turn` 从 1 起，`Bans` 按提交顺序累计。
- `Action.Kind` 必须是 ban 或 pick；ban 阶段提交 pick、pick 阶段提交 ban 都返回 `ErrInvalidPhase`。
- 行动方不是当前回合方返回 `ErrNotYourTurn`；英雄不在池里返回 `ErrUnknownChampion`；重复 ban 已 ban 的英雄返回 `ErrAlreadyBanned`；pick 已被 ban 或已被 pick 的英雄返回 `ErrChampionTaken`。
- 动作号幂等：已接受过的 `Action.ID` 再次提交返回 `ErrDuplicateAction`，这条判断要排在回合与英雄校验之前，重放不能推进回合；`History` 只保留被接受的动作。
- 每个被接受的动作把 `Deadline` 重置为 `now+TurnTimeout`；未到截止时刻调用 `Timeout` 返回 `ErrTurnNotExpired`，到点后自动按英雄 ID 升序取第一个可用英雄完成当前 ban 或 pick，动作号记为 `auto-<Turn>`，并照常推进回合、写入历史。
- 全部 ban 和 pick 完成后 `Phase` 变为 done，再提交任何动作返回 `ErrDraftClosed`。
- `Available(role)` 返回未被 ban、未被 pick 的英雄 ID，按字典序升序；`role` 为空表示全部职责；返回值是副本。
- `State`、`Champions`、`History` 都返回副本，调用方改写不影响内部状态。
- `Snapshot` 恢复后 `Phase`、`Turn`、当前行动方、已 ban 与已 pick 列表、`History`、`Deadline` 以及已接受的动作号都要保留；恢复后重放旧动作号仍返回 `ErrDuplicateAction`，并且能继续提交后续动作。
- 并发提交同一回合的多个动作只能有一个成功，其余返回错误，回合只前进一格。
- 每个英雄在整个征召里最多被 ban 一次、被 pick 一次；超时自动选择的英雄同样占用名额，后续任何人不能再选。
- 客户端提交里带的时间戳只用于审计和展示，回合是否超时一律以传入的服务端时间为准。

硬约束：回合推进与动作落账必须原子，同一回合的并发提交只能有一个赢家；重启后状态、截止时间与动作号集合保持一致；动作号幂等；已完成征召拒绝一切后续动作。

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
