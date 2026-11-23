package task

import (
	"errors"
	"reflect"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-c-01"
)

func testConfig() taskpkg.Config {
	return taskpkg.Config{Seed: 0xC0FFEE, MaxStreams: 8, MaxDrawsPerStream: 4096}
}

func newRig(t *testing.T) *taskpkg.DeterministicRng {
	t.Helper()
	r := taskpkg.NewDeterministicRng(testConfig())
	for _, name := range []string{"combat", "loot", "spawn"} {
		if err := r.AddStream(name); err != nil {
			t.Fatalf("AddStream(%q): %v", name, err)
		}
	}
	return r
}

func mustNext(t *testing.T, r *taskpkg.DeterministicRng, stream string) uint32 {
	t.Helper()
	v, err := r.Next(stream)
	if err != nil {
		t.Fatalf("Next(%q): %v", stream, err)
	}
	return v
}

func mustDigest(t *testing.T, r *taskpkg.DeterministicRng, stream string) uint64 {
	t.Helper()
	d, err := r.StreamDigest(stream)
	if err != nil {
		t.Fatalf("StreamDigest(%q): %v", stream, err)
	}
	return d
}

// The same seed and stream names must produce the same sequence in two rigs.
func TestSameSeedSameSequence(t *testing.T) {
	a := newRig(t)
	b := newRig(t)
	for i := 0; i < 8; i++ {
		got, want := mustNext(t, a, "combat"), mustNext(t, b, "combat")
		if got != want {
			t.Fatalf("draw %d differs: %d != %d", i, got, want)
		}
	}
}

// Interleaved draws from other streams must not shift a stream's sequence.
func TestStreamsAreIndependent(t *testing.T) {
	a := newRig(t)
	b := newRig(t)
	want := make([]uint32, 6)
	for i := range want {
		want[i] = mustNext(t, b, "combat")
	}
	got := make([]uint32, 0, 6)
	for i := 0; i < 6; i++ {
		mustNext(t, a, "loot")
		got = append(got, mustNext(t, a, "combat"))
		mustNext(t, a, "spawn")
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("combat sequence changed after other streams drew: %v != %v", got, want)
	}
}

// DrawRange must stay inside the inclusive interval and honour low == high.
func TestDrawRangeWithinBounds(t *testing.T) {
	r := newRig(t)
	for i := 0; i < 200; i++ {
		v, err := r.DrawRange("loot", 10, 20)
		if err != nil {
			t.Fatalf("DrawRange: %v", err)
		}
		if v < 10 || v > 20 {
			t.Fatalf("draw %d outside [10,20]", v)
		}
	}
	v, err := r.DrawRange("loot", 7, 7)
	if err != nil {
		t.Fatalf("DrawRange fixed point: %v", err)
	}
	if v != 7 {
		t.Fatalf("fixed point range returned %d, want 7", v)
	}
}

// An inverted interval must be rejected without consuming randomness.
func TestDrawRangeInvalid(t *testing.T) {
	r := newRig(t)
	before := mustDigest(t, r, "loot")
	if _, err := r.DrawRange("loot", 20, 10); err == nil {
		t.Fatalf("inverted range was accepted")
	} else if !errors.Is(err, taskpkg.ErrRangeInvalid) {
		t.Fatalf("inverted range error = %v, want ErrRangeInvalid", err)
	}
	if after := mustDigest(t, r, "loot"); after != before {
		t.Fatalf("rejected draw changed the stream digest")
	}
}

// Stream count must be capped by Config.MaxStreams.
func TestStreamLimit(t *testing.T) {
	r := taskpkg.NewDeterministicRng(taskpkg.Config{Seed: 5, MaxStreams: 3, MaxDrawsPerStream: 16})
	for _, name := range []string{"a", "b", "c"} {
		if err := r.AddStream(name); err != nil {
			t.Fatalf("AddStream(%q): %v", name, err)
		}
	}
	if err := r.AddStream("d"); !errors.Is(err, taskpkg.ErrStreamLimit) {
		t.Fatalf("fourth stream error = %v, want ErrStreamLimit", err)
	}
}

// Duplicate stream names must be rejected.
func TestDuplicateStream(t *testing.T) {
	r := taskpkg.NewDeterministicRng(testConfig())
	if err := r.AddStream("combat"); err != nil {
		t.Fatalf("first AddStream: %v", err)
	}
	if err := r.AddStream("combat"); !errors.Is(err, taskpkg.ErrStreamExists) {
		t.Fatalf("duplicate AddStream error = %v, want ErrStreamExists", err)
	}
}

