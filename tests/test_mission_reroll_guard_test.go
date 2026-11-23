package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-a-06"
)

var baseNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func newBoard(t *testing.T, slots, budget, poolCap int) *taskpkg.MissionReroll {
	t.Helper()
	return taskpkg.NewMissionReroll("player-77", taskpkg.Policy{
		Slots:       slots,
		DailyBudget: budget,
		MinTier:     1,
		MaxTier:     5,
		PoolCap:     poolCap,
	})
}

func mission(id string, tier int) taskpkg.Mission {
	return taskpkg.Mission{ID: id, Kind: "combat", Tier: tier, Reward: 100 * tier}
}

func mustSeed(t *testing.T, g *taskpkg.MissionReroll, slot int, m taskpkg.Mission) {
	t.Helper()
	if err := g.SeedSlot(slot, m); err != nil {
		t.Fatalf("SeedSlot(%d, %s): %v", slot, m.ID, err)
	}
}

func TestSeedAndReadSlot(t *testing.T) {
	g := newBoard(t, 3, 2, 5)
	mustSeed(t, g, 1, mission("m-201", 3))

	got, err := g.Slot(1)
	if err != nil {
		t.Fatalf("Slot(1): %v", err)
	}
	if got.ID != "m-201" || got.Tier != 3 || got.Reward != 300 {
		t.Fatalf("unexpected slot offer: %+v", got)
	}
	if _, err := g.Slot(3); !errors.Is(err, taskpkg.ErrBadSlot) {
		t.Fatalf("Slot(3) must be ErrBadSlot, got %v", err)
	}
	if _, err := g.Slot(-1); !errors.Is(err, taskpkg.ErrBadSlot) {
		t.Fatalf("Slot(-1) must be ErrBadSlot, got %v", err)
	}
}

func TestRerollConsumesOneBudgetPoint(t *testing.T) {
	g := newBoard(t, 2, 3, 8)
	mustSeed(t, g, 0, mission("m-100", 2))
	if n, err := g.AddToPool(mission("m-101", 2), mission("m-102", 3)); err != nil || n != 2 {
		t.Fatalf("AddToPool: n=%d err=%v", n, err)
	}
	if left := g.RerollsLeft(baseNow); left != 3 {
		t.Fatalf("initial budget: want 3, got %d", left)
	}

	got, err := g.Reroll(0, "req-1", baseNow)
	if err != nil {
		t.Fatalf("Reroll: %v", err)
	}
	if got.ID == "m-100" {
		t.Fatalf("reroll returned the current offer again")
	}
	if left := g.RerollsLeft(baseNow); left != 2 {
		t.Fatalf("budget after one reroll: want 2, got %d", left)
	}
	hist, err := g.History(0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("history length: want 1, got %d", len(hist))
	}
	if hist[0].FromID != "m-100" || hist[0].ToID != got.ID || hist[0].RequestID != "req-1" {
		t.Fatalf("unexpected history record: %+v", hist[0])
	}
}

func TestBudgetExhaustionKeepsBoard(t *testing.T) {
	g := newBoard(t, 1, 2, 8)
	mustSeed(t, g, 0, mission("m-100", 2))
	if _, err := g.AddToPool(mission("m-101", 2), mission("m-102", 3), mission("m-103", 4)); err != nil {
		t.Fatalf("AddToPool: %v", err)
	}
	if _, err := g.Reroll(0, "r1", baseNow); err != nil {
		t.Fatalf("first reroll: %v", err)
	}
	second, err := g.Reroll(0, "r2", baseNow)
	if err != nil {
		t.Fatalf("second reroll: %v", err)
	}
	if second.ID == "" {
		t.Fatalf("second reroll returned an empty mission")
	}
	before, err := g.Slot(0)
	if err != nil {
		t.Fatalf("Slot: %v", err)
	}
	if _, err := g.Reroll(0, "r3", baseNow); !errors.Is(err, taskpkg.ErrNoReroll) {
		t.Fatalf("third reroll must be ErrNoReroll, got %v", err)
	}
	after, err := g.Slot(0)
	if err != nil {
		t.Fatalf("Slot after failure: %v", err)
	}
	if after.ID != before.ID {
		t.Fatalf("failed reroll changed the offer: %s -> %s", before.ID, after.ID)
	}
	if left := g.RerollsLeft(baseNow); left != 0 {
		t.Fatalf("budget after exhaustion: want 0, got %d", left)
	}
	hist, err := g.History(0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("history length: want 2, got %d", len(hist))
	}
}

