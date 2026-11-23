package task

import (
	"errors"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-e-04"
)

func mustPool(t *testing.T, frames, setsPerFrame int, perFrame taskpkg.DescCounts) *taskpkg.DescriptorPool {
	t.Helper()
	p, err := taskpkg.NewDescriptorPool(frames, setsPerFrame, perFrame)
	if err != nil {
		t.Fatalf("NewDescriptorPool(%d, %d, %+v): %v", frames, setsPerFrame, perFrame, err)
	}
	return p
}

func want(u, s, sa, i int) taskpkg.DescCounts {
	return taskpkg.DescCounts{Uniforms: u, Storages: s, Samplers: sa, Images: i}
}

func setCount(t *testing.T, p *taskpkg.DescriptorPool, frame int) int {
	t.Helper()
	n, err := p.SetCount(frame)
	if err != nil {
		t.Fatalf("SetCount(%d): %v", frame, err)
	}
	return n
}

// Sets are numbered from one inside each frame and charge the frame budget.
func TestAllocSetAssignsLowestSlot(t *testing.T) {
	p := mustPool(t, 2, 4, want(8, 4, 4, 4))
	first, err := p.AllocSet(0, want(1, 0, 0, 0))
	if err != nil {
		t.Fatalf("AllocSet #1: %v", err)
	}
	if first != "f0-1" {
		t.Fatalf("first set id = %q, want f0-1", first)
	}
	second, err := p.AllocSet(0, want(1, 0, 0, 0))
	if err != nil {
		t.Fatalf("AllocSet #2: %v", err)
	}
	if second != "f0-2" {
		t.Fatalf("second set id = %q, want f0-2", second)
	}
	if setCount(t, p, 0) != 2 || p.ActiveSets() != 2 {
		t.Fatalf("set count/active = %d/%d, want 2/2", setCount(t, p, 0), p.ActiveSets())
	}
	usage, err := p.Usage(0)
	if err != nil {
		t.Fatalf("Usage(0): %v", err)
	}
	if usage != want(2, 0, 0, 0) {
		t.Fatalf("Usage(0) = %+v, want 2 uniforms", usage)
	}
}

// The per-frame set count is a hard cap and every rejection is a fault.
func TestSetBudgetExhausted(t *testing.T) {
	p := mustPool(t, 2, 2, want(8, 4, 4, 4))
	for i := 0; i < 2; i++ {
		if _, err := p.AllocSet(0, want(0, 1, 0, 0)); err != nil {
			t.Fatalf("AllocSet #%d: %v", i, err)
		}
	}
	if _, err := p.AllocSet(0, want(0, 1, 0, 0)); !errors.Is(err, taskpkg.ErrOutOfSets) {
		t.Fatalf("third AllocSet error = %v, want ErrOutOfSets", err)
	}
	if setCount(t, p, 0) != 2 || p.Faults() != 1 {
		t.Fatalf("set count/faults = %d/%d, want 2/1", setCount(t, p, 0), p.Faults())
	}
}

// A set that needs more uniform buffers than the frame budget is refused
// before any counter is charged.
func TestUniformBudgetExhausted(t *testing.T) {
	p := mustPool(t, 2, 4, want(4, 0, 0, 0))
	if _, err := p.AllocSet(0, want(5, 0, 0, 0)); !errors.Is(err, taskpkg.ErrOutOfUniforms) {
		t.Fatalf("AllocSet with 5 uniforms error = %v, want ErrOutOfUniforms", err)
	}
	usage, err := p.Usage(0)
	if err != nil {
		t.Fatalf("Usage(0): %v", err)
	}
	if usage != want(0, 0, 0, 0) || setCount(t, p, 0) != 0 || p.ActiveSets() != 0 {
		t.Fatalf("pool charged after a rejected allocation: usage %+v count %d", usage, setCount(t, p, 0))
	}
	if p.Faults() != 1 {
		t.Fatalf("Faults = %d, want 1", p.Faults())
	}
}

