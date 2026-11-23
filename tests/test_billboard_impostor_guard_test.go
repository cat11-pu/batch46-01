package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-f-05"
)

func testImpostorConfig() taskpkg.ImpostorConfig {
	return taskpkg.ImpostorConfig{
		AtlasCells:         16,
		MaxBakeOpsPerFrame: 16,
		LODDistance:        100,
		FadeBand:           20,
	}
}

func newImpostorGuard(t *testing.T) *taskpkg.ImpostorGuard {
	t.Helper()
	g, err := taskpkg.NewImpostorGuard(testImpostorConfig())
	if err != nil {
		t.Fatalf("NewImpostorGuard: %v", err)
	}
	return g
}

func mustBake(t *testing.T, g *taskpkg.ImpostorGuard, id string, frames, cellSize int) taskpkg.BakeResult {
	t.Helper()
	res, err := g.Bake(taskpkg.ImpostorRequest{AssetID: id, Frames: frames, CellSize: cellSize})
	if err != nil {
		t.Fatalf("Bake(%q, %d): %v", id, frames, err)
	}
	return res
}

func mustUsage(t *testing.T, g *taskpkg.ImpostorGuard) taskpkg.Usage {
	t.Helper()
	u, err := g.AtlasUsage()
	if err != nil {
		t.Fatalf("AtlasUsage: %v", err)
	}
	return u
}

// Requests outside the supported ranges are rejected up front.
func TestInvalidBakeRequestRejected(t *testing.T) {
	g := newImpostorGuard(t)
	bad := []taskpkg.ImpostorRequest{
		{AssetID: "", Frames: 4, CellSize: 128},
		{AssetID: "tree", Frames: 0, CellSize: 128},
		{AssetID: "tree", Frames: 65, CellSize: 128},
		{AssetID: "tree", Frames: 4, CellSize: 0},
	}
	for _, req := range bad {
		if _, err := g.Bake(req); !errors.Is(err, taskpkg.ErrInvalidRequest) {
			t.Fatalf("request %+v must yield ErrInvalidRequest, got %v", req, err)
		}
	}
	u := mustUsage(t, g)
	if u.Used != 0 || u.Assets != 0 {
		t.Fatalf("rejected requests must not allocate, got %+v", u)
	}
}

// Cells are handed out as contiguous runs starting at the lowest free cell.
func TestBakeAllocatesContiguousCells(t *testing.T) {
	g := newImpostorGuard(t)
	first := mustBake(t, g, "oak-tree", 4, 128)
	if first.FirstCell != 0 || first.Allocated != 4 || first.Frames != 4 {
		t.Fatalf("unexpected first allocation: %+v", first)
	}
	second := mustBake(t, g, "bush", 2, 128)
	if second.FirstCell != 4 || second.Allocated != 2 {
		t.Fatalf("second allocation must start at cell 4, got %+v", second)
	}
	u := mustUsage(t, g)
	if u.Used != 6 || u.Total != 16 || u.Assets != 2 {
		t.Fatalf("unexpected usage: %+v", u)
	}
}

// Repeating the identical request must not allocate again.
func TestDuplicateBakeIsIdempotent(t *testing.T) {
	g := newImpostorGuard(t)
	mustBake(t, g, "oak-tree", 4, 128)
	again := mustBake(t, g, "oak-tree", 4, 128)
	if again.Allocated != 0 || again.FirstCell != 0 || again.Frames != 4 {
		t.Fatalf("cache hit must reuse the first run, got %+v", again)
	}
	u := mustUsage(t, g)
	if u.Used != 4 || u.Assets != 1 {
		t.Fatalf("cache hit must not grow the atlas, got %+v", u)
	}
	stats := g.Stats()
	if stats.Bakes != 1 || stats.CacheHits != 1 {
		t.Fatalf("unexpected bake stats: %+v", stats)
	}
}

