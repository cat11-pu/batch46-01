# GSB Bugfix Task

## 题面

合服前夜，运营做了一批角色跨服迁移，第二天对账发现有三个人同时拥有两边的金币：源服没扣，目标服还加了两遍；另有一批道具迁移卡在中间态，玩家既不能卖也不能用，工单排到 40 多号。迁移服务当时只写了个“先转出再转入”的脚本，没有状态机也没有幂等键。这单挂在 `gsb/batch46-game-b-04` 模块，逻辑在 `cross_server_transfer_guard.go` 的 `CrossServerTransfer` 上，任务类型 Bugfix：把冻结、预留、提交、回滚这套两阶段流程补完整。

需要保持以下签名不变：`NewCrossServerTransfer(cfg Config) *CrossServerTransfer`、`NewCrossServerTransferFromSnapshot(cfg Config, data []byte) (*CrossServerTransfer, error)`、`RegisterServer(serverID string, goldCapacity int64) error`、`Deposit(serverID, playerID string, gold int64, items map[string]int) error`、`Balance(serverID, playerID string) (Balance, error)`、`Begin(req TransferRequest, now time.Time) (string, error)`、`Approve(transferID string, now time.Time) error`、`Commit(transferID string, now time.Time) error`、`Rollback(transferID string, now time.Time) error`、`State(transferID string) (TransferState, error)`、`Expire(now time.Time) ([]string, error)`、`Snapshot() ([]byte, error)`。错误值使用包里已声明的 ErrUnknownServer、ErrSameServer、ErrInsufficientFunds、ErrInvalidTransfer、ErrTransferConflict、ErrTransferInFlight、ErrTransferNotFound、ErrInvalidTransition、ErrDestinationFull、ErrCapacityInvalid。

状态机只有 frozen、reserved、committed、rolled_back 四种：Begin 校验通过后从源服玩家余额里扣除金币和道具并进入 frozen；Approve 只接受 frozen，检查目标服剩余容量后进入 reserved；Commit 只接受 reserved，把资产加到目标服同 ID 玩家名下并进入 committed；Rollback 接受 frozen 和 reserved，把冻结的资产原样还给源服并进入 rolled_back。每次 Begin 前先查 TransferID 是否已存在，同 ID 同内容直接返回原 ID 且不再扣款，同 ID 不同内容返回冲突；这一步要先于“同一玩家只能有一笔在途迁移”的检查。

要修的具体问题：

1. 转出成功后源服余额没扣，玩家复制出了一份资产。
2. 客户端重试导致目标服加了两遍金币。
3. 同一个 TransferID 换个金额重新提交也被接受，第一笔记录被覆盖。
4. 同一玩家能同时发起多笔迁移，累计扣款超过余额。
5. 目标服容量已经满了 Approve 还放行，单服金币总量突破上限。
6. 已经 reserved 的单子再次 Approve 会把状态改回去。
7. Commit 只写目标服不落状态，重试时又加一次。
8. 已提交的迁移还能回滚，同一批道具两边各留一份。
9. 超时没人处理的冻结单永远不释放，玩家资产长期卡住。
10. 重启后冻结单从内存里消失，玩家资产停在中间态无人认领。
11. 回滚只退金币不退道具，宝石凭空消失。
12. 提交和回滚顺序颠倒时没有报错，靠对账才发现问题。

硬约束：Begin、Approve、Commit、Rollback 和 Expire 必须原子执行，8 个请求并发提交同一笔迁移时全部返回成功、目标服只入账一次、状态只变一次；重启恢复后冻结中的迁移、源服已扣的余额和目标服容量占用必须与重启前一致，恢复后仍能正常走完并保持提交幂等；Begin 和 Commit 对相同输入必须幂等，被拒绝的调用不能留下任何余额或状态变化；单玩家最多一笔在途迁移，目标服金币总量不得超过注册时给的容量，超过一律拒绝而不是按需扩容。

验收：`go test ./...` 全部通过，现有测试不许改、不许跳过、不许按固定输入写死；并发提交和重启恢复要自己验证；只做本地实现，不接真实跨服网关。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
