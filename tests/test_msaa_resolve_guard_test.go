package task

import (
	"errors"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-f-01"
)

func testMsaaConfig() taskpkg.MsaaConfig {
	return taskpkg.MsaaConfig{
		TileSize:        64,
		MaxTargets:      8,
		MaxResolveBytes: 1 << 20,
	}
}

func newMsaaGuard(t *testing.T) *taskpkg.MsaaResolve {
	t.Helper()
	g, err := taskpkg.NewMsaaResolve(testMsaaConfig())
	if err != nil {
		t.Fatalf("NewMsaaResolve: %v", err)
	}
	return g
}

func mustRegisterMsaa(t *testing.T, g *taskpkg.MsaaResolve, name string, w, h, samples int) {
	t.Helper()
	if err := g.RegisterTarget(name, w, h, samples); err != nil {
		t.Fatalf("RegisterTarget(%q): %v", name, err)
	}
}

func mustResolveMsaa(t *testing.T, g *taskpkg.MsaaResolve, name string) taskpkg.ResolveResult {
	t.Helper()
	res, err := g.Resolve(name)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", name, err)
	}
	return res
}

// A renderer only supports 1x, 2x, 4x and 8x multisampling.
func TestRegisterRejectsUnsupportedSampleCount(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 64, 64, 4)
	if err := g.RegisterTarget("bad-16x", 64, 64, 16); !errors.Is(err, taskpkg.ErrUnsupportedSamples) {
		t.Fatalf("16 samples must yield ErrUnsupportedSamples, got %v", err)
	}
	if err := g.RegisterTarget("bad-zero", 64, 64, 0); !errors.Is(err, taskpkg.ErrUnsupportedSamples) {
		t.Fatalf("zero samples must yield ErrUnsupportedSamples, got %v", err)
	}
	if _, err := g.Target("bad-16x"); !errors.Is(err, taskpkg.ErrTargetUnknown) {
		t.Fatalf("rejected target must not be stored, got %v", err)
	}
}

// The same colour target must not be registered twice.
func TestDuplicateRegistrationRejected(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 128, 96, 4)
	err := g.RegisterTarget("color-hdr", 256, 256, 2)
	if !errors.Is(err, taskpkg.ErrTargetExists) {
		t.Fatalf("duplicate target must yield ErrTargetExists, got %v", err)
	}
	target, err := g.Target("color-hdr")
	if err != nil {
		t.Fatalf("Target: %v", err)
	}
	if target.Width != 128 || target.Height != 96 || target.Samples != 4 {
		t.Fatalf("duplicate registration overwrote the target: %+v", target)
	}
}

// The target table has a hard cap from the config.
func TestTargetLimitEnforced(t *testing.T) {
	g, err := taskpkg.NewMsaaResolve(taskpkg.MsaaConfig{TileSize: 64, MaxTargets: 2, MaxResolveBytes: 1 << 20})
	if err != nil {
		t.Fatalf("NewMsaaResolve: %v", err)
	}
	mustRegisterMsaa(t, g, "gbuffer", 128, 128, 4)
	mustRegisterMsaa(t, g, "shadow", 128, 128, 2)
	if err := g.RegisterTarget("reflection", 128, 128, 4); !errors.Is(err, taskpkg.ErrTargetLimit) {
		t.Fatalf("third target must yield ErrTargetLimit, got %v", err)
	}
	if _, err := g.Target("reflection"); !errors.Is(err, taskpkg.ErrTargetUnknown) {
		t.Fatalf("target over the limit must not be stored, got %v", err)
	}
}

// A freshly registered target is fully dirty: 192x128 with 64px tiles is 6.
func TestFullTargetDirtyTileCount(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 192, 128, 4)
	dirty, err := g.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles: %v", err)
	}
	if dirty != 6 {
		t.Fatalf("want 6 dirty tiles for 192x128, got %d", dirty)
	}
}

// Overlapping dirty rectangles count each tile once.
func TestOverlappingDirtyRegionsCountUniqueTiles(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 192, 128, 4)
	if err := g.MarkDirty("color-hdr", 0, 0, 64, 64); err != nil {
		t.Fatalf("MarkDirty 1: %v", err)
	}
	dirty, err := g.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles: %v", err)
	}
	if dirty != 6 {
		t.Fatalf("registered target is already fully dirty, got %d", dirty)
	}
	if _, err := g.Resolve("color-hdr"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := g.MarkDirty("color-hdr", 0, 0, 64, 64); err != nil {
		t.Fatalf("MarkDirty 1: %v", err)
	}
	if err := g.MarkDirty("color-hdr", 32, 32, 64, 64); err != nil {
		t.Fatalf("MarkDirty 2: %v", err)
	}
	dirty, err = g.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles: %v", err)
	}
	if dirty != 4 {
		t.Fatalf("overlapping dirty regions must union to 4 tiles, got %d", dirty)
	}
}