// Each descriptor type has its own budget; exhausting one does not consume
// the others.
func TestPerTypeBudgetsIndependent(t *testing.T) {
	p := mustPool(t, 2, 4, want(2, 1, 1, 1))
	if _, err := p.AllocSet(0, want(2, 0, 0, 0)); err != nil {
		t.Fatalf("AllocSet for uniforms: %v", err)
	}
	if _, err := p.AllocSet(0, want(0, 0, 2, 0)); !errors.Is(err, taskpkg.ErrOutOfSamplers) {
		t.Fatalf("sampler overflow error = %v, want ErrOutOfSamplers", err)
	}
	if _, err := p.AllocSet(0, want(0, 2, 0, 0)); !errors.Is(err, taskpkg.ErrOutOfStorages) {
		t.Fatalf("storage overflow error = %v, want ErrOutOfStorages", err)
	}
	if _, err := p.AllocSet(0, want(0, 0, 0, 2)); !errors.Is(err, taskpkg.ErrOutOfImages) {
		t.Fatalf("image overflow error = %v, want ErrOutOfImages", err)
	}
	if _, err := p.AllocSet(0, want(0, 0, 0, 1)); err != nil {
		t.Fatalf("AllocSet for one image: %v", err)
	}
	usage, err := p.Usage(0)
	if err != nil {
		t.Fatalf("Usage(0): %v", err)
	}
	if usage != want(2, 0, 0, 1) {
		t.Fatalf("Usage(0) = %+v, want 2 uniforms and 1 image", usage)
	}
	if p.Faults() != 3 {
		t.Fatalf("Faults = %d, want 3", p.Faults())
	}
}

// Negative demands are caller bugs, not budget faults.
func TestNegativeNeedRejected(t *testing.T) {
	p := mustPool(t, 2, 4, want(4, 4, 4, 4))
	if _, err := p.AllocSet(0, want(-1, 0, 0, 0)); !errors.Is(err, taskpkg.ErrNegativeCount) {
		t.Fatalf("negative uniform demand error = %v, want ErrNegativeCount", err)
	}
	if setCount(t, p, 0) != 0 || p.Faults() != 0 {
		t.Fatalf("pool charged or faulted by a negative demand: count %d faults %d", setCount(t, p, 0), p.Faults())
	}
}

// Every frame-indexed method rejects indices outside the ring.
func TestBadFrameRejected(t *testing.T) {
	p := mustPool(t, 2, 4, want(4, 4, 4, 4))
	if _, err := p.AllocSet(-1, want(0, 0, 0, 0)); !errors.Is(err, taskpkg.ErrBadFrame) {
		t.Fatalf("AllocSet(-1) error = %v, want ErrBadFrame", err)
	}
	if _, err := p.AllocSet(2, want(0, 0, 0, 0)); !errors.Is(err, taskpkg.ErrBadFrame) {
		t.Fatalf("AllocSet(2) error = %v, want ErrBadFrame", err)
	}
	if _, err := p.Usage(5); !errors.Is(err, taskpkg.ErrBadFrame) {
		t.Fatalf("Usage(5) error = %v, want ErrBadFrame", err)
	}
	if _, err := p.SetCount(-2); !errors.Is(err, taskpkg.ErrBadFrame) {
		t.Fatalf("SetCount(-2) error = %v, want ErrBadFrame", err)
	}
}

// Freeing an unknown set and freeing one twice are distinct failures.
func TestFreeSetErrors(t *testing.T) {
	p := mustPool(t, 2, 4, want(4, 4, 4, 4))
	if err := p.FreeSet("f0-9"); !errors.Is(err, taskpkg.ErrUnknownSet) {
		t.Fatalf("FreeSet(unknown) error = %v, want ErrUnknownSet", err)
	}
	id, err := p.AllocSet(0, want(1, 0, 0, 0))
	if err != nil {
		t.Fatalf("AllocSet: %v", err)
	}
	if err := p.FreeSet(id); err != nil {
		t.Fatalf("first FreeSet: %v", err)
	}
	if err := p.FreeSet(id); !errors.Is(err, taskpkg.ErrDoubleFree) {
		t.Fatalf("second FreeSet error = %v, want ErrDoubleFree", err)
	}
	if _, err := p.Describe("f7-1"); !errors.Is(err, taskpkg.ErrUnknownSet) {
		t.Fatalf("Describe(unknown) error = %v, want ErrUnknownSet", err)
	}
}

