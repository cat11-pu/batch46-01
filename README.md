# GSB Bugfix Task

## 题面

上周做内核紧急升级，值班同学在密钥轮换的窗口里用 kexec 加载了一个没签名的镜像，机器直接起了新内核，事后审计才发现签名校验被一段"临时放行"的开关关掉了。更糟的是撤销列表只放在进程内存里，kexec 服务一重启就清零，三个月前已经作废的那把私钥又能签名通过；另外两条加载路径 kexec_load 和 kexec_file_load 混着用，机器上的锁定级别根本没看，过期密钥也没人拦，镜像摘要只比了长度没比内容，一个被替换过的 initrd 就这么上线了。审计报告还提到，加载流水里只有时间和镜像名，没有签名者，出了事没人能证明是哪个团队签的包。请在模块 gsb/batch46-safety-e-10 里修 kexec_image_guard.go，类型 KexecGuard，任务类型 Bugfix，把密钥信任、撤销、摘要校验、锁定级别和加载流水一起补齐。

对外接口：
- NewKexecGuard(cfg Config) *KexecGuard
- NewKexecGuardFromSnapshot(cfg Config, data []byte) (*KexecGuard, error)
- (g *KexecGuard) AddTrustedKey(k TrustedKey) (uint64, error)
- (g *KexecGuard) RevokeKey(id string, epoch uint64) (uint64, error)
- (g *KexecGuard) Verify(img Image, now time.Time) error
- (g *KexecGuard) Load(img Image, now time.Time) (LoadRecord, error)
- (g *KexecGuard) Records() []LoadRecord
- (g *KexecGuard) SetLockdown(l LockdownLevel) error
- (g *KexecGuard) Snapshot() ([]byte, error)

判断顺序要固定：先看锁定级别，再看加载路径，然后比体积、签名、密钥和摘要；顺序一乱，同一个镜像在两条路径上就会得到不同结论，重启之后更对不上账；流水一旦缺字段，事后追责就只能靠翻 shell 历史。

需要落到的行为：
1. 签名、密钥、摘要、租户全部对得上时 Verify 返回 nil，Load 追加一条记录，Bytes 等于镜像长度，Epoch 等于加载时的版本。
2. Signed 为假返回 ErrUnsignedImage，SignatureValid 为假返回 ErrSignatureInvalid，两者不能互相顶替。
3. 签名者不在信任列表、或者该密钥只属于别的租户，都返回 ErrKeyUnknown。
4. 被撤销的密钥返回 ErrKeyRevoked，过期（NotAfter 早于 now）返回 ErrKeyExpired，撤销错误不能报成未知密钥。
5. 镜像内容重新算 SHA-256 后与 Digest 不符，返回 ErrDigestMismatch；摘要用小写十六进制比较，大小写不同的同一摘要按不匹配处理。
6. 镜像长度超过 Config.MaxImageBytes 返回 ErrImageTooLarge，且要在校验摘要之前就拒绝，避免对大包做无谓哈希。
7. LockdownConfidentiality 下任何镜像都返回 ErrLockdownDenied；LockdownIntegrity 下只有 kexec_file_load 能过，kexec_load 返回 ErrLockdownDenied。
8. Config.AllowedSyscalls 非空时，Syscall 不在其中返回 ErrSyscallDenied；SetLockdown 收到 0 到 2 之外的值返回 ErrInvalidLockdown。
9. 信任密钥数不能超过 Config.MaxKeys，超出返回 ErrKeyQuota；重复 id 返回 ErrDuplicateKey，原有密钥不被覆盖。
10. RevokeKey 的 epoch 不是当前值返回 ErrStaleEpoch，且 IsRevoked 保持为假；撤销成功后才变真，并返回新的 epoch。
11. Snapshot 要带上密钥、撤销状态和加载记录，恢复后 KeyCount 与重启前一致，已撤销的密钥仍然返回 ErrKeyRevoked。
12. 并发 AddTrustedKey 不能把密钥数顶过 MaxKeys，Records 返回的记录顺序要和加载顺序一致，被拒的加载也要计入统计。

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
