package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-b-02"
)

func newGuard(t *testing.T, size int) *taskpkg.BracketGuard {
	t.Helper()
	g, err := taskpkg.NewBracketGuard(size)
	if err != nil {
		t.Fatalf("NewBracketGuard(%d): %v", size, err)
	}
	return g
}

func seeds(n int) []taskpkg.Entrant {
	out := make([]taskpkg.Entrant, 0, n)
	for i := 1; i <= n; i++ {
		out = append(out, taskpkg.Entrant{ID: fmt.Sprintf("s%d", i), Seed: i, Region: "cn-east"})
	}
	return out
}

func seeded(t *testing.T, size, n int) *taskpkg.BracketGuard {
	t.Helper()
	g := newGuard(t, size)
	if err := g.Seed(seeds(n)); err != nil {
		t.Fatalf("Seed(%d entrants into %d slots): %v", n, size, err)
	}
	return g
}

func getMatch(t *testing.T, g *taskpkg.BracketGuard, id string) taskpkg.Match {
	t.Helper()
	m, err := g.Match(id)
	if err != nil {
		t.Fatalf("Match(%q): %v", id, err)
	}
	return m
}

// Only power-of-two sizes from 4 to 256 are accepted.
func TestNewBracketGuardRejectsInvalidSize(t *testing.T) {
	for _, size := range []int{0, 2, 6, 512} {
		g, err := taskpkg.NewBracketGuard(size)
		if g != nil {
			t.Fatalf("size %d returned a bracket", size)
		}
		if !errors.Is(err, taskpkg.ErrBracketConfig) {
			t.Fatalf("size %d error = %v, want ErrBracketConfig", size, err)
		}
	}
	if _, err := taskpkg.NewBracketGuard(8); err != nil {
		t.Fatalf("size 8 rejected: %v", err)
	}
}

// Two entrants cannot share an id.
func TestSeedRejectsDuplicateEntrantID(t *testing.T) {
	g := newGuard(t, 4)
	err := g.Seed([]taskpkg.Entrant{
		{ID: "a", Seed: 1},
		{ID: "a", Seed: 2},
		{ID: "c", Seed: 3},
		{ID: "d", Seed: 4},
	})
	if !errors.Is(err, taskpkg.ErrDuplicateEntrant) {
		t.Fatalf("duplicate entrant id must yield ErrDuplicateEntrant, got %v", err)
	}
}

// Seeds must be unique and fall inside 1..len(entrants).
func TestSeedRejectsDuplicateAndOutOfRangeSeeds(t *testing.T) {
	g := newGuard(t, 4)
	dup := []taskpkg.Entrant{
		{ID: "a", Seed: 1},
		{ID: "b", Seed: 1},
		{ID: "c", Seed: 3},
		{ID: "d", Seed: 4},
	}
	if err := g.Seed(dup); !errors.Is(err, taskpkg.ErrSeedConflict) {
		t.Fatalf("duplicate seeds must yield ErrSeedConflict, got %v", err)
	}
	outOfRange := []taskpkg.Entrant{
		{ID: "a", Seed: 1},
		{ID: "b", Seed: 2},
		{ID: "c", Seed: 3},
		{ID: "d", Seed: 9},
	}
	if err := g.Seed(outOfRange); !errors.Is(err, taskpkg.ErrSeedConflict) {
		t.Fatalf("out of range seed must yield ErrSeedConflict, got %v", err)
	}
}

// A bracket must be more than half full and never over-subscribed.
func TestSeedRejectsWrongEntrantCount(t *testing.T) {
	g := newGuard(t, 8)
	if err := g.Seed(seeds(4)); !errors.Is(err, taskpkg.ErrBracketConfig) {
		t.Fatalf("4 entrants in 8 slots must yield ErrBracketConfig, got %v", err)
	}
	if err := g.Seed(seeds(9)); !errors.Is(err, taskpkg.ErrBracketConfig) {
		t.Fatalf("9 entrants in 8 slots must yield ErrBracketConfig, got %v", err)
	}
}

// The classic seeding order keeps seed 1 and seed 2 apart until the final.
func TestStandardSeedLayoutN8(t *testing.T) {
	g := seeded(t, 8, 8)
	cases := []struct{ id, home, away string }{
		{"R1-M1", "s1", "s8"},
		{"R1-M2", "s4", "s5"},
		{"R1-M3", "s2", "s7"},
		{"R1-M4", "s3", "s6"},
	}
	for _, c := range cases {
		m := getMatch(t, g, c.id)
		if m.HomeID != c.home || m.AwayID != c.away {
			t.Fatalf("%s = %s vs %s, want %s vs %s", c.id, m.HomeID, m.AwayID, c.home, c.away)
		}
		if m.Status != taskpkg.StatusReady {
			t.Fatalf("%s status = %q, want %q", c.id, m.Status, taskpkg.StatusReady)
		}
	}
	if got, err := g.RoundMatches(1); err != nil || len(got) != 4 {
		t.Fatalf("RoundMatches(1) = %d matches, err %v, want 4", len(got), err)
	}
	if got, err := g.RoundMatches(2); err != nil || len(got) != 2 {
		t.Fatalf("RoundMatches(2) = %d matches, err %v, want 2", len(got), err)
	}
	if got, err := g.RoundMatches(3); err != nil || len(got) != 1 {
		t.Fatalf("RoundMatches(3) = %d matches, err %v, want 1", len(got), err)
	}
	if _, err := g.RoundMatches(4); !errors.Is(err, taskpkg.ErrMatchNotFound) {
		t.Fatalf("RoundMatches(4) must yield ErrMatchNotFound, got %v", err)
	}
}

