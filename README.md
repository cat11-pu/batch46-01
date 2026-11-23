# GSB Bugfix Task

## 题面

固定团本周换了新团长，开打前名单还在改：治疗超编也能塞进来，替补席满了还在往下压人，锁名单之后照样能加人，打完的 boss 换个人又能再打一次拿双份；有人趁换人窗口把已经替补的输出和治疗对调，结果治疗数量悄悄变成了三个。团长要求名单、职责配额和击杀记录三者对得上，锁定之后的每一次换人都要留下明确结果，重启维护也不能把锁和击杀记录弄丢。模块路径 `gsb/batch46-game-b-10`，根目录文件 `raid_roster_guard.go`，类型 `RaidRoster`，任务类型 Bugfix。骨架里的 `Config`、`Member`、`Slot`、`PullRecord` 和错误值已经就位，方法体都是桩。

要实现的公开接口：

```go
func NewRaidRoster(cfg Config, raidID string) *RaidRoster
func NewRaidRosterFromSnapshot(cfg Config, data []byte) (*RaidRoster, error)
func (r *RaidRoster) Add(m Member, now time.Time) error
func (r *RaidRoster) Bench(name string, now time.Time) error
func (r *RaidRoster) Swap(benchedName, activeName string, now time.Time) error
func (r *RaidRoster) Lock(now time.Time) error
func (r *RaidRoster) Unlock() error
func (r *RaidRoster) Locked() bool
func (r *RaidRoster) Pull(bossID string, now time.Time) (PullRecord, error)
func (r *RaidRoster) UnlockBoss(bossID string) error
func (r *RaidRoster) Roster() []Slot
func (r *RaidRoster) BenchList() []string
func (r *RaidRoster) Attendance(bossID string) ([]string, error)
func (r *RaidRoster) Count(role Role) int
func (r *RaidRoster) Snapshot() ([]byte, error)
```

规则与边界：

- `Add` 在名单锁定后返回 `ErrRosterLocked`；名字重复返回 `ErrDuplicateMember`；活跃人数达到 `Size` 返回 `ErrRaidFull`；对应职责活跃人数达到 `MaxTanks`、`MaxHealers`、`MaxDPS` 返回 `ErrRoleQuota`。
- 职责配额只统计活跃成员；`Bench` 把活跃成员移到替补并释放配额，替补席上限是 `BenchSize`，满了返回 `ErrIllegalBench`；对不在活跃名单的人执行 `Bench` 返回 `ErrNotInRaid`。
- 锁定后的 `SwapWindow` 内允许 `Bench` 和 `Swap`，超过窗口返回 `ErrSwapWindowClosed`。`Swap` 要求第一个参数在替补、第二个在活跃，否则返回 `ErrNotInRaid`；换入后职责超编返回 `ErrRoleQuota`，且双方位置都不能变。
- `Pull` 只能在锁定状态调用，否则返回 `ErrRosterUnlocked`；同一 boss 已记录且未 `UnlockBoss` 时返回 `ErrLockoutActive`；成功后把当时活跃成员按名字升序记入考勤。
- `Attendance` 返回最近一次该 boss 的考勤副本，没打过的 boss 返回 `ErrUnknownBoss`。`Roster` 按职责顺序（坦克、治疗、输出）再按名字升序，替补排在最后，返回副本；`BenchList` 升序返回替补名字。
- `Lock` 记录首次锁定时间，重复调用不改变窗口起点；`Unlock` 解除名单锁，但不解除 boss 击杀记录。
- `Count` 只统计活跃成员。
- `Snapshot` 要恢复活跃名单、替补、锁定状态与锁定时间、boss 记录；恢复后加人仍被拒，重复 `Pull` 仍返回 `ErrLockoutActive`，超出窗口的 `Swap` 仍被拒，窗口内 `Swap` 正常。
- 并发 `Add` 不能超员：`Size` 为 5 时 10 个并发输出位添加只能成功 5 个，其余返回 `ErrRaidFull`。
- 考勤只记录当时的活跃成员，替补不参与分装备也不进考勤；不同 boss 的击杀记录互相独立，解锁一个不影响另一个。
- 任何一次换人要么双方同时生效，要么完全不生效；名单快照要能直接用于重开团队，锁定时间要原样带回。

硬约束：职责配额与人数必须在一次事务里校验，不能先占位后失败；进程重启后锁定时间与击杀记录不能丢；替补与活跃名额互斥；并发添加不超员。

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
