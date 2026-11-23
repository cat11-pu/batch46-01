package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-a-05"
)

func energyCfg() taskpkg.Config {
	return taskpkg.Config{
		RegenInterval:  6 * time.Minute,
		ReserveCap:     10,
		MaxOfflineSpan: 8 * time.Hour,
	}
}

func energyStart() time.Time { return time.Unix(1730000000, 0) }

func newEnergy(t *testing.T, energy, reserve, maxEnergy int) (*taskpkg.EnergyRegen, time.Time) {
	t.Helper()
	now := energyStart()
	e := taskpkg.NewEnergyRegen(energyCfg())
	if err := e.RegisterPlayer(taskpkg.Player{
		PlayerID:    "p-1",
		Energy:      energy,
		Reserve:     reserve,
		MaxEnergy:   maxEnergy,
		LastRegenAt: now,
	}); err != nil {
		t.Fatalf("RegisterPlayer: %v", err)
	}
	return e, now
}

// Registration validates the record and the state view is readable.
func TestRegisterPlayerAndState(t *testing.T) {
	e, now := newEnergy(t, 100, 0, 120)
	st, err := e.State("p-1", now)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 100 || st.MaxEnergy != 120 || st.Reserve != 0 || st.ReserveCap != 10 {
		t.Fatalf("state = %+v, want 100/0/120 with reserve cap 10", st)
	}
	if st.PendingPoints != 0 {
		t.Fatalf("PendingPoints right after registration = %d, want 0", st.PendingPoints)
	}
	if err := e.RegisterPlayer(taskpkg.Player{PlayerID: "p-1", Energy: 10, MaxEnergy: 120, LastRegenAt: now}); !errors.Is(err, taskpkg.ErrPlayerExists) {
		t.Fatalf("duplicate registration = %v, want ErrPlayerExists", err)
	}
	if err := e.RegisterPlayer(taskpkg.Player{PlayerID: "p-over", Energy: 200, MaxEnergy: 120, LastRegenAt: now}); !errors.Is(err, taskpkg.ErrInvalidPlayer) {
		t.Fatalf("energy above the cap = %v, want ErrInvalidPlayer", err)
	}
	if err := e.RegisterPlayer(taskpkg.Player{PlayerID: "p-neg", Energy: -1, MaxEnergy: 120, LastRegenAt: now}); !errors.Is(err, taskpkg.ErrInvalidPlayer) {
		t.Fatalf("negative energy = %v, want ErrInvalidPlayer", err)
	}
	if _, err := e.State("p-ghost", now); !errors.Is(err, taskpkg.ErrUnknownPlayer) {
		t.Fatalf("State of an unknown player = %v, want ErrUnknownPlayer", err)
	}
}

// One point per full interval and the pending prediction must agree.
func TestRegenAddsPointPerInterval(t *testing.T) {
	e, now := newEnergy(t, 100, 0, 120)
	later := now.Add(18 * time.Minute)
	if p := e.PendingPoints("p-1", later); p != 3 {
		t.Fatalf("PendingPoints after 18m = %d, want 3", p)
	}
	if d := e.NextPointIn("p-1", later); d != 6*time.Minute {
		t.Fatalf("NextPointIn after 18m = %v, want 6m", d)
	}
	grant, err := e.Regen("p-1", later)
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if grant.ToEnergy != 3 || grant.ToReserve != 0 || grant.Dropped != 0 {
		t.Fatalf("grant = %+v, want 3 points to the bar", grant)
	}
	st, err := e.State("p-1", later)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 103 {
		t.Fatalf("energy after 18m = %d, want 103", st.Energy)
	}
	if st.PendingPoints != 0 {
		t.Fatalf("PendingPoints after settling = %d, want 0", st.PendingPoints)
	}
}

