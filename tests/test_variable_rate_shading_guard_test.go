package task

import (
	"errors"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-f-02"
)

func testVrsConfig() taskpkg.VrsConfig {
	return taskpkg.VrsConfig{TileSize: 32, MaxRateChangesPerFrame: 10}
}

func newVrsGuard(t *testing.T) *taskpkg.VrsGuard {
	t.Helper()
	g, err := taskpkg.NewVrsGuard(testVrsConfig())
	if err != nil {
		t.Fatalf("NewVrsGuard: %v", err)
	}
	return g
}

func mustSetVrsSurface(t *testing.T, g *taskpkg.VrsGuard, name string, w, h int) {
	t.Helper()
	if err := g.SetSurface(name, w, h); err != nil {
		t.Fatalf("SetSurface(%q): %v", name, err)
	}
}

// Only 1x1, 2x2 and 4x4 shading rates are legal.
func TestInvalidShadingRatesRejected(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	for _, rate := range []int{0, 3, 8, -2} {
		if _, err := g.SetRectRate("gbuffer", 0, 0, 32, 32, rate); !errors.Is(err, taskpkg.ErrInvalidRate) {
			t.Fatalf("rate %d must yield ErrInvalidRate, got %v", rate, err)
		}
	}
	pending, err := g.PendingChanges("gbuffer")
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if pending != 0 {
		t.Fatalf("rejected rates must not change tiles, got %d", pending)
	}
}

// Overlapping requests pick the coarser rate, never the last writer.
func TestCombineRatesPicksCoarsest(t *testing.T) {
	if got := taskpkg.CombineRates(1, 2); got != 2 {
		t.Fatalf("CombineRates(1, 2) = %d, want 2", got)
	}
	if got := taskpkg.CombineRates(2, 4); got != 4 {
		t.Fatalf("CombineRates(2, 4) = %d, want 4", got)
	}
	if got := taskpkg.CombineRates(4, 1); got != 4 {
		t.Fatalf("CombineRates(4, 1) = %d, want 4", got)
	}
	if got := taskpkg.CombineRates(1, 1); got != 1 {
		t.Fatalf("CombineRates(1, 1) = %d, want 1", got)
	}
}

// The rate image grid rounds up and starts fully at 1x1.
func TestSurfaceGridRoundsUp(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	info, err := g.SurfaceInfo("gbuffer")
	if err != nil {
		t.Fatalf("SurfaceInfo: %v", err)
	}
	if info.Cols != 4 || info.Rows != 3 {
		t.Fatalf("128x96 with 32px tiles must be 4x3, got %dx%d", info.Cols, info.Rows)
	}
	if rate, err := g.RateAt("gbuffer", 0, 0); err != nil || rate != 1 {
		t.Fatalf("new surface must start at 1x1, got rate=%d err=%v", rate, err)
	}
	if err := g.SetSurface("gbuffer", 64, 64); !errors.Is(err, taskpkg.ErrSurfaceExists) {
		t.Fatalf("duplicate surface must yield ErrSurfaceExists, got %v", err)
	}
	if _, err := g.SurfaceInfo("missing"); !errors.Is(err, taskpkg.ErrSurfaceUnknown) {
		t.Fatalf("unknown surface must yield ErrSurfaceUnknown, got %v", err)
	}
}

// Pixel coordinates map into the correct tile and out of range is rejected.
func TestPixelToTileMapping(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	if _, err := g.SetRectRate("gbuffer", 64, 0, 32, 32, 4); err != nil {
		t.Fatalf("SetRectRate: %v", err)
	}
	if _, err := g.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	checks := []struct {
		x, y int
		want int
	}{
		{63, 31, 1},
		{64, 0, 4},
		{95, 31, 4},
		{96, 0, 1},
	}
	for _, c := range checks {
		got, err := g.RateAt("gbuffer", c.x, c.y)
		if err != nil {
			t.Fatalf("RateAt(%d,%d): %v", c.x, c.y, err)
		}
		if got != c.want {
			t.Fatalf("RateAt(%d,%d) = %d, want %d", c.x, c.y, got, c.want)
		}
	}
	if _, err := g.RateAt("gbuffer", 128, 0); !errors.Is(err, taskpkg.ErrInvalidSurface) {
		t.Fatalf("pixel outside the surface must yield ErrInvalidSurface, got %v", err)
	}
}

