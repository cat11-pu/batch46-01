# GSB Bugfix Task

## 题面

夜班收到美术组和 QA 同时反馈：雨天关卡里手电筒照进雾中，近处一片惨白，远处出现明显断层，玩家转身时雾墙跟着跳动，开镜后断层位置还会随视距变化。白天的晴天关卡没有这个问题，只有开启体积雾叠加动态光源时才复现。问题落在体积雾模块：模块路径 gsb/batch46-game-d-01，文件名 volumetric_fog_guard.go，主类型 VolumetricFog，任务类型 Bugfix，深度重建和 froxel 累积目前都是空的，需要按下列契约补齐。

数据结构用 Frustum{Near, Far, TanHalfFovY, Aspect}、FroxelGrid{Width, Height, Depth} 和 FogSample{Slice, Density}。关键接口：
NewVolumetricFog(grid FroxelGrid, fr Frustum) (*VolumetricFog, error)
SliceBoundary(slice int) (float64, error)
SliceCenter(slice int) (float64, error)
FroxelForDepth(viewZ float64) (int, error)
ReconstructViewZ(depth float64) (float64, error)
AddSample(s FogSample) error
AddSamples(samples []FogSample) error
OpticalDepth(slice int) (float64, error)
Transmittance(slice int) (float64, error)
InScatter(slice int) (float64, error)
ScreenToFroxel(uv Vec2, viewZ float64) ([3]int, error)
Reset()
Snapshot() ([]byte, error)
Restore(data []byte) error

具体故障与边界：
1. 切片必须按 boundary(i)=Near*pow(Far/Near,i/Depth) 做指数切分，切片 0 贴近平面对应 Near，最后一层对应 Far；i 超出 [0,Depth] 返回 ErrSliceRange，切片中心取相邻两条边界的几何平均，供散射噪声采样使用。
2. 深度缓冲只有 [0,1]，按 Near*Far/(Far-depth*(Far-Near)) 还原视距；depth 小于 0、大于 1 或为 NaN 都返回 ErrInvalidDepth，不能悄悄夹到边界。
3. 视距小于 Near 或大于等于 Far 返回 ErrDepthRange；恰好落在切片边界上的深度归入后一个切片，只有 Far 侧最后一个切片例外，避免玩家贴墙时整层雾被吞掉。
4. 屏幕 uv 换算 froxel 时 x 取 min(int(u*Width),Width-1)，y 轴翻转，uv 分量超出 [0,1] 返回 ErrBadUV，uv 恰好为 1 时取最后一列或最后一行。
5. 光学深度按密度乘切片长度累加；负数、NaN 或无穷密度返回 ErrNegativeDensity，且这一次写入必须完全回退，不能留下半笔数据。
6. 批量写入必须整批校验后落库，任何一条非法就整批拒绝，避免一次光照 pass 只上传了一半，下一帧再从错误状态里恢复。
7. 透过率取 exp(-累计光学深度)，InScatter 为 1 减透过率，两者都必须随切片单调不增；切片越界统一返回 ErrSliceRange。
8. 构造校验 Near>0、Far>Near、TanHalfFovY 与 Aspect 为正，否则 ErrBadFrustum；网格任一维非正或 Depth 超过 256 返回 ErrBadGrid。
9. 快照要跨进程保留每个切片的密度，切片数不符或数据损坏返回 ErrSnapshot，并且不能破坏调用方现有的密度表。
10. 所有方法可并发调用：200 个协程各写 0.01 密度到同一 slice，累计必须正好等于 2.0 倍切片长度，一个更新都不能丢。
11. 256 层网格下单次查询不得重新分配整张切片表，单帧重建与采样开销保持在毫秒级，不能因为一层雾拖慢整个后处理链。
12. 错误必须可区分：非法 uv 只报 ErrBadUV，视距问题只报 ErrDepthRange，切片和密度问题各自返回对应错误，方便值班同学按日志定位。

验收时 go test ./... 的 15 条必须全部通过，不能改测试、跳过或按固定输入写死结果；并发写入和重启恢复需要自行验证；只做本地实现，不涉及上游提交，也不要求改动渲染管线其他部分。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 地狱