// The same asset with different parameters is a conflict.
func TestConflictingAssetRejected(t *testing.T) {
	g := newImpostorGuard(t)
	mustBake(t, g, "oak-tree", 4, 128)
	if _, err := g.Bake(taskpkg.ImpostorRequest{AssetID: "oak-tree", Frames: 8, CellSize: 128}); !errors.Is(err, taskpkg.ErrAssetConflict) {
		t.Fatalf("different frame count must yield ErrAssetConflict, got %v", err)
	}
	if _, err := g.Bake(taskpkg.ImpostorRequest{AssetID: "oak-tree", Frames: 4, CellSize: 64}); !errors.Is(err, taskpkg.ErrAssetConflict) {
		t.Fatalf("different cell size must yield ErrAssetConflict, got %v", err)
	}
	if u := mustUsage(t, g); u.Used != 4 || u.Assets != 1 {
		t.Fatalf("conflicts must not change usage, got %+v", u)
	}
}

// The atlas has a hard cell count.
func TestAtlasFullRejected(t *testing.T) {
	g, err := taskpkg.NewImpostorGuard(taskpkg.ImpostorConfig{
		AtlasCells:         3,
		MaxBakeOpsPerFrame: 16,
		LODDistance:        100,
		FadeBand:           20,
	})
	if err != nil {
		t.Fatalf("NewImpostorGuard: %v", err)
	}
	if _, err := g.Bake(taskpkg.ImpostorRequest{AssetID: "oak-tree", Frames: 4, CellSize: 128}); !errors.Is(err, taskpkg.ErrAtlasFull) {
		t.Fatalf("4 cells in a 3 cell atlas must yield ErrAtlasFull, got %v", err)
	}
	if u := mustUsage(t, g); u.Used != 0 {
		t.Fatalf("failed bake must not occupy cells, got %+v", u)
	}
	mustBake(t, g, "oak-tree", 3, 128)
	if _, err := g.Bake(taskpkg.ImpostorRequest{AssetID: "pine", Frames: 1, CellSize: 128}); !errors.Is(err, taskpkg.ErrAtlasFull) {
		t.Fatalf("one more cell in a full atlas must yield ErrAtlasFull, got %v", err)
	}
}

// The per-frame bake budget resets with BeginFrame.
func TestBakeBudgetPerFrame(t *testing.T) {
	g, err := taskpkg.NewImpostorGuard(taskpkg.ImpostorConfig{
		AtlasCells:         16,
		MaxBakeOpsPerFrame: 3,
		LODDistance:        100,
		FadeBand:           20,
	})
	if err != nil {
		t.Fatalf("NewImpostorGuard: %v", err)
	}
	if _, err := g.Bake(taskpkg.ImpostorRequest{AssetID: "oak-tree", Frames: 4, CellSize: 128}); !errors.Is(err, taskpkg.ErrBakeBudget) {
		t.Fatalf("4 cells over a budget of 3 must yield ErrBakeBudget, got %v", err)
	}
	if u := mustUsage(t, g); u.Used != 0 {
		t.Fatalf("failed bake must not occupy cells, got %+v", u)
	}
	mustBake(t, g, "ash", 2, 128)
	if _, err := g.Bake(taskpkg.ImpostorRequest{AssetID: "birch", Frames: 2, CellSize: 128}); !errors.Is(err, taskpkg.ErrBakeBudget) {
		t.Fatalf("budget must span the frame, got %v", err)
	}
	if err := g.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame: %v", err)
	}
	res := mustBake(t, g, "birch", 2, 128)
	if res.FirstCell != 2 || res.Allocated != 2 {
		t.Fatalf("bake after BeginFrame must succeed at cell 2, got %+v", res)
	}
}

// Angles wrap at 2*pi and map to the centre of each frame bin.
func TestFrameForWrapsAngles(t *testing.T) {
	g := newImpostorGuard(t)
	mustBake(t, g, "oak-tree", 4, 128)
	checks := []struct {
		angle float32
		want  int
	}{
		{0, 0},
		{float32(math.Pi / 4), 0},
		{float32(3 * math.Pi / 4), 1},
		{float32(5 * math.Pi / 4), 2},
		{float32(7 * math.Pi / 4), 3},
		{float32(11 * math.Pi / 4), 1},
		{float32(2 * math.Pi), 0},
	}
	for _, c := range checks {
		got, err := g.FrameFor("oak-tree", c.angle)
		if err != nil {
			t.Fatalf("FrameFor(%v): %v", c.angle, err)
		}
		if got != c.want {
			t.Fatalf("FrameFor(%v) = %d, want %d", c.angle, got, c.want)
		}
	}
	if _, err := g.FrameFor("ghost", 0); !errors.Is(err, taskpkg.ErrAssetUnknown) {
		t.Fatalf("unknown asset must yield ErrAssetUnknown, got %v", err)
	}
}

