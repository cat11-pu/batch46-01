# GSB Bugfix Task

## 题面

主城场景切到 4x MSAA 之后，每次拖拽窗口都会先黑一帧再闪白；回放器重启后同一张渲染目标在一帧里被反复解析十几次，帧时间从 6ms 涨到 40ms；分块解析在 8 线程下偶发整块丢失，重叠的脏矩形还会被重复计算两次。问题落在模块 gsb/batch46-game-f-01、文件 msaa_resolve_guard.go、类型 MsaaResolve 上，任务类型 Bugfix。

接口按这个契约补齐：NewMsaaResolve(cfg MsaaConfig) (*MsaaResolve, error)；NewMsaaResolveFromSnapshot(cfg MsaaConfig, data []byte) (*MsaaResolve, error)；RegisterTarget(name string, width, height, samples int) error；ResizeTarget(name string, width, height int) error；Target(name string) (RenderTarget, error)；MarkDirty(name string, x, y, w, h int) error；DirtyTiles(name string) (int, error)；Resolve(name string) (ResolveResult, error)；ResolveFrame(names []string) (FrameResult, error)；Snapshot() ([]byte, error)；Stats() MsaaStats。MsaaConfig 含 TileSize、MaxTargets、MaxResolveBytes，RenderTarget 至少给出 Width、Height、Samples，ResolveResult 含 Samples、Bytes、FromCache，MsaaStats 含 Resolves、CachedResolves、BytesResolved。

故障和边界逐条落定：1) 采样数只接受 1、2、4、8，16 或 0 返回 ErrUnsupportedSamples；2) 同名注册返回 ErrTargetExists，注册数超过 MaxTargets 返回 ErrTargetLimit，失败不得写入；3) 空名、非正尺寸或超过 8192 像素返回 ErrInvalidTarget；4) 新注册目标整张脏，192x128 配 64 瓦片时 DirtyTiles 为 6；5) 重叠的 MarkDirty 只累计一次瓦片，整块落在目标外返回 ErrRegionOutside，部分相交按裁剪后的像素计算；6) 单采样目标解析返回 ErrNoResolveNeeded；7) 真实解析的 Bytes 固定为 width*height*4，成功后脏瓦片清零；8) 没有脏瓦片时再次解析必须 FromCache=true、Bytes=0，并计入 CachedResolves；9) 脏标记与 ResizeTarget 都要让缓存失效，resize 后新尺寸整张脏；10) ResolveFrame 先整体校验名字和总字节，未知名字返回 ErrTargetUnknown，总字节按需要真实解析的目标累加、超过 MaxResolveBytes 返回 ErrBudgetExceeded，两种情况都不许改动任何目标；11) 8 个协程同时解析同一目标时只允许一个做真实工作，其余走缓存；12) Snapshot 必须保存目标表、脏瓦片和已解析状态，重启恢复后脏瓦片数不变、已解析目标仍命中缓存，损坏数据返回 ErrCorruptSnapshot。

补充约定：目标表按注册顺序保存，Snapshot 往返后 Target 的查询结果必须与注册时完全一致；ResolveFrame 的 Results 按入参顺序排列，TotalBytes 等于本次真实写入字节之和；Stats 返回的 Targets、DirtyTiles 必须与逐个查询的结果对得上，恢复后以快照里的计数为准。校验顺序固定为目标存在性、采样数、尺寸、数量上限，先命中的错误直接返回，已经通过的部分也不许写进表里；ResizeTarget 对单采样目标同样生效，只改尺寸与脏标记，不改采样数。本机是本地回放节点，不连接上游服务，所有状态只存在于进程内。窗口拖动与分辨率切换会连续触发 ResizeTarget，短时间内的多次调用按最后一次尺寸生效，中间状态不能被并发的解析任务读到；已解析且无脏区的目标，DirtyTiles 必须返回 0。

硬约束：注册、脏标记、解析共用一把锁，任何失败路径都不能留下半更新；ResolveFrame 是事务操作；单帧解析字节数是不可突破的预算。验收标准是 go test ./... 的 15 条测试必须全部通过，不得修改测试、跳过用例或按固定坐标写死结果；并发与重启场景自己验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
