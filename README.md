# GSB Bugfix Task

## 题面

版权方临时要求把点播密钥从 24 小时改成 60 秒一换，切完之后老机顶盒在换钥那一分钟里报“无法解密”，新机顶盒又提前 90 秒拿不到下一把钥匙；更糟的是上周做过一次紧急吊销，服务回滚之后被判死的密钥又复活了，同一个设备重新签出了许可证。三个问题指向同一个地方：密钥窗口、轮换边界、宽限期和吊销状态没有放在一个状态机里管理。

请在模块 `gsb/batch46-game-h-04` 的 `drm_key_rotation_guard.go` 里补全 `DrmKeyRotation` 类型，任务类型为 Bugfix，包名 task，测试文件不要修改。对外签名保持骨架现状：

- `NewDrmKeyRotation(cfg Config) *DrmKeyRotation`
- `AddKey(k Key) error`
- `KeyByID(keyID string) (Key, error)`
- `ActiveKey(tenantID string, at time.Time) (Key, error)`
- `Rotate(tenantID, keyID string, at time.Time) (Key, error)`
- `NextKey(tenantID string, at time.Time) (Key, bool)`
- `IssueLicense(tenantID, keyID, deviceID string, now time.Time) (License, error)`
- `DecryptAllowed(keyID string, at time.Time) bool`
- `Revoke(keyID string) error`
- `Keys(tenantID string) []Key` / `Stats() Stats` / `Snapshot() ([]byte, error)` / `NewDrmKeyRotationFromSnapshot(cfg Config, data []byte) (*DrmKeyRotation, error)`

需要修掉的现象和边界：

1. 注册时没有写结束时间的密钥，窗口长度是零，任何时刻都取不到它；结束时间留空时应按 NotBefore 加一个轮换周期补齐。
2. 空 ID、空租户、起始时间为零、结束不晚于开始、注册时就带吊销标记的密钥都被收下；一律返回 `ErrInvalidKey`，且不进入密钥表。
3. 同一个 ID 注册两次会静默覆盖前一把，密钥表和许可证引用的密钥可能对不上；应返回 `ErrDuplicateKey`。同租户的窗口重叠（0 到 60 秒与 30 到 90 秒）也要拒绝，返回 `ErrKeyOverlap`；不同租户使用相同时段是允许的。
4. 密钥数量没有上限，一次批量导入几百把也能成功；配置上限必须硬生效，超过返回 `ErrTooManyKeys`。
5. 窗口结束时刻仍返回旧密钥，空档期返回空结构而不是错误；活动密钥按左闭右开选取，窗口外和未知租户都返回 `ErrNoActiveKey`。
6. 轮换可以跳过空档：前一把 60 秒结束、后一把 70 秒才开始，在 70 秒执行轮换也被放行，中间 10 秒没有可用密钥；轮换点必须落在此前密钥的结束边界上，否则返回 `ErrRotationBoundary`。租户的第一把密钥只允许在自己的起始时刻激活。
7. 轮换目标未知、属于别的租户、已被吊销、或给的时间不在该密钥窗口内时，错误必须区分开，分别返回 `ErrUnknownKey`、`ErrCrossTenant`、`ErrKeyRevoked`、`ErrRotationBoundary`，失败不影响轮换计数。
8. 预取接口返回的是当前密钥或已过期的密钥，客户端因此重复下载；`NextKey` 必须严格返回 NotBefore 晚于给定时刻的最近一把，没有则返回 false。
9. 许可证过期时间直接按 TTL 加满，越过了密钥宽限期结束时刻，播放器在密钥早已不可解密时还拿着有效许可证；签发时要把 ExpiresAt 截断到 NotAfter 加宽限期末尾。
10. 跨租户的 key、空设备号、还没生效的 key、宽限期已经过去的 key 都成功签出了许可证；分别返回 `ErrCrossTenant`、`ErrInvalidDevice`、`ErrKeyExpired`，且不增加许可证计数。
11. 宽限期结束后 `DecryptAllowed` 仍返回 true，被吊销的密钥也能继续解密；必须按 `[NotBefore, NotAfter+Grace)` 判定，吊销立即生效。
12. 服务重启后吊销标记、许可证计数、轮换次数全部归零，被吊销的密钥可以重新签许可证；快照恢复要带回这些状态，损坏的快照返回 `ErrCorruptSnapshot`。

硬性要求：多个设备并发申请许可证、同时执行轮换时，许可证计数不能丢，也不能出现签发了许可证但密钥状态只更新一半的情况；轮换必须是跨方法的事务，空档、重叠、吊销任何一项不满足都不能落地；重启后从快照恢复的密钥表要继续拒绝已吊销密钥，并保持同样的活动密钥选择结果。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过用例或按固定输入写死结果；并发与重启场景自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 地狱
