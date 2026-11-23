# GSB Bugfix Task

## 题面

运营海域上线后，海面在远处出现规则网格接缝，船开过时水面像纸片一样抖动，靠岸处浪高没有衰减，直接切进沙滩，低画质档切换 LOD 时还会闪一下整片网格。更奇怪的是，镜头拉近时接缝会跟着移动，暂停游戏后水面又恢复正常。需要修复水面波浪模块：模块路径 gsb/batch46-game-d-02，文件名 water_wave_guard.go，主类型 WaterWaves，任务类型 Bugfix，目前所有波形计算和网格构建都是空壳。

数据用 WaveParam{Dir, Amplitude, Wavelength, Speed, Steepness}、Vec3、Vertex{Position, Normal} 和 MeshInfo{Rows, Cols, Triangles, Spacing}。关键接口：
NewWaterWaves(extent float64, resolution int) (*WaterWaves, error)
AddWave(p WaveParam) error
WaveCount() int
Reset()
Height(x, z, t float64) (float64, error)
Displacement(x, z, t float64) (Vec3, error)
Normal(x, z, t float64) (Vec3, error)
BuildMesh(t float64, lod int) ([]Vertex, error)
MeshInfo(lod int) (MeshInfo, error)
MaxHeight() float64
ShoreFactor(x, z float64) (float64, error)
SetShoreFade(distance float64) error
Snapshot() ([]byte, error)
Restore(data []byte) error

要修的点和边界：
1. 水面范围必须正且有限，否则 ErrBadExtent；分辨率必须是 [8,512] 内的 2 的幂，否则 ErrBadResolution，避免上游把贴图尺寸当网格尺寸传进来。
2. 波形方向非零、振幅与波长大于零、陡度在 [0,1]、速度有限，否则 ErrBadWave；超过 8 条返回 ErrTooManyWaves 且保留已有波形，不能把栈清空；每条波的参数会随天气切换动态增删，校验失败不能影响正在跑的波。
3. 高度为各波 A*sin(k*(Dir·p)-k*Speed*t) 之和，k=2*pi/Wavelength；振幅 2、波长 20、t=0 时 x=5 处峰值为 2，x=15 处谷值为 -2，误差不超过 1e-9。
4. 水平位移为各波 Steepness*Amplitude*Dir*cos(相位) 之和，Y 恒为 0，垂直部分只由高度提供，渲染端会把两者叠加成最终顶点。
5. 法线必须归一化，无波浪时严格为 (0,1,0)，有波浪时 Y 分量为正，不能出现朝下的翻转法线导致光照全黑。
6. BuildMesh 按 z 行优先、行内按 x 排列，共 (n+1)^2 个顶点，n=resolution>>lod；第一个顶点在左下角，最后一个在右上角，顶点位置等于基础网格加水平位移，Y 取高度。
7. MeshInfo 给出 Rows=Cols=n+1、Triangles=2*n*n、Spacing=extent/n；lod 为负或细分到 n 小于 1 返回 ErrBadLOD。
8. MaxHeight 返回全部振幅之和，供 CPU 剔除和包围盒计算使用，波形增删后必须立即反映新值，包围盒高度按它乘以二。
9. 岸线因子为到最近边界的距离除以岸线距离，截断到 [0,1]；默认岸线距离 extent/8，设置值超出 (0,extent/2] 返回 ErrBadShoreFade。
10. 时间为 NaN 或负数时，所有带时间参数的方法统一返回 ErrBadTime，不能让 NaN 沿顶点数据流进 GPU，也不能污染后续帧。
11. 快照保存波形栈与岸线距离，重建时范围或分辨率不一致、数据损坏返回 ErrSnapshot，且不能改坏当前波形。
12. 8 个渲染线程并发构建 65x65 网格时必须全部成功且顶点数一致，不能读到只写了一半的波形栈，也不得每次构建都复制整份网格。

验收要求 go test ./... 的 15 条全部通过，禁止改测试、跳过或用固定输入写死结果；并发构建和快照恢复需要自行验证；所有计算只依赖传入参数，不要引入全局状态；只做本地实现，不涉及渲染管线其他模块。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 图形渲染
- type: Bugfix
- difficulty: 困难