// Per-stream draw budget must stop draws and freeze the cursor.
func TestDrawBudgetExhausted(t *testing.T) {
	r := taskpkg.NewDeterministicRng(taskpkg.Config{Seed: 11, MaxStreams: 2, MaxDrawsPerStream: 5})
	if err := r.AddStream("combat"); err != nil {
		t.Fatalf("AddStream: %v", err)
	}
	for i := 0; i < 5; i++ {
		if _, err := r.Next("combat"); err != nil {
			t.Fatalf("draw %d: %v", i, err)
		}
	}
	if _, err := r.Next("combat"); !errors.Is(err, taskpkg.ErrDrawBudget) {
		t.Fatalf("sixth draw error = %v, want ErrDrawBudget", err)
	}
	consumed, err := r.Consumed("combat")
	if err != nil {
		t.Fatalf("Consumed: %v", err)
	}
	if consumed != 5 {
		t.Fatalf("cursor moved past the budget: %d", consumed)
	}
}

// Unknown stream names must be reported by every draw accessor.
func TestUnknownStream(t *testing.T) {
	r := newRig(t)
	if _, err := r.Next("ghost"); !errors.Is(err, taskpkg.ErrUnknownStream) {
		t.Fatalf("Next error = %v, want ErrUnknownStream", err)
	}
	if _, err := r.DrawRange("ghost", 0, 10); !errors.Is(err, taskpkg.ErrUnknownStream) {
		t.Fatalf("DrawRange error = %v, want ErrUnknownStream", err)
	}
	if _, err := r.Consumed("ghost"); !errors.Is(err, taskpkg.ErrUnknownStream) {
		t.Fatalf("Consumed error = %v, want ErrUnknownStream", err)
	}
}

// Restoring a checkpoint must replay the same values and digests.
func TestCheckpointRestoreReplays(t *testing.T) {
	r := newRig(t)
	for i := 0; i < 3; i++ {
		mustNext(t, r, "combat")
	}
	cp, err := r.Checkpoint()
	if err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	digestBefore := mustDigest(t, r, "combat")
	recorded := make([]uint32, 4)
	for i := range recorded {
		recorded[i] = mustNext(t, r, "combat")
	}
	if err := r.Restore(cp); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	consumed, err := r.Consumed("combat")
	if err != nil {
		t.Fatalf("Consumed: %v", err)
	}
	if consumed != 3 {
		t.Fatalf("cursor after restore = %d, want 3", consumed)
	}
	if got := mustDigest(t, r, "combat"); got != digestBefore {
		t.Fatalf("digest after restore = %d, want %d", got, digestBefore)
	}
	for i := range recorded {
		if got := mustNext(t, r, "combat"); got != recorded[i] {
			t.Fatalf("redraw %d = %d, want %d", i, got, recorded[i])
		}
	}
}

// A checkpoint may only be restored once.
func TestStaleCheckpointRejected(t *testing.T) {
	r := newRig(t)
	mustNext(t, r, "combat")
	cp, err := r.Checkpoint()
	if err != nil {
		t.Fatalf("Checkpoint: %v", err)
	}
	if err := r.Restore(cp); err != nil {
		t.Fatalf("first Restore: %v", err)
	}
	if err := r.Restore(cp); !errors.Is(err, taskpkg.ErrStaleCheckpoint) {
		t.Fatalf("second Restore error = %v, want ErrStaleCheckpoint", err)
	}
}

// Fork must derive a deterministic stream without advancing its parent.
func TestForkDeterministicAndNonAdvancing(t *testing.T) {
	a := newRig(t)
	b := newRig(t)
	for _, r := range []*taskpkg.DeterministicRng{a, b} {
		for i := 0; i < 3; i++ {
			mustNext(t, r, "combat")
		}
		if err := r.Fork("combat", "forked"); err != nil {
			t.Fatalf("Fork: %v", err)
		}
	}
	consumed, err := a.Consumed("combat")
	if err != nil {
		t.Fatalf("Consumed(combat): %v", err)
	}
	if consumed != 3 {
		t.Fatalf("Fork advanced the parent cursor to %d", consumed)
	}
	forkedConsumed, err := a.Consumed("forked")
	if err != nil {
		t.Fatalf("Consumed(forked): %v", err)
	}
	if forkedConsumed != 0 {
		t.Fatalf("forked stream starts at %d draws, want 0", forkedConsumed)
	}
	for i := 0; i < 5; i++ {
		got, want := mustNext(t, a, "forked"), mustNext(t, b, "forked")
		if got != want {
			t.Fatalf("forked draw %d differs: %d != %d", i, got, want)
		}
	}
}

