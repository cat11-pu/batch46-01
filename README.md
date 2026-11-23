# GSB Bugfix Task

## 题面

上周一次灰度把一台边缘节点的网卡打挂了：新上的 XDP 程序里有一个向后跳的循环，校验器没拦住，程序在内核软中断里原地打转，队列直接卡死。同一批里还有个程序用着只对 GPL 开放的辅助函数，却挂着 Proprietary 许可证混了过去；map 配额按全局算，一个租户批量加载之后别的租户连一个 map 都建不出来，扩容窗口整整拖了四十分钟；加载到一半失败的请求还会在账上留下半个程序和两张 map，事后对账时谁也说不清这些半成品算谁的。这次事故之后我们把校验器日志全打开了，光是指令计数一项，过去一周就有一百多次接近上限的提交，说明上限和配额的口径必须一次定清楚。请在模块 gsb/batch46-safety-e-09 里修 ebpf_verifier_guard.go，类型 EbpfGuard，任务类型 Bugfix，把校验、配额、加载事务和重启恢复一起补齐。

对外接口：
- NewEbpfGuard(cfg Config) *EbpfGuard
- NewEbpfGuardFromSnapshot(cfg Config, data []byte) (*EbpfGuard, error)
- (g *EbpfGuard) Verify(p Program) error
- (g *EbpfGuard) Load(tenant string, p Program, maps []MapSpec) (ProgramHandle, error)
- (g *EbpfGuard) Unload(id uint64, gen uint64) error
- (g *EbpfGuard) Lookup(tenant, name string) (ProgramHandle, error)
- (g *EbpfGuard) SetAllowedHelpers(ids []int) error
- (g *EbpfGuard) Generation() uint64
- (g *EbpfGuard) Snapshot() ([]byte, error)

要求每一次裁决都能复述出理由：是被指令上限挡的、被跳转挡的、被许可证挡的，还是被租户配额挡的；同时 Load 必须是一个事务，失败的那次不能在账上留下任何痕迹。

需要落到的行为：
1. 指令数为 0 或超过 Config.MaxInsns 直接拒绝，分别对应 ErrEmptyProgram 和 ErrInstrLimit。
2. 跳转目标按 i+1+Off 计算，落在程序外返回 ErrBadJump；Off 为负的向后跳返回 ErrLoop。
3. 最后一条指令必须是 OpExit，否则返回 ErrNoExit；只有前面指令合法才会走到这一步，写在中间的 OpExit 不算结尾。
4. OpCall 的 Imm 是辅助函数编号：不在白名单返回 ErrHelperDenied；在 GPLOnlyHelpers 里而 License 不是精确的 GPL，返回 ErrLicense。
5. map 的 KeySize、ValueSize、MaxEntries 任一项超过 Config 对应上限，返回 ErrMapTooLarge，三项要分别判。
6. 一次 Load 携带的 map 数加上该租户已有 map 数超过 Config.MaxMapsPerTenant 时返回 ErrMapQuota，配额按租户各算各的。
7. 同一租户内程序名重复返回 ErrProgramExists；换个租户同名可以加载，两个租户的程序互不可见。
8. Load 必须整体成功或整体失败：任何一项检查不过，程序数、map 数、Generation 都不能变。
9. 成功的 Load 返回带 ID 和 Insns 的 handle，Gen 等于当次加载后的 Generation，Stats 的程序数、map 数、指令数同步更新。
10. Unload 的 gen 不等于当前 Generation 返回 ErrStaleGen；id 从未加载过返回 ErrUnknownProgram，两种错误不能混。
11. Snapshot 要带上已加载程序的名称、租户、指令数和各租户 map 数，恢复后 Lookup 和 ProgramCount 与重启前一致。
12. 并发 Load 不能把某个租户的 map 数顶过配额，也不能让两条同租户同名程序同时登记成功，Generation 必须严格递增。

验收：`go test ./...` 的 15 条必须全过；不能改测试、跳过或按固定输入写死；并发与重启恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 安全
- sub: 系统安全
- type: Bugfix
- difficulty: 地狱
