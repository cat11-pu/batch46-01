package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-a-03"
)

func craftCfg() taskpkg.Config {
	return taskpkg.Config{
		SlotsPerCharacter:   4,
		MaxOrdersPerAccount: 8,
		ReservationTTL:      30 * time.Minute,
		MaxPriority:         5,
	}
}

func craftStart() time.Time { return time.Unix(1730000000, 0) }

func arrowRecipe() taskpkg.Recipe {
	return taskpkg.Recipe{
		ID:        "r-arrow",
		Materials: []taskpkg.Material{{ItemID: "WOOD", Qty: 5}, {ItemID: "ORE", Qty: 2}},
		Output:    []taskpkg.Material{{ItemID: "ARROW", Qty: 10}},
	}
}

func swordRecipe() taskpkg.Recipe {
	return taskpkg.Recipe{
		ID:        "r-sword",
		Materials: []taskpkg.Material{{ItemID: "WOOD", Qty: 30}, {ItemID: "ORE", Qty: 10}},
		Output:    []taskpkg.Material{{ItemID: "SWORD", Qty: 1}},
	}
}

func shieldRecipe() taskpkg.Recipe {
	return taskpkg.Recipe{
		ID:        "r-shield",
		Materials: []taskpkg.Material{{ItemID: "WOOD", Qty: 20}, {ItemID: "ORE", Qty: 25}},
		Output:    []taskpkg.Material{{ItemID: "SHIELD", Qty: 1}},
	}
}

// newCraft opens acc-A with chr-A1, 100 WOOD, 40 ORE and three recipes.
func newCraft(t *testing.T) (*taskpkg.CraftingQueue, time.Time) {
	t.Helper()
	now := craftStart()
	q := taskpkg.NewCraftingQueue(craftCfg())
	if err := q.OpenAccount("acc-A"); err != nil {
		t.Fatalf("OpenAccount: %v", err)
	}
	if err := q.OpenCharacter("acc-A", "chr-A1"); err != nil {
		t.Fatalf("OpenCharacter: %v", err)
	}
	if err := q.Grant("acc-A", "WOOD", 100); err != nil {
		t.Fatalf("Grant WOOD: %v", err)
	}
	if err := q.Grant("acc-A", "ORE", 40); err != nil {
		t.Fatalf("Grant ORE: %v", err)
	}
	for _, r := range []taskpkg.Recipe{arrowRecipe(), swordRecipe(), shieldRecipe()} {
		if err := q.RegisterRecipe(r); err != nil {
			t.Fatalf("RegisterRecipe %s: %v", r.ID, err)
		}
	}
	return q, now
}

func craftOrder(t *testing.T, q *taskpkg.CraftingQueue, orderID, characterID, recipeID string, priority int, now time.Time) taskpkg.Order {
	t.Helper()
	o, err := q.Enqueue(taskpkg.EnqueueRequest{
		OrderID:     orderID,
		AccountID:   "acc-A",
		CharacterID: characterID,
		RecipeID:    recipeID,
		Priority:    priority,
	}, now)
	if err != nil {
		t.Fatalf("Enqueue %s: %v", orderID, err)
	}
	return o
}