// Empty seed slots are byes and advance their opponent immediately.
func TestByeAutoAdvancesWinner(t *testing.T) {
	g := seeded(t, 8, 6)
	for _, c := range []struct{ id, home, winner string }{
		{"R1-M1", "s1", "s1"},
		{"R1-M3", "s2", "s2"},
	} {
		m := getMatch(t, g, c.id)
		if m.Status != taskpkg.StatusBye {
			t.Fatalf("%s status = %q, want %q", c.id, m.Status, taskpkg.StatusBye)
		}
		if m.HomeID != c.home || m.AwayID != "" || m.WinnerID != c.winner {
			t.Fatalf("%s = home %q away %q winner %q", c.id, m.HomeID, m.AwayID, m.WinnerID)
		}
	}
	if m := getMatch(t, g, "R1-M2"); m.Status != taskpkg.StatusReady || m.HomeID != "s4" || m.AwayID != "s5" {
		t.Fatalf("R1-M2 = %+v, want ready s4 vs s5", m)
	}
	next := getMatch(t, g, "R2-M1")
	if next.HomeID != "s1" || next.AwayID != "" || next.Status != taskpkg.StatusPending {
		t.Fatalf("R2-M1 = %+v, want pending with home s1", next)
	}
	next2 := getMatch(t, g, "R2-M2")
	if next2.HomeID != "s2" || next2.Status != taskpkg.StatusPending {
		t.Fatalf("R2-M2 = %+v, want pending with home s2", next2)
	}
}

// The reported winner must be one of the two participants.
func TestReportResultRejectsNonParticipant(t *testing.T) {
	g := seeded(t, 4, 4)
	if err := g.ReportResult("R1-M1", "ghost"); !errors.Is(err, taskpkg.ErrNotParticipant) {
		t.Fatalf("foreign winner must yield ErrNotParticipant, got %v", err)
	}
	if m := getMatch(t, g, "R1-M1"); m.Status != taskpkg.StatusReady {
		t.Fatalf("match status after rejected report = %q, want ready", m.Status)
	}
}

// A bye is already decided and cannot receive a reported result.
func TestReportResultOnByeRejected(t *testing.T) {
	g := seeded(t, 8, 6)
	if err := g.ReportResult("R1-M1", "s1"); !errors.Is(err, taskpkg.ErrMatchClosed) {
		t.Fatalf("reporting a bye must yield ErrMatchClosed, got %v", err)
	}
}

// A match whose second participant is unknown cannot be reported yet.
func TestReportResultOnPendingMatchRejected(t *testing.T) {
	g := seeded(t, 4, 4)
	if err := g.ReportResult("R2-M1", "s1"); !errors.Is(err, taskpkg.ErrMatchNotReady) {
		t.Fatalf("reporting a pending match must yield ErrMatchNotReady, got %v", err)
	}
}

// Replaying the same winner is a no-op and must not bump the revision.
func TestReportResultIdempotentSameWinner(t *testing.T) {
	g := seeded(t, 4, 4)
	rev := g.Revision()
	if rev != 1 {
		t.Fatalf("revision after Seed = %d, want 1", rev)
	}
	if err := g.ReportResult("R1-M1", "s1"); err != nil {
		t.Fatalf("first report: %v", err)
	}
	if got := g.Revision(); got != rev+1 {
		t.Fatalf("revision after first report = %d, want %d", got, rev+1)
	}
	if err := g.ReportResult("R1-M1", "s1"); err != nil {
		t.Fatalf("replayed report: %v", err)
	}
	if got := g.Revision(); got != rev+1 {
		t.Fatalf("revision after replayed report = %d, want %d", got, rev+1)
	}
	m := getMatch(t, g, "R1-M1")
	if m.Status != taskpkg.StatusDone || m.WinnerID != "s1" {
		t.Fatalf("match after replay = %+v", m)
	}
}

// A different winner for a decided match is a conflict.
func TestReportResultConflictingWinnerRejected(t *testing.T) {
	g := seeded(t, 4, 4)
	if err := g.ReportResult("R1-M1", "s1"); err != nil {
		t.Fatalf("first report: %v", err)
	}
	rev := g.Revision()
	if err := g.ReportResult("R1-M1", "s4"); !errors.Is(err, taskpkg.ErrConflictResult) {
		t.Fatalf("conflicting winner must yield ErrConflictResult, got %v", err)
	}
	if got := g.Revision(); got != rev {
		t.Fatalf("revision after conflict = %d, want %d", got, rev)
	}
	if m := getMatch(t, g, "R1-M1"); m.WinnerID != "s1" {
		t.Fatalf("winner after conflict = %q, want s1", m.WinnerID)
	}
}

