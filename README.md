# GSB Bugfix Task

## 题面

夜战关卡在 2060 上每帧多出 300 次无效 draw call，阴影 pass 与主 pass 的深度状态对不上，角色边缘出现黑边；已被深度预通道判为遮挡的建筑仍在颜色 pass 重画；提交顺序跟着加载顺序走，从远到近的 overdraw 把带宽打满；预算超了以后前面已经提交的批次收不回来，只能整帧重启。涉及模块 gsb/batch46-game-f-03，文件 depth_prepass_guard.go，类型 DepthPrepass，任务类型 Bugfix。

按此接口补齐：NewDepthPrepass(cfg DepthConfig) (*DepthPrepass, error)；NewDepthPrepassFromSnapshot(cfg DepthConfig, data []byte) (*DepthPrepass, error)；AddBatch(b DrawBatch) error；RemoveBatch(id string) error；Batch(id string) (DrawBatch, error)；Plan() ([]DrawBatch, error)；SetOccluded(id string, occluded bool) error；Submit(ids []string) (SubmitResult, error)；BeginFrame() error；FrameDraws() int；Stats() PrepassStats；Snapshot() ([]byte, error)。DepthConfig 含 MaxDrawCalls，DrawBatch 含 ID、Depth、IndexCount、CastsDepth、Occluded，SubmitResult 含 DrawCalls、Triangles，PrepassStats 含 Batches、PlanDraws、FrameDraws、SubmittedTriangles。

必须覆盖：1) 批次 ID 重复返回 ErrBatchExists，原批次不得被覆盖；2) 深度为 NaN、小于 0 或大于 1 返回 ErrInvalidDepth，批次不入库；3) Plan 按深度从近到远排序，深度相同按 ID 升序；4) CastsDepth 为 false 的批次和已标记遮挡的批次都不进 Plan，遮挡标记清除后恢复；5) Submit 单次与累计都不得超过 MaxDrawCalls，超出返回 ErrDrawBudget 且本次一票不提交；6) 未知批次返回 ErrBatchUnknown；7) 一批里同一 ID 出现两次返回 ErrDuplicateBatch；8) 提交顺序与 Plan 的相对顺序不一致返回 ErrOrderViolation；9) 已提交过的批次再次提交返回 ErrAlreadySubmitted，已标记遮挡的批次被提交返回 ErrBatchOccluded；10) 同帧多次 Submit 累计 DrawCalls 与 Triangles（IndexCount/3），BeginFrame 清零并允许重提；11) 删除批次后 Plan 不再包含它，再删返回 ErrBatchUnknown；12) Snapshot 保存批次表、遮挡标记、当前帧已提交集合与计数，恢复后 Plan 一致、FrameDraws 不变、重复提交仍被拒，损坏数据返回 ErrCorruptSnapshot。并发要求：预算为 3 时 8 个协程各提交一个不同批次，只能恰好 3 个成功，其余返回 ErrDrawBudget。

补充约定：Batch 返回副本，调用方改写字段不得影响内部批次表；Plan 每次调用都要反映最新的增删和遮挡状态，Stats.PlanDraws 与 Plan 长度一致；SubmitResult 只统计本次调用新增的 draw call 与三角形，被拒绝的调用不得让 Stats 变化。提交校验顺序固定为未知、重复、遮挡、顺序、预算，任何一步失败都返回对应错误且不写入；BeginFrame 同时清零 FrameDraws、已提交集合与本帧三角形计数。并发调用 Plan、AddBatch、RemoveBatch 与 Submit 时不能出现数据竞争，批次表与帧状态必须互斥更新。批次入库时就固定 IndexCount，后续查询与统计都以入库值为准；同一批次可以在不同帧重复提交，唯一性限制只在一帧之内生效。被拒绝的 Submit 之后，Plan 与 FrameDraws 必须和调用前完全一致，便于上层原样重试；Stats.Batches 随增删实时变化，统计口径在所有帧之间保持一致。

硬约束：提交是事务性的，未知、重复、顺序、预算四类校验必须在写入前一次完成；批次表与帧状态在一把锁内更新；单帧 draw call 数是硬预算。验收：go test ./... 的 15 条测试必须全部通过，不改测试、不跳过、不按固定批次写死；并发和重启自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