// Negative angles wrap into the same frame ring.
func TestFrameForNegativeAngle(t *testing.T) {
	g := newImpostorGuard(t)
	mustBake(t, g, "oak-tree", 4, 128)
	checks := []struct {
		angle float32
		want  int
	}{
		{float32(-math.Pi / 4), 3},
		{float32(-3 * math.Pi / 4), 2},
		{float32(-2 * math.Pi), 0},
	}
	for _, c := range checks {
		got, err := g.FrameFor("oak-tree", c.angle)
		if err != nil {
			t.Fatalf("FrameFor(%v): %v", c.angle, err)
		}
		if got != c.want {
			t.Fatalf("FrameFor(%v) = %d, want %d", c.angle, got, c.want)
		}
	}
}

// Close to the camera the real mesh is used.
func TestSelectModeMesh(t *testing.T) {
	g := newImpostorGuard(t)
	mode, err := g.SelectMode(50)
	if err != nil {
		t.Fatalf("SelectMode: %v", err)
	}
	if mode != "mesh" {
		t.Fatalf("distance 50 with LOD 100 must be mesh, got %q", mode)
	}
	mode, err = g.SelectMode(99.5)
	if err != nil {
		t.Fatalf("SelectMode: %v", err)
	}
	if mode != "mesh" {
		t.Fatalf("distance 99.5 must still be mesh, got %q", mode)
	}
}

// Inside the fade band the billboard crosses over gradually.
func TestSelectModeBlend(t *testing.T) {
	g := newImpostorGuard(t)
	mustBake(t, g, "oak-tree", 4, 128)
	cases := []struct {
		distance float32
		want     float32
	}{
		{100, 1},
		{110, 0.5},
		{115, 0.25},
	}
	for _, c := range cases {
		mode, err := g.SelectMode(c.distance)
		if err != nil {
			t.Fatalf("SelectMode(%v): %v", c.distance, err)
		}
		if mode != "blend" {
			t.Fatalf("distance %v must be blend, got %q", c.distance, mode)
		}
		b, err := g.Instance("oak-tree", 0, c.distance)
		if err != nil {
			t.Fatalf("Instance(%v): %v", c.distance, err)
		}
		if b.Blend != c.want {
			t.Fatalf("blend at %v = %v, want %v", c.distance, b.Blend, c.want)
		}
	}
}

// Beyond the fade band only the impostor is drawn.
func TestSelectModeImpostor(t *testing.T) {
	g := newImpostorGuard(t)
	mode, err := g.SelectMode(150)
	if err != nil {
		t.Fatalf("SelectMode: %v", err)
	}
	if mode != "impostor" {
		t.Fatalf("distance 150 must be impostor, got %q", mode)
	}
	if _, err := g.SelectMode(-1); !errors.Is(err, taskpkg.ErrInvalidRequest) {
		t.Fatalf("negative distance must yield ErrInvalidRequest, got %v", err)
	}
}

// Instances combine the frame, mode, blend and atlas U coordinate.
func TestBillboardUsesFrameAndMode(t *testing.T) {
	g := newImpostorGuard(t)
	mustBake(t, g, "oak-tree", 4, 128)
	b, err := g.Instance("oak-tree", float32(5*math.Pi/4), 110)
	if err != nil {
		t.Fatalf("Instance: %v", err)
	}
	if b.Frame != 2 {
		t.Fatalf("frame = %d, want 2", b.Frame)
	}
	if b.Mode != "blend" {
		t.Fatalf("mode = %q, want blend", b.Mode)
	}
	if b.Blend != 0.5 {
		t.Fatalf("blend = %v, want 0.5", b.Blend)
	}
	if b.U != 0.625 {
		t.Fatalf("U = %v, want 0.625", b.U)
	}
	if _, err := g.Instance("ghost", 0, 50); !errors.Is(err, taskpkg.ErrAssetUnknown) {
		t.Fatalf("unknown asset must yield ErrAssetUnknown, got %v", err)
	}
	if _, err := g.Instance("oak-tree", 0, -5); !errors.Is(err, taskpkg.ErrInvalidRequest) {
		t.Fatalf("negative distance must yield ErrInvalidRequest, got %v", err)
	}
}