// Freeing a set does not release its descriptors until the frame is reset,
// and the freed slot is reused after the reset.
func TestFreedSetKeepsBudgetUntilReset(t *testing.T) {
	p := mustPool(t, 2, 4, want(4, 4, 4, 4))
	id, err := p.AllocSet(0, want(1, 1, 0, 0))
	if err != nil {
		t.Fatalf("AllocSet: %v", err)
	}
	if err := p.FreeSet(id); err != nil {
		t.Fatalf("FreeSet: %v", err)
	}
	usage, err := p.Usage(0)
	if err != nil {
		t.Fatalf("Usage(0): %v", err)
	}
	if usage != want(1, 1, 0, 0) {
		t.Fatalf("Usage after free = %+v, want the descriptors still charged", usage)
	}
	if pending, err := p.FreedPending(0); err != nil || pending != 1 {
		t.Fatalf("FreedPending(0) = %d, %v, want 1, nil", pending, err)
	}
	if err := p.ResetFrame(0); err != nil {
		t.Fatalf("ResetFrame(0): %v", err)
	}
	if usage, err := p.Usage(0); err != nil || usage != want(0, 0, 0, 0) {
		t.Fatalf("Usage after reset = %+v, %v, want zero", usage, err)
	}
	if setCount(t, p, 0) != 0 || p.ActiveSets() != 0 {
		t.Fatalf("set count/active after reset = %d/%d, want 0/0", setCount(t, p, 0), p.ActiveSets())
	}
	again, err := p.AllocSet(0, want(1, 1, 0, 0))
	if err != nil {
		t.Fatalf("AllocSet after reset: %v", err)
	}
	if again != "f0-1" {
		t.Fatalf("set id after reset = %q, want f0-1", again)
	}
	if pending, err := p.FreedPending(0); err != nil || pending != 0 {
		t.Fatalf("FreedPending after reset = %d, %v, want 0, nil", pending, err)
	}
}

// A frame with a live set cannot be reset, and the whole frame keeps its
// allocations until the set is freed.
func TestResetFrameBusy(t *testing.T) {
	p := mustPool(t, 2, 4, want(4, 4, 4, 4))
	id, err := p.AllocSet(0, want(1, 0, 0, 0))
	if err != nil {
		t.Fatalf("AllocSet: %v", err)
	}
	if err := p.ResetFrame(0); !errors.Is(err, taskpkg.ErrFrameBusy) {
		t.Fatalf("ResetFrame with a live set error = %v, want ErrFrameBusy", err)
	}
	if setCount(t, p, 0) != 1 || p.ActiveSets() != 1 {
		t.Fatalf("set count/active after a rejected reset = %d/%d, want 1/1", setCount(t, p, 0), p.ActiveSets())
	}
	if err := p.FreeSet(id); err != nil {
		t.Fatalf("FreeSet: %v", err)
	}
	if err := p.ResetFrame(0); err != nil {
		t.Fatalf("ResetFrame after freeing: %v", err)
	}
}

// Frames must be reset in ring order; a reset that jumps ahead is refused.
func TestResetFrameOrder(t *testing.T) {
	p := mustPool(t, 2, 4, want(4, 4, 4, 4))
	if err := p.ResetFrame(1); !errors.Is(err, taskpkg.ErrFrameOrder) {
		t.Fatalf("first ResetFrame(1) error = %v, want ErrFrameOrder", err)
	}
	if err := p.ResetFrame(0); err != nil {
		t.Fatalf("ResetFrame(0): %v", err)
	}
	if err := p.ResetFrame(0); !errors.Is(err, taskpkg.ErrFrameOrder) {
		t.Fatalf("second ResetFrame(0) error = %v, want ErrFrameOrder", err)
	}
	if err := p.ResetFrame(1); err != nil {
		t.Fatalf("ResetFrame(1) after frame 0: %v", err)
	}
}

