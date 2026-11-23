# GSB Bugfix Task

## 题面

传送门关卡在镜面走廊里出现无限递归，GPU 0.2 秒被打满；单向门能从背面穿进去，玩家直接掉到地图外；同一扇门一帧内被提交两次，模板计数错乱，门框出现裂纹；递归深度调小后旧计划仍按旧深度绘制；服务重启后帧内已画集合丢失，门被重画导致模板值溢出。模块 gsb/batch46-game-f-04，文件 stencil_portal_guard.go，类型 StencilPortal，任务类型 Bugfix。

接口：NewStencilPortal(cfg PortalConfig) (*StencilPortal, error)；NewStencilPortalFromSnapshot(cfg PortalConfig, data []byte) (*StencilPortal, error)；AddRoom(id string) error；AddPortal(p Portal) error；SetCamera(roomID string) error；VisiblePortals() ([]string, error)；RenderPlan() (Plan, error)；SetRecursionLimit(n int) error；Submit(portalIDs []string) (SubmitResult, error)；ResetFrame() error；Stats() PortalStats；Snapshot() ([]byte, error)。PortalConfig 含 MaxRecursion、MaxDrawCalls，Portal 含 ID、FromRoom、ToRoom、TwoSided，Plan 含 Steps、Truncated，PlannedPortal 含 PortalID、Depth、StencilRef、DrawCost，SubmitResult 含 Portals、DrawCost，PortalStats 含 Rooms、Portals、Visible、Submitted。

规则：1) 房间重复返回 ErrRoomExists，空 ID 返回 ErrInvalidRoom；2) 门两端相同返回 ErrSameRoom，任一房间不存在返回 ErrRoomUnknown，门 ID 重复返回 ErrPortalExists，空 ID 返回 ErrInvalidPortal；3) VisiblePortals 从相机房间逐层展开，按深度升序、同层按 ID 升序，每扇门只出现一次，遇到环时已访问房间不再展开；4) 单向门只能从 FromRoom 进入，TwoSided 为 true 才允许反向，不可达房间里的门不出现；5) RenderPlan 给出每扇可见门的 Depth、StencilRef（Depth+1）和 DrawCost（Depth+1），深度不小于递归上限的门被截掉并把 Truncated 置位；6) SetRecursionLimit 只接受正整数，0 或负数返回 ErrInvalidRecursion；7) Submit 只接受当前 Plan 里的门，不可见或被截断的门返回 ErrPortalNotVisible；8) 提交顺序必须与 Plan 一致，否则返回 ErrOrderViolation；9) 同一扇门在一批里重复返回 ErrDuplicatePortal，已提交过的再次提交返回 ErrAlreadySubmitted；10) 帧内累计 DrawCost 超过 MaxDrawCalls 返回 ErrDrawBudget，本次不写入任何状态；11) ResetFrame 清空帧内集合但保留房间、门、相机和递归上限；12) Snapshot 保存图结构、相机、递归上限与帧内已提交集合，恢复后可见序列一致、重复提交仍被拒，损坏数据返回 ErrCorruptSnapshot。并发：预算为 2 时 8 个协程各提交一扇独立的一层门，只能恰好 2 个成功，其余返回 ErrDrawBudget。

补充约定：SetCamera 传入未知房间返回 ErrRoomUnknown，在同一房间重复设置是幂等操作；VisiblePortals 连续调用必须返回完全相同的序列，Stats.Visible 与序列长度一致；Plan.Truncated 为 false 时 Steps 覆盖全部可见门，为 true 时只保留深度小于上限的部分。SubmitResult.Portals 与 DrawCost 只统计本次新增的部分，被拒绝的调用不得改变 Stats.Submitted。帧内代价预算按累计值判定，ResetFrame 与重启恢复都不能把已画的门当成没画；恢复后 RenderPlan 仍按快照里的递归上限计算，不重新取构造参数。门的 TwoSided 在建门时固定，后续遍历与查询都按注册值执行；房间和门一旦注册不能改名，重复 ID 一律拒绝。相机房间里没有任何门时，VisiblePortals 返回空列表而不是错误。

硬约束：遍历结果不得依赖 map 迭代顺序，必须稳定可复现；提交的校验与写入在同一临界区完成，任何错误都不留部分状态；单帧绘制代价是硬预算。验收：go test ./... 的 15 条测试必须全部通过，不改测试、不跳过、不按固定门名写死；并发与重启自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 地狱