// Setting the same rectangle twice does not inflate the change count.
func TestSetRectRateCountsChangedTiles(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	changed, err := g.SetRectRate("gbuffer", 0, 0, 64, 64, 4)
	if err != nil {
		t.Fatalf("SetRectRate: %v", err)
	}
	if changed != 4 {
		t.Fatalf("64x64 rectangle covers 4 tiles, got %d", changed)
	}
	again, err := g.SetRectRate("gbuffer", 0, 0, 64, 64, 4)
	if err != nil {
		t.Fatalf("SetRectRate repeat: %v", err)
	}
	if again != 0 {
		t.Fatalf("repeating the same request must change zero tiles, got %d", again)
	}
	pending, err := g.PendingChanges("gbuffer")
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if pending != 4 {
		t.Fatalf("want 4 pending tiles, got %d", pending)
	}
}

// Conflicting overlapping rectangles in one batch resolve to the coarsest.
func TestOverlappingRectsUseCoarsest(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	set, err := g.SetRates("gbuffer", []taskpkg.RateRect{
		{X: 0, Y: 0, W: 64, H: 64, Rate: 1},
		{X: 0, Y: 0, W: 64, H: 64, Rate: 4},
	})
	if err != nil {
		t.Fatalf("SetRates: %v", err)
	}
	if set.ChangedTiles != 4 {
		t.Fatalf("want 4 changed tiles, got %d", set.ChangedTiles)
	}
	if _, err := g.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	if rate, _ := g.RateAt("gbuffer", 0, 0); rate != 4 {
		t.Fatalf("coarsest rate must win regardless of order, got %d", rate)
	}
	if _, err := g.SetRates("gbuffer", []taskpkg.RateRect{
		{X: 64, Y: 0, W: 32, H: 32, Rate: 4},
		{X: 64, Y: 0, W: 32, H: 32, Rate: 1},
	}); err != nil {
		t.Fatalf("SetRates reverse order: %v", err)
	}
	if _, err := g.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	if rate, _ := g.RateAt("gbuffer", 64, 0); rate != 4 {
		t.Fatalf("later finer request must not shrink an existing coarse tile, got %d", rate)
	}
}

// A batch that would blow the frame budget changes nothing.
func TestBatchBudgetExceededRollsBack(t *testing.T) {
	g, err := taskpkg.NewVrsGuard(taskpkg.VrsConfig{TileSize: 32, MaxRateChangesPerFrame: 4})
	if err != nil {
		t.Fatalf("NewVrsGuard: %v", err)
	}
	mustSetVrsSurface(t, g, "gbuffer", 192, 96)
	if _, err := g.SetRates("gbuffer", []taskpkg.RateRect{
		{X: 0, Y: 0, W: 192, H: 64, Rate: 4},
	}); !errors.Is(err, taskpkg.ErrRateBudget) {
		t.Fatalf("12 tiles over a budget of 4 must yield ErrRateBudget, got %v", err)
	}
	pending, err := g.PendingChanges("gbuffer")
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if pending != 0 {
		t.Fatalf("rejected batch must leave no pending tiles, got %d", pending)
	}
	if rate, _ := g.PendingRateAt("gbuffer", 0, 0); rate != 1 {
		t.Fatalf("rejected batch leaked a pending rate: %d", rate)
	}
	if _, err := g.SetRectRate("gbuffer", 0, 0, 64, 64, 4); err != nil {
		t.Fatalf("SetRectRate within budget: %v", err)
	}
	if _, err := g.SetRectRate("gbuffer", 64, 0, 32, 32, 4); !errors.Is(err, taskpkg.ErrRateBudget) {
		t.Fatalf("fifth tile over a budget of 4 must yield ErrRateBudget, got %v", err)
	}
	pending, err = g.PendingChanges("gbuffer")
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if pending != 4 {
		t.Fatalf("failed increment must keep the 4 accepted tiles, got %d", pending)
	}
	if rate, _ := g.PendingRateAt("gbuffer", 64, 0); rate != 1 {
		t.Fatalf("failed increment must roll back its own tile, got %d", rate)
	}
}