// Freeing an asset returns its run and first-fit reuses it.
func TestFreeAssetReturnsCells(t *testing.T) {
	g := newImpostorGuard(t)
	mustBake(t, g, "oak-tree", 4, 128)
	mustBake(t, g, "bush", 2, 128)
	if err := g.FreeAsset("oak-tree"); err != nil {
		t.Fatalf("FreeAsset: %v", err)
	}
	u := mustUsage(t, g)
	if u.Used != 2 || u.Assets != 1 {
		t.Fatalf("free must return four cells, got %+v", u)
	}
	res := mustBake(t, g, "oak-tree", 3, 128)
	if res.FirstCell != 0 || res.Allocated != 3 {
		t.Fatalf("first-fit must reuse the freed run at 0, got %+v", res)
	}
	if err := g.FreeAsset("oak-tree"); err != nil {
		t.Fatalf("FreeAsset second time: %v", err)
	}
	if err := g.FreeAsset("oak-tree"); !errors.Is(err, taskpkg.ErrAssetUnknown) {
		t.Fatalf("freeing twice must yield ErrAssetUnknown, got %v", err)
	}
	if stats := g.Stats(); stats.Frees != 2 {
		t.Fatalf("free counter = %d, want 2", stats.Frees)
	}
}

// Concurrent bakes of one asset must converge on a single allocation.
func TestConcurrentBakeSameAssetSingleAllocation(t *testing.T) {
	g := newImpostorGuard(t)
	const workers = 8
	var wg sync.WaitGroup
	results := make([]taskpkg.BakeResult, workers)
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = g.Bake(taskpkg.ImpostorRequest{AssetID: "oak-tree", Frames: 4, CellSize: 128})
		}(i)
	}
	wg.Wait()
	allocations := 0
	reuses := 0
	for i := 0; i < workers; i++ {
		if errs[i] != nil {
			t.Fatalf("worker %d failed: %v", i, errs[i])
		}
		if results[i].Allocated == 4 {
			allocations++
		} else if results[i].Allocated == 0 {
			reuses++
		} else {
			t.Fatalf("worker %d returned an unexpected allocation: %+v", i, results[i])
		}
	}
	if allocations != 1 || reuses != workers-1 {
		t.Fatalf("want one allocation and %d reuses, got %d/%d", workers-1, allocations, reuses)
	}
	if u := mustUsage(t, g); u.Used != 4 {
		t.Fatalf("concurrent bakes must occupy one run, got %+v", u)
	}
	if stats := g.Stats(); stats.CacheHits != workers-1 {
		t.Fatalf("cache hits = %d, want %d", stats.CacheHits, workers-1)
	}
}

// Allocations and free cells must survive a restart.
func TestSnapshotRestorePreservesAtlas(t *testing.T) {
	g := newImpostorGuard(t)
	mustBake(t, g, "oak-tree", 4, 128)
	mustBake(t, g, "bush", 2, 128)
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewImpostorGuardFromSnapshot(testImpostorConfig(), snap)
	if err != nil {
		t.Fatalf("NewImpostorGuardFromSnapshot: %v", err)
	}
	if u := mustUsage(t, restored); u.Used != 6 || u.Assets != 2 {
		t.Fatalf("restored usage = %+v, want 6 used over 2 assets", u)
	}
	again := mustBake(t, restored, "oak-tree", 4, 128)
	if again.Allocated != 0 || again.FirstCell != 0 {
		t.Fatalf("restored cache must serve the same run, got %+v", again)
	}
	fresh := mustBake(t, restored, "fern", 3, 128)
	if fresh.FirstCell != 6 || fresh.Allocated != 3 {
		t.Fatalf("fresh allocation after restore must be cell 6, got %+v", fresh)
	}
	if _, err := taskpkg.NewImpostorGuardFromSnapshot(testImpostorConfig(), []byte("oops")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot must yield ErrCorruptSnapshot, got %v", err)
	}
}
