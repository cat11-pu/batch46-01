# GSB Bugfix Task

## 题面

美术在样板间验收时发现角色走到两个光照探针中间，墙面颜色会忽明忽暗，墙角出现整块黑色，烘焙列表在进程重启后全部丢失，重新加载场景时又只能看到一半探针的亮度。我们把探针密度从 4x4x4 提到 8x8x8 后问题更明显，低配机还会明显卡顿。需要补全光照探针网格：模块路径 gsb/batch46-game-d-03，文件名 light_probe_guard.go，主类型 LightProbe，任务类型 Bugfix。

系数用 SH9{C [9]Vec3}，约定 0 号是直流项，1 到 3 号是 Y、Z、X 线性项，4 到 8 号是 XY、YZ、3Z^2-1、XZ、X^2-Y^2 二次项。关键接口：
NewLightProbe(origin Vec3, spacing float64, counts [3]int) (*LightProbe, error)
ProbeCount() int
Index(x, y, z int) (int, error)
Coordinates(index int) ([3]int, error)
SetProbe(x, y, z int, sh SH9) error
GetProbe(x, y, z int) (SH9, error)
CellForPosition(p Vec3) ([3]int, Vec3, error)
Trilinear(p Vec3) (SH9, error)
Tetrahedral(p Vec3) (SH9, error)
Irradiance(sh SH9, n Vec3) Vec3
Blend(a, b SH9, w float64) SH9
Invalidate(x, y, z int) error
DirtyCount() int
Snapshot() ([]byte, error)
Restore(data []byte) error

需要落地的行为：
1. 间距必须正且有限，否则 ErrBadSpacing；每轴 2 到 16 个探针且总数不超过 4096，否则 ErrBadGrid，避免美术误填 0 或超大网格导致启动即崩。
2. 索引按行主序 x + counts[0]*(y+counts[1]*z)，越界返回 ErrIndexRange；反解越界同样处理，槽位和坐标必须一一对应。
3. 从未设置过的探针返回零 SH9 和 nil 错误，不能被当成越界，烘焙未完成时渲染端仍然要能安全采样。
4. 位置定位返回所在单元与 [0,1] 局部坐标；位于远侧面时归入最后一格且局部坐标为 1；体积外或 NaN 返回 ErrOutsideVolume。
5. 三线性插值使用 8 个角点，四面体插值使用包含该点的 4 个角点；常量场插值结果必须等于常量，落在角点上必须精确等于该探针，不能因权重误差把墙角压黑。
6. 辐照度按直流项乘 pi、线性项乘 2*pi/3、二次项乘 pi/4 计算，n 内部归一化，零向量返回零，方向翻转时结果要符合余弦瓣形状。
7. Blend 的权重截断到 [0,1]，逐系数线性插值，w 为 0 和 1 时必须原样返回输入，不能引入额外衰减。
8. Invalidate 必须幂等，重复标记不增加计数；SetProbe 清除该探针的脏标记，脏集合是重烘焙队列的唯一依据。
9. 快照保存全部探针和脏集合，原点、间距或轴数不一致、数据损坏返回 ErrSnapshot，不能改坏当前网格。
10. 32 个协程并发读取插值和探针数据时不得报错或读到撕裂值，烘焙线程写入时渲染线程也不能看到半个系数。
11. 4096 个探针是硬上限，运行期不得按查询次数扩容或反复分配整张表，查询应该只读当前数组，避免每帧产生大量临时对象。
12. 错误语义要分清楚：索引问题只报 ErrIndexRange，位置问题只报 ErrOutsideVolume，快照问题只报 ErrSnapshot。

验收时 go test ./... 的 15 条必须全过，不能改测试、跳过或针对固定坐标写死；并发读取和快照恢复自行验证；探针数据只来自调用方写入，不要读取外部文件；只做本地实现，不涉及烘焙工具链的其他部分。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 地狱
