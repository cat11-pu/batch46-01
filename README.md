# GSB Bugfix Task

## 题面

上周三凌晨做 vCPU 热迁移演练，一台跑了很久的虚拟机在目标宿主上直接触发 #GP，guest 内核 panic，业务侧只看到整机重连。翻 KVM 日志定位到三件事：guest 启动阶段对 0x400000F0 附近的厂商私有 MSR 发了一次写请求，当时下发的是整段放行，本该被保留区间挡掉；有个 vCPU 在策略重加载的瞬间读到了新旧混合的白名单，读出来的 IA32_MISC_ENABLE 和预期对不上，前后两代策略的版本号也没人能对上；演练前手工补的一条区间把 Lo 和 Hi 写反了，接口照收不误，还把这条脏规则一起写进了快照。请在模块 gsb/batch46-safety-e-06 里修 kvm_msr_guard.go，类型 KvmMsrGuard，任务类型 Bugfix，把规则表、撤销记录、每 vCPU 计数和快照这几块一起理顺。

对外接口：
- NewKvmMsrGuard(cfg Config) *KvmMsrGuard
- NewKvmMsrGuardFromSnapshot(cfg Config, data []byte) (*KvmMsrGuard, error)
- (g *KvmMsrGuard) AddRule(rule MsrRule) (uint64, error)
- (g *KvmMsrGuard) RemoveRule(rule MsrRule, revision uint64) error
- (g *KvmMsrGuard) Check(vcpu int, msr uint32, access MsrAccess) error
- (g *KvmMsrGuard) CheckBatch(vcpu int, msrs []uint32, access MsrAccess) error
- (g *KvmMsrGuard) Revoke(msr uint32, access MsrAccess) (uint64, error)
- (g *KvmMsrGuard) Counters(vcpu int) VCPUCounters
- (g *KvmMsrGuard) Snapshot() ([]byte, error)

这次改动覆盖策略装载、请求裁决、运维撤销和重启恢复四条路径，任何一条只修一半，都会在下一轮演练里重新暴露；评级看的是同一个 MSR 在不同路径上得到一致结论。

需要落到的行为：
1. 没有任何规则覆盖的 MSR 一律拒绝，返回 ErrMsrDenied，并把该 vCPU 的拒绝计数加一；命中白名单的读请求要把放行计数加一。
2. 区间是闭区间，只有落在规则区间内且访问模式一致才放行；只放了读的规则不能拿来写，写请求按 ErrMsrDenied 处理。
3. Config.Reserved 里的区间任何规则都盖不上去，区间只要与保留区有交集就返回 ErrRangeOverlap，被拒之后规则数和版本号都保持不变。
4. Lo 大于 Hi 返回 ErrInvalidRange；规则数不得超过 Config.MaxRules，第 MaxRules+1 条返回 ErrRuleQuota，且不覆盖任何已有规则。
5. 命中 Config.ReadOnly 的写请求返回 ErrMsrReadOnly，哪怕存在写白名单；只读区间对读请求仍然放行。
6. 一个批次里只要有一条被拒，整批返回 ErrMsrDenied，已放行的条目也不能增加放行计数，事后的对账要能对上。
7. 批次长度超过 Config.MaxBatch 返回 ErrBatchTooLarge，且不动任何计数；空批次视为无操作，直接成功返回。
8. RemoveRule 传入的 revision 不等于当前版本时返回 ErrStaleRevision，否则真删除并让 RuleCount 减一；删除一条不存在的规则不能悄悄成功。
9. Revoke 优先级高于任何白名单规则，撤销之后同一 MSR 立即按 ErrMsrDenied 拒绝，IsRevoked 返回真，版本号随之加一。
10. vCPU 编号超出 Config.VCPUs 返回 ErrUnknownVCPU；规则里 VCPU 为 -1 代表全部 vCPU，指定编号时只作用于那一个；计数按 vCPU 分开，ResetCounters 只清指定的那个。
11. Snapshot 要带上规则、撤销记录、当前版本和各 vCPU 的计数，恢复后撤销过的 MSR 仍然拒绝，规则数量、版本号和计数都要和快照前对得上。
12. 策略加载要原子：并发 AddRule 既不能把规则数顶过 MaxRules，也不能让 Check 看到半新半旧的规则表；并发撤销和放行之间，以最后落地的那个版本为准。

验收：`go test ./...` 的 15 条必须全过；不能改测试、跳过或用固定输入写死；并发和重启恢复的场景自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 安全
- sub: 系统安全
- type: Bugfix
- difficulty: 地狱
