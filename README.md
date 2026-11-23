# GSB Bugfix Task

## 题面

团本结束后掉落分配被投诉：一件掉两枚的宝石全被同一个角色拿走；需求 roll 打平后系统选人结果每次都不一样，复盘对不上；有人在分配完成后补交需求，把已发出的装备又算了一遍；团长重复点击关闭，第二次直接把分配记录清空。公会仓库管理员把三周的分配记录导出来核对，发现同类争议每周都有。范围是 gsb/batch46-game-a-04 的 loot_distribution_guard.go，类型 LootDistributor，任务类型 Bugfix。

接口：NewLootDistributor(cfg Config) *LootDistributor、NewLootDistributorFromSnapshot(cfg Config, data []byte) (*LootDistributor, error)、OpenRun(run Run) error、AddLoot(runID string, items []Item) error、SubmitClaim(runID string, c Claim) error、Resolve(runID string) ([]Award, error)、Awards(runID string) ([]Award, error)、AwardCount(runID, memberID string) (int, error)、MemberClaims(runID, memberID string) ([]Claim, error)、CloseRun(runID string) error、RunClosed(runID string) bool、Snapshot() ([]byte, error)。

契约：一、空团队返回 ErrEmptyRaid，成员号重复返回 ErrDuplicateMember，团队号重复返回 ErrRunExists，未知团队的操作返回 ErrRunNotFound，RunClosed 对未知团队返回 false。二、掉落数量必须为正否则 ErrInvalidQuantity，同一团队内物品号唯一否则 ErrDuplicateItem，加入顺序就是分配顺序。三、成员或物品不存在分别返回 ErrUnknownMember、ErrUnknownItem；同一成员对同一物品重复出价返回 ErrDuplicateClaim；出价类型越界返回 ErrInvalidChoice；MemberClaims 按提交顺序返回该成员的全部出价，放弃出价同样记录并可查询。四、需求与贪婪的 roll 必须在 1 到 100，否则 ErrInvalidRoll；放弃出价不需要 roll。五、成员资历低于物品 MinSeniority 时提交需求返回 ErrNotEligible，但允许贪婪。六、每份掉落先看需求出价，没有需求才轮到贪婪；排序按 roll 从高到低、资历从高到低、成员号字典序从小到大，打平时必须给出唯一确定的胜者。七、数量大于一的掉落要分给不同成员，同一成员的出价对同一物品只能用一次，数量为一的掉落只产生一条分配记录。八、每人每团的获奖数受 Config.MaxItemsPerMember 限制，按掉落顺序逐个结算，超限成员跳过并把机会顺延；AwardCount 只统计发给该成员的数量，不含公会仓库，统计口径要与导出报表一致。九、无人出价或全部超限的掉落记给 Config.GuildBankID，Reason 必须是 bank；正常分配的 Reason 分别为 need 和 greed，Roll 与 Choice 要反映获胜出价；分配结果一经生成就不能被后续出价改写。十、Resolve 重复调用必须返回完全相同的分配结果，并发调用返回的每一条记录都要逐字段相同；分配完成后补交出价或加掉落返回 ErrAlreadyResolved。十一、CloseRun 会先完成分配再冻结团队，关闭后出价与加掉落返回 ErrRunClosed，重复关闭不报错，Awards 仍可读。十二、Snapshot 恢复后分配结果、出价、获奖计数和关闭状态必须一致，恢复后再次关闭不得改写已发出的分配，导出的报表要能逐条复现。

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
