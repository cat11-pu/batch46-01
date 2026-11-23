package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-e-01"
)

func near(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func mustPool(t *testing.T, size int, tick float64) *taskpkg.GpuTimestamp {
	t.Helper()
	g, err := taskpkg.NewGpuTimestamp(size, tick)
	if err != nil {
		t.Fatalf("NewGpuTimestamp(%d, %v): %v", size, tick, err)
	}
	return g
}

// The pool rejects a zero slot count and a non-positive tick period; every
// other constructor argument is accepted as-is.
func TestNewRejectsBadConfiguration(t *testing.T) {
	if _, err := taskpkg.NewGpuTimestamp(0, 1); !errors.Is(err, taskpkg.ErrBadPoolSize) {
		t.Fatalf("NewGpuTimestamp(0, 1) error = %v, want ErrBadPoolSize", err)
	}
	if _, err := taskpkg.NewGpuTimestamp(4, 0); !errors.Is(err, taskpkg.ErrBadTickPeriod) {
		t.Fatalf("NewGpuTimestamp(4, 0) error = %v, want ErrBadTickPeriod", err)
	}
	if g := mustPool(t, 4, 1); g.ActiveCount() != 0 {
		t.Fatalf("fresh pool ActiveCount = %d, want 0", g.ActiveCount())
	}
}

// Allocation always takes the smallest free index and a released slot is
// handed out again before any higher unused index.
func TestAllocQueryLowestFreeIndex(t *testing.T) {
	g := mustPool(t, 4, 1)
	for want := 0; want < 3; want++ {
		idx, err := g.AllocQuery("gbuffer")
		if err != nil {
			t.Fatalf("AllocQuery #%d: %v", want, err)
		}
		if idx != want {
			t.Fatalf("AllocQuery #%d returned index %d, want %d", want, idx, want)
		}
	}
	if err := g.Release(1); err != nil {
		t.Fatalf("Release(1) on an idle slot: %v", err)
	}
	idx, err := g.AllocQuery("shadow")
	if err != nil {
		t.Fatalf("AllocQuery after release: %v", err)
	}
	if idx != 1 {
		t.Fatalf("AllocQuery after release returned %d, want 1", idx)
	}
	if g.ActiveCount() != 3 {
		t.Fatalf("ActiveCount = %d, want 3", g.ActiveCount())
	}
}

// A pool with two slots refuses the third allocation with a dedicated error.
func TestAllocQueryPoolExhausted(t *testing.T) {
	g := mustPool(t, 2, 1)
	if _, err := g.AllocQuery("a"); err != nil {
		t.Fatalf("AllocQuery a: %v", err)
	}
	if _, err := g.AllocQuery("b"); err != nil {
		t.Fatalf("AllocQuery b: %v", err)
	}
	if _, err := g.AllocQuery("c"); !errors.Is(err, taskpkg.ErrPoolExhausted) {
		t.Fatalf("third AllocQuery error = %v, want ErrPoolExhausted", err)
	}
	if g.ActiveCount() != 2 {
		t.Fatalf("ActiveCount = %d, want 2", g.ActiveCount())
	}
}

// A begin request for an index that was never allocated is rejected.
func TestBeginUnknownQueryRejected(t *testing.T) {
	g := mustPool(t, 8, 1)
	if _, err := g.AllocQuery("gbuffer"); err != nil {
		t.Fatalf("AllocQuery: %v", err)
	}
	if err := g.BeginQuery(5, 100, 1000); !errors.Is(err, taskpkg.ErrUnknownQuery) {
		t.Fatalf("BeginQuery(5) error = %v, want ErrUnknownQuery", err)
	}
}

// The same slot cannot be begun twice before it is ended.
func TestDoubleBeginRejected(t *testing.T) {
	g := mustPool(t, 4, 1)
	idx, err := g.AllocQuery("gbuffer")
	if err != nil {
		t.Fatalf("AllocQuery: %v", err)
	}
	if err := g.BeginQuery(idx, 100, 1000); err != nil {
		t.Fatalf("first BeginQuery: %v", err)
	}
	if err := g.BeginQuery(idx, 100, 1200); !errors.Is(err, taskpkg.ErrQueryInFlight) {
		t.Fatalf("second BeginQuery error = %v, want ErrQueryInFlight", err)
	}
}

// Ending a slot that never began is an ordering bug, and the slot still has
// no duration afterwards.
func TestEndWithoutBeginRejected(t *testing.T) {
	g := mustPool(t, 4, 1)
	idx, err := g.AllocQuery("lighting")
	if err != nil {
		t.Fatalf("AllocQuery: %v", err)
	}
	if err := g.EndQuery(idx, 100, 500); !errors.Is(err, taskpkg.ErrNoBegin) {
		t.Fatalf("EndQuery without begin error = %v, want ErrNoBegin", err)
	}
	if _, err := g.DurationMs(idx); !errors.Is(err, taskpkg.ErrNotReady) {
		t.Fatalf("DurationMs error = %v, want ErrNotReady", err)
	}
	if g.Ready(idx) {
		t.Fatalf("Ready reported true for a slot that never began")
	}
}

// Begin and end stamps of one slot must belong to the same frame.
func TestEndFrameMismatchRejected(t *testing.T) {
	g := mustPool(t, 4, 1)
	idx, err := g.AllocQuery("gbuffer")
	if err != nil {
		t.Fatalf("AllocQuery: %v", err)
	}
	if err := g.BeginQuery(idx, 10, 1000); err != nil {
		t.Fatalf("BeginQuery: %v", err)
	}
	if err := g.EndQuery(idx, 11, 2000); !errors.Is(err, taskpkg.ErrFrameMismatch) {
		t.Fatalf("EndQuery on another frame error = %v, want ErrFrameMismatch", err)
	}
	if g.Ready(idx) {
		t.Fatalf("mismatched end frame still resolved the slot")
	}
	if err := g.EndQuery(idx, 10, 2000); err != nil {
		t.Fatalf("EndQuery on the matching frame: %v", err)
	}
}

// An end stamp that precedes the begin stamp must not produce a duration.
func TestEndBeforeBeginRejected(t *testing.T) {
	g := mustPool(t, 4, 1)
	idx, err := g.AllocQuery("post")
	if err != nil {
		t.Fatalf("AllocQuery: %v", err)
	}
	if err := g.BeginQuery(idx, 3, 5000); err != nil {
		t.Fatalf("BeginQuery: %v", err)
	}
	if err := g.EndQuery(idx, 3, 4000); !errors.Is(err, taskpkg.ErrBadRange) {
		t.Fatalf("EndQuery with an earlier stamp error = %v, want ErrBadRange", err)
	}
	if g.Ready(idx) {
		t.Fatalf("slot resolved after a rejected end stamp")
	}
}

// Tick counts are converted with the configured tick period in nanoseconds.
func TestDurationUsesTickPeriod(t *testing.T) {
	g := mustPool(t, 4, 1)
	idx, err := g.AllocQuery("composite")
	if err != nil {
		t.Fatalf("AllocQuery: %v", err)
	}
	if err := g.BeginQuery(idx, 1, 1000000); err != nil {
		t.Fatalf("BeginQuery: %v", err)
	}
	if err := g.EndQuery(idx, 1, 3500000); err != nil {
		t.Fatalf("EndQuery: %v", err)
	}
	got, err := g.DurationMs(idx)
	if err != nil {
		t.Fatalf("DurationMs: %v", err)
	}
	if !near(got, 2.5, 1e-9) {
		t.Fatalf("DurationMs = %v, want 2.5", got)
	}
	if !g.Ready(idx) {
		t.Fatalf("Ready = false after a successful end")
	}
}

// An unavailable result is excluded from every aggregate and from the resolved
// listing, which otherwise keeps index order.
func TestMarkUnavailableExcludesFromTotals(t *testing.T) {
	g := mustPool(t, 4, 1)
	first, err := g.AllocQuery("shadow")
	if err != nil {
		t.Fatalf("AllocQuery first: %v", err)
	}
	second, err := g.AllocQuery("shadow")
	if err != nil {
		t.Fatalf("AllocQuery second: %v", err)
	}
	for _, step := range []struct {
		idx int
		end uint64
	}{{first, 2_000_000}, {second, 4_000_000}} {
		if err := g.BeginQuery(step.idx, 7, 1_000_000); err != nil {
			t.Fatalf("BeginQuery(%d): %v", step.idx, err)
		}
		if err := g.EndQuery(step.idx, 7, step.end); err != nil {
			t.Fatalf("EndQuery(%d): %v", step.idx, err)
		}
	}
	if err := g.MarkUnavailable(second, "TIMESTAMP_NOT_READY"); err != nil {
		t.Fatalf("MarkUnavailable: %v", err)
	}
	if g.Ready(second) {
		t.Fatalf("unavailable slot still reports Ready")
	}
	if got := g.PassMs(7, "shadow"); !near(got, 1, 1e-9) {
		t.Fatalf("PassMs = %v, want 1", got)
	}
	if got := g.FrameMs(7); !near(got, 1, 1e-9) {
		t.Fatalf("FrameMs = %v, want 1", got)
	}
	list := g.ResolvedQueries(7)
	if len(list) != 1 {
		t.Fatalf("ResolvedQueries returned %d entries, want 1", len(list))
	}
	if list[0].Index != first || list[0].State != taskpkg.QueryResolved || !near(list[0].DurationMs, 1, 1e-9) {
		t.Fatalf("ResolvedQueries[0] = %+v, want index %d resolved with 1 ms", list[0], first)
	}
}

// A slot that is still begun cannot be released; after the end it is reusable.
func TestReleaseBusyThenReuse(t *testing.T) {
	g := mustPool(t, 4, 1)
	idx, err := g.AllocQuery("taa")
	if err != nil {
		t.Fatalf("AllocQuery: %v", err)
	}
	if err := g.BeginQuery(idx, 2, 1000); err != nil {
		t.Fatalf("BeginQuery: %v", err)
	}
	if err := g.Release(idx); !errors.Is(err, taskpkg.ErrReleaseBusy) {
		t.Fatalf("Release on a begun slot error = %v, want ErrReleaseBusy", err)
	}
	if g.ActiveCount() != 1 {
		t.Fatalf("ActiveCount = %d, want 1", g.ActiveCount())
	}
	if err := g.EndQuery(idx, 2, 3000); err != nil {
		t.Fatalf("EndQuery: %v", err)
	}
	if err := g.Release(idx); err != nil {
		t.Fatalf("Release after end: %v", err)
	}
	if g.ActiveCount() != 0 {
		t.Fatalf("ActiveCount = %d, want 0", g.ActiveCount())
	}
	again, err := g.AllocQuery("taa")
	if err != nil {
		t.Fatalf("AllocQuery after release: %v", err)
	}
	if again != idx {
		t.Fatalf("reallocated index = %d, want %d", again, idx)
	}
}

// Per-pass and per-frame totals aggregate only resolved results.
func TestPassAndFrameTotals(t *testing.T) {
	g := mustPool(t, 8, 1)
	plan := []struct {
		label string
		begin uint64
		end   uint64
	}{
		{"gbuffer", 1_000_000, 3_000_000},
		{"gbuffer", 3_000_000, 6_000_000},
		{"lighting", 6_000_000, 11_000_000},
	}
	for i, p := range plan {
		idx, err := g.AllocQuery(p.label)
		if err != nil {
			t.Fatalf("AllocQuery #%d: %v", i, err)
		}
		if err := g.BeginQuery(idx, 7, p.begin); err != nil {
			t.Fatalf("BeginQuery #%d: %v", i, err)
		}
		if err := g.EndQuery(idx, 7, p.end); err != nil {
			t.Fatalf("EndQuery #%d: %v", i, err)
		}
	}
	if got := g.PassMs(7, "gbuffer"); !near(got, 5, 1e-9) {
		t.Fatalf("PassMs(gbuffer) = %v, want 5", got)
	}
	if got := g.PassMs(7, "lighting"); !near(got, 5, 1e-9) {
		t.Fatalf("PassMs(lighting) = %v, want 5", got)
	}
	if got := g.PassMs(7, "missing"); got != 0 {
		t.Fatalf("PassMs of an unknown pass = %v, want 0", got)
	}
	if got := g.FrameMs(7); !near(got, 10, 1e-9) {
		t.Fatalf("FrameMs = %v, want 10", got)
	}
	if got := g.FrameMs(8); got != 0 {
		t.Fatalf("FrameMs of an empty frame = %v, want 0", got)
	}
}

// Budget violations are reported once per pass, sorted by label.
func TestOverBudgetSorted(t *testing.T) {
	g := mustPool(t, 8, 1)
	for i, p := range []struct {
		label string
		end   uint64
	}{{"shadow", 2_000_000}, {"lighting", 6_000_000}, {"gbuffer", 5_000_000}} {
		idx, err := g.AllocQuery(p.label)
		if err != nil {
			t.Fatalf("AllocQuery #%d: %v", i, err)
		}
		if err := g.BeginQuery(idx, 4, 1_000_000); err != nil {
			t.Fatalf("BeginQuery #%d: %v", i, err)
		}
		if err := g.EndQuery(idx, 4, p.end); err != nil {
			t.Fatalf("EndQuery #%d: %v", i, err)
		}
	}
	g.SetBudget("shadow", 0.5)
	g.SetBudget("lighting", 4)
	g.SetBudget("gbuffer", 100)
	got := g.OverBudget(4)
	want := []string{"lighting", "shadow"}
	if len(got) != len(want) {
		t.Fatalf("OverBudget = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("OverBudget = %v, want %v", got, want)
		}
	}
	if other := g.OverBudget(5); len(other) != 0 {
		t.Fatalf("OverBudget of an empty frame = %v, want none", other)
	}
}

// A snapshot keeps pass totals and budgets across a device-loss restart while
// starting with an empty allocation table.
func TestSnapshotRestoresHistory(t *testing.T) {
	g := mustPool(t, 8, 1)
	for i, p := range []struct {
		label string
		end   uint64
	}{{"gbuffer", 4_000_000}, {"lighting", 9_000_000}} {
		idx, err := g.AllocQuery(p.label)
		if err != nil {
			t.Fatalf("AllocQuery #%d: %v", i, err)
		}
		if err := g.BeginQuery(idx, 9, 1_000_000); err != nil {
			t.Fatalf("BeginQuery #%d: %v", i, err)
		}
		if err := g.EndQuery(idx, 9, p.end); err != nil {
			t.Fatalf("EndQuery #%d: %v", i, err)
		}
	}
	g.SetBudget("gbuffer", 2)
	blob, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewGpuTimestampFromSnapshot(8, 1, blob)
	if err != nil {
		t.Fatalf("NewGpuTimestampFromSnapshot: %v", err)
	}
	if restored.ActiveCount() != 0 {
		t.Fatalf("restored pool ActiveCount = %d, want 0", restored.ActiveCount())
	}
	if got := restored.PassMs(9, "gbuffer"); !near(got, 3, 1e-9) {
		t.Fatalf("restored PassMs(gbuffer) = %v, want 3", got)
	}
	if got := restored.FrameMs(9); !near(got, 11, 1e-9) {
		t.Fatalf("restored FrameMs = %v, want 11", got)
	}
	over := restored.OverBudget(9)
	if len(over) != 1 || over[0] != "gbuffer" {
		t.Fatalf("restored OverBudget = %v, want [gbuffer]", over)
	}
	if list := restored.ResolvedQueries(9); len(list) != 0 {
		t.Fatalf("restored ResolvedQueries = %v, want empty", list)
	}
}

// Slots are handed to concurrent callers exactly once, and the totals of one
// frame stay exact under that concurrency.
func TestConcurrentAllocAndTotals(t *testing.T) {
	const workers = 8
	g := mustPool(t, workers, 1)
	indices := make([]int, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			idx, err := g.AllocQuery(fmt.Sprintf("pass%d", slot))
			if err != nil {
				errs[slot] = err
				return
			}
			indices[slot] = idx
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent AllocQuery #%d: %v", i, err)
		}
	}
	seen := make(map[int]bool, workers)
	for _, idx := range indices {
		if seen[idx] {
			t.Fatalf("index %d was handed out twice: %v", idx, indices)
		}
		seen[idx] = true
	}
	for _, idx := range indices {
		if err := g.BeginQuery(idx, 21, 1_000_000); err != nil {
			t.Fatalf("BeginQuery(%d): %v", idx, err)
		}
		if err := g.EndQuery(idx, 21, 2_000_000); err != nil {
			t.Fatalf("EndQuery(%d): %v", idx, err)
		}
	}
	if got := g.FrameMs(21); !near(got, 8, 1e-9) {
		t.Fatalf("FrameMs = %v, want 8", got)
	}
	if got := g.ActiveCount(); got != workers {
		t.Fatalf("ActiveCount = %d, want %d", got, workers)
	}
	if _, err := g.AllocQuery("overflow"); !errors.Is(err, taskpkg.ErrPoolExhausted) {
		t.Fatalf("allocation past the pool error = %v, want ErrPoolExhausted", err)
	}
}
