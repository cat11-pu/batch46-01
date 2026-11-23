package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-a-01"
)

func ladderCfg() taskpkg.Config {
	return taskpkg.Config{
		DecayInterval: 24 * time.Hour,
		DecayPerDay:   12,
		MinRating:     100,
		HistoryCap:    20,
	}
}

func ladderStart() time.Time { return time.Unix(1730000000, 0) }

func newLadder(t *testing.T) (*taskpkg.RatingDecay, time.Time) {
	t.Helper()
	now := ladderStart()
	g := taskpkg.NewRatingDecay(ladderCfg())
	if err := g.RegisterPlayer(taskpkg.PlayerRating{
		PlayerID:  "p-1001",
		SeasonID:  "s-12",
		Rating:    1500,
		DecayFrom: now,
	}); err != nil {
		t.Fatalf("RegisterPlayer: %v", err)
	}
	return g, now
}

// Rating must be readable for a registered player, unknown ids and duplicate
// registrations must be distinguishable.
func TestRegisteredPlayerRatingVisible(t *testing.T) {
	g, now := newLadder(t)
	got, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating(p-1001): %v", err)
	}
	if got != 1500 {
		t.Fatalf("Rating(p-1001) = %d, want 1500", got)
	}
	if _, err := g.Rating("p-ghost"); !errors.Is(err, taskpkg.ErrUnknownPlayer) {
		t.Fatalf("Rating(unknown) = %v, want ErrUnknownPlayer", err)
	}
	if err := g.RegisterPlayer(taskpkg.PlayerRating{PlayerID: "p-1001", SeasonID: "s-12", Rating: 900, DecayFrom: now}); !errors.Is(err, taskpkg.ErrPlayerExists) {
		t.Fatalf("duplicate RegisterPlayer = %v, want ErrPlayerExists", err)
	}
	if err := g.RegisterPlayer(taskpkg.PlayerRating{PlayerID: "", SeasonID: "s-12", Rating: 1500, DecayFrom: now}); !errors.Is(err, taskpkg.ErrInvalidPlayer) {
		t.Fatalf("empty player id = %v, want ErrInvalidPlayer", err)
	}
}

// A positive delta must be applied and shown in the returned rating as well as
// in the player history.
func TestApplyMatchUpdatesRatingAndHistory(t *testing.T) {
	g, now := newLadder(t)
	got, err := g.ApplyMatch("p-1001", taskpkg.MatchResult{MatchID: "m-7f21", RatingDelta: 25, PlayedAt: now}, now)
	if err != nil {
		t.Fatalf("ApplyMatch: %v", err)
	}
	if got != 1525 {
		t.Fatalf("ApplyMatch returned %d, want 1525", got)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating after match: %v", err)
	}
	if rating != 1525 {
		t.Fatalf("Rating after match = %d, want 1525", rating)
	}
	h := g.History("p-1001")
	if len(h) != 1 {
		t.Fatalf("History len = %d, want 1", len(h))
	}
	if h[0].MatchID != "m-7f21" || h[0].RatingDelta != 25 {
		t.Fatalf("History[0] = %+v, want match m-7f21 with delta 25", h[0])
	}
}