// Winners propagate into the next round, which becomes ready only when both
// sides are known.
func TestWinnerAdvancesAndNextMatchBecomesReady(t *testing.T) {
	g := seeded(t, 4, 4)
	if err := g.ReportResult("R1-M1", "s1"); err != nil {
		t.Fatalf("report R1-M1: %v", err)
	}
	next := getMatch(t, g, "R2-M1")
	if next.HomeID != "s1" || next.AwayID != "" || next.Status != taskpkg.StatusPending {
		t.Fatalf("R2-M1 after one result = %+v, want pending s1 vs empty", next)
	}
	if err := g.ReportResult("R1-M2", "s3"); err != nil {
		t.Fatalf("report R1-M2: %v", err)
	}
	next = getMatch(t, g, "R2-M1")
	if next.HomeID != "s1" || next.AwayID != "s3" || next.Status != taskpkg.StatusReady {
		t.Fatalf("R2-M1 after both results = %+v, want ready s1 vs s3", next)
	}
}

// The champion exists only after the final has a winner.
func TestChampionGatedByFinalResult(t *testing.T) {
	g := seeded(t, 4, 4)
	if _, err := g.Champion(); !errors.Is(err, taskpkg.ErrTournamentRunning) {
		t.Fatalf("champion before any result must yield ErrTournamentRunning, got %v", err)
	}
	if err := g.ReportResult("R1-M1", "s1"); err != nil {
		t.Fatalf("report R1-M1: %v", err)
	}
	if _, err := g.Champion(); !errors.Is(err, taskpkg.ErrTournamentRunning) {
		t.Fatalf("champion with the final undecided must yield ErrTournamentRunning, got %v", err)
	}
	if err := g.ReportResult("R1-M2", "s3"); err != nil {
		t.Fatalf("report R1-M2: %v", err)
	}
	if _, err := g.Champion(); !errors.Is(err, taskpkg.ErrTournamentRunning) {
		t.Fatalf("champion with a ready final must yield ErrTournamentRunning, got %v", err)
	}
	if err := g.ReportResult("R2-M1", "s3"); err != nil {
		t.Fatalf("report final: %v", err)
	}
	champ, err := g.Champion()
	if err != nil {
		t.Fatalf("Champion after final: %v", err)
	}
	if champ != "s3" {
		t.Fatalf("champion = %q, want s3", champ)
	}
}

// A restart keeps results, the revision and the idempotency of old reports.
func TestSnapshotRestorePreservesProgress(t *testing.T) {
	g := seeded(t, 4, 4)
	if err := g.ReportResult("R1-M1", "s1"); err != nil {
		t.Fatalf("report R1-M1: %v", err)
	}
	rev := g.Revision()
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap) == 0 {
		t.Fatal("Snapshot returned no bytes")
	}
	restored, err := taskpkg.NewBracketGuardFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewBracketGuardFromSnapshot: %v", err)
	}
	if got := restored.Revision(); got != rev {
		t.Fatalf("restored revision = %d, want %d", got, rev)
	}
	m := getMatch(t, restored, "R1-M1")
	if m.Status != taskpkg.StatusDone || m.WinnerID != "s1" {
		t.Fatalf("restored R1-M1 = %+v", m)
	}
	if err := restored.ReportResult("R1-M1", "s1"); err != nil {
		t.Fatalf("replayed report after restore: %v", err)
	}
	if got := restored.Revision(); got != rev {
		t.Fatalf("revision after replayed report = %d, want %d", got, rev)
	}
	if err := restored.ReportResult("R1-M2", "s3"); err != nil {
		t.Fatalf("report R1-M2 after restore: %v", err)
	}
	if err := restored.ReportResult("R2-M1", "s3"); err != nil {
		t.Fatalf("report final after restore: %v", err)
	}
	if champ, err := restored.Champion(); err != nil || champ != "s3" {
		t.Fatalf("champion after restore = %q, err %v, want s3", champ, err)
	}
	if _, err := taskpkg.NewBracketGuardFromSnapshot([]byte("{broken")); !errors.Is(err, taskpkg.ErrBracketConfig) {
		t.Fatalf("corrupt snapshot must yield ErrBracketConfig, got %v", err)
	}
}

// Concurrent replays of one result must be accepted but count exactly once.
func TestConcurrentDuplicateReportsBumpRevisionOnce(t *testing.T) {
	g := seeded(t, 4, 4)
	const workers = 16
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = g.ReportResult("R1-M1", "s1")
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent report %d: %v", i, err)
		}
	}
	if got := g.Revision(); got != 2 {
		t.Fatalf("revision after concurrent replays = %d, want 2", got)
	}
	m := getMatch(t, g, "R1-M1")
	if m.WinnerID != "s1" || m.Status != taskpkg.StatusDone {
		t.Fatalf("match after concurrent replays = %+v", m)
	}
}
