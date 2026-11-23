# GSB Bugfix Task

## 题面

赛事回放平台收到一批可疑对局：提交的录像缺了红方第 96 tick 的输入却仍然被判有效，选手申诉后平台无法指出缺口位置；有录像被重排了输入顺序，重放出来的走位和直播不一致；还有片段被截断后重新拼接，平台照样盖章通过。平台每天要处理上万份回放，人工抽检只能覆盖百分之一，选手对判定有异议时还会要求给出具体 tick 的链头值作为仲裁证据。现有校验只看总帧数，没有逐帧校验链。请修复模块 `gsb/batch46-game-c-05` 中文件 `replay_validation_guard.go` 的类型 `ReplayValidator`，任务类型 Bugfix。

每帧输入带一个 64 位校验值，必须从上一条记录链式推导，任何篡改、缺帧或乱序都要定位到具体 tick。需要实现并保持下列接口：
- `NewReplayValidator(cfg Config) *ReplayValidator`、`NewReplayValidatorFromSnapshot(data []byte) (*ReplayValidator, error)`
- `Open(h ReplayHeader) error`、`Append(f InputFrame) error`、`Seal() (ValidationReport, error)`
- `HeadChecksum() uint64`、`TickChecksum(tick uint32) (uint64, error)`、`FrameCount() int`、`PlayerInputs(playerID string) int`
- `Rewind(toTick uint32) (int, error)`
- `ChecksumChain(prev uint64, tick uint32, playerID, input string) uint64`
- `Snapshot() ([]byte, error)`

同一 tick 允许不同玩家先后提交，但同一玩家只能有一条记录；校验过程不能把整份录像一次性读进内存，帧总量由配置控制。校验链算法固定：以链头为 64 位初值，依次喂入 tick 的 4 字节小端编码、玩家 ID 字节、一个 0x1F 分隔字节、输入字符串字节；每个字节先与状态异或、再乘以 1099511628211，得到新的链头。必须处理的具体问题：1）`Open` 检查版本必须等于 `Config.Version`（不符返回 `ErrVersionUnsupported`），玩家重复返回 `ErrDuplicatePlayer`，超过 `MaxPlayers` 返回 `ErrTooManyPlayers`，时长为 0、超过 `MaxTicks`、`TickRate` 非正或 `MatchID` 为空返回 `ErrInvalidHeader` 或 `ErrReplayTooLong`，重复 `Open` 返回 `ErrAlreadyOpen`；2）未 `Open` 就 `Append` 返回 `ErrNoReplay`，未知玩家返回 `ErrUnknownPlayer`，tick 不小于时长返回 `ErrTickOutOfRange`，tick 小于上一帧返回 `ErrTickOutOfOrder`，同一玩家同一 tick 重复返回 `ErrDuplicateInput`；3）每帧的 `Checksum` 必须等于用当前链头算出的值，否则返回 `ErrChecksumMismatch`，且该帧不得入账；4）`Seal` 要求 0 到时长减 1 的每个 tick 都有全部玩家的输入，缺任何一条返回 `ErrReplayIncomplete`，`ValidationReport` 的 `FirstMissingTick` 指向第一个缺口、`Valid` 为假；完整时 `Valid` 为真，`Frames`、`Ticks`、`HeadChecksum` 与实际一致；5）`TickChecksum` 返回该 tick 最后一条记录后的链头，没有记录返回 `ErrTickNotRecorded`；6）`Rewind` 丢掉晚于目标 tick 的帧并返回删除数量，链头回退到该 tick 末尾，目标 tick 没有记录返回 `ErrTickNotRecorded`；7）`PlayerInputs` 返回玩家已提交的帧数，用于核对每个选手的上报数量是否与赛程一致；8）`Snapshot` 保存头信息、帧和链头，重启后继续追加的帧必须与不重启时得到相同链头，重放旧帧仍被校验链拒绝；9）追加与读取可能并发，帧列表、链头和计数必须保持一致；10）`Rewind` 之后可以继续追加被丢弃的 tick，重新追加的帧必须拿到与第一次完全相同的链头。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过或者按固定输入写死结果；并发与重启场景自行构造验证；只做本地实现；缺帧位置、链头和每个选手的上报数量都要可复算，方法签名不得更改。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