// Sub-interval time must survive settlement.
func TestRegenKeepsSubIntervalRemainder(t *testing.T) {
	e, now := newEnergy(t, 100, 0, 120)
	first := now.Add(20 * time.Minute)
	grant, err := e.Regen("p-1", first)
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if grant.ToEnergy != 3 {
		t.Fatalf("grant at 20m = %+v, want 3 points", grant)
	}
	if d := e.NextPointIn("p-1", first); d != 4*time.Minute {
		t.Fatalf("NextPointIn at 20m = %v, want 4m", d)
	}
	second := now.Add(24 * time.Minute)
	grant, err = e.Regen("p-1", second)
	if err != nil {
		t.Fatalf("second Regen: %v", err)
	}
	if grant.ToEnergy != 1 {
		t.Fatalf("grant at 24m = %+v, want 1 more point", grant)
	}
	st, err := e.State("p-1", second)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 104 {
		t.Fatalf("energy after 24m = %d, want 104", st.Energy)
	}
}

// Overflow above the bar goes into the reserve bucket.
func TestRegenOverflowsIntoReserve(t *testing.T) {
	e, now := newEnergy(t, 118, 0, 120)
	grant, err := e.Regen("p-1", now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if grant.ToEnergy != 2 || grant.ToReserve != 3 || grant.Dropped != 0 {
		t.Fatalf("grant = %+v, want 2 to the bar, 3 to the reserve", grant)
	}
	st, err := e.State("p-1", now.Add(30*time.Minute))
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 120 || st.Reserve != 3 {
		t.Fatalf("state = %+v, want 120 energy and 3 reserve", st)
	}
}

// Overflow beyond the reserve cap is dropped, not stored.
func TestRegenDropsBeyondReserveCap(t *testing.T) {
	e, now := newEnergy(t, 120, 8, 120)
	grant, err := e.Regen("p-1", now.Add(60*time.Minute))
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if grant.ToEnergy != 0 || grant.ToReserve != 2 || grant.Dropped != 8 {
		t.Fatalf("grant = %+v, want 2 to the reserve and 8 dropped", grant)
	}
	st, err := e.State("p-1", now.Add(60*time.Minute))
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 120 || st.Reserve != 10 {
		t.Fatalf("state = %+v, want a full bar and a full reserve", st)
	}
}

// Idle time beyond MaxOfflineSpan is not compensated and settling twice at the
// same instant grants nothing.
func TestOfflineSpanCapped(t *testing.T) {
	e, now := newEnergy(t, 100, 0, 120)
	late := now.Add(10 * time.Hour)
	grant, err := e.Regen("p-1", late)
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if grant.ToEnergy != 20 || grant.ToReserve != 10 || grant.Dropped != 70 {
		t.Fatalf("grant after 10 idle hours = %+v, want 20/10/70", grant)
	}
	st, err := e.State("p-1", late)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 120 || st.Reserve != 10 {
		t.Fatalf("state = %+v, want a full bar and a full reserve", st)
	}
	again, err := e.Regen("p-1", late)
	if err != nil {
		t.Fatalf("second Regen: %v", err)
	}
	if again.ToEnergy != 0 || again.ToReserve != 0 || again.Dropped != 0 {
		t.Fatalf("second settlement at the same instant = %+v, want nothing", again)
	}
	if p := e.PendingPoints("p-1", late); p != 0 {
		t.Fatalf("PendingPoints after settlement = %d, want 0", p)
	}
}

// PendingPoints is a prediction and must not settle anything.
func TestPendingPointsDoesNotMutate(t *testing.T) {
	e, now := newEnergy(t, 100, 0, 120)
	later := now.Add(25 * time.Minute)
	if p := e.PendingPoints("p-1", later); p != 4 {
		t.Fatalf("PendingPoints at 25m = %d, want 4", p)
	}
	st, err := e.State("p-1", later)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 100 {
		t.Fatalf("PendingPoints mutated energy to %d, want 100", st.Energy)
	}
	if st.PendingPoints != 4 {
		t.Fatalf("State PendingPoints = %d, want 4", st.PendingPoints)
	}
	grant, err := e.Regen("p-1", later)
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if grant.ToEnergy != 4 {
		t.Fatalf("grant after the prediction = %+v, want 4 points", grant)
	}
}

// A clock that moved backwards must be refused without touching the player.
func TestClockRollbackRejected(t *testing.T) {
	e, now := newEnergy(t, 100, 0, 120)
	back := now.Add(-time.Minute)
	if _, err := e.Regen("p-1", back); !errors.Is(err, taskpkg.ErrClockSkew) {
		t.Fatalf("Regen with a moved-back clock = %v, want ErrClockSkew", err)
	}
	if _, err := e.Spend(taskpkg.SpendRequest{RequestID: "sp-back", PlayerID: "p-1", Amount: 1}, back); !errors.Is(err, taskpkg.ErrClockSkew) {
		t.Fatalf("Spend with a moved-back clock = %v, want ErrClockSkew", err)
	}
	if _, err := e.Refill(taskpkg.RefillRequest{RequestID: "rf-back", PlayerID: "p-1", Amount: 1}, back); !errors.Is(err, taskpkg.ErrClockSkew) {
		t.Fatalf("Refill with a moved-back clock = %v, want ErrClockSkew", err)
	}
	st, err := e.State("p-1", now)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 100 {
		t.Fatalf("energy after refused calls = %d, want 100", st.Energy)
	}
	grant, err := e.Regen("p-1", now.Add(6*time.Minute))
	if err != nil {
		t.Fatalf("Regen after the refusal: %v", err)
	}
	if grant.ToEnergy != 1 {
		t.Fatalf("grant after the refusal = %+v, want 1 point", grant)
	}
}

// Spending drains the bar first and the reserve bucket second.
func TestSpendUsesEnergyThenReserve(t *testing.T) {
	e, now := newEnergy(t, 3, 5, 120)
	res, err := e.Spend(taskpkg.SpendRequest{RequestID: "sp-1", PlayerID: "p-1", Amount: 6}, now)
	if err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if res.FromEnergy != 3 || res.FromReserve != 3 || res.EnergyLeft != 0 || res.ReserveLeft != 2 {
		t.Fatalf("result = %+v, want 3 from the bar, 3 from the reserve, 0/2 left", res)
	}
	if res.Amount != 6 || res.PlayerID != "p-1" || res.RequestID != "sp-1" {
		t.Fatalf("result identifiers = %+v", res)
	}
	if _, err := e.Spend(taskpkg.SpendRequest{RequestID: "sp-2", PlayerID: "p-1", Amount: 5}, now); !errors.Is(err, taskpkg.ErrInsufficientEnergy) {
		t.Fatalf("spend beyond the total = %v, want ErrInsufficientEnergy", err)
	}
	st, err := e.State("p-1", now)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 0 || st.Reserve != 2 {
		t.Fatalf("state after the rejected spend = %+v, want 0/2", st)
	}
}

// Amounts are validated before anything settles, while a genuine shortfall is
// reported after the regeneration was already applied.
func TestSpendValidationAndRegenSettlement(t *testing.T) {
	e, now := newEnergy(t, 0, 0, 120)
	later := now.Add(30 * time.Minute)
	if _, err := e.Spend(taskpkg.SpendRequest{RequestID: "sp-zero", PlayerID: "p-1", Amount: 0}, later); !errors.Is(err, taskpkg.ErrInvalidAmount) {
		t.Fatalf("spend 0 = %v, want ErrInvalidAmount", err)
	}
	if _, err := e.Spend(taskpkg.SpendRequest{RequestID: "sp-neg", PlayerID: "p-1", Amount: -3}, later); !errors.Is(err, taskpkg.ErrInvalidAmount) {
		t.Fatalf("spend -3 = %v, want ErrInvalidAmount", err)
	}
	st, err := e.State("p-1", later)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 0 || st.PendingPoints != 5 {
		t.Fatalf("rejected amounts settled something: %+v, want 0 energy and 5 pending", st)
	}
	if _, err := e.Spend(taskpkg.SpendRequest{RequestID: "sp-big", PlayerID: "p-1", Amount: 500}, later); !errors.Is(err, taskpkg.ErrInsufficientEnergy) {
		t.Fatalf("spend beyond the total = %v, want ErrInsufficientEnergy", err)
	}
	st, err = e.State("p-1", later)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 5 {
		t.Fatalf("energy after a shortfall = %d, want the 5 regenerated points", st.Energy)
	}
	if p := e.PendingPoints("p-1", later); p != 0 {
		t.Fatalf("PendingPoints after the shortfall = %d, want 0", p)
	}
}

// Replaying a spend returns the stored result without charging again.
func TestSpendIdempotentReplayAndConflict(t *testing.T) {
	e, now := newEnergy(t, 100, 0, 120)
	req := taskpkg.SpendRequest{RequestID: "sp-1", PlayerID: "p-1", Amount: 10}
	first, err := e.Spend(req, now)
	if err != nil {
		t.Fatalf("Spend: %v", err)
	}
	replay, err := e.Spend(req, now)
	if err != nil {
		t.Fatalf("replayed Spend: %v", err)
	}
	if replay != first {
		t.Fatalf("replay = %+v, want the stored result %+v", replay, first)
	}
	st, err := e.State("p-1", now)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 90 {
		t.Fatalf("energy after the replay = %d, want 90 (charged once)", st.Energy)
	}
	conflict := req
	conflict.Amount = 20
	if _, err := e.Spend(conflict, now); !errors.Is(err, taskpkg.ErrIDConflict) {
		t.Fatalf("conflicting payload = %v, want ErrIDConflict", err)
	}
	if st, err = e.State("p-1", now); err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 90 {
		t.Fatalf("energy after the conflict = %d, want 90", st.Energy)
	}
}

// Concurrent spends must never drive the balance negative.
func TestConcurrentSpendsNeverNegative(t *testing.T) {
	e, now := newEnergy(t, 10, 0, 120)
	errs := make([]error, 20)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = e.Spend(taskpkg.SpendRequest{
				RequestID: fmt.Sprintf("sp-race-%d", i),
				PlayerID:  "p-1",
				Amount:    1,
			}, now)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, taskpkg.ErrInsufficientEnergy):
			t.Fatalf("concurrent Spend returned an unexpected error: %v", err)
		}
	}
	if ok != 10 {
		t.Fatalf("%d concurrent spends succeeded, want exactly 10", ok)
	}
	st, err := e.State("p-1", now)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 0 {
		t.Fatalf("energy after the race = %d, want 0", st.Energy)
	}
	if st.Reserve != 0 {
		t.Fatalf("reserve after the race = %d, want 0", st.Reserve)
	}
}