// Overlapping rectangles in a batch are counted once.
func TestBatchWithinBudgetCountsUniqueTiles(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 192, 96)
	set, err := g.SetRates("gbuffer", []taskpkg.RateRect{
		{X: 0, Y: 0, W: 64, H: 64, Rate: 4},
		{X: 32, Y: 32, W: 64, H: 64, Rate: 4},
	})
	if err != nil {
		t.Fatalf("SetRates: %v", err)
	}
	if set.ChangedTiles != 7 {
		t.Fatalf("overlapping rectangles union to 7 tiles, got %d", set.ChangedTiles)
	}
	if set.Surface != "gbuffer" {
		t.Fatalf("change set names %q, want gbuffer", set.Surface)
	}
	pending, err := g.PendingChanges("gbuffer")
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if pending != 7 {
		t.Fatalf("want 7 pending tiles, got %d", pending)
	}
}

// Commit promotes pending rates and reports the affected surface.
func TestCommitAppliesPendingRates(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	if _, err := g.SetRectRate("gbuffer", 0, 0, 64, 64, 4); err != nil {
		t.Fatalf("SetRectRate: %v", err)
	}
	res, err := g.CommitFrame()
	if err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	if res.ChangedTiles != 4 || len(res.Surfaces) != 1 || res.Surfaces[0] != "gbuffer" {
		t.Fatalf("unexpected commit result: %+v", res)
	}
	if rate, _ := g.RateAt("gbuffer", 0, 0); rate != 4 {
		t.Fatalf("commit did not apply the rate: %d", rate)
	}
	pending, err := g.PendingChanges("gbuffer")
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if pending != 0 {
		t.Fatalf("commit must clear pending changes, got %d", pending)
	}
	if stats := g.Stats(); stats.CommitTiles != 4 {
		t.Fatalf("stats must count committed tiles, got %+v", stats)
	}
}

// A commit with nothing pending is a no-op.
func TestSecondCommitIsIdempotent(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	if _, err := g.SetRectRate("gbuffer", 0, 0, 32, 32, 2); err != nil {
		t.Fatalf("SetRectRate: %v", err)
	}
	if _, err := g.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	res, err := g.CommitFrame()
	if err != nil {
		t.Fatalf("second CommitFrame: %v", err)
	}
	if res.ChangedTiles != 0 || len(res.Surfaces) != 0 {
		t.Fatalf("empty commit must report zero changes, got %+v", res)
	}
	if stats := g.Stats(); stats.CommitTiles != 1 {
		t.Fatalf("empty commit must not inflate stats, got %+v", stats)
	}
}

// Commit flushes surfaces in a stable, sorted order.
func TestCommitSurfaceNamesSorted(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "zeta", 64, 64)
	mustSetVrsSurface(t, g, "alpha", 64, 64)
	if _, err := g.SetRectRate("zeta", 0, 0, 32, 32, 4); err != nil {
		t.Fatalf("SetRectRate zeta: %v", err)
	}
	if _, err := g.SetRectRate("alpha", 0, 0, 32, 32, 2); err != nil {
		t.Fatalf("SetRectRate alpha: %v", err)
	}
	res, err := g.CommitFrame()
	if err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	if res.ChangedTiles != 2 {
		t.Fatalf("want 2 changed tiles, got %d", res.ChangedTiles)
	}
	if len(res.Surfaces) != 2 || res.Surfaces[0] != "alpha" || res.Surfaces[1] != "zeta" {
		t.Fatalf("commit surfaces must be sorted, got %v", res.Surfaces)
	}
}

// Clearing a surface restores 1x1 shading.
func TestClearRatesResetsSurface(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	if _, err := g.SetRectRate("gbuffer", 0, 0, 64, 64, 4); err != nil {
		t.Fatalf("SetRectRate: %v", err)
	}
	if _, err := g.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	cleared, err := g.ClearRates("gbuffer")
	if err != nil {
		t.Fatalf("ClearRates: %v", err)
	}
	if cleared != 4 {
		t.Fatalf("clearing 4 coarse tiles must report 4, got %d", cleared)
	}
	if _, err := g.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	if rate, _ := g.RateAt("gbuffer", 0, 0); rate != 1 {
		t.Fatalf("clear must restore 1x1, got %d", rate)
	}
	cleared, err = g.ClearRates("gbuffer")
	if err != nil {
		t.Fatalf("ClearRates again: %v", err)
	}
	if cleared != 0 {
		t.Fatalf("clearing an already 1x1 surface must report 0, got %d", cleared)
	}
}