// Fork must reject an existing target and an unknown source.
func TestForkErrors(t *testing.T) {
	r := newRig(t)
	if err := r.Fork("combat", "loot"); !errors.Is(err, taskpkg.ErrStreamExists) {
		t.Fatalf("fork onto existing stream error = %v, want ErrStreamExists", err)
	}
	if err := r.Fork("ghost", "derived"); !errors.Is(err, taskpkg.ErrUnknownStream) {
		t.Fatalf("fork from unknown stream error = %v, want ErrUnknownStream", err)
	}
}

// ResetStream must restore the values and digest of a fresh stream.
func TestResetStream(t *testing.T) {
	r := newRig(t)
	fresh := newRig(t)
	for i := 0; i < 4; i++ {
		mustNext(t, r, "combat")
	}
	if err := r.ResetStream("combat"); err != nil {
		t.Fatalf("ResetStream: %v", err)
	}
	consumed, err := r.Consumed("combat")
	if err != nil {
		t.Fatalf("Consumed: %v", err)
	}
	if consumed != 0 {
		t.Fatalf("cursor after reset = %d, want 0", consumed)
	}
	if got, want := mustDigest(t, r, "combat"), mustDigest(t, fresh, "combat"); got != want {
		t.Fatalf("digest after reset = %d, want fresh digest %d", got, want)
	}
	for i := 0; i < 3; i++ {
		got, want := mustNext(t, r, "combat"), mustNext(t, fresh, "combat")
		if got != want {
			t.Fatalf("post-reset draw %d = %d, want %d", i, got, want)
		}
	}
}

// A snapshot restored in a restarted process must continue the same sequence.
func TestSnapshotRestoreContinues(t *testing.T) {
	r := newRig(t)
	for i := 0; i < 5; i++ {
		mustNext(t, r, "combat")
	}
	snap, err := r.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewDeterministicRngFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewDeterministicRngFromSnapshot: %v", err)
	}
	consumed, err := restored.Consumed("combat")
	if err != nil {
		t.Fatalf("Consumed on restored rig: %v", err)
	}
	if consumed != 5 {
		t.Fatalf("restored cursor = %d, want 5", consumed)
	}
	for i := 0; i < 6; i++ {
		got, want := mustNext(t, restored, "combat"), mustNext(t, r, "combat")
		if got != want {
			t.Fatalf("restored draw %d = %d, want %d", i, got, want)
		}
	}
}

// Concurrent draws on distinct streams must stay identical to sequential work.
func TestConcurrentDistinctStreams(t *testing.T) {
	names := []string{"s0", "s1", "s2", "s3", "s4", "s5"}
	cfg := taskpkg.Config{Seed: 0xDEADBEEF, MaxStreams: 16, MaxDrawsPerStream: 4096}
	concurrent := taskpkg.NewDeterministicRng(cfg)
	sequential := taskpkg.NewDeterministicRng(cfg)
	for _, name := range names {
		if err := concurrent.AddStream(name); err != nil {
			t.Fatalf("AddStream(%q): %v", name, err)
		}
		if err := sequential.AddStream(name); err != nil {
			t.Fatalf("AddStream(%q) sequential: %v", name, err)
		}
	}
	var wg sync.WaitGroup
	errCh := make(chan error, len(names))
	for _, name := range names {
		name := name
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 40; i++ {
				if _, err := concurrent.Next(name); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatalf("concurrent Next: %v", err)
	}
	for _, name := range names {
		for i := 0; i < 40; i++ {
			if _, err := sequential.Next(name); err != nil {
				t.Fatalf("sequential Next: %v", err)
			}
		}
		got, err := concurrent.StreamDigest(name)
		if err != nil {
			t.Fatalf("StreamDigest(%q): %v", name, err)
		}
		want, err := sequential.StreamDigest(name)
		if err != nil {
			t.Fatalf("StreamDigest(%q) sequential: %v", name, err)
		}
		if got != want {
			t.Fatalf("concurrent digest of %q = %d, want %d", name, got, want)
		}
		consumed, err := concurrent.Consumed(name)
		if err != nil {
			t.Fatalf("Consumed(%q): %v", name, err)
		}
		if consumed != 40 {
			t.Fatalf("Consumed(%q) = %d, want 40", name, consumed)
		}
	}
}
