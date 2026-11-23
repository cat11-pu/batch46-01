# GSB Bugfix Task

## 题面

游客账号合并活动上线第一周，客服收到两类工单：一类说合并后金币直接翻倍，另一类说辛苦攒的 400 瓶药水全没了；还有人发现自己的 30 级大号被并进了 5 级小号，排行榜上出现了两个同名玩家。翻代码发现合并脚本只是把两个账号字段相加，既没有校验上限，也没有把源账号作废。这单挂在 `gsb/batch46-game-b-05` 模块，逻辑在 `account_merge_guard.go` 的 `AccountMerge` 上，任务类型 Bugfix：把合并计划、预览、执行和撤销这套流程补起来。

需要保持以下签名不变：`NewAccountMerge() *AccountMerge`、`NewAccountMergeFromSnapshot(data []byte) (*AccountMerge, error)`、`Put(a Account) error`、`Get(id string) (Account, error)`、`Plan(p MergePlan, at time.Time) (MergePreview, error)`、`Apply(mergeID string, at time.Time) (Account, error)`、`Cancel(mergeID string) error`、`Pending() []MergePlan`、`History() []MergeRecord`、`Snapshot() ([]byte, error)`。错误值使用包里已声明的 ErrInvalidAccount、ErrAccountNotFound、ErrAccountMerged、ErrSameAccount、ErrDuplicatePlan、ErrPlanNotFound、ErrPlanCancelled、ErrMergeCommitted、ErrNicknameTaken、ErrGoldOverflow、ErrStackOverflow、ErrInvalidPolicy。

合并规则：单账号金币上限 MaxGold 十亿，单格道具上限 MaxStack 999；合并后金币取两账号之和，道具按品类相加，等级取较高者；合并后金币或任一品类超过上限必须拒绝，不允许截断。Plan 返回的预览里，Target 是合并后的目标账号（昵称、金币、道具、等级都是合并结果），ItemsMerged 是合并后的完整道具表。昵称策略只有 keep_target、keep_source、suffix 三种：keep_target 用目标昵称；keep_source 用源昵称，但被第三个账号占用时返回 ErrNicknameTaken；suffix 从“目标昵称-2”开始逐个尝试，取第一个没被占用的。Put 不校验昵称唯一，历史数据里允许重名，唯一性在合并时才强制。

要修的具体问题：

1. 合并后源账号还留在库里，金币凭空多出一份。
2. 源账号作废后 Get 仍然返回它，调用方以为还能登录。
3. 道具只取目标侧的值，源侧 400 瓶药水合并后消失。
4. 等级直接取目标值，30 级游客被并进 5 级小号。
5. 合并后金币超过十亿照样写入，余额回绕成负数。
6. 单格道具超过 999 没有拦截，背包界面显示异常。
7. keep_source 策略在昵称被第三人占用时仍然生效，排行榜出现重名。
8. suffix 策略永远只拼一个 -2，撞名后直接失败。
9. 同一次合并被重试两次，金币加了两遍，历史里出现两条记录。
10. 已提交的合并还能被取消，源账号复活且资产回到合并前。
11. 已取消的计划还能被执行，账号被悄悄并掉。
12. 重启后待执行计划全部丢失，已经合并的账号重新出现。

硬约束：Plan 只做校验和预览，不得改动任何账号；Apply 必须原子完成“目标覆盖、源作废、计划出队、历史落账”，8 个请求并发执行同一计划时只合并一次、历史只留一条；重启恢复后待执行计划、已作废账号和历史必须与重启前一致，已执行计划的重复 Apply 仍返回同一个合并结果且不再加钱；Pending 按 MergeID 升序返回，History 按执行先后返回，不允许因遍历 map 导致顺序抖动；账号字段、单账号上限和单格上限都是硬上限，超过就拒绝而不是按需扩容。

验收：`go test ./...` 全部通过，现有测试不许改、不许跳过、不许按固定输入写死；并发执行和重启恢复要自己跑；只做本地实现，不接真实账号中心。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 困难
