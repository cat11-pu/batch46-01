# GSB Bugfix Task

## 题面

上周一台支付网关进程 OOM 之后落了一个 2.1GB 的 core，运维按流程把整包传到对象存储，第二天安全审计在包里翻出了明文 RSA 私钥和两段 JWT 签名密钥。复盘时发现：登记过的脱敏规则确实存在，但一条只该对 tenant-b 生效的规则把所有租户的 dump 都擦了一遍，另一个租户的故障现场被毁；两条区间重叠的规则谁生效全看注册顺序，同样的输入重跑两次结果不一样；输出体积上限压根没生效，一个 8GB 的 dump 直接写满了落盘分区，落盘失败之后还丢了一段没写完的段。请在模块 gsb/batch46-safety-e-07 里修 coredump_redact_guard.go，类型 CoreDumpGuard，任务类型 Bugfix，把规则、规划、脱敏和快照串成一条能对账的链路。

对外接口：
- NewCoreDumpGuard(cfg Config) *CoreDumpGuard
- NewCoreDumpGuardFromSnapshot(cfg Config, data []byte) (*CoreDumpGuard, error)
- (g *CoreDumpGuard) RegisterRule(r Rule) (uint64, error)
- (g *CoreDumpGuard) RemoveRule(id string, epoch uint64) error
- (g *CoreDumpGuard) Plan(req DumpRequest) ([]Segment, error)
- (g *CoreDumpGuard) Redact(req DumpRequest) ([]byte, error)
- (g *CoreDumpGuard) Protected(addr uint64, tenant string) bool
- (g *CoreDumpGuard) Stats() Stats
- (g *CoreDumpGuard) Snapshot() ([]byte, error)

判断标准不是单次输出看起来像样，而是同一批段在重叠规则、跨租户和进程重启之后仍然给出同一个结果，Protected 给出的答案也要和 Redact 真正擦掉的位置一致。

需要落到的行为：
1. ModeZero 只把规则覆盖的字节写成 0x00，区间外的字节一个都不许动；ModeFill 覆盖的字节全部写成 Rule.Fill，其余字节原样保留。
2. 没有规则覆盖的 dump 原样输出，RedactedBytes 不增加；一次 Redact 只累计一次，重复调用同一份输入不会让计数翻倍。
3. Plan 按起始地址升序输出段，相邻段（前一段的 End 等于后一段的 Start）合并成一段，Data 按地址顺序拼接。
4. 输入段互相重叠返回 ErrRegionOverlap；Start 不小于 End、或者 Data 长度与区间大小不等，返回 ErrInvalidRegion。
5. 规则 id 重复返回 ErrDuplicateRule；规则数超过 Config.MaxRules 返回 ErrRuleLimit，两种情况下规则表都不变。
6. RemoveRule 的 epoch 不是当前值返回 ErrStaleEpoch；删掉之后 Rule 查询返回 ErrUnknownRule，重复删除也按未知规则处理。
7. 整份 dump 超过 Config.MaxOutputBytes 返回 ErrBudgetExceeded，此时不产出任何字节，也不修改任何计数。
8. 带 Tenant 的规则只作用于该租户的请求，Tenant 为空的规则对所有租户生效；Protected 的答案必须和 Redact 的实际结果一致。
9. 两条规则区间重叠时按 Priority 决定，数值大的赢；Priority 相同时 id 字典序小的赢，结果必须可重复，不能依赖注册顺序。
10. Stats 要能给出当前规则数、Redact 次数和累计被改写的字节数；ModeKeep 覆盖的字节与未被覆盖的字节都不计入。
11. Snapshot 要带上全部规则、epoch 和计数，恢复后 RuleCount 与 Redact 的结果必须和重启前一致，旧 epoch 的删除请求仍按过期处理。
12. 并发 RegisterRule 不能把规则数顶过 MaxRules，不能出现两条同 id 的规则同时登记成功，也不能让 Plan 看到写了一半的规则表。

验收：`go test ./...` 的 15 条必须全过；不能改测试、跳过或按固定输入写死；并发与重启恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 安全
- sub: 系统安全
- type: Bugfix
- difficulty: 困难
