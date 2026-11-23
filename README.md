# GSB Bugfix Task

## 题面

观景台场景用 300 个公告板替身渲染远景树，第一次进场景贴图集就被写爆，后面的树全变白色方块；同一份资产被两个加载器并发烘焙，各占一份格子，显存翻倍；替身角度在 2π 处跳回第 0 帧造成一帧闪黑；网格切替身的距离区间没有过渡，出现硬切；释放资产后格子不回收，长时间游玩后重载直接失败；进程重启后分配表丢失，再分配一遍就写越界。模块 gsb/batch46-game-f-05，文件 billboard_impostor_guard.go，类型 ImpostorGuard，任务类型 Bugfix。

接口：NewImpostorGuard(cfg ImpostorConfig) (*ImpostorGuard, error)；NewImpostorGuardFromSnapshot(cfg ImpostorConfig, data []byte) (*ImpostorGuard, error)；Bake(req ImpostorRequest) (BakeResult, error)；Instance(assetID string, angle, distance float32) (Billboard, error)；FrameFor(assetID string, angle float32) (int, error)；SelectMode(distance float32) (string, error)；BeginFrame() error；FreeAsset(assetID string) error；AtlasUsage() (Usage, error)；Stats() ImpostorStats；Snapshot() ([]byte, error)。各结构体：ImpostorConfig 含 AtlasCells、MaxBakeOpsPerFrame、LODDistance、FadeBand；ImpostorRequest 含 AssetID、Frames、CellSize；BakeResult 含 FirstCell、Allocated、Frames；Billboard 含 Frame、Mode、Blend、U；Usage 含 Used、Total、Assets；ImpostorStats 含 Bakes、CacheHits、Frees。

边界：1) 资产 ID 为空、Frames 不在 1..64、CellSize 非正返回 ErrInvalidRequest；2) 同资产同参数重复烘焙是幂等操作，Allocated 为 0、FirstCell 与首次一致、CacheHits 加一；3) 同资产不同 Frames 或 CellSize 返回 ErrAssetConflict，占用不变；4) 剩余格子不足返回 ErrAtlasFull，占用不变；5) 单帧烘焙格数超过 MaxBakeOpsPerFrame 返回 ErrBakeBudget，BeginFrame 后额度恢复；6) 分配是连续区间，空闲格子按首次适配复用，释放后再次烘焙从最低空位开始；7) FrameFor 把角度归一到 [0,2π)，按归一后角度占整圈的比例乘以帧数向下取整，2π 回到第 0 帧，负角度回绕，多圈角度也要落回合法帧；8) SelectMode 在 LODDistance 以下返回 mesh，在 [LODDistance, LODDistance+FadeBand) 返回 blend 且 Blend=1-(d-LOD)/FadeBand，更远返回 impostor；9) Instance 组合 Frame、Mode、Blend 与图集 U 坐标，U 取 (Frame+0.5)/Frames，未知资产返回 ErrAssetUnknown，负距离返回 ErrInvalidRequest；10) FreeAsset 回收格子与资产计数，重复释放返回 ErrAssetUnknown；11) 8 个协程并发烘焙同一资产只能真正分配一次，其余 Allocated 为 0，总占用等于一份；12) Snapshot 保存分配表、空闲表与统计，恢复后重复烘焙仍命中、新烘焙从下一个空位开始，损坏数据返回 ErrCorruptSnapshot。

补充约定：AtlasUsage 的 Total 恒等于 AtlasCells，Used 等于所有存活资产的帧数之和，Assets 等于存活资产数；Bake 命中缓存不占用帧预算，也不改变 FirstCell。Instance 的 Mode 与 SelectMode 对同一距离的返回值必须一致，Frame 与 FrameFor 对同一角度必须一致；释放资产后该资产再查询、再取实例都返回 ErrAssetUnknown。Snapshot 恢复后 BeginFrame、Bake、FreeAsset 继续正常工作，分配表与空闲表不允许出现重叠或越界。校验顺序固定为请求合法性、资产冲突、缓存命中、帧预算、图集容量，缓存命中优先于预算判定。Bake 成功后 Bakes 加一，释放成功后 Frees 加一，命中缓存只增加 CacheHits；这些统计在 Snapshot 往返后必须保持连续。空闲格子数可以由 Total 减 Used 推出，恢复后这份账也必须自洽。

硬约束：分配、释放和预算判定在同一把锁内完成，失败路径不得留下半分配；AtlasCells 是硬上限；同资产并发烘焙必须收敛为一份。验收：go test ./... 的 15 条测试必须全部通过，不改测试、不跳过、不按固定角度写死；并发与恢复自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
