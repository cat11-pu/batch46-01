# GSB Bugfix Task

## 题面

上个月一台构建机的 CI runner 被攻破，攻击者用同一个 uid 的进程对正在签名的发布进程发起 PTRACE_ATTACH，把内存里的签名私钥读走。回溯配置发现宿主上 yama 的 ptrace_scope 是 0，本来指望的"只能父进程 attach"根本没开；取证同学还发现代码里 uid 比对用错了字段，拿 tracee 的 uid 和 tracee 自己比，同 uid 判断永远是真；目标进程设了 no_new_privs 也没人管，容器里带这个标记的进程照样被 attach；整个服务没有任何频率限制，攻击脚本被拒之后原地重试，日志被刷了几十万行，真正的告警被埋掉了。请在模块 gsb/batch46-safety-e-08 里修 ptrace_scope_guard.go，类型 PtraceGuard，任务类型 Bugfix，把范围判断、能力校验、频率限制和 exec 后的失效一起补齐。

对外接口：
- NewPtraceGuard(cfg Config) *PtraceGuard
- NewPtraceGuardFromSnapshot(cfg Config, data []byte) (*PtraceGuard, error)
- (g *PtraceGuard) RegisterTask(t Task) error
- (g *PtraceGuard) SetMode(m ScopeMode) error
- (g *PtraceGuard) IsAncestor(tracer, tracee int) (bool, error)
- (g *PtraceGuard) RequestAttach(req AttachRequest) (AttachGrant, error)
- (g *PtraceGuard) ReleaseAttach(nonce string) error
- (g *PtraceGuard) ActiveGrants(pid int) int
- (g *PtraceGuard) OnExec(pid int) error
- (g *PtraceGuard) Snapshot() ([]byte, error)

任务表只约束被 attach 的一方，发起 attach 的进程不要求在册，只要 pid 和 uid 能对上就行；祖先判断才需要沿链查到已知的父进程。

这套判断要求"先定范围、再看能力、最后算额度"，顺序错了就会出现有能力的进程被范围挡住、或者被拒的尝试把额度耗光；重启之后的存活关系和内存里必须一致，否则运维会丢掉一个已经建立的 attach。取证脚本还会拿 ActiveGrants 判断某个发布进程是否正在被调试，这个数值错了，正常的发布会被当成入侵，真正的入侵反而被放过。

需要落到的行为：
1. ScopeClassic 下 uid 相同的 attach 放行，返回带 nonce 的 grant，ActiveGrants 立刻变成 1；uid 不同返回 ErrUidMismatch。
2. ScopeRestricted 下只有祖先链上的 tracer 能 attach，爷爷进程也算祖先；与这条链无关的进程返回 ErrNotAncestor。
3. IsAncestor 要沿 PPID 一路往上走，方向反了返回 false，pid 没登记过返回 ErrUnknownTask，链断了就按不是祖先处理。
4. ScopeAdminOnly 下没有 CAP_SYS_PTRACE 一律 ErrScopeDenied；带上能力可以绕过 uid 检查。
5. ScopeNoAttach 下即使有 CAP_SYS_PTRACE 也拒绝；SetMode 收到 0 到 3 之外的值返回 ErrInvalidMode，模式保持不变。
6. 目标进程带 no_new_privs 时，没有能力的 tracer 返回 ErrNoNewPrivs，有能力则放行；这个判断要在 uid 之后、范围之前。
7. 每个窗口内放行的 attach 数不能超过 Config.MaxAttachesPerWindow，超出返回 ErrAttachQuota；被拒的那次不占额度，时间滑过 Window 之后重新有额度。窗口按请求自带的 Now 计算，不看系统时钟。
8. OnExec 让该进程的 generation 加一，并吊销所有牵涉它的 grant；此后 ReleaseAttach 旧 nonce 返回 ErrStaleGeneration，而不是"未知 nonce"。
9. 请求里带的 TraceeGen 和当前 generation 不一致时返回 ErrStaleGeneration；带 0 表示按当前代处理。
10. 任务表不能超过 Config.MaxTasks，满了返回 ErrTaskQuota；重复 pid 返回 ErrDuplicateTask，且不覆盖已有条目。
11. Snapshot 要带上任务、存活 grant 和被吊销的 nonce，恢复后 ActiveGrants 与重启前一致，旧 nonce 仍能正常释放。
12. 并发 RequestAttach 不能突破窗口额度，统计里的放行数与 ActiveGrants 要能对上，被拒的次数也要如实计入。

验收：`go test ./...` 的 15 条必须全过；不能改测试、跳过或按固定输入写死；并发和重启恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 安全
- sub: 系统安全
- type: Bugfix
- difficulty: 困难
