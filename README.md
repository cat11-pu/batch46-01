# GSB Bugfix Task

## 题面

公会仓库在版本更新后被搬空了两格材料，值班同学翻完操作日志才拼出原因：低权限成员点开军官页签也能取货，同一批矿石在每日额度用完之后还能继续取，两个管理员同时点取货，最后几份材料被重复领走；更麻烦的是审计流水存的是拼出来的纯文本，谁都能改一行再存回去，客服拿着流水说不清哪次是真人操作、哪次是事后补录。仓库长要求这次把每个动作都做成可复核的记录，出了问题能按操作号追到人、追到页签、追到具体数量，并且重启维护之后这些记录仍然可信。现在把仓库守卫补齐，模块路径 `gsb/batch46-game-b-06`，根目录文件 `guild_bank_guard.go`，类型 `GuildBankGuard`，任务类型 Bugfix。骨架里的结构体、常量、错误值和接口已经就位，行为都是桩，需要按下面的契约实现。

要实现的公开接口：

```go
func NewGuildBankGuard(cfg Config) *GuildBankGuard
func NewGuildBankGuardFromSnapshot(cfg Config, data []byte) (*GuildBankGuard, error)
func (g *GuildBankGuard) AddMember(m Member) error
func (g *GuildBankGuard) CreateTab(index int, minRank Rank) error
func (g *GuildBankGuard) LockTab(index int) error
func (g *GuildBankGuard) Deposit(opID, member string, tabIndex int, item Item, at time.Time) (AuditEntry, error)
func (g *GuildBankGuard) Withdraw(req WithdrawalRequest) (AuditEntry, error)
func (g *GuildBankGuard) TabContents(index int) ([]Item, error)
func (g *GuildBankGuard) Ledger() []AuditEntry
func (g *GuildBankGuard) VerifyChain() error
func (g *GuildBankGuard) Snapshot() ([]byte, error)
func (g *GuildBankGuard) DailyTotal(member string, day time.Time) int
```

故障与边界：

- 未知成员取货返回 `ErrUnknownMember`，未知页签返回 `ErrUnknownTab`，成员等级低于页签要求返回 `ErrRankDenied`，已上锁页签返回 `ErrTabLocked`，存入和取出都要判。
- 数量必须大于 0，否则返回 `ErrInvalidCount`；超过库存返回 `ErrInsufficientCount`，库存不能扣减。
- 每个成员每个 UTC 自然日的取货总量受 `DailyQuota` 限制，跨多次操作累加；超出返回 `ErrQuotaExceeded`，失败的这次不能扣库存、不能写流水，UTC 零点后额度重新计算。
- 操作号 `opID` 是幂等键：成功执行过的 `opID` 再次提交（无论存取）返回 `ErrDuplicateOp`，库存与流水都不变。
- 同一页签内相同物品 ID 的入库要合并数量并保留原有位置，新物品追加到队尾；库存取到 0 时该堆从页签移除。
- 审计流水序号从 1 递增，每条记录的 `Hash` 为 SHA-256(`PrevHash|OpID|Member|TabIndex|ItemID|Count` 十进制) 的十六进制，字段用竖线连接；首条 `PrevHash` 是 64 个 0，后续 `PrevHash` 必须等于上一条 `Hash`。`VerifyChain` 逐条重算，不一致返回 `ErrChainBroken`。
- 流水条数达到 `LedgerCapacity` 后，新操作返回 `ErrLedgerFull`，且不能产生任何状态变化。
- `Snapshot` 要能恢复成员与等级、页签与库存、流水以及已用过的 `opID`；从快照恢复后重复 `opID` 仍要判 `ErrDuplicateOp`，`VerifyChain` 仍通过。
- 并发下扣减必须原子：同一物品库存只剩 4 份时，10 个并发取 1 份的请求只能成功 4 个，其余返回 `ErrInsufficientCount`，不能出现负库存。
- `TabContents` 和 `Ledger` 返回副本，调用方改写返回的切片不能影响仓库内部状态。
- `DailyTotal` 按成员和 UTC 自然日统计成功取货的数量。
- 任何失败的操作都不能改动库存、额度计数和流水，成功操作则三者同时落账；入库与出库都要写流水，流水里的操作类型要能区分。

硬约束：额度、库存、流水三者必须在一次事务里一起成功或一起失败；进程重启后去重集合与审计链不能断；流水有硬容量上限；并发扣减不能超卖。

验收：`go test ./...` 的 15 条测试全部通过，不得修改测试、跳过或用固定输入写死结果；并发和重启场景自行验证；只做本地实现，不涉及上游提交与平台操作。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
