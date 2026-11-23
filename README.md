# GSB Bugfix Task

## 题面

运营在 4K 电视上投放的新版播放器，过去一周收到两类互斥的反馈：一类说画面一直在 720p 和 1080p 之间来回跳，一分钟能换四次；另一类说客厅宽带明明有 200M，播放器却始终停在 800kbps 的糊画质。抓包后发现码率估计和切档决策是两套逻辑：估计值每来一个采样就被后一个覆盖，切档又完全不看缓冲、不设冷却，档位还允许一次从最低爬到最高。

请在模块 `gsb/batch46-game-h-05` 的 `abr_ladder_guard.go` 中补全 `AbrLadder` 类型，任务类型为 Bugfix，包名 task，测试文件不要修改。公开签名保持如下：

- `NewAbrLadder(cfg Config) *AbrLadder`
- `SetLadder(rungs []Rung) error`
- `Rungs() []Rung`
- `RungForBandwidth(bandwidth int) (Rung, error)`
- `RungForHeight(height int) (Rung, error)`
- `ObserveSample(bps int, now time.Time) error`
- `Estimate() int`
- `Decide(buffer time.Duration, now time.Time) (Rung, bool, error)`
- `ForceRung(id int, now time.Time) error`
- `Current() (Rung, bool)`
- `Cooldown(now time.Time) time.Duration`
- `Stats() Stats` / `Snapshot() ([]byte, error)` / `NewAbrLadderFromSnapshot(cfg Config, data []byte) (*AbrLadder, error)`

需要修复的具体现象与边界：

1. 阶梯里出现带宽倒序（先 2400kbps 再 800kbps）、重复 ID、带宽或高度为 0、ID 为 0 的档位都能注册成功；分别返回 `ErrLadderOrder`、`ErrDuplicateRung`、`ErrInvalidRung`。
2. 阶梯只有一档也放行，自适应失去了意义；少于配置的最少档位数要返回 `ErrTooFewRungs`。
3. 实测 4Mbps 时选中了 800kbps 的最低档，实测正好 2400kbps 时又退回低一档；`RungForBandwidth` 要取不超过实测值的最高档，低于最低档时返回 `ErrNoRungFits`。
4. 设置页按高度选档，输入 800 却给出 1080 的档位，设备直接解码失败；`RungForHeight` 必须取不超过输入高度的最高档。
5. 第一个带宽采样被当成增量叠加，估计值从 0 缓慢爬升，起播阶段一直锁在最低档；首个采样应直接作为初始估计。
6. 之后的采样没有按权重平滑，估计值跟着单次抖动上下跳；按配置的 EWMAlpha 做指数加权，新样本权重由该参数决定。
7. 采样值为 0 或负数时仍被计入，估计值被拉到 0 并永远切不回高清；应返回 `ErrInvalidSample`，估计值和计数都不变。
8. 没有配置阶梯时 `Decide` 返回零值档位，上层以为播放正常；应返回 `ErrLadderNotSet`。
9. 缓冲只有 3 秒、远低于 5 秒的起播保护水位时，仍然从 800kbps 直接切到 5.2Mbps，切换后立刻二次卡顿；向上切换前必须满足缓冲水位。
10. 实测带宽崩到 900kbps 时，因为缓冲不足一直不敢降档，持续卡顿十几秒；向下切换不受缓冲水位限制，应立即执行。
11. 切换后 10 秒冷却期内又连续切换，档位反复横跳；冷却期内 `Decide` 不得改变档位，`Cooldown` 要返回剩余时间，冷却结束才恢复。
12. 一次决策从 800kbps 跳到 8Mbps（跨三档），起播缓冲瞬间耗尽；单次最多只能爬配置的档数，剩余档位留到下一次决策。此外 `ForceRung` 传未知 ID 必须返回 `ErrUnknownRung`，重复钉住同一档不得重复计数。

硬性要求：多个采样 goroutine 与读取估计值、统计信息的调用并发时，估计值只能是更新前或更新后的完整值，不能读到算了一半的结果；服务重启后从快照恢复，阶梯、估计值、当前档位、采样与切换计数、剩余冷却都要延续，损坏快照返回 `ErrCorruptSnapshot`；单次跳档上限和冷却期是硬约束，调用方无法绕过。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过用例或按固定输入写死结果；并发与重启场景自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 困难
