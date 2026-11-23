# GSB Bugfix Task

## 题面

公会开荒当晚制作队列连环出事：玩家连点两次制作，同一份材料被扣了两遍；一件要木头加矿石的盾牌在矿石不足时仍然把木头锁住，之后所有配方都提示缺料；队长把订单优先级调到最高，结算时却是低优先级的先完成；服务重启后待制作订单全部消失，材料却还显示被占用。客服临时给受影响玩家补了材料，第二天同样的投诉又来了。需要把队列重做，位置在 gsb/batch46-game-a-03 的 crafting_queue_guard.go，类型 CraftingQueue，任务类型 Bugfix。

接口：NewCraftingQueue(cfg Config) *CraftingQueue、NewCraftingQueueFromSnapshot(cfg Config, data []byte) (*CraftingQueue, error)、OpenAccount(accountID string) error、OpenCharacter(accountID, characterID string) error、Grant(accountID, itemID string, qty int) error、AvailableQty(accountID, itemID string) int、ReservedQty(accountID, itemID string) int、RegisterRecipe(r Recipe) error、Enqueue(req EnqueueRequest, now time.Time) (Order, error)、NextOrder(accountID string) (Order, error)、Complete(accountID, orderID string, now time.Time) ([]Material, error)、Cancel(accountID, orderID string, now time.Time) ([]Material, error)、PendingOrders(accountID string) []Order、Expire(now time.Time) int、Snapshot() ([]byte, error)。

契约：一、Grant 数量必须为正否则 ErrInvalidQty，账号不存在返回 ErrUnknownAccount；角色号全局唯一，抢注别人的角色或拿别人的角色下单返回 ErrCrossAccount，未知角色返回 ErrUnknownCharacter；重复开通同一账号或同一角色是幂等的。二、配方号或材料为空返回 ErrInvalidRecipe，配方号重复返回 ErrDuplicateRecipe。三、下单必须一次性预留全部材料，任何一项不足返回 ErrInsufficientMaterials，且不得留下部分预留。四、订单号重复返回 ErrDuplicateOrder，订单取消后该号仍然不能复用。五、优先级必须落在 0 到 Config.MaxPriority，否则 ErrInvalidPriority，校验失败不得预占材料。六、每个角色挂单数达到 Config.SlotsPerCharacter 返回 ErrQueueFull，账号挂单总数达到 Config.MaxOrdersPerAccount 返回 ErrAccountLimit，取消后名额要归还。七、NextOrder 取优先级最高、入队最早、序号最小的待制作订单，PendingOrders 按同一顺序返回且不含已完成、已取消和已过期的订单，空队列返回 ErrQueueEmpty。八、Complete 只能结算队首，越序返回 ErrNotHead；结算消耗预留且不退还材料，按配方 Output 入账并作为返回值，非待制作状态返回 ErrOrderNotPending。九、Cancel 可取消任意待制作订单，按配方材料顺序原样退还并作为返回值，退还顺序要与配方登记顺序一致，二次取消返回 ErrOrderNotPending，未知订单返回 ErrOrderNotFound。十、超过 Config.ReservationTTL 的订单由 Expire 统一退还，恰好到期即算过期，返回本次过期数量，重复调用不得重复退还，过期订单不能再结算或取消。十一、并发提交同一订单号只能成功一次，只预留一份材料。十二、Snapshot 恢复后库存、预留、订单顺序与已用订单号必须一致，重复下单仍被拒，等待中的订单仍能按原顺序结算。同一账号下的角色共享库存但队列彼此独立，制作产出的物品进入账号库存后可作为其他配方的材料，所有读写都要并发安全。

验收：go test ./... 的 15 条全过；不得改测试、不得跳过、不得按固定输入写死。并发与重启自行验证；只做本地实现。

## 运行方式

```bash
go test ./...
```

## 平台字段

- domain: 游戏、图形与多媒体
- sub: 游戏开发
- type: Bugfix
- difficulty: 地狱
