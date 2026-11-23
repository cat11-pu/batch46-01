# GSB Bugfix Task

## 题面

社区赛秋季杯开打前，运营用后台重新生成了一次 128 人对阵表，结果 1 号种子第二轮就撞上了 2 号种子，轮空名额被当成真实选手填进表里，已经淘汰的人又出现在半决赛。技术复盘发现对阵服务只做了“按人数切两半”，种子顺序、轮空和晋级全是错的。这单挂在 `gsb/batch46-game-b-02` 模块，逻辑在 `tournament_bracket_guard.go` 的 `BracketGuard` 上，任务类型 Bugfix：把单败淘汰赛的建表、轮空和结果上报整套补齐。

需要保持以下签名不变：`NewBracketGuard(size int) (*BracketGuard, error)`、`Seed(entrants []Entrant) error`、`Matches() []Match`、`Match(id string) (Match, error)`、`RoundMatches(round int) ([]Match, error)`、`ReportResult(matchID, winnerID string) error`、`Champion() (string, error)`、`Revision() int64`、`Snapshot() ([]byte, error)`、`NewBracketGuardFromSnapshot(data []byte) (*BracketGuard, error)`。错误值使用包里已声明的 ErrBracketConfig、ErrDuplicateEntrant、ErrSeedConflict、ErrMatchNotFound、ErrNotParticipant、ErrMatchClosed、ErrMatchNotReady、ErrConflictResult、ErrTournamentRunning。

对阵表规则：人数上限只接受 4 到 256 的 2 的幂；槽位序列按标准递推，两人是 1、2，人数翻倍时把上一轮序列里每个种子 s 依次写成 s、N+1-s，所以 4 人是 1、4、2、3，8 人是 1、8、4、5、2、7、3、6；相邻两个槽位组成首轮一场，编号形如 R1-M1，轮次和场次都从 1 开始。报名人数 E 必须大于 N/2 且不超过 N，种子号大于 E 的槽位是轮空，首轮遇到轮空的场次直接记胜者进入下一轮。每场比赛状态只有 pending、ready、bye、done 四种。上报结果时严格按顺序检查：场次不存在、轮空场、两名选手还没凑齐、胜者不是本场参赛者、已有结果但胜者不同，只有全部通过才落结果并推进下一轮。

要修的具体问题：

1. 首轮槽位没按标准种子序排，强手提前相遇，1 号种子第二轮就撞 2 号。
2. 轮空槽位被当成真实选手写进对阵，凭空多出几个“选手”。
3. 轮空场还能被上报结果，把自动晋级的人覆盖掉。
4. 上报的胜者根本不在本场两名选手之间，后台也照单全收。
5. 胜者不跟场次走，第 1 场胜者被填到第 2 场的位置，第 k 场胜者应进下一轮第 (k+1)/2 场，奇数场进主位、偶数场进客位。
6. 下一轮两边选手没凑齐就能上报，空位被填成胜者。
7. 同一场同一胜者重复上报，第二次被当成新结果又加一次 Revision。
8. 已出结果的场次再报另一个胜者，第一次结果被悄悄覆盖。
9. 决赛没打完就能取到冠军，颁奖脚本提前发了奖杯。
10. 重启后对阵结果全丢，已淘汰的选手重新出现在后续轮次。
11. 快照内容损坏或为空时恢复函数还返回半成品，调用方一读就崩。
12. 人数超过 256 或不是 2 的幂也照样建表，内存被无限放大。

硬约束：同一场比赛的并发上报必须原子，16 个请求同时上报同一个胜者时全部返回成功、Revision 只加一次、后续轮次只推进一次；Seed 成功算一次变更，之后每接受一条结果加一，被拒绝或幂等重复的上报不加；重启恢复后旧结果的重复上报仍要幂等，后续轮次状态和冠军判定与重启前一致；单场赛事最多 256 人、最多 255 场，超过上限直接 ErrBracketConfig，不允许按需扩容。

验收：`go test ./...` 全部通过，现有测试不许改、不许跳过、不许按固定输入写死；并发上报和重启恢复要自己验证；只做本地实现，不接真实赛事系统。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