// A refill fills the bar, then the reserve, then drops the rest.
func TestRefillOverflowToReserveThenDropped(t *testing.T) {
	e, now := newEnergy(t, 115, 0, 120)
	first, err := e.Refill(taskpkg.RefillRequest{RequestID: "rf-1", PlayerID: "p-1", Amount: 10}, now)
	if err != nil {
		t.Fatalf("Refill: %v", err)
	}
	if first.ToEnergy != 5 || first.ToReserve != 5 || first.Dropped != 0 {
		t.Fatalf("first refill = %+v, want 5 to the bar and 5 to the reserve", first)
	}
	second, err := e.Refill(taskpkg.RefillRequest{RequestID: "rf-2", PlayerID: "p-1", Amount: 10}, now)
	if err != nil {
		t.Fatalf("second Refill: %v", err)
	}
	if second.ToEnergy != 0 || second.ToReserve != 5 || second.Dropped != 5 {
		t.Fatalf("second refill = %+v, want 5 to the reserve and 5 dropped", second)
	}
	st, err := e.State("p-1", now)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.Energy != 120 || st.Reserve != 10 {
		t.Fatalf("state after refills = %+v, want 120/10", st)
	}
}

// Raising the cap never fills the bar and must strictly increase it.
func TestRaiseCapDoesNotRefillEnergy(t *testing.T) {
	e, now := newEnergy(t, 120, 0, 120)
	if err := e.RaiseCap("p-1", 130); err != nil {
		t.Fatalf("RaiseCap(130): %v", err)
	}
	st, err := e.State("p-1", now)
	if err != nil {
		t.Fatalf("State: %v", err)
	}
	if st.MaxEnergy != 130 {
		t.Fatalf("max energy = %d, want 130", st.MaxEnergy)
	}
	if st.Energy != 120 {
		t.Fatalf("energy after raising the cap = %d, want to stay at 120", st.Energy)
	}
	if err := e.RaiseCap("p-1", 125); !errors.Is(err, taskpkg.ErrInvalidCap) {
		t.Fatalf("lowering the cap = %v, want ErrInvalidCap", err)
	}
	if err := e.RaiseCap("p-1", 0); !errors.Is(err, taskpkg.ErrInvalidCap) {
		t.Fatalf("zero cap = %v, want ErrInvalidCap", err)
	}
	if err := e.RaiseCap("p-ghost", 150); !errors.Is(err, taskpkg.ErrUnknownPlayer) {
		t.Fatalf("raising the cap of an unknown player = %v, want ErrUnknownPlayer", err)
	}
	grant, err := e.Regen("p-1", now.Add(6*time.Minute))
	if err != nil {
		t.Fatalf("Regen: %v", err)
	}
	if grant.ToEnergy != 1 {
		t.Fatalf("grant after the cap raise = %+v, want 1 point", grant)
	}
}