func TestLockedSlotIsNotRerolled(t *testing.T) {
	g := newBoard(t, 2, 2, 6)
	mustSeed(t, g, 0, mission("m-100", 2))
	mustSeed(t, g, 1, mission("m-200", 2))
	if _, err := g.AddToPool(mission("m-101", 3)); err != nil {
		t.Fatalf("AddToPool: %v", err)
	}
	if err := g.Lock(0); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if _, err := g.Reroll(0, "lk-1", baseNow); !errors.Is(err, taskpkg.ErrLocked) {
		t.Fatalf("reroll of a locked slot must be ErrLocked, got %v", err)
	}
	if left := g.RerollsLeft(baseNow); left != 2 {
		t.Fatalf("locked reroll must not consume budget, left=%d", left)
	}
	cur, err := g.Slot(0)
	if err != nil {
		t.Fatalf("Slot: %v", err)
	}
	if cur.ID != "m-100" {
		t.Fatalf("locked slot offer changed to %s", cur.ID)
	}
	hist, err := g.History(0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 0 {
		t.Fatalf("locked slot must have no history, got %d", len(hist))
	}
}

func TestPoolCapacityAndDuplicateIDs(t *testing.T) {
	g := newBoard(t, 1, 5, 3)
	accepted, err := g.AddToPool(
		mission("p-1", 2),
		mission("p-2", 2),
		mission("p-1", 3),
		mission("p-3", 3),
		mission("p-4", 4),
	)
	if !errors.Is(err, taskpkg.ErrPoolFull) {
		t.Fatalf("overflowing AddToPool must be ErrPoolFull, got %v", err)
	}
	if accepted != 3 {
		t.Fatalf("accepted count: want 3, got %d", accepted)
	}
	if got := g.PoolSize(); got != 3 {
		t.Fatalf("PoolSize: want 3, got %d", got)
	}
	n, err := g.AddToPool(mission("p-5", 2))
	if n != 0 || !errors.Is(err, taskpkg.ErrPoolFull) {
		t.Fatalf("add on a full pool: n=%d err=%v", n, err)
	}
}

func TestInvalidPoolMissionRejected(t *testing.T) {
	g := newBoard(t, 1, 2, 5)
	if _, err := g.AddToPool(taskpkg.Mission{ID: "p-9", Kind: "combat", Tier: 9}); !errors.Is(err, taskpkg.ErrBadMission) {
		t.Fatalf("tier above MaxTier must be ErrBadMission, got %v", err)
	}
	if _, err := g.AddToPool(taskpkg.Mission{ID: "", Kind: "combat", Tier: 2}); !errors.Is(err, taskpkg.ErrBadMission) {
		t.Fatalf("empty id must be ErrBadMission, got %v", err)
	}
	if got := g.PoolSize(); got != 0 {
		t.Fatalf("rejected candidates must not enter the pool, size=%d", got)
	}
	if err := g.SeedSlot(0, taskpkg.Mission{ID: "m-x", Kind: "combat", Tier: 0}); !errors.Is(err, taskpkg.ErrBadMission) {
		t.Fatalf("seeding an out-of-range tier must be ErrBadMission, got %v", err)
	}
}

func TestRerollSkipsMissionsAlreadyOnBoard(t *testing.T) {
	g := newBoard(t, 2, 3, 6)
	mustSeed(t, g, 0, mission("m-100", 2))
	mustSeed(t, g, 1, mission("m-200", 2))
	if _, err := g.AddToPool(mission("m-100", 2), mission("m-300", 3)); err != nil {
		t.Fatalf("AddToPool: %v", err)
	}

	got, err := g.Reroll(0, "skip-1", baseNow)
	if err != nil {
		t.Fatalf("Reroll: %v", err)
	}
	if got.ID != "m-300" {
		t.Fatalf("reroll must skip missions already on the board, got %s", got.ID)
	}
}

func TestRerollSequenceIsReproducible(t *testing.T) {
	mk := func() *taskpkg.MissionReroll {
		g := newBoard(t, 1, 4, 8)
		mustSeed(t, g, 0, mission("m-100", 2))
		if _, err := g.AddToPool(
			mission("p-1", 1),
			mission("p-2", 2),
			mission("p-3", 3),
			mission("p-4", 4),
			mission("p-5", 5),
		); err != nil {
			t.Fatalf("AddToPool: %v", err)
		}
		return g
	}
	a, b := mk(), mk()
	for i := 0; i < 4; i++ {
		reqID := fmt.Sprintf("det-%d", i)
		ma, err := a.Reroll(0, reqID, baseNow)
		if err != nil {
			t.Fatalf("board A reroll %d: %v", i, err)
		}
		mb, err := b.Reroll(0, reqID, baseNow)
		if err != nil {
			t.Fatalf("board B reroll %d: %v", i, err)
		}
		if ma.ID != mb.ID {
			t.Fatalf("reroll %d diverged: %s vs %s", i, ma.ID, mb.ID)
		}
	}
}

func TestRequestIDReplayIsIdempotent(t *testing.T) {
	g := newBoard(t, 2, 3, 8)
	mustSeed(t, g, 0, mission("m-100", 2))
	mustSeed(t, g, 1, mission("m-200", 2))
	if _, err := g.AddToPool(mission("p-1", 2), mission("p-2", 2), mission("p-3", 2)); err != nil {
		t.Fatalf("AddToPool: %v", err)
	}

	first, err := g.Reroll(0, "req-x", baseNow)
	if err != nil {
		t.Fatalf("first Reroll: %v", err)
	}
	again, err := g.Reroll(0, "req-x", baseNow)
	if err != nil {
		t.Fatalf("replayed Reroll must succeed: %v", err)
	}
	if again.ID != first.ID {
		t.Fatalf("replayed request returned %s, want %s", again.ID, first.ID)
	}
	if left := g.RerollsLeft(baseNow); left != 2 {
		t.Fatalf("replay consumed budget, left=%d", left)
	}
	hist, err := g.History(0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("replay wrote history, length=%d", len(hist))
	}
	if _, err := g.Reroll(1, "req-x", baseNow); !errors.Is(err, taskpkg.ErrRequestConflict) {
		t.Fatalf("request id on another slot must be ErrRequestConflict, got %v", err)
	}
}

func TestBudgetResetsAtUTCMidnight(t *testing.T) {
	day1 := time.Date(2026, 3, 1, 23, 59, 0, 0, time.UTC)
	day2 := time.Date(2026, 3, 2, 0, 0, 1, 0, time.UTC)
	g := newBoard(t, 1, 1, 8)
	mustSeed(t, g, 0, mission("m-100", 2))
	if _, err := g.AddToPool(mission("p-1", 2), mission("p-2", 3), mission("p-3", 4)); err != nil {
		t.Fatalf("AddToPool: %v", err)
	}

	first, err := g.Reroll(0, "night-1", day1)
	if err != nil {
		t.Fatalf("reroll on day one: %v", err)
	}
	if left := g.RerollsLeft(day1); left != 0 {
		t.Fatalf("day one budget: want 0, got %d", left)
	}
	if _, err := g.Reroll(0, "night-2", day1); !errors.Is(err, taskpkg.ErrNoReroll) {
		t.Fatalf("second reroll on day one must be ErrNoReroll, got %v", err)
	}

	next, err := g.Reroll(0, "night-2", day2)
	if err != nil {
		t.Fatalf("reroll on day two must use the new budget: %v", err)
	}
	if next.ID == first.ID {
		t.Fatalf("day two reroll repeated the day one offer")
	}
	if left := g.RerollsLeft(day2); left != 0 {
		t.Fatalf("day two budget: want 0, got %d", left)
	}
	hist, err := g.History(0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 2 || hist[1].Day != "2026-03-02" {
		t.Fatalf("unexpected history: %+v", hist)
	}
}

func TestSnapshotRestoresBoard(t *testing.T) {
	g := newBoard(t, 2, 3, 8)
	mustSeed(t, g, 0, mission("m-100", 2))
	if _, err := g.AddToPool(mission("p-1", 2), mission("p-2", 3), mission("p-3", 4)); err != nil {
		t.Fatalf("AddToPool: %v", err)
	}
	if _, err := g.Reroll(0, "s-1", baseNow); err != nil {
		t.Fatalf("first reroll: %v", err)
	}
	cur, err := g.Reroll(0, "s-2", baseNow)
	if err != nil {
		t.Fatalf("second reroll: %v", err)
	}

	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewMissionRerollFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewMissionRerollFromSnapshot: %v", err)
	}
	if got := restored.PlayerID(); got != "player-77" {
		t.Fatalf("restored player: want player-77, got %q", got)
	}
	if left := restored.RerollsLeft(baseNow); left != 1 {
		t.Fatalf("restored budget: want 1, got %d", left)
	}
	slot, err := restored.Slot(0)
	if err != nil {
		t.Fatalf("restored Slot: %v", err)
	}
	if slot.ID != cur.ID {
		t.Fatalf("restored offer %s, want %s", slot.ID, cur.ID)
	}
	hist, err := restored.History(0)
	if err != nil {
		t.Fatalf("restored History: %v", err)
	}
	if len(hist) != 2 || hist[1].RequestID != "s-2" {
		t.Fatalf("unexpected restored history: %+v", hist)
	}
	req, ok := restored.RequestResult("s-2")
	if !ok || req.ID != cur.ID {
		t.Fatalf("request result lost across restore: ok=%v mission=%+v", ok, req)
	}
}

func TestSnapshotRejectsGarbage(t *testing.T) {
	if _, err := taskpkg.NewMissionRerollFromSnapshot(nil); !errors.Is(err, taskpkg.ErrSnapshotInvalid) {
		t.Fatalf("nil snapshot must be ErrSnapshotInvalid, got %v", err)
	}
	if _, err := taskpkg.NewMissionRerollFromSnapshot([]byte{0x01, 0x02, 0x03}); !errors.Is(err, taskpkg.ErrSnapshotInvalid) {
		t.Fatalf("binary garbage must be ErrSnapshotInvalid, got %v", err)
	}
}

func TestConcurrentRerollsRespectBudget(t *testing.T) {
	g := newBoard(t, 12, 4, 30)
	for i := 0; i < 12; i++ {
		mustSeed(t, g, i, mission(fmt.Sprintf("m-%d", 100+i), 2))
	}
	pool := make([]taskpkg.Mission, 0, 20)
	for i := 0; i < 20; i++ {
		pool = append(pool, mission(fmt.Sprintf("p-%d", 200+i), 3))
	}
	if _, err := g.AddToPool(pool...); err != nil {
		t.Fatalf("AddToPool: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 12)
	for i := 0; i < 12; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = g.Reroll(i, fmt.Sprintf("c-%d", i), baseNow)
		}()
	}
	wg.Wait()

	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		if !errors.Is(err, taskpkg.ErrNoReroll) {
			t.Fatalf("unexpected concurrent reroll error: %v", err)
		}
	}
	if ok != 4 {
		t.Fatalf("budget 4 must allow exactly 4 concurrent rerolls, got %d", ok)
	}
	if left := g.RerollsLeft(baseNow); left != 0 {
		t.Fatalf("budget after concurrent rerolls: want 0, got %d", left)
	}
}

