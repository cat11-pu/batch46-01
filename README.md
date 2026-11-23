# GSB Bugfix Task

## 题面

开放世界切到可变速率着色以后，远景地形出现成片块状色带，角色边缘又被糊掉；同一帧里先后两次设置互相覆盖，提交后的着色率与最后一次请求相反；预算超限时界面已提示成功，下一帧整片回滚造成闪屏；驱动不支持 VRS 的机器直接黑屏；热重载后所有表面恢复默认值，画面一秒内抖两次。要改的模块是 gsb/batch46-game-f-02，文件 variable_rate_shading_guard.go，类型 VrsGuard，任务类型 Bugfix。

接口：NewVrsGuard(cfg VrsConfig) (*VrsGuard, error)；NewVrsGuardFromSnapshot(cfg VrsConfig, data []byte) (*VrsGuard, error)；SetSurface(name string, width, height int) error；SurfaceInfo(name string) (Surface, error)；RateAt(name string, x, y int) (int, error)；PendingRateAt(name string, x, y int) (int, error)；SetRectRate(name string, x, y, w, h int, rate int) (int, error)；SetRates(name string, rects []RateRect) (ChangeSet, error)；CombineRates(a, b int) int；PendingChanges(name string) (int, error)；CommitFrame() (CommitResult, error)；ClearRates(name string) (int, error)；DegradeTo1x1() (DegradeResult, error)；Snapshot() ([]byte, error)；Stats() VrsStats。VrsConfig 含 TileSize 与 MaxRateChangesPerFrame，Surface 含 Cols、Rows，RateRect 含 X、Y、W、H、Rate，ChangeSet 含 Surface、ChangedTiles，CommitResult 含 Surfaces、ChangedTiles，VrsStats 含 CommitTiles。

规则：1) 着色率只允许 1、2、4，其余返回 ErrInvalidRate；2) 重叠请求取最粗，CombineRates(1,2) 为 2、CombineRates(2,4) 为 4，与先后顺序无关；3) 网格按 TileSize 向上取整，128x96 配 32 瓦片是 4x3，像素 64 落在 0 基第 2 列；4) 重复注册表面返回 ErrSurfaceExists，未知表面返回 ErrSurfaceUnknown，越界像素返回 ErrInvalidSurface；5) SetRectRate 返回本次调用新改变的待提交瓦片数，重复设置同一区域返回 0；6) 一批请求让本帧待提交变化数超过 MaxRateChangesPerFrame 时返回 ErrRateBudget，并把这一批整体回滚，之前已接受的批次保留；7) CommitFrame 把待提交写成已提交，返回变化瓦片数与按名字升序的表面列表，并重置本帧预算；8) 没有待提交内容时提交返回 0 和空列表，不得重复计数；9) ClearRates 把整张表面的待提交值恢复为 1x1，返回变化瓦片数；10) DegradeTo1x1 把所有表面强制为 1x1，返回表面数和重置瓦片数，之后任何设置返回 ErrVrsUnsupported，提交不再产生变化；11) 8 个协程各改一个独立瓦片、预算为 4 时只能成功 4 个，其余返回 ErrRateBudget，PendingChanges 必须为 4；12) Snapshot 保存已提交与待提交状态及降级标记，恢复后 RateAt、PendingRateAt、PendingChanges 与快照前一致，损坏数据返回 ErrCorruptSnapshot。

补充约定：SurfaceInfo 返回的名字、宽高与网格必须与注册时一致；SetRates 收到空列表返回 0 变化且不报错，矩形先裁剪到表面边界，完全落在表面外返回 ErrInvalidSurface 并整批回滚，同一瓦片被多次覆盖只按最终最粗值计一次变化。降级是单向的，进入降级后 Snapshot 往返仍要保留降级标记和已提交的 1x1 网格，CommitResult.Surfaces 只列出真正发生变化的表面。Stats 的 CommitTiles 是累计值，ClearRates 与 DegradeTo1x1 引起的变化也要在下一次提交后计入。同一帧里对同一瓦片先设 2x2 再设 4x4 结果必须是 4x4，反过来也必须是 4x4，不允许后写覆盖前写；提交之后 PendingRateAt 必须等于 RateAt。

硬约束：状态变更和预算判定在同一临界区完成，不满足预算的批次一点都不能落库；提交与降级是事务操作；每帧变化瓦片数是硬预算。验收：go test ./... 的 15 条测试必须全部通过，不改测试、不跳过、不按固定坐标写死；并发和恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 地狱
