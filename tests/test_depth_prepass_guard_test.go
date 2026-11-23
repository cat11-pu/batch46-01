package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-f-03"
)

func testDepthConfig() taskpkg.DepthConfig {
	return taskpkg.DepthConfig{MaxDrawCalls: 10}
}

func newPrepass(t *testing.T) *taskpkg.DepthPrepass {
	t.Helper()
	g, err := taskpkg.NewDepthPrepass(testDepthConfig())
	if err != nil {
		t.Fatalf("NewDepthPrepass: %v", err)
	}
	return g
}

func batch(id string, depth float32, indices int) taskpkg.DrawBatch {
	return taskpkg.DrawBatch{ID: id, Depth: depth, IndexCount: indices, CastsDepth: true}
}

func mustAddBatch(t *testing.T, g *taskpkg.DepthPrepass, b taskpkg.DrawBatch) {
	t.Helper()
	if err := g.AddBatch(b); err != nil {
		t.Fatalf("AddBatch(%q): %v", b.ID, err)
	}
}

func planIDs(t *testing.T, g *taskpkg.DepthPrepass) []string {
	t.Helper()
	plan, err := g.Plan()
	if err != nil {
		t.Fatalf("Plan: %v", err)
	}
	ids := make([]string, len(plan))
	for i, b := range plan {
		ids[i] = b.ID
	}
	return ids
}

func mustSubmit(t *testing.T, g *taskpkg.DepthPrepass, ids ...string) taskpkg.SubmitResult {
	t.Helper()
	res, err := g.Submit(ids)
	if err != nil {
		t.Fatalf("Submit(%v): %v", ids, err)
	}
	return res
}

// The same batch id must not be registered twice.
func TestDuplicateBatchRejected(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("wall-01", 0.30, 300))
	err := g.AddBatch(batch("wall-01", 0.10, 900))
	if !errors.Is(err, taskpkg.ErrBatchExists) {
		t.Fatalf("duplicate batch must yield ErrBatchExists, got %v", err)
	}
	stored, err := g.Batch("wall-01")
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	if stored.Depth != 0.30 || stored.IndexCount != 300 {
		t.Fatalf("duplicate registration overwrote the batch: %+v", stored)
	}
	if stats := g.Stats(); stats.Batches != 1 {
		t.Fatalf("want one batch, got %+v", stats)
	}
}

// Depth must be a finite value in [0, 1].
func TestInvalidDepthRejected(t *testing.T) {
	g := newPrepass(t)
	for _, depth := range []float32{-0.01, 1.01, float32(math.NaN())} {
		if err := g.AddBatch(batch("bad", depth, 300)); !errors.Is(err, taskpkg.ErrInvalidDepth) {
			t.Fatalf("depth %v must yield ErrInvalidDepth, got %v", depth, err)
		}
	}
	if _, err := g.Batch("bad"); !errors.Is(err, taskpkg.ErrBatchUnknown) {
		t.Fatalf("rejected batch must not be stored, got %v", err)
	}
}

// The plan is front to back with an ID tie-break.
func TestPlanSortsFrontToBack(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("z-far", 0.90, 300))
	mustAddBatch(t, g, batch("b-near", 0.20, 300))
	mustAddBatch(t, g, batch("mid", 0.50, 300))
	mustAddBatch(t, g, batch("a-near", 0.20, 300))
	got := planIDs(t, g)
	want := []string{"a-near", "b-near", "mid", "z-far"}
	if len(got) != len(want) {
		t.Fatalf("plan has %d batches, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("plan order = %v, want %v", got, want)
		}
	}
	if stats := g.Stats(); stats.PlanDraws != 4 {
		t.Fatalf("plan draw count = %d, want 4", stats.PlanDraws)
	}
}

// Batches that do not write depth and occluded batches are skipped.
func TestPlanExcludesNonCasters(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("crate", 0.20, 300))
	noDepth := batch("decal", 0.25, 300)
	noDepth.CastsDepth = false
	mustAddBatch(t, g, noDepth)
	mustAddBatch(t, g, batch("facade", 0.40, 300))
	if err := g.SetOccluded("facade", true); err != nil {
		t.Fatalf("SetOccluded: %v", err)
	}
	got := planIDs(t, g)
	if len(got) != 1 || got[0] != "crate" {
		t.Fatalf("plan must contain only the caster crate, got %v", got)
	}
	if stats := g.Stats(); stats.PlanDraws != 1 {
		t.Fatalf("plan draw count = %d, want 1", stats.PlanDraws)
	}
}