// Dirty regions are clipped to the target; fully outside regions are errors.
func TestDirtyRegionClippingAndOutsideRejected(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 192, 128, 4)
	if _, err := g.Resolve("color-hdr"); err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if err := g.MarkDirty("color-hdr", 200, 0, 32, 32); !errors.Is(err, taskpkg.ErrRegionOutside) {
		t.Fatalf("fully outside region must yield ErrRegionOutside, got %v", err)
	}
	if err := g.MarkDirty("color-hdr", 0, 0, 0, 10); !errors.Is(err, taskpkg.ErrRegionOutside) {
		t.Fatalf("zero width region must yield ErrRegionOutside, got %v", err)
	}
	dirty, err := g.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles: %v", err)
	}
	if dirty != 0 {
		t.Fatalf("rejected regions must not dirty tiles, got %d", dirty)
	}
	if err := g.MarkDirty("color-hdr", 184, 120, 64, 64); err != nil {
		t.Fatalf("partially outside region must be clipped, got %v", err)
	}
	dirty, err = g.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles: %v", err)
	}
	if dirty != 1 {
		t.Fatalf("clipped region touches one tile, got %d", dirty)
	}
}

// A real resolve writes width*height*4 bytes and clears the dirty set.
func TestResolveReportsWorkBytes(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 192, 128, 4)
	res := mustResolveMsaa(t, g, "color-hdr")
	if res.FromCache {
		t.Fatalf("first resolve must not come from the cache")
	}
	if res.Bytes != 192*128*4 {
		t.Fatalf("want %d resolved bytes, got %d", 192*128*4, res.Bytes)
	}
	if res.Samples != 4 {
		t.Fatalf("want 4 samples in the result, got %d", res.Samples)
	}
	dirty, err := g.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles: %v", err)
	}
	if dirty != 0 {
		t.Fatalf("resolve must clear dirty tiles, got %d", dirty)
	}
	if stats := g.Stats(); stats.Resolves != 1 || stats.BytesResolved != 192*128*4 {
		t.Fatalf("unexpected stats after resolve: %+v", stats)
	}
}

// Resolving an unchanged target must reuse the previous result.
func TestSecondResolveServedFromCache(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 128, 128, 4)
	mustResolveMsaa(t, g, "color-hdr")
	res := mustResolveMsaa(t, g, "color-hdr")
	if !res.FromCache {
		t.Fatalf("second resolve must be served from the cache")
	}
	if res.Bytes != 0 {
		t.Fatalf("cache hit must write zero bytes, got %d", res.Bytes)
	}
	stats := g.Stats()
	if stats.Resolves != 1 || stats.CachedResolves != 1 {
		t.Fatalf("unexpected stats after cache hit: %+v", stats)
	}
}

// New dirty pixels and a resize both invalidate the resolve cache.
func TestDirtyAndResizeInvalidateCache(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 128, 128, 4)
	mustResolveMsaa(t, g, "color-hdr")
	if err := g.MarkDirty("color-hdr", 0, 0, 64, 64); err != nil {
		t.Fatalf("MarkDirty: %v", err)
	}
	if err := g.ResizeTarget("color-hdr", 128, 64); err != nil {
		t.Fatalf("ResizeTarget: %v", err)
	}
	target, err := g.Target("color-hdr")
	if err != nil {
		t.Fatalf("Target: %v", err)
	}
	if target.Width != 128 || target.Height != 64 {
		t.Fatalf("resize was not applied: %+v", target)
	}
	dirty, err := g.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles: %v", err)
	}
	if dirty != 2 {
		t.Fatalf("resize must dirty the whole 128x64 surface, got %d tiles", dirty)
	}
	res := mustResolveMsaa(t, g, "color-hdr")
	if res.FromCache {
		t.Fatalf("resolve after resize must do real work")
	}
	if res.Bytes != 128*64*4 {
		t.Fatalf("want %d bytes after resize, got %d", 128*64*4, res.Bytes)
	}
}

// A single-sample target has nothing to resolve.
func TestSingleSampleTargetNeedsNoResolve(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "final-ldr", 64, 64, 1)
	if _, err := g.Resolve("final-ldr"); !errors.Is(err, taskpkg.ErrNoResolveNeeded) {
		t.Fatalf("single sample resolve must yield ErrNoResolveNeeded, got %v", err)
	}
	if _, err := g.ResolveFrame([]string{"final-ldr"}); !errors.Is(err, taskpkg.ErrNoResolveNeeded) {
		t.Fatalf("frame resolve of a single sample target must yield ErrNoResolveNeeded, got %v", err)
	}
}