// Devices without VRS force every surface to 1x1 and refuse later requests.
func TestDegradeTo1x1(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 64, 64)
	mustSetVrsSurface(t, g, "shadow", 64, 64)
	if _, err := g.SetRectRate("gbuffer", 0, 0, 64, 32, 4); err != nil {
		t.Fatalf("SetRectRate: %v", err)
	}
	if _, err := g.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	res, err := g.DegradeTo1x1()
	if err != nil {
		t.Fatalf("DegradeTo1x1: %v", err)
	}
	if res.Surfaces != 2 || res.TilesReset != 2 {
		t.Fatalf("want 2 surfaces and 2 reset tiles, got %+v", res)
	}
	if rate, _ := g.RateAt("gbuffer", 0, 0); rate != 1 {
		t.Fatalf("degrade must force 1x1, got %d", rate)
	}
	if _, err := g.SetRectRate("gbuffer", 0, 0, 32, 32, 4); !errors.Is(err, taskpkg.ErrVrsUnsupported) {
		t.Fatalf("rate requests after degrade must yield ErrVrsUnsupported, got %v", err)
	}
	commit, err := g.CommitFrame()
	if err != nil {
		t.Fatalf("CommitFrame after degrade: %v", err)
	}
	if commit.ChangedTiles != 0 {
		t.Fatalf("no work may be committed after degrade, got %d", commit.ChangedTiles)
	}
}

// The frame budget must hold under concurrent requests.
func TestConcurrentRateBudget(t *testing.T) {
	g, err := taskpkg.NewVrsGuard(taskpkg.VrsConfig{TileSize: 32, MaxRateChangesPerFrame: 4})
	if err != nil {
		t.Fatalf("NewVrsGuard: %v", err)
	}
	mustSetVrsSurface(t, g, "gbuffer", 256, 256)
	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	changes := make([]int, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			changes[idx], errs[idx] = g.SetRectRate("gbuffer", idx*32, 0, 32, 32, 4)
		}(i)
	}
	wg.Wait()
	accepted := 0
	rejected := 0
	for i := 0; i < workers; i++ {
		switch {
		case errs[i] == nil:
			accepted++
			if changes[i] != 1 {
				t.Fatalf("accepted request %d must change one tile, got %d", i, changes[i])
			}
		case errors.Is(errs[i], taskpkg.ErrRateBudget):
			rejected++
		default:
			t.Fatalf("unexpected error from worker %d: %v", i, errs[i])
		}
	}
	if accepted != 4 || rejected != 4 {
		t.Fatalf("want exactly 4 accepted and 4 rejected requests, got %d/%d", accepted, rejected)
	}
	pending, err := g.PendingChanges("gbuffer")
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if pending != 4 {
		t.Fatalf("budget must cap pending tiles at 4, got %d", pending)
	}
}

// Committed and pending rates must survive a restart.
func TestSnapshotRestorePreservesRates(t *testing.T) {
	g := newVrsGuard(t)
	mustSetVrsSurface(t, g, "gbuffer", 128, 96)
	if _, err := g.SetRectRate("gbuffer", 0, 0, 32, 32, 4); err != nil {
		t.Fatalf("SetRectRate: %v", err)
	}
	if _, err := g.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame: %v", err)
	}
	if _, err := g.SetRectRate("gbuffer", 32, 0, 32, 32, 2); err != nil {
		t.Fatalf("SetRectRate pending: %v", err)
	}
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewVrsGuardFromSnapshot(testVrsConfig(), snap)
	if err != nil {
		t.Fatalf("NewVrsGuardFromSnapshot: %v", err)
	}
	if rate, err := restored.RateAt("gbuffer", 0, 0); err != nil || rate != 4 {
		t.Fatalf("restored committed rate = %d, err=%v, want 4", rate, err)
	}
	if rate, err := restored.PendingRateAt("gbuffer", 32, 0); err != nil || rate != 2 {
		t.Fatalf("restored pending rate = %d, err=%v, want 2", rate, err)
	}
	pending, err := restored.PendingChanges("gbuffer")
	if err != nil {
		t.Fatalf("PendingChanges: %v", err)
	}
	if pending != 1 {
		t.Fatalf("restored pending changes = %d, want 1", pending)
	}
	res, err := restored.CommitFrame()
	if err != nil {
		t.Fatalf("CommitFrame after restore: %v", err)
	}
	if res.ChangedTiles != 1 {
		t.Fatalf("commit after restore must flush one tile, got %d", res.ChangedTiles)
	}
	if _, err := taskpkg.NewVrsGuardFromSnapshot(testVrsConfig(), []byte("garbage")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot must yield ErrCorruptSnapshot, got %v", err)
	}
}