// Inventory grants are additive, positive-only and per account.
func TestGrantAndAvailability(t *testing.T) {
	q, _ := newCraft(t)
	if got := q.AvailableQty("acc-A", "WOOD"); got != 100 {
		t.Fatalf("available WOOD = %d, want 100", got)
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 0 {
		t.Fatalf("reserved WOOD = %d, want 0", got)
	}
	if err := q.Grant("acc-A", "WOOD", 10); err != nil {
		t.Fatalf("Grant 10 WOOD: %v", err)
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 110 {
		t.Fatalf("available WOOD after grant = %d, want 110", got)
	}
	if err := q.Grant("acc-A", "WOOD", 0); !errors.Is(err, taskpkg.ErrInvalidQty) {
		t.Fatalf("Grant(0) = %v, want ErrInvalidQty", err)
	}
	if err := q.Grant("acc-A", "WOOD", -5); !errors.Is(err, taskpkg.ErrInvalidQty) {
		t.Fatalf("Grant(-5) = %v, want ErrInvalidQty", err)
	}
	if err := q.Grant("acc-Z", "WOOD", 5); !errors.Is(err, taskpkg.ErrUnknownAccount) {
		t.Fatalf("Grant to unknown account = %v, want ErrUnknownAccount", err)
	}
	if got := q.AvailableQty("acc-A", "MITHRIL"); got != 0 {
		t.Fatalf("available MITHRIL = %d, want 0", got)
	}
}

// Enqueueing a recipe reserves every material and exposes the order as ready.
func TestEnqueueReservesMaterials(t *testing.T) {
	q, now := newCraft(t)
	o := craftOrder(t, q, "ord-1", "chr-A1", "r-sword", 0, now)
	if o.State != taskpkg.OrderPending {
		t.Fatalf("new order state = %d, want OrderPending", o.State)
	}
	if o.OrderID != "ord-1" || o.RecipeID != "r-sword" || o.CharacterID != "chr-A1" {
		t.Fatalf("order = %+v, want the submitted identifiers", o)
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 70 {
		t.Fatalf("available WOOD = %d, want 70", got)
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 30 {
		t.Fatalf("reserved WOOD = %d, want 30", got)
	}
	if got := q.AvailableQty("acc-A", "ORE"); got != 30 {
		t.Fatalf("available ORE = %d, want 30", got)
	}
	if got := q.ReservedQty("acc-A", "ORE"); got != 10 {
		t.Fatalf("reserved ORE = %d, want 10", got)
	}
	pending := q.PendingOrders("acc-A")
	if len(pending) != 1 || pending[0].OrderID != "ord-1" {
		t.Fatalf("PendingOrders = %+v, want just ord-1", pending)
	}
	head, err := q.NextOrder("acc-A")
	if err != nil {
		t.Fatalf("NextOrder: %v", err)
	}
	if head.OrderID != "ord-1" {
		t.Fatalf("NextOrder = %s, want ord-1", head.OrderID)
	}
}

// A shortage on the second material must not leave the first one reserved.
func TestEnqueueRollbackWhenAnyMaterialShort(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-1", "chr-A1", "r-shield", 0, now)
	if got := q.AvailableQty("acc-A", "ORE"); got != 15 {
		t.Fatalf("available ORE after one shield = %d, want 15", got)
	}
	_, err := q.Enqueue(taskpkg.EnqueueRequest{OrderID: "ord-2", AccountID: "acc-A", CharacterID: "chr-A1", RecipeID: "r-shield"}, now)
	if !errors.Is(err, taskpkg.ErrInsufficientMaterials) {
		t.Fatalf("short ORE enqueue = %v, want ErrInsufficientMaterials", err)
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 80 {
		t.Fatalf("available WOOD after the rejected enqueue = %d, want 80", got)
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 20 {
		t.Fatalf("reserved WOOD after the rejected enqueue = %d, want 20", got)
	}
	if got := q.ReservedQty("acc-A", "ORE"); got != 25 {
		t.Fatalf("reserved ORE after the rejected enqueue = %d, want 25", got)
	}
	if pending := q.PendingOrders("acc-A"); len(pending) != 1 {
		t.Fatalf("PendingOrders = %d entries, want 1", len(pending))
	}
}

// An order id is a permanent idempotency key.
func TestDuplicateOrderIdRejected(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-1", "chr-A1", "r-arrow", 0, now)
	_, err := q.Enqueue(taskpkg.EnqueueRequest{OrderID: "ord-1", AccountID: "acc-A", CharacterID: "chr-A1", RecipeID: "r-sword"}, now)
	if !errors.Is(err, taskpkg.ErrDuplicateOrder) {
		t.Fatalf("duplicate order id = %v, want ErrDuplicateOrder", err)
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 95 {
		t.Fatalf("available WOOD = %d, want 95 (only the first order reserved)", got)
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 5 {
		t.Fatalf("reserved WOOD = %d, want 5", got)
	}
	if _, err := q.Cancel("acc-A", "ord-1", now); err != nil {
		t.Fatalf("Cancel ord-1: %v", err)
	}
	if _, err := q.Enqueue(taskpkg.EnqueueRequest{OrderID: "ord-1", AccountID: "acc-A", CharacterID: "chr-A1", RecipeID: "r-arrow"}, now); !errors.Is(err, taskpkg.ErrDuplicateOrder) {
		t.Fatalf("reusing a cancelled order id = %v, want ErrDuplicateOrder", err)
	}
}

// The head is the highest priority, then the earliest enqueue, then the
// lowest sequence number.
func TestPriorityOrdersQueueHead(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-low", "chr-A1", "r-arrow", 1, now)
	craftOrder(t, q, "ord-b", "chr-A1", "r-arrow", 3, now)
	craftOrder(t, q, "ord-c", "chr-A1", "r-arrow", 3, now)
	pending := q.PendingOrders("acc-A")
	if len(pending) != 3 || pending[0].OrderID != "ord-b" || pending[1].OrderID != "ord-c" || pending[2].OrderID != "ord-low" {
		t.Fatalf("PendingOrders = %+v, want ord-b, ord-c, ord-low", pending)
	}
	for _, want := range []string{"ord-b", "ord-c", "ord-low"} {
		head, err := q.NextOrder("acc-A")
		if err != nil {
			t.Fatalf("NextOrder before %s: %v", want, err)
		}
		if head.OrderID != want {
			t.Fatalf("NextOrder = %s, want %s", head.OrderID, want)
		}
		if _, err := q.Complete("acc-A", want, now); err != nil {
			t.Fatalf("Complete %s: %v", want, err)
		}
	}
	if _, err := q.NextOrder("acc-A"); !errors.Is(err, taskpkg.ErrQueueEmpty) {
		t.Fatalf("NextOrder on an empty queue = %v, want ErrQueueEmpty", err)
	}
}

// Completion is strictly head-first.
func TestCompleteOutOfOrderRejected(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-low", "chr-A1", "r-arrow", 1, now)
	craftOrder(t, q, "ord-high", "chr-A1", "r-arrow", 4, now)
	if _, err := q.Complete("acc-A", "ord-low", now); !errors.Is(err, taskpkg.ErrNotHead) {
		t.Fatalf("completing a non-head order = %v, want ErrNotHead", err)
	}
	if pending := q.PendingOrders("acc-A"); len(pending) != 2 {
		t.Fatalf("PendingOrders after the rejected completion = %d, want 2", len(pending))
	}
	if _, err := q.Complete("acc-A", "ord-high", now); err != nil {
		t.Fatalf("Complete head: %v", err)
	}
	if _, err := q.Complete("acc-A", "ord-low", now); err != nil {
		t.Fatalf("Complete the new head: %v", err)
	}
}

// Completing consumes the reservation (no refund) and credits the output.
func TestCompleteConsumesReservationAndCreditsOutput(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-1", "chr-A1", "r-sword", 0, now)
	out, err := q.Complete("acc-A", "ord-1", now)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if len(out) != 1 || out[0].ItemID != "SWORD" || out[0].Qty != 1 {
		t.Fatalf("credited output = %+v, want one SWORD", out)
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 0 {
		t.Fatalf("reserved WOOD after completion = %d, want 0", got)
	}
	if got := q.ReservedQty("acc-A", "ORE"); got != 0 {
		t.Fatalf("reserved ORE after completion = %d, want 0", got)
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 70 {
		t.Fatalf("available WOOD after completion = %d, want 70 (consumed, not refunded)", got)
	}
	if got := q.AvailableQty("acc-A", "SWORD"); got != 1 {
		t.Fatalf("available SWORD = %d, want 1", got)
	}
	if pending := q.PendingOrders("acc-A"); len(pending) != 0 {
		t.Fatalf("PendingOrders after completion = %d, want 0", len(pending))
	}
	if _, err := q.Complete("acc-A", "ord-1", now); !errors.Is(err, taskpkg.ErrOrderNotPending) {
		t.Fatalf("second completion = %v, want ErrOrderNotPending", err)
	}
}

// Cancelling refunds the reservation in recipe order and frees the slot.
func TestCancelRefundsMaterialsInOrder(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-1", "chr-A1", "r-sword", 0, now)
	refund, err := q.Cancel("acc-A", "ord-1", now)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	want := []taskpkg.Material{{ItemID: "WOOD", Qty: 30}, {ItemID: "ORE", Qty: 10}}
	if len(refund) != len(want) {
		t.Fatalf("refund = %+v, want %+v", refund, want)
	}
	for i := range want {
		if refund[i] != want[i] {
			t.Fatalf("refund[%d] = %+v, want %+v", i, refund[i], want[i])
		}
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 100 {
		t.Fatalf("available WOOD after cancel = %d, want 100", got)
	}
	if got := q.AvailableQty("acc-A", "ORE"); got != 40 {
		t.Fatalf("available ORE after cancel = %d, want 40", got)
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 0 {
		t.Fatalf("reserved WOOD after cancel = %d, want 0", got)
	}
}

// Cancelling twice and cancelling an unknown id are different failures.
func TestCancelTwiceRejected(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-1", "chr-A1", "r-arrow", 0, now)
	if _, err := q.Cancel("acc-A", "ord-1", now); err != nil {
		t.Fatalf("first Cancel: %v", err)
	}
	if _, err := q.Cancel("acc-A", "ord-1", now); !errors.Is(err, taskpkg.ErrOrderNotPending) {
		t.Fatalf("second Cancel = %v, want ErrOrderNotPending", err)
	}
	if _, err := q.Cancel("acc-A", "ord-missing", now); !errors.Is(err, taskpkg.ErrOrderNotFound) {
		t.Fatalf("Cancel of an unknown order = %v, want ErrOrderNotFound", err)
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 100 {
		t.Fatalf("available WOOD after two cancels = %d, want 100", got)
	}
}

// Priority is bounded by Config.MaxPriority and validated before reserving.
func TestPriorityRangeValidated(t *testing.T) {
	q, now := newCraft(t)
	for _, p := range []int{6, -1} {
		_, err := q.Enqueue(taskpkg.EnqueueRequest{OrderID: "ord-bad", AccountID: "acc-A", CharacterID: "chr-A1", RecipeID: "r-arrow", Priority: p}, now)
		if !errors.Is(err, taskpkg.ErrInvalidPriority) {
			t.Fatalf("priority %d = %v, want ErrInvalidPriority", p, err)
		}
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 0 {
		t.Fatalf("reserved WOOD after rejected priorities = %d, want 0", got)
	}
	for i, p := range []int{0, 5} {
		orderID := "ord-p0"
		if i == 1 {
			orderID = "ord-p5"
		}
		o := craftOrder(t, q, orderID, "chr-A1", "r-arrow", p, now)
		if o.Priority != p {
			t.Fatalf("order %s priority = %d, want %d", orderID, o.Priority, p)
		}
	}
}

// Per-character slots and the per-account budget are both enforced, and a
// cancel frees capacity again.
func TestQueueCapsEnforced(t *testing.T) {
	now := craftStart()
	cfg := craftCfg()
	cfg.SlotsPerCharacter = 2
	cfg.MaxOrdersPerAccount = 5
	q := taskpkg.NewCraftingQueue(cfg)
	if err := q.OpenAccount("acc-C"); err != nil {
		t.Fatalf("OpenAccount: %v", err)
	}
	for _, chr := range []string{"chr-C1", "chr-C2", "chr-C3"} {
		if err := q.OpenCharacter("acc-C", chr); err != nil {
			t.Fatalf("OpenCharacter %s: %v", chr, err)
		}
	}
	if err := q.Grant("acc-C", "WOOD", 100); err != nil {
		t.Fatalf("Grant WOOD: %v", err)
	}
	if err := q.Grant("acc-C", "ORE", 40); err != nil {
		t.Fatalf("Grant ORE: %v", err)
	}
	if err := q.RegisterRecipe(arrowRecipe()); err != nil {
		t.Fatalf("RegisterRecipe: %v", err)
	}
	push := func(orderID, chr string) error {
		_, err := q.Enqueue(taskpkg.EnqueueRequest{OrderID: orderID, AccountID: "acc-C", CharacterID: chr, RecipeID: "r-arrow"}, now)
		return err
	}
	if err := push("ord-1", "chr-C1"); err != nil {
		t.Fatalf("push ord-1: %v", err)
	}
	if err := push("ord-2", "chr-C1"); err != nil {
		t.Fatalf("push ord-2: %v", err)
	}
	if err := push("ord-3", "chr-C1"); !errors.Is(err, taskpkg.ErrQueueFull) {
		t.Fatalf("third order on a full character = %v, want ErrQueueFull", err)
	}
	if err := push("ord-4", "chr-C2"); err != nil {
		t.Fatalf("push ord-4: %v", err)
	}
	if err := push("ord-5", "chr-C2"); err != nil {
		t.Fatalf("push ord-5: %v", err)
	}
	if err := push("ord-6", "chr-C3"); err != nil {
		t.Fatalf("push ord-6: %v", err)
	}
	if err := push("ord-7", "chr-C3"); !errors.Is(err, taskpkg.ErrAccountLimit) {
		t.Fatalf("sixth order on the account = %v, want ErrAccountLimit", err)
	}
	if _, err := q.Cancel("acc-C", "ord-1", now); err != nil {
		t.Fatalf("Cancel ord-1: %v", err)
	}
	if err := push("ord-8", "chr-C1"); err != nil {
		t.Fatalf("push after cancel: %v", err)
	}
}

// Reservations expire after the TTL; expired orders are refunded and closed.
func TestReservationTTLExpiryRefunds(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-1", "chr-A1", "r-arrow", 0, now)
	if n := q.Expire(now.Add(29 * time.Minute)); n != 0 {
		t.Fatalf("Expire before the TTL released %d orders, want 0", n)
	}
	if n := q.Expire(now.Add(30 * time.Minute)); n != 1 {
		t.Fatalf("Expire at the TTL released %d orders, want 1", n)
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 100 {
		t.Fatalf("available WOOD after expiry = %d, want 100", got)
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 0 {
		t.Fatalf("reserved WOOD after expiry = %d, want 0", got)
	}
	if _, err := q.Complete("acc-A", "ord-1", now.Add(31*time.Minute)); !errors.Is(err, taskpkg.ErrOrderNotPending) {
		t.Fatalf("completing an expired order = %v, want ErrOrderNotPending", err)
	}
	if _, err := q.Cancel("acc-A", "ord-1", now.Add(31*time.Minute)); !errors.Is(err, taskpkg.ErrOrderNotPending) {
		t.Fatalf("cancelling an expired order = %v, want ErrOrderNotPending", err)
	}
	if pending := q.PendingOrders("acc-A"); len(pending) != 0 {
		t.Fatalf("PendingOrders after expiry = %d, want 0", len(pending))
	}
}

// Eight concurrent submissions of one order id must reserve once.
func TestConcurrentDuplicateEnqueueReservesOnce(t *testing.T) {
	q, now := newCraft(t)
	req := taskpkg.EnqueueRequest{OrderID: "ord-race", AccountID: "acc-A", CharacterID: "chr-A1", RecipeID: "r-arrow"}
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = q.Enqueue(req, now)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, taskpkg.ErrDuplicateOrder):
			t.Fatalf("concurrent Enqueue returned an unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d concurrent enqueues succeeded, want exactly 1", ok)
	}
	if got := q.ReservedQty("acc-A", "WOOD"); got != 5 {
		t.Fatalf("reserved WOOD after the race = %d, want 5", got)
	}
	if got := q.ReservedQty("acc-A", "ORE"); got != 2 {
		t.Fatalf("reserved ORE after the race = %d, want 2", got)
	}
	if got := q.AvailableQty("acc-A", "WOOD"); got != 95 {
		t.Fatalf("available WOOD after the race = %d, want 95", got)
	}
}

// Characters and orders are account scoped.
func TestCrossAccountIsolation(t *testing.T) {
	q, now := newCraft(t)
	if err := q.OpenAccount("acc-B"); err != nil {
		t.Fatalf("OpenAccount acc-B: %v", err)
	}
	if err := q.OpenCharacter("acc-B", "chr-B1"); err != nil {
		t.Fatalf("OpenCharacter chr-B1: %v", err)
	}
	if err := q.OpenCharacter("acc-B", "chr-A1"); !errors.Is(err, taskpkg.ErrCrossAccount) {
		t.Fatalf("hijacking chr-A1 = %v, want ErrCrossAccount", err)
	}
	_, err := q.Enqueue(taskpkg.EnqueueRequest{OrderID: "ord-b1", AccountID: "acc-B", CharacterID: "chr-A1", RecipeID: "r-arrow"}, now)
	if !errors.Is(err, taskpkg.ErrCrossAccount) {
		t.Fatalf("enqueue on a foreign character = %v, want ErrCrossAccount", err)
	}
	craftOrder(t, q, "ord-A1", "chr-A1", "r-arrow", 0, now)
	if _, err := q.Complete("acc-B", "ord-A1", now); !errors.Is(err, taskpkg.ErrCrossAccount) {
		t.Fatalf("completing a foreign order = %v, want ErrCrossAccount", err)
	}
	if _, err := q.Cancel("acc-B", "ord-A1", now); !errors.Is(err, taskpkg.ErrCrossAccount) {
		t.Fatalf("cancelling a foreign order = %v, want ErrCrossAccount", err)
	}
	if _, err := q.NextOrder("acc-B"); !errors.Is(err, taskpkg.ErrQueueEmpty) {
		t.Fatalf("foreign queue = %v, want ErrQueueEmpty", err)
	}
	head, err := q.NextOrder("acc-A")
	if err != nil {
		t.Fatalf("NextOrder acc-A: %v", err)
	}
	if head.OrderID != "ord-A1" {
		t.Fatalf("acc-A head = %s, want ord-A1", head.OrderID)
	}
}

// A restart keeps inventories, reservations, orders and idempotency keys.
func TestSnapshotRestoresOrdersAndReservations(t *testing.T) {
	q, now := newCraft(t)
	craftOrder(t, q, "ord-persist", "chr-A1", "r-sword", 2, now)
	snap, err := q.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewCraftingQueueFromSnapshot(craftCfg(), snap)
	if err != nil {
		t.Fatalf("NewCraftingQueueFromSnapshot: %v", err)
	}
	if got := restored.AvailableQty("acc-A", "WOOD"); got != 70 {
		t.Fatalf("restored available WOOD = %d, want 70", got)
	}
	if got := restored.ReservedQty("acc-A", "WOOD"); got != 30 {
		t.Fatalf("restored reserved WOOD = %d, want 30", got)
	}
	head, err := restored.NextOrder("acc-A")
	if err != nil {
		t.Fatalf("restored NextOrder: %v", err)
	}
	if head.OrderID != "ord-persist" || head.Priority != 2 {
		t.Fatalf("restored head = %+v, want ord-persist with priority 2", head)
	}
	_, err = restored.Enqueue(taskpkg.EnqueueRequest{OrderID: "ord-persist", AccountID: "acc-A", CharacterID: "chr-A1", RecipeID: "r-arrow"}, now)
	if !errors.Is(err, taskpkg.ErrDuplicateOrder) {
		t.Fatalf("re-enqueue after restart = %v, want ErrDuplicateOrder", err)
	}
	out, err := restored.Complete("acc-A", "ord-persist", now)
	if err != nil {
		t.Fatalf("Complete after restart: %v", err)
	}
	if len(out) != 1 || out[0].ItemID != "SWORD" {
		t.Fatalf("output after restart = %+v, want one SWORD", out)
	}
	if got := restored.ReservedQty("acc-A", "ORE"); got != 0 {
		t.Fatalf("restored reserved ORE after completion = %d, want 0", got)
	}
}