func TestConcurrentSameSlotHasOneWinner(t *testing.T) {
	g := newBoard(t, 1, 1, 8)
	mustSeed(t, g, 0, mission("m-100", 2))
	if _, err := g.AddToPool(
		mission("p-1", 2),
		mission("p-2", 2),
		mission("p-3", 3),
		mission("p-4", 3),
		mission("p-5", 4),
		mission("p-6", 4),
	); err != nil {
		t.Fatalf("AddToPool: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := 0; i < 8; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = g.Reroll(0, fmt.Sprintf("same-%d", i), baseNow)
		}()
	}
	wg.Wait()

	ok := 0
	for _, err := range errs {
		if err == nil {
			ok++
			continue
		}
		if !errors.Is(err, taskpkg.ErrNoReroll) {
			t.Fatalf("unexpected concurrent reroll error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("budget 1 must allow exactly one concurrent reroll, got %d", ok)
	}
	hist, err := g.History(0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 1 {
		t.Fatalf("history after race: want 1 record, got %d", len(hist))
	}
	if left := g.RerollsLeft(baseNow); left != 0 {
		t.Fatalf("budget after race: want 0, got %d", left)
	}
}

func TestPoolExhaustedDoesNotChargeBudget(t *testing.T) {
	g := newBoard(t, 1, 2, 5)
	mustSeed(t, g, 0, mission("m-100", 2))
	if n, err := g.AddToPool(mission("m-100", 2)); err != nil || n != 1 {
		t.Fatalf("AddToPool: n=%d err=%v", n, err)
	}

	if _, err := g.Reroll(0, "none-1", baseNow); !errors.Is(err, taskpkg.ErrPoolExhausted) {
		t.Fatalf("reroll without a free candidate must be ErrPoolExhausted, got %v", err)
	}
	if left := g.RerollsLeft(baseNow); left != 2 {
		t.Fatalf("failed reroll must not charge budget, left=%d", left)
	}
	hist, err := g.History(0)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 0 {
		t.Fatalf("failed reroll must not write history, got %d", len(hist))
	}
	cur, err := g.Slot(0)
	if err != nil {
		t.Fatalf("Slot: %v", err)
	}
	if cur.ID != "m-100" {
		t.Fatalf("failed reroll changed the offer to %s", cur.ID)
	}
}