// The same match id must never be applied twice, even with another delta.
func TestDuplicateMatchIdRejected(t *testing.T) {
	g, now := newLadder(t)
	first := taskpkg.MatchResult{MatchID: "m-a1", RatingDelta: 25, PlayedAt: now}
	if _, err := g.ApplyMatch("p-1001", first, now); err != nil {
		t.Fatalf("first ApplyMatch: %v", err)
	}
	conflict := taskpkg.MatchResult{MatchID: "m-a1", RatingDelta: -400, PlayedAt: now}
	if _, err := g.ApplyMatch("p-1001", conflict, now); !errors.Is(err, taskpkg.ErrDuplicateMatch) {
		t.Fatalf("duplicate match = %v, want ErrDuplicateMatch", err)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1525 {
		t.Fatalf("rating after rejected duplicate = %d, want 1525", rating)
	}
	if h := g.History("p-1001"); len(h) != 1 {
		t.Fatalf("history grew to %d entries after a duplicate", len(h))
	}
}

// Three idle days cost three decay steps.
func TestDecayAccruesPerFullInterval(t *testing.T) {
	g, now := newLadder(t)
	dropped, err := g.ApplyDecay("p-1001", now.Add(72*time.Hour))
	if err != nil {
		t.Fatalf("ApplyDecay: %v", err)
	}
	if dropped != 36 {
		t.Fatalf("ApplyDecay dropped %d, want 36", dropped)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1464 {
		t.Fatalf("rating after three idle days = %d, want 1464", rating)
	}
}

// Settling the same instant twice must not decay twice.
func TestDecayIsIdempotentForSameInstant(t *testing.T) {
	g, now := newLadder(t)
	first, err := g.ApplyDecay("p-1001", now.Add(72*time.Hour))
	if err != nil {
		t.Fatalf("first ApplyDecay: %v", err)
	}
	if first != 36 {
		t.Fatalf("first ApplyDecay = %d, want 36", first)
	}
	second, err := g.ApplyDecay("p-1001", now.Add(72*time.Hour))
	if err != nil {
		t.Fatalf("second ApplyDecay: %v", err)
	}
	if second != 0 {
		t.Fatalf("second ApplyDecay at the same instant = %d, want 0", second)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1464 {
		t.Fatalf("rating after replaying the same decay = %d, want 1464", rating)
	}
}

// A partial interval must not be rounded up into an extra decay step.
func TestDecayIgnoresPartialInterval(t *testing.T) {
	g, now := newLadder(t)
	dropped, err := g.ApplyDecay("p-1001", now.Add(47*time.Hour))
	if err != nil {
		t.Fatalf("ApplyDecay: %v", err)
	}
	if dropped != 12 {
		t.Fatalf("ApplyDecay at 47h = %d, want 12", dropped)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1488 {
		t.Fatalf("rating after 47 idle hours = %d, want 1488", rating)
	}
}

// Decay must stop exactly at the configured floor.
func TestDecayStopsAtRatingFloor(t *testing.T) {
	now := ladderStart()
	g := taskpkg.NewRatingDecay(ladderCfg())
	if err := g.RegisterPlayer(taskpkg.PlayerRating{PlayerID: "p-low", SeasonID: "s-12", Rating: 105, DecayFrom: now}); err != nil {
		t.Fatalf("RegisterPlayer: %v", err)
	}
	dropped, err := g.ApplyDecay("p-low", now.Add(72*time.Hour))
	if err != nil {
		t.Fatalf("ApplyDecay: %v", err)
	}
	if dropped != 5 {
		t.Fatalf("decay at the floor dropped %d, want 5", dropped)
	}
	rating, err := g.Rating("p-low")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 100 {
		t.Fatalf("rating at the floor = %d, want 100", rating)
	}
	later, err := g.ApplyDecay("p-low", now.Add(96*time.Hour))
	if err != nil {
		t.Fatalf("ApplyDecay one day later: %v", err)
	}
	if later != 0 {
		t.Fatalf("decay below the floor = %d, want 0", later)
	}
}

// PendingDecay is a prediction: it must report the coming loss without
// changing the rating, and the real call must remove exactly that amount.
func TestPendingDecayIsPurePrediction(t *testing.T) {
	g, now := newLadder(t)
	later := now.Add(50 * time.Hour)
	pending := g.PendingDecay("p-1001", later)
	if pending != 24 {
		t.Fatalf("PendingDecay at 50h = %d, want 24", pending)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1500 {
		t.Fatalf("PendingDecay mutated the rating to %d, want 1500", rating)
	}
	dropped, err := g.ApplyDecay("p-1001", later)
	if err != nil {
		t.Fatalf("ApplyDecay: %v", err)
	}
	if dropped != pending {
		t.Fatalf("ApplyDecay dropped %d but PendingDecay promised %d", dropped, pending)
	}
	rating, err = g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1476 {
		t.Fatalf("rating after predicted decay = %d, want 1476", rating)
	}
}

// Rollback must take the last applied match, not an arbitrary one.
func TestRevertIsLastInFirstOut(t *testing.T) {
	g, now := newLadder(t)
	if _, err := g.ApplyMatch("p-1001", taskpkg.MatchResult{MatchID: "m-1", RatingDelta: 25, PlayedAt: now}, now); err != nil {
		t.Fatalf("ApplyMatch m-1: %v", err)
	}
	if _, err := g.ApplyMatch("p-1001", taskpkg.MatchResult{MatchID: "m-2", RatingDelta: 30, PlayedAt: now}, now); err != nil {
		t.Fatalf("ApplyMatch m-2: %v", err)
	}
	if _, err := g.RevertMatch("p-1001", "m-1", now); !errors.Is(err, taskpkg.ErrOutOfOrder) {
		t.Fatalf("revert of an older match = %v, want ErrOutOfOrder", err)
	}
	back, err := g.RevertMatch("p-1001", "m-2", now)
	if err != nil {
		t.Fatalf("revert m-2: %v", err)
	}
	if back != 30 {
		t.Fatalf("revert m-2 returned %d, want 30", back)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1525 {
		t.Fatalf("rating after reverting m-2 = %d, want 1525", rating)
	}
	back, err = g.RevertMatch("p-1001", "m-1", now)
	if err != nil {
		t.Fatalf("revert m-1: %v", err)
	}
	if back != 25 {
		t.Fatalf("revert m-1 returned %d, want 25", back)
	}
	rating, err = g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1500 {
		t.Fatalf("rating after reverting both matches = %d, want 1500", rating)
	}
	if _, err := g.RevertMatch("p-1001", "m-1", now); !errors.Is(err, taskpkg.ErrAlreadyReverted) {
		t.Fatalf("second revert of m-1 = %v, want ErrAlreadyReverted", err)
	}
	if _, err := g.RevertMatch("p-1001", "m-missing", now); !errors.Is(err, taskpkg.ErrNothingToRevert) {
		t.Fatalf("revert of an unknown match = %v, want ErrNothingToRevert", err)
	}
}

// Rolling back a match must not refund decay that was settled in between.
func TestRevertKeepsSettledDecay(t *testing.T) {
	g, now := newLadder(t)
	if _, err := g.ApplyMatch("p-1001", taskpkg.MatchResult{MatchID: "m-boost", RatingDelta: 100, PlayedAt: now}, now); err != nil {
		t.Fatalf("ApplyMatch: %v", err)
	}
	decayed := now.Add(72 * time.Hour)
	if dropped, err := g.ApplyDecay("p-1001", decayed); err != nil || dropped != 36 {
		t.Fatalf("ApplyDecay = %d, %v, want 36, nil", dropped, err)
	}
	back, err := g.RevertMatch("p-1001", "m-boost", decayed)
	if err != nil {
		t.Fatalf("RevertMatch: %v", err)
	}
	if back != 100 {
		t.Fatalf("RevertMatch returned %d, want 100", back)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1464 {
		t.Fatalf("rating after revert = %d, want 1464 (decay must survive)", rating)
	}
}

// A loss clamped by the floor must only give back what actually took effect.
func TestRevertRestoresOnlyAppliedDeltaAtFloor(t *testing.T) {
	now := ladderStart()
	g := taskpkg.NewRatingDecay(ladderCfg())
	if err := g.RegisterPlayer(taskpkg.PlayerRating{PlayerID: "p-low", SeasonID: "s-12", Rating: 105, DecayFrom: now}); err != nil {
		t.Fatalf("RegisterPlayer: %v", err)
	}
	rating, err := g.ApplyMatch("p-low", taskpkg.MatchResult{MatchID: "m-crash", RatingDelta: -400, PlayedAt: now}, now)
	if err != nil {
		t.Fatalf("ApplyMatch: %v", err)
	}
	if rating != 100 {
		t.Fatalf("clamped loss left rating at %d, want 100", rating)
	}
	back, err := g.RevertMatch("p-low", "m-crash", now)
	if err != nil {
		t.Fatalf("RevertMatch: %v", err)
	}
	if back != 5 {
		t.Fatalf("revert gave back %d, want the 5 points that actually applied", back)
	}
	rating, err = g.Rating("p-low")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 105 {
		t.Fatalf("rating after reverting a clamped loss = %d, want 105", rating)
	}
}

// Eight concurrent reports of the same match id must apply it exactly once.
func TestConcurrentDuplicateMatchAppliedOnce(t *testing.T) {
	g, now := newLadder(t)
	m := taskpkg.MatchResult{MatchID: "m-race-9", RatingDelta: 25, PlayedAt: now}
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = g.ApplyMatch("p-1001", m, now)
		}(i)
	}
	wg.Wait()
	ok := 0
	for _, err := range errs {
		switch {
		case err == nil:
			ok++
		case !errors.Is(err, taskpkg.ErrDuplicateMatch):
			t.Fatalf("concurrent ApplyMatch returned an unexpected error: %v", err)
		}
	}
	if ok != 1 {
		t.Fatalf("%d concurrent reports succeeded, want exactly 1", ok)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1525 {
		t.Fatalf("rating after the race = %d, want 1525", rating)
	}
	if h := g.History("p-1001"); len(h) != 1 {
		t.Fatalf("history has %d entries after the race, want 1", len(h))
	}
}

// A closed season must reject every mutation while keeping ratings readable.
func TestClosedSeasonRejectsMutations(t *testing.T) {
	g, now := newLadder(t)
	if err := g.CloseSeason("s-12"); err != nil {
		t.Fatalf("CloseSeason: %v", err)
	}
	if !g.SeasonClosed("s-12") {
		t.Fatal("SeasonClosed(s-12) = false after CloseSeason")
	}
	if _, err := g.ApplyMatch("p-1001", taskpkg.MatchResult{MatchID: "m-late", RatingDelta: 25, PlayedAt: now}, now); !errors.Is(err, taskpkg.ErrSeasonClosed) {
		t.Fatalf("ApplyMatch in a closed season = %v, want ErrSeasonClosed", err)
	}
	if _, err := g.ApplyDecay("p-1001", now.Add(48*time.Hour)); !errors.Is(err, taskpkg.ErrSeasonClosed) {
		t.Fatalf("ApplyDecay in a closed season = %v, want ErrSeasonClosed", err)
	}
	if _, err := g.RevertMatch("p-1001", "m-1", now); !errors.Is(err, taskpkg.ErrSeasonClosed) {
		t.Fatalf("RevertMatch in a closed season = %v, want ErrSeasonClosed", err)
	}
	if err := g.RegisterPlayer(taskpkg.PlayerRating{PlayerID: "p-2002", SeasonID: "s-12", Rating: 1200, DecayFrom: now}); !errors.Is(err, taskpkg.ErrSeasonClosed) {
		t.Fatalf("RegisterPlayer in a closed season = %v, want ErrSeasonClosed", err)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating in a closed season: %v", err)
	}
	if rating != 1500 {
		t.Fatalf("rating in a closed season = %d, want 1500", rating)
	}
}

// After a restart the rating, the decay anchor and the applied match ids must
// still be known.
func TestSnapshotRestoresRatingAndDedupeState(t *testing.T) {
	g, now := newLadder(t)
	if _, err := g.ApplyMatch("p-1001", taskpkg.MatchResult{MatchID: "m-1", RatingDelta: 25, PlayedAt: now}, now); err != nil {
		t.Fatalf("ApplyMatch: %v", err)
	}
	settleAt := now.Add(72 * time.Hour)
	if dropped, err := g.ApplyDecay("p-1001", settleAt); err != nil || dropped != 36 {
		t.Fatalf("ApplyDecay = %d, %v, want 36, nil", dropped, err)
	}
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewRatingDecayFromSnapshot(ladderCfg(), snap)
	if err != nil {
		t.Fatalf("NewRatingDecayFromSnapshot: %v", err)
	}
	rating, err := restored.Rating("p-1001")
	if err != nil {
		t.Fatalf("restored Rating: %v", err)
	}
	if rating != 1489 {
		t.Fatalf("restored rating = %d, want 1489", rating)
	}
	if _, err := restored.ApplyMatch("p-1001", taskpkg.MatchResult{MatchID: "m-1", RatingDelta: 25, PlayedAt: now}, now); !errors.Is(err, taskpkg.ErrDuplicateMatch) {
		t.Fatalf("replayed match after restart = %v, want ErrDuplicateMatch", err)
	}
	if pending := restored.PendingDecay("p-1001", settleAt); pending != 0 {
		t.Fatalf("decay anchor lost across restart: PendingDecay = %d, want 0", pending)
	}
	if again, err := restored.ApplyDecay("p-1001", settleAt); err != nil || again != 0 {
		t.Fatalf("ApplyDecay after restart = %d, %v, want 0, nil", again, err)
	}
}

// The applied-match window is capped, and evicted matches stop being
// revertible.
func TestHistoryCapEvictsOldestMatch(t *testing.T) {
	now := ladderStart()
	cfg := ladderCfg()
	cfg.HistoryCap = 3
	g := taskpkg.NewRatingDecay(cfg)
	if err := g.RegisterPlayer(taskpkg.PlayerRating{PlayerID: "p-1001", SeasonID: "s-12", Rating: 1500, DecayFrom: now}); err != nil {
		t.Fatalf("RegisterPlayer: %v", err)
	}
	for i := 1; i <= 5; i++ {
		m := taskpkg.MatchResult{MatchID: fmt.Sprintf("m-%02d", i), RatingDelta: 10, PlayedAt: now}
		if _, err := g.ApplyMatch("p-1001", m, now); err != nil {
			t.Fatalf("ApplyMatch %s: %v", m.MatchID, err)
		}
	}
	h := g.History("p-1001")
	if len(h) != 3 {
		t.Fatalf("History len = %d, want the cap of 3", len(h))
	}
	if h[0].MatchID != "m-03" || h[2].MatchID != "m-05" {
		t.Fatalf("History = [%s ... %s], want m-03 ... m-05", h[0].MatchID, h[2].MatchID)
	}
	if _, err := g.RevertMatch("p-1001", "m-01", now); !errors.Is(err, taskpkg.ErrNothingToRevert) {
		t.Fatalf("revert of an evicted match = %v, want ErrNothingToRevert", err)
	}
	back, err := g.RevertMatch("p-1001", "m-05", now)
	if err != nil {
		t.Fatalf("revert of the newest match: %v", err)
	}
	if back != 10 {
		t.Fatalf("revert of the newest match returned %d, want 10", back)
	}
	rating, err := g.Rating("p-1001")
	if err != nil {
		t.Fatalf("Rating: %v", err)
	}
	if rating != 1540 {
		t.Fatalf("rating after five wins and one revert = %d, want 1540", rating)
	}
}