// The draw call budget is checked before anything is admitted.
func TestSubmitBudgetEnforced(t *testing.T) {
	g, err := taskpkg.NewDepthPrepass(taskpkg.DepthConfig{MaxDrawCalls: 2})
	if err != nil {
		t.Fatalf("NewDepthPrepass: %v", err)
	}
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	mustAddBatch(t, g, batch("b2", 0.20, 300))
	mustAddBatch(t, g, batch("b3", 0.30, 300))
	if _, err := g.Submit([]string{"b1", "b2", "b3"}); !errors.Is(err, taskpkg.ErrDrawBudget) {
		t.Fatalf("three draws over a budget of two must yield ErrDrawBudget, got %v", err)
	}
	if got := g.FrameDraws(); got != 0 {
		t.Fatalf("rejected submission must admit nothing, got %d draws", got)
	}
	if stats := g.Stats(); stats.SubmittedTriangles != 0 {
		t.Fatalf("rejected submission must count no triangles: %+v", stats)
	}
}

// Submitting an unknown id rejects the whole call.
func TestSubmitUnknownBatchRejected(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	if _, err := g.Submit([]string{"b1", "ghost"}); !errors.Is(err, taskpkg.ErrBatchUnknown) {
		t.Fatalf("unknown batch must yield ErrBatchUnknown, got %v", err)
	}
	if got := g.FrameDraws(); got != 0 {
		t.Fatalf("rejected submission must admit nothing, got %d draws", got)
	}
}

// Batches must be submitted in plan order.
func TestSubmitOutOfOrderRejected(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("near", 0.10, 300))
	mustAddBatch(t, g, batch("far", 0.80, 300))
	if _, err := g.Submit([]string{"far", "near"}); !errors.Is(err, taskpkg.ErrOrderViolation) {
		t.Fatalf("back to front submission must yield ErrOrderViolation, got %v", err)
	}
	if got := g.FrameDraws(); got != 0 {
		t.Fatalf("rejected submission must admit nothing, got %d draws", got)
	}
}

// A single call must not repeat an id.
func TestSubmitDuplicateIDsRejected(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	if _, err := g.Submit([]string{"b1", "b1"}); !errors.Is(err, taskpkg.ErrDuplicateBatch) {
		t.Fatalf("duplicate id in one submission must yield ErrDuplicateBatch, got %v", err)
	}
	if got := g.FrameDraws(); got != 0 {
		t.Fatalf("rejected submission must admit nothing, got %d draws", got)
	}
}

// Several submissions in one frame accumulate draw calls and triangles.
func TestSubmitAccumulatesAcrossCalls(t *testing.T) {
	g, err := taskpkg.NewDepthPrepass(taskpkg.DepthConfig{MaxDrawCalls: 3})
	if err != nil {
		t.Fatalf("NewDepthPrepass: %v", err)
	}
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	mustAddBatch(t, g, batch("b2", 0.20, 600))
	mustAddBatch(t, g, batch("b3", 0.30, 900))
	first := mustSubmit(t, g, "b1")
	if first.DrawCalls != 1 || first.Triangles != 100 {
		t.Fatalf("unexpected first result: %+v", first)
	}
	second := mustSubmit(t, g, "b2", "b3")
	if second.DrawCalls != 2 || second.Triangles != 500 {
		t.Fatalf("unexpected second result: %+v", second)
	}
	if got := g.FrameDraws(); got != 3 {
		t.Fatalf("frame draw count = %d, want 3", got)
	}
	if stats := g.Stats(); stats.SubmittedTriangles != 600 {
		t.Fatalf("submitted triangles = %d, want 600", stats.SubmittedTriangles)
	}
}

// Re-submitting a batch already admitted this frame is rejected.
func TestSubmitTwiceSameBatchRejected(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	mustSubmit(t, g, "b1")
	if _, err := g.Submit([]string{"b1"}); !errors.Is(err, taskpkg.ErrAlreadySubmitted) {
		t.Fatalf("second submission must yield ErrAlreadySubmitted, got %v", err)
	}
	if got := g.FrameDraws(); got != 1 {
		t.Fatalf("rejected resubmission must not add draws, got %d", got)
	}
}

// BeginFrame clears the submission set so batches can be drawn again.
func TestBeginFrameResetsSubmission(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	mustSubmit(t, g, "b1")
	if err := g.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame: %v", err)
	}
	if got := g.FrameDraws(); got != 0 {
		t.Fatalf("BeginFrame must reset the frame counter, got %d", got)
	}
	res := mustSubmit(t, g, "b1")
	if res.DrawCalls != 1 {
		t.Fatalf("batch must be submittable after BeginFrame, got %+v", res)
	}
}