// A restart keeps the stamina state, the anchor and settled request ids.
func TestSnapshotRestoresStateAndDedupe(t *testing.T) {
	e, now := newEnergy(t, 100, 0, 120)
	req := taskpkg.SpendRequest{RequestID: "sp-1", PlayerID: "p-1", Amount: 10}
	first, err := e.Spend(req, now)
	if err != nil {
		t.Fatalf("Spend: %v", err)
	}
	settled := now.Add(18 * time.Minute)
	if _, err := e.Regen("p-1", settled); err != nil {
		t.Fatalf("Regen: %v", err)
	}
	snap, err := e.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewEnergyRegenFromSnapshot(energyCfg(), snap)
	if err != nil {
		t.Fatalf("NewEnergyRegenFromSnapshot: %v", err)
	}
	st, err := restored.State("p-1", settled)
	if err != nil {
		t.Fatalf("restored State: %v", err)
	}
	if st.Energy != 93 || st.Reserve != 0 {
		t.Fatalf("restored state = %+v, want 93 energy", st)
	}
	if p := restored.PendingPoints("p-1", settled); p != 0 {
		t.Fatalf("restored PendingPoints = %d, want 0 (anchor must survive)", p)
	}
	replay, err := restored.Spend(req, settled.Add(12*time.Minute))
	if err != nil {
		t.Fatalf("replayed Spend after restart: %v", err)
	}
	if replay != first {
		t.Fatalf("replay after restart = %+v, want %+v", replay, first)
	}
	if st, err = restored.State("p-1", settled.Add(12*time.Minute)); err != nil {
		t.Fatalf("State: %v", err)
	}
	stNow := st
	if stNow.Energy != 93 {
		t.Fatalf("energy after replay = %d, want 93 (no re-charge, no re-settle)", stNow.Energy)
	}
	if err := restored.RegisterPlayer(taskpkg.Player{PlayerID: "p-1", Energy: 10, MaxEnergy: 120, LastRegenAt: settled}); !errors.Is(err, taskpkg.ErrPlayerExists) {
		t.Fatalf("duplicate registration after restart = %v, want ErrPlayerExists", err)
	}
}