// Each frame owns an independent set and descriptor budget.
func TestFramesAreIsolated(t *testing.T) {
	p := mustPool(t, 2, 1, want(1, 1, 1, 1))
	if _, err := p.AllocSet(0, want(1, 0, 0, 0)); err != nil {
		t.Fatalf("AllocSet frame 0: %v", err)
	}
	if _, err := p.AllocSet(0, want(0, 0, 0, 0)); !errors.Is(err, taskpkg.ErrOutOfSets) {
		t.Fatalf("second set in frame 0 error = %v, want ErrOutOfSets", err)
	}
	if _, err := p.AllocSet(1, want(1, 0, 0, 0)); err != nil {
		t.Fatalf("AllocSet frame 1: %v", err)
	}
	for frame, expected := range map[int]taskpkg.DescCounts{0: want(1, 0, 0, 0), 1: want(1, 0, 0, 0)} {
		usage, err := p.Usage(frame)
		if err != nil {
			t.Fatalf("Usage(%d): %v", frame, err)
		}
		if usage != expected {
			t.Fatalf("Usage(%d) = %+v, want %+v", frame, usage, expected)
		}
	}
}

// A snapshot restores allocations, freed flags and the reset sequence, so the
// pool continues exactly where it stopped.
func TestSnapshotRestoresPool(t *testing.T) {
	p := mustPool(t, 2, 3, want(2, 2, 2, 2))
	first, err := p.AllocSet(0, want(1, 0, 0, 0))
	if err != nil {
		t.Fatalf("AllocSet frame 0: %v", err)
	}
	if err := p.FreeSet(first); err != nil {
		t.Fatalf("FreeSet: %v", err)
	}
	second, err := p.AllocSet(1, want(0, 1, 0, 0))
	if err != nil {
		t.Fatalf("AllocSet frame 1: %v", err)
	}
	blob, err := p.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewDescriptorPoolFromSnapshot(2, 3, want(2, 2, 2, 2), blob)
	if err != nil {
		t.Fatalf("NewDescriptorPoolFromSnapshot: %v", err)
	}
	if restored.ActiveSets() != 2 || setCount(t, restored, 0) != 1 || setCount(t, restored, 1) != 1 {
		t.Fatalf("restored active/frame counts = %d/%d/%d, want 2/1/1", restored.ActiveSets(), setCount(t, restored, 0), setCount(t, restored, 1))
	}
	if pending, err := restored.FreedPending(0); err != nil || pending != 1 {
		t.Fatalf("restored FreedPending(0) = %d, %v, want 1, nil", pending, err)
	}
	info, err := restored.Describe(first)
	if err != nil {
		t.Fatalf("restored Describe(%q): %v", first, err)
	}
	if !info.Freed || info.Frame != 0 || info.Need != want(1, 0, 0, 0) {
		t.Fatalf("restored set record = %+v, want freed set in frame 0", info)
	}
	usage, err := restored.Usage(0)
	if err != nil || usage != want(1, 0, 0, 0) {
		t.Fatalf("restored Usage(0) = %+v, %v, want one uniform", usage, err)
	}
	if err := restored.ResetFrame(0); err != nil {
		t.Fatalf("restored ResetFrame(0): %v", err)
	}
	if usage, err := restored.Usage(0); err != nil || usage != want(0, 0, 0, 0) {
		t.Fatalf("Usage after restored reset = %+v, %v, want zero", usage, err)
	}
	if err := restored.ResetFrame(1); !errors.Is(err, taskpkg.ErrFrameBusy) {
		t.Fatalf("restored ResetFrame(1) with a live set error = %v, want ErrFrameBusy", err)
	}
	if _, err := restored.Describe(second); err != nil {
		t.Fatalf("restored Describe(%q): %v", second, err)
	}
}