// The per-frame byte budget is checked before any target is touched.
func TestResolveFrameBudgetExceededLeavesTargetsDirty(t *testing.T) {
	g, err := taskpkg.NewMsaaResolve(taskpkg.MsaaConfig{TileSize: 64, MaxTargets: 4, MaxResolveBytes: 100000})
	if err != nil {
		t.Fatalf("NewMsaaResolve: %v", err)
	}
	mustRegisterMsaa(t, g, "color-hdr", 128, 128, 4)
	mustRegisterMsaa(t, g, "shadow", 128, 128, 4)
	if _, err := g.ResolveFrame([]string{"color-hdr", "shadow"}); !errors.Is(err, taskpkg.ErrBudgetExceeded) {
		t.Fatalf("two 65536-byte resolves must exceed the 100000 budget, got %v", err)
	}
	if stats := g.Stats(); stats.Resolves != 0 || stats.BytesResolved != 0 {
		t.Fatalf("failed frame resolve must not resolve anything: %+v", stats)
	}
	for _, name := range []string{"color-hdr", "shadow"} {
		dirty, err := g.DirtyTiles(name)
		if err != nil {
			t.Fatalf("DirtyTiles(%q): %v", name, err)
		}
		if dirty != 4 {
			t.Fatalf("%q must stay dirty after a rejected frame, got %d", name, dirty)
		}
	}
	if res := mustResolveMsaa(t, g, "color-hdr"); res.FromCache {
		t.Fatalf("target was resolved despite the rejected frame")
	}
}

// An unknown name rejects the whole frame resolve.
func TestResolveFrameUnknownTargetLeavesTargetsDirty(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 128, 128, 4)
	if _, err := g.ResolveFrame([]string{"color-hdr", "missing"}); !errors.Is(err, taskpkg.ErrTargetUnknown) {
		t.Fatalf("unknown target in frame must yield ErrTargetUnknown, got %v", err)
	}
	if stats := g.Stats(); stats.Resolves != 0 {
		t.Fatalf("failed frame resolve must not resolve anything: %+v", stats)
	}
	dirty, err := g.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles: %v", err)
	}
	if dirty != 4 {
		t.Fatalf("target must stay dirty after a rejected frame, got %d", dirty)
	}
}

// Two jobs resolving the same target must not both do the work.
func TestConcurrentResolveSingleWorker(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 256, 256, 4)
	const workers = 8
	var wg sync.WaitGroup
	results := make([]taskpkg.ResolveResult, workers)
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = g.Resolve("color-hdr")
		}(i)
	}
	wg.Wait()
	work := 0
	cached := 0
	for i := 0; i < workers; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d failed: %v", i, errs[i])
		}
		if results[i].FromCache {
			cached++
		} else {
			work++
		}
	}
	if work != 1 || cached != workers-1 {
		t.Fatalf("want exactly one real resolve and %d cache hits, got work=%d cached=%d", workers-1, work, cached)
	}
}

// Dirty bookkeeping must survive a restart through the snapshot.
func TestSnapshotRestorePreservesDirtyState(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 192, 128, 4)
	mustResolveMsaa(t, g, "color-hdr")
	if err := g.MarkDirty("color-hdr", 0, 0, 64, 64); err != nil {
		t.Fatalf("MarkDirty 1: %v", err)
	}
	if err := g.MarkDirty("color-hdr", 64, 64, 64, 64); err != nil {
		t.Fatalf("MarkDirty 2: %v", err)
	}
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewMsaaResolveFromSnapshot(testMsaaConfig(), snap)
	if err != nil {
		t.Fatalf("NewMsaaResolveFromSnapshot: %v", err)
	}
	dirty, err := restored.DirtyTiles("color-hdr")
	if err != nil {
		t.Fatalf("DirtyTiles after restore: %v", err)
	}
	if dirty != 2 {
		t.Fatalf("restored guard lost dirty tiles, got %d", dirty)
	}
	target, err := restored.Target("color-hdr")
	if err != nil {
		t.Fatalf("Target after restore: %v", err)
	}
	if target.Width != 192 || target.Height != 128 || target.Samples != 4 {
		t.Fatalf("restored target mismatch: %+v", target)
	}
	if _, err := taskpkg.NewMsaaResolveFromSnapshot(testMsaaConfig(), []byte("{not-json")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot must yield ErrCorruptSnapshot, got %v", err)
	}
}

// A cleared target restored from a snapshot must hit the cache again.
func TestSnapshotRestorePreservesResolveCache(t *testing.T) {
	g := newMsaaGuard(t)
	mustRegisterMsaa(t, g, "color-hdr", 128, 128, 4)
	mustResolveMsaa(t, g, "color-hdr")
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewMsaaResolveFromSnapshot(testMsaaConfig(), snap)
	if err != nil {
		t.Fatalf("NewMsaaResolveFromSnapshot: %v", err)
	}
	res, err := restored.Resolve("color-hdr")
	if err != nil {
		t.Fatalf("Resolve after restore: %v", err)
	}
	if !res.FromCache || res.Bytes != 0 {
		t.Fatalf("restored resolved target must hit the cache, got %+v", res)
	}
}
