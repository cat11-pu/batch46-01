# GSB Bugfix Task

## 题面

客服后台收到一批举报，运营同学准备封号时发现证据链对不上：同一个截图被重复上传了四次，每条证据的摘要都是上线时补的，谁也说不清哪条先到；有位玩家一天之内往三个举报里塞了十几段录像，把存储打满；已经结案的举报还在收新材料；更尴尬的是清理任务把两条还在申诉期的证据一起删了，复核时拿不出原始记录。法务要求每条证据都能按摘要和顺序复现，谁上传、什么时候上传、归属哪个举报都要查得到，过期的只按结案时间清理。模块路径 `gsb/batch46-game-b-08`，根目录文件 `report_evidence_guard.go`，类型 `EvidenceGuard`，任务类型 Bugfix。骨架里的结构体、错误值和接口都已就位，行为都是桩。

要实现的公开接口：

```go
func NewEvidenceGuard(cfg Config) *EvidenceGuard
func NewEvidenceGuardFromSnapshot(cfg Config, data []byte) (*EvidenceGuard, error)
func (g *EvidenceGuard) OpenReport(id, reporter, accused, reason string, sev Severity, now time.Time) error
func (g *EvidenceGuard) CloseReport(id string, now time.Time) error
func (g *EvidenceGuard) AddEvidence(reportID string, e Evidence, now time.Time) (ChainEntry, error)
func (g *EvidenceGuard) GetEvidence(reportID, evidenceID string) (Evidence, error)
func (g *EvidenceGuard) Chain(reportID string) []ChainEntry
func (g *EvidenceGuard) VerifyChain(reportID string) error
func (g *EvidenceGuard) Report(id string) (Report, bool)
func (g *EvidenceGuard) ReporterDailyCount(reporter string, day time.Time) int
func (g *EvidenceGuard) TotalBytes(reportID string) int64
func (g *EvidenceGuard) PurgeExpired(now time.Time) int
func (g *EvidenceGuard) Snapshot() ([]byte, error)
```

故障与边界：

- `OpenReport` 的 `Severity` 只接受 1、2、3，其余返回 `ErrInvalidSeverity`；举报号重复返回 `ErrDuplicateReport`。
- 向不存在的举报加证据返回 `ErrUnknownReport`；已结案举报返回 `ErrReportClosed`；证据 ID 为空或摘要为空返回 `ErrEvidenceInvalid`。
- 同一举报内证据 ID 重复、或 SHA-256 内容重复，都返回 `ErrDuplicateEvidence`，条数与字节数不变；查询不存在的证据返回 `ErrEvidenceMissing`。
- `MaxEvidencePerReport` 限制单举报条数，`MaxTotalBytes` 限制单举报总字节数，任一超限返回 `ErrQuotaExceeded` 且不落链；`ReporterDailyQuota` 按举报人加 UTC 自然日统计所有举报的成功上传条数，超限同样返回 `ErrQuotaExceeded`。
- 链的 `Seq` 从 1 起，`Hash` 为 SHA-256(`PrevHash|ReportID|EvidenceID|SHA256|Size` 十进制) 的十六进制，字段用竖线连接；首条 `PrevHash` 为 64 个 0；`VerifyChain` 重算失败返回 `ErrChainBroken`。
- `CloseReport` 幂等，重复结案返回 nil；未知举报返回 `ErrUnknownReport`，结案时间以第一次为准。
- `PurgeExpired` 只清理结案超过 `Retention` 的举报及其证据与链，返回清理的举报数；未结案举报永久保留，已清理的举报再查返回不存在。
- `Chain` 返回副本，改写返回值不能影响内部链，也不能让 `VerifyChain` 失败。
- `Snapshot` 要恢复举报、证据、链与每日计数；恢复后重复证据 ID 和重复内容仍被拒，`VerifyChain` 仍通过，举报人当日计数不变。
- 并发上传下条数上限必须原子：单举报上限为 3 时，8 个并发上传只能成功 3 个，链长恰好 3 且校验通过。
- 链上每条记录都要带举报号、证据号、摘要、字节数和上传时间，序号连续，不能出现跳号或并发写入造成的乱序。

硬约束：条数、字节数、链追加必须在一次事务里完成；进程重启后去重信息与链完整；单举报与举报人两级配额都有硬上限；并发上传不能超发。

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
