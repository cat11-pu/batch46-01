# GSB Bugfix Task

## 题面

东南亚短剧合集上线后，字幕出了两类事故：第二集整集字幕比人声早 1.2 秒，第三集片尾两条字幕叠在同一行；字幕组交付的是按 25fps 母版做的原始时间轴，发行版是 24fps，平台做帧率换算时直接按毫秒截断，一条 0.4 秒的提示字幕被压成零时长，之后就再也显示不出来。运营在后台把相邻两条字幕手工合并，合并后文本少了后半句。回滚一次服务之后，已经修好的顺序和重叠检查又全部失效。

请在模块 `gsb/batch46-game-h-03` 的 `subtitle_sync_guard.go` 里补全 `SubtitleSync` 类型，任务类型为 Bugfix，包名 task，测试文件不要改。请保持这些公开签名：

- `NewSubtitleSync(cfg Config) *SubtitleSync`
- `LoadCues(cues []Cue) error`
- `AppendCue(c Cue) error`
- `Cues() []Cue`
- `At(t time.Duration) (Cue, bool)`
- `Shift(offset time.Duration) error`
- `Rescale(fromFPS, toFPS int) error`
- `Merge(maxGap time.Duration) int`
- `Split(index int, at time.Duration) error`
- `Remove(index int) error`
- `Stats() Stats` / `Snapshot() ([]byte, error)` / `NewSubtitleSyncFromSnapshot(cfg Config, data []byte) (*SubtitleSync, error)`

需要修复的具体现象与边界：

1. 字幕组导出的条目是乱序的，加载后按原始顺序存着，播放时先出后面的台词；必须按开始时间排序后再落库。
2. 结束时间等于或早于开始时间的条目被照单全收，这种字幕永远不会被命中；应返回 `ErrInvalidCue`，整批加载失败且旧轨道不变。
3. 时间区间重叠的两条字幕（0 到 2 秒与 1.5 到 2.5 秒）能同时导入，渲染时叠成两行；应返回 `ErrCueOverlap`。
4. 一次导入 3000 条字幕也不报错，内存和渲染队列被打满；配置的条数上限要硬生效，超过返回 `ErrTooManyCues`。
5. 时长 300 毫秒的字幕被接受，短于最小显示时长（500 毫秒）应该返回 `ErrCueTooShort`。
6. 追加单条字幕时不按开始时间插入，轨道重新变乱；新条目必须落在正确位置，后续 `At` 才能二分命中。
7. 完全相同的字幕（同起止同文本）可以被追加两次，轨道出现重复；应返回 `ErrDuplicateCue`，并与重叠错误区分开。
8. 查询用闭区间，字幕结束时刻仍然返回该条；区间必须左闭右开，相邻两条在交界时刻只能命中后一条。
9. 整体平移后出现负时间码仍然落库，部分条目还被改了一半；只要有任一条会移到零点之前就返回 `ErrNegativeTime`，整次平移不生效。
10. 帧率从 30 换到 24 时时间码没有乘 1.25，整集字幕对不上口型；帧率为 0 或负数返回 `ErrInvalidFPS`，换算后短于最小时长的条目也要整批拒绝（`ErrCueTooShort`），不能只改一半。
11. 合并相邻字幕时把间隔 3 秒的两条也并了，文本只保留前一条；只有间隔不超过阈值的才合并，文本用空格拼接，返回合并次数。
12. 切割点落在条目边界之外仍然切、切出 100 毫秒的碎片、索引越界与切点越界返回同一个错误；应分别返回 `ErrSplitOutsideCue`、`ErrCueTooShort`、`ErrIndexOutOfRange`，失败时轨道保持原样。

硬性要求：字幕读取（`Cues`、`At`）与整体平移并发时，读方必须拿到完整、有序、互不重叠的轨道，不能看到改了一半的时间码；进程重启后从快照恢复，时间码、顺序、条数和重叠判定都要延续；条数上限是最小资源约束，不能靠调用方自觉。

验收：`go test ./...` 的 15 条测试必须全部通过；不得修改测试、跳过用例或按固定输入写死结果；并发与重启场景自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 音视频处理
- type: Bugfix
- difficulty: 困难