// Restoring a snapshot into a differently configured pool is refused.
func TestSnapshotConfigMismatch(t *testing.T) {
	p := mustPool(t, 2, 2, want(1, 1, 1, 1))
	if _, err := p.AllocSet(0, want(1, 0, 0, 0)); err != nil {
		t.Fatalf("AllocSet: %v", err)
	}
	blob, err := p.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if _, err := taskpkg.NewDescriptorPoolFromSnapshot(3, 2, want(1, 1, 1, 1), blob); !errors.Is(err, taskpkg.ErrConfigMismatch) {
		t.Fatalf("restore with a different frame count error = %v, want ErrConfigMismatch", err)
	}
	if _, err := taskpkg.NewDescriptorPoolFromSnapshot(2, 3, want(1, 1, 1, 1), blob); !errors.Is(err, taskpkg.ErrConfigMismatch) {
		t.Fatalf("restore with a different set budget error = %v, want ErrConfigMismatch", err)
	}
	if _, err := taskpkg.NewDescriptorPoolFromSnapshot(2, 2, want(2, 1, 1, 1), blob); !errors.Is(err, taskpkg.ErrConfigMismatch) {
		t.Fatalf("restore with a different descriptor budget error = %v, want ErrConfigMismatch", err)
	}
}

// Only exhausted budgets count as faults; bad indices and bad counts do not.
func TestFaultsCountOnlyBudgetFailures(t *testing.T) {
	p := mustPool(t, 2, 2, want(2, 0, 0, 0))
	if _, err := p.AllocSet(0, want(1, 0, 0, 0)); err != nil {
		t.Fatalf("AllocSet #1: %v", err)
	}
	if _, err := p.AllocSet(0, want(2, 0, 0, 0)); !errors.Is(err, taskpkg.ErrOutOfUniforms) {
		t.Fatalf("uniform overflow error = %v, want ErrOutOfUniforms", err)
	}
	if _, err := p.AllocSet(0, want(0, 0, 0, 0)); err != nil {
		t.Fatalf("AllocSet #2: %v", err)
	}
	if _, err := p.AllocSet(0, want(0, 0, 0, 0)); !errors.Is(err, taskpkg.ErrOutOfSets) {
		t.Fatalf("set overflow error = %v, want ErrOutOfSets", err)
	}
	if _, err := p.AllocSet(9, want(0, 0, 0, 0)); !errors.Is(err, taskpkg.ErrBadFrame) {
		t.Fatalf("bad frame error = %v, want ErrBadFrame", err)
	}
	if _, err := p.AllocSet(0, want(0, -1, 0, 0)); !errors.Is(err, taskpkg.ErrNegativeCount) {
		t.Fatalf("negative demand error = %v, want ErrNegativeCount", err)
	}
	if p.Faults() != 2 {
		t.Fatalf("Faults = %d, want 2", p.Faults())
	}
}

// Concurrent allocations receive distinct set slots and never oversell the
// frame budget.
func TestConcurrentAllocSets(t *testing.T) {
	const workers = 30
	p := mustPool(t, 2, workers, want(workers, 0, 0, 0))
	ids := make([]string, workers)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			ids[slot], errs[slot] = p.AllocSet(0, want(1, 0, 0, 0))
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent AllocSet #%d: %v", i, err)
		}
	}
	seen := make(map[string]bool, workers)
	for _, id := range ids {
		if seen[id] {
			t.Fatalf("set id %q handed out twice: %v", id, ids)
		}
		seen[id] = true
	}
	if setCount(t, p, 0) != workers || p.ActiveSets() != workers {
		t.Fatalf("set count/active = %d/%d, want %d/%d", setCount(t, p, 0), p.ActiveSets(), workers, workers)
	}
	usage, err := p.Usage(0)
	if err != nil {
		t.Fatalf("Usage(0): %v", err)
	}
	if usage != want(workers, 0, 0, 0) {
		t.Fatalf("Usage(0) = %+v, want %d uniforms", usage, workers)
	}
	if _, err := p.AllocSet(0, want(0, 0, 0, 0)); !errors.Is(err, taskpkg.ErrOutOfSets) {
		t.Fatalf("allocation past the frame budget error = %v, want ErrOutOfSets", err)
	}
}