// An occluded batch cannot be submitted and disappears from the plan.
func TestOccludedBatchRejected(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	mustAddBatch(t, g, batch("b2", 0.20, 300))
	if err := g.SetOccluded("b1", true); err != nil {
		t.Fatalf("SetOccluded: %v", err)
	}
	got := planIDs(t, g)
	if len(got) != 1 || got[0] != "b2" {
		t.Fatalf("occluded batch must leave the plan, got %v", got)
	}
	if _, err := g.Submit([]string{"b1"}); !errors.Is(err, taskpkg.ErrBatchOccluded) {
		t.Fatalf("submitting an occluded batch must yield ErrBatchOccluded, got %v", err)
	}
	if err := g.SetOccluded("b1", false); err != nil {
		t.Fatalf("SetOccluded clear: %v", err)
	}
	got = planIDs(t, g)
	if len(got) != 2 || got[0] != "b1" || got[1] != "b2" {
		t.Fatalf("clearing occlusion must restore the batch, got %v", got)
	}
}

// Removing a batch updates the plan and later calls fail cleanly.
func TestRemoveBatchUpdatesPlan(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	mustAddBatch(t, g, batch("b2", 0.20, 300))
	if err := g.RemoveBatch("b1"); err != nil {
		t.Fatalf("RemoveBatch: %v", err)
	}
	got := planIDs(t, g)
	if len(got) != 1 || got[0] != "b2" {
		t.Fatalf("removed batch must leave the plan, got %v", got)
	}
	if _, err := g.Batch("b1"); !errors.Is(err, taskpkg.ErrBatchUnknown) {
		t.Fatalf("removed batch must be unknown, got %v", err)
	}
	if err := g.RemoveBatch("b1"); !errors.Is(err, taskpkg.ErrBatchUnknown) {
		t.Fatalf("removing twice must yield ErrBatchUnknown, got %v", err)
	}
}

// Batches and frame state must survive a restart.
func TestSnapshotRestorePreservesState(t *testing.T) {
	g := newPrepass(t)
	mustAddBatch(t, g, batch("b1", 0.10, 300))
	mustAddBatch(t, g, batch("b2", 0.20, 600))
	mustSubmit(t, g, "b1")
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewDepthPrepassFromSnapshot(testDepthConfig(), snap)
	if err != nil {
		t.Fatalf("NewDepthPrepassFromSnapshot: %v", err)
	}
	got := planIDs(t, restored)
	if len(got) != 2 || got[0] != "b1" || got[1] != "b2" {
		t.Fatalf("restored plan = %v, want [b1 b2]", got)
	}
	if draws := restored.FrameDraws(); draws != 1 {
		t.Fatalf("restored frame draws = %d, want 1", draws)
	}
	if _, err := restored.Submit([]string{"b1"}); !errors.Is(err, taskpkg.ErrAlreadySubmitted) {
		t.Fatalf("restored submission set must reject b1 again, got %v", err)
	}
	if res := mustSubmit(t, restored, "b2"); res.DrawCalls != 1 {
		t.Fatalf("b2 must still be submittable after restore, got %+v", res)
	}
	if _, err := taskpkg.NewDepthPrepassFromSnapshot(testDepthConfig(), []byte("broken")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot must yield ErrCorruptSnapshot, got %v", err)
	}
}

// The draw call budget must hold under concurrent submissions.
func TestConcurrentSubmitBudgetRespected(t *testing.T) {
	g, err := taskpkg.NewDepthPrepass(taskpkg.DepthConfig{MaxDrawCalls: 3})
	if err != nil {
		t.Fatalf("NewDepthPrepass: %v", err)
	}
	const workers = 8
	for i := 0; i < workers; i++ {
		mustAddBatch(t, g, batch("c"+string(rune('0'+i)), 0.10+float32(i)*0.01, 300))
	}
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = g.Submit([]string{"c" + string(rune('0'+idx))})
		}(i)
	}
	wg.Wait()
	accepted := 0
	rejected := 0
	for i := 0; i < workers; i++ {
		switch {
		case errs[i] == nil:
			accepted++
		case errors.Is(errs[i], taskpkg.ErrDrawBudget):
			rejected++
		default:
			t.Fatalf("unexpected error from worker %d: %v", i, errs[i])
		}
	}
	if accepted != 3 || rejected != 5 {
		t.Fatalf("want exactly 3 accepted and 5 rejected draws, got %d/%d", accepted, rejected)
	}
	if got := g.FrameDraws(); got != 3 {
		t.Fatalf("frame draw count = %d, want 3", got)
	}
}
