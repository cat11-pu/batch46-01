package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-b-03"
)

func testConfig() taskpkg.Config {
	return taskpkg.Config{
		Delay:         0,
		MaxDelay:      time.Minute,
		MaxPending:    10,
		MaxSpectators: 10,
	}
}

func event(seq int64, payload string, at time.Time) taskpkg.Event {
	return taskpkg.Event{Seq: seq, MatchID: "m1", ServerTime: at, Kind: "kill", Payload: payload}
}

func register(t *testing.T, s *taskpkg.SpectatorDelay, matchID, spectatorID string, at time.Time) {
	t.Helper()
	if err := s.RegisterSpectator(matchID, spectatorID, at); err != nil {
		t.Fatalf("RegisterSpectator(%q, %q): %v", matchID, spectatorID, err)
	}
}

func publish(t *testing.T, s *taskpkg.SpectatorDelay, ev taskpkg.Event) {
	t.Helper()
	if err := s.Publish(ev); err != nil {
		t.Fatalf("Publish(seq %d): %v", ev.Seq, err)
	}
}

// Sequences must be positive and a reused sequence cannot carry new content.
func TestPublishRejectsInvalidSeqAndConflictingPayload(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	if err := s.Publish(event(0, "a", base)); !errors.Is(err, taskpkg.ErrInvalidSeq) {
		t.Fatalf("sequence 0 must yield ErrInvalidSeq, got %v", err)
	}
	publish(t, s, event(1, "a", base))
	if err := s.Publish(event(1, "b", base)); !errors.Is(err, taskpkg.ErrConflictingEvent) {
		t.Fatalf("same sequence with new payload must yield ErrConflictingEvent, got %v", err)
	}
}

// Re-publishing an identical event is idempotent and buffers it once.
func TestPublishDuplicateIsIdempotent(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	ev := event(1, "first-blood", base)
	publish(t, s, ev)
	publish(t, s, ev)
	pending, err := s.Pending("viewer-1")
	if err != nil {
		t.Fatalf("Pending: %v", err)
	}
	if pending != 1 {
		t.Fatalf("pending after duplicate publish = %d, want 1", pending)
	}
}

// Nothing is delivered before ServerTime plus the viewer's delay.
func TestNextDeliversOnlyReleasedEvents(t *testing.T) {
	cfg := testConfig()
	cfg.Delay = 30 * time.Second
	s := taskpkg.NewSpectatorDelay(cfg)
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	publish(t, s, event(1, "ace", base))

	early, err := s.Next("viewer-1", base.Add(29*time.Second), 10)
	if err != nil {
		t.Fatalf("Next before release: %v", err)
	}
	if len(early) != 0 {
		t.Fatalf("Next before release returned %d packets, want 0", len(early))
	}
	onTime, err := s.Next("viewer-1", base.Add(30*time.Second), 10)
	if err != nil {
		t.Fatalf("Next at release: %v", err)
	}
	if len(onTime) != 1 {
		t.Fatalf("Next at release returned %d packets, want 1", len(onTime))
	}
	if onTime[0].Seq != 1 || onTime[0].MatchID != "m1" || onTime[0].Kind != "kill" || onTime[0].Payload != "ace" {
		t.Fatalf("packet = %+v", onTime[0])
	}
	if !onTime[0].ReleaseAt.Equal(base.Add(30 * time.Second)) {
		t.Fatalf("ReleaseAt = %v, want %v", onTime[0].ReleaseAt, base.Add(30*time.Second))
	}
}

// Events are delivered in sequence order even when published out of order.
func TestNextDeliversInSeqOrder(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	publish(t, s, event(3, "third", base))
	publish(t, s, event(1, "first", base))
	publish(t, s, event(2, "second", base))
	got, err := s.Next("viewer-1", base, 10)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("Next returned %d packets, want 3", len(got))
	}
	for i, want := range []int64{1, 2, 3} {
		if got[i].Seq != want {
			t.Fatalf("packet[%d].Seq = %d, want %d", i, got[i].Seq, want)
		}
	}
}

// max limits one call without losing the rest of the backlog.
func TestNextRespectsMaxAndKeepsRemainder(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	for i := int64(1); i <= 5; i++ {
		publish(t, s, event(i, "e", base))
	}
	first, err := s.Next("viewer-1", base, 2)
	if err != nil {
		t.Fatalf("Next 1: %v", err)
	}
	if len(first) != 2 || first[0].Seq != 1 || first[1].Seq != 2 {
		t.Fatalf("first batch = %+v, want seq 1,2", first)
	}
	if pending, err := s.Pending("viewer-1"); err != nil || pending != 3 {
		t.Fatalf("Pending after first batch = %d, err %v, want 3", pending, err)
	}
	second, err := s.Next("viewer-1", base, 2)
	if err != nil {
		t.Fatalf("Next 2: %v", err)
	}
	if len(second) != 2 || second[0].Seq != 3 || second[1].Seq != 4 {
		t.Fatalf("second batch = %+v, want seq 3,4", second)
	}
	if pending, err := s.Pending("viewer-1"); err != nil || pending != 1 {
		t.Fatalf("Pending after second batch = %d, err %v, want 1", pending, err)
	}
	rest, err := s.Next("viewer-1", base, 0)
	if err != nil {
		t.Fatalf("Next with no limit: %v", err)
	}
	if len(rest) != 1 || rest[0].Seq != 5 {
		t.Fatalf("rest = %+v, want seq 5", rest)
	}
	if pending, err := s.Pending("viewer-1"); err != nil || pending != 0 {
		t.Fatalf("Pending after drain = %d, err %v, want 0", pending, err)
	}
}

// The cursor prevents the same event from being delivered twice.
func TestNextIsCursorBasedNoRepeats(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	publish(t, s, event(1, "a", base))
	publish(t, s, event(2, "b", base))
	first, err := s.Next("viewer-1", base, 10)
	if err != nil || len(first) != 2 {
		t.Fatalf("first Next = %d packets, err %v, want 2", len(first), err)
	}
	again, err := s.Next("viewer-1", base, 10)
	if err != nil {
		t.Fatalf("second Next: %v", err)
	}
	if len(again) != 0 {
		t.Fatalf("second Next replayed %d packets, want 0", len(again))
	}
}

// A late joiner starts at the newest buffered sequence.
func TestLateJoinerSkipsPriorEvents(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	publish(t, s, event(1, "a", base))
	publish(t, s, event(2, "b", base))
	register(t, s, "m1", "late-viewer", base)
	publish(t, s, event(3, "c", base))
	got, err := s.Next("late-viewer", base, 10)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(got) != 1 || got[0].Seq != 3 {
		t.Fatalf("late joiner got %+v, want only seq 3", got)
	}
}

// Overflowing the buffer forces a resync instead of skipping silently.
func TestOverflowRequiresResync(t *testing.T) {
	cfg := testConfig()
	cfg.MaxPending = 3
	s := taskpkg.NewSpectatorDelay(cfg)
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	for i := int64(1); i <= 4; i++ {
		publish(t, s, event(i, "e", base))
	}
	if _, err := s.Next("viewer-1", base, 10); !errors.Is(err, taskpkg.ErrResyncRequired) {
		t.Fatalf("Next after overflow must yield ErrResyncRequired, got %v", err)
	}
}

// After a resync the viewer resumes from the newest buffered sequence.
func TestResyncResumesFromNewest(t *testing.T) {
	cfg := testConfig()
	cfg.MaxPending = 3
	s := taskpkg.NewSpectatorDelay(cfg)
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	for i := int64(1); i <= 4; i++ {
		publish(t, s, event(i, "e", base))
	}
	if err := s.Resync("viewer-1", base); err != nil {
		t.Fatalf("Resync: %v", err)
	}
	publish(t, s, event(5, "e", base))
	got, err := s.Next("viewer-1", base, 10)
	if err != nil {
		t.Fatalf("Next after resync: %v", err)
	}
	if len(got) != 1 || got[0].Seq != 5 {
		t.Fatalf("after resync got %+v, want only seq 5", got)
	}
}

// One match cannot host more viewers than MaxSpectators, and ids are unique.
func TestRegisterEnforcesSpectatorCap(t *testing.T) {
	cfg := testConfig()
	cfg.MaxSpectators = 2
	s := taskpkg.NewSpectatorDelay(cfg)
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	if err := s.RegisterSpectator("m1", "viewer-1", base); !errors.Is(err, taskpkg.ErrSpectatorExists) {
		t.Fatalf("duplicate viewer must yield ErrSpectatorExists, got %v", err)
	}
	register(t, s, "m1", "viewer-2", base)
	if err := s.RegisterSpectator("m1", "viewer-3", base); !errors.Is(err, taskpkg.ErrTooManySpectators) {
		t.Fatalf("third viewer must yield ErrTooManySpectators, got %v", err)
	}
}

// Events of one match never leak into another match's stream.
func TestMatchIsolation(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-a", base)
	register(t, s, "m2", "viewer-b", base)
	publish(t, s, taskpkg.Event{Seq: 1, MatchID: "m1", ServerTime: base, Kind: "kill", Payload: "a-kill"})
	publish(t, s, taskpkg.Event{Seq: 1, MatchID: "m2", ServerTime: base, Kind: "score", Payload: "b-score"})
	gotA, err := s.Next("viewer-a", base, 10)
	if err != nil || len(gotA) != 1 {
		t.Fatalf("viewer-a Next = %d packets, err %v, want 1", len(gotA), err)
	}
	if gotA[0].MatchID != "m1" || gotA[0].Payload != "a-kill" {
		t.Fatalf("viewer-a packet = %+v", gotA[0])
	}
	gotB, err := s.Next("viewer-b", base, 10)
	if err != nil || len(gotB) != 1 {
		t.Fatalf("viewer-b Next = %d packets, err %v, want 1", len(gotB), err)
	}
	if gotB[0].MatchID != "m2" || gotB[0].Payload != "b-score" {
		t.Fatalf("viewer-b packet = %+v", gotB[0])
	}
}

// Unregistering an unknown viewer fails; a removed viewer cannot read on.
func TestUnregisterUnknownFails(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	if err := s.Unregister("ghost"); !errors.Is(err, taskpkg.ErrSpectatorNotFound) {
		t.Fatalf("unknown viewer must yield ErrSpectatorNotFound, got %v", err)
	}
	register(t, s, "m1", "viewer-1", base)
	if err := s.Unregister("viewer-1"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if _, err := s.Next("viewer-1", base, 10); !errors.Is(err, taskpkg.ErrSpectatorNotFound) {
		t.Fatalf("removed viewer must yield ErrSpectatorNotFound, got %v", err)
	}
}

// Delay overrides outside [0, MaxDelay] are rejected and the old delay stays.
func TestSetDelayOutOfRangeRejected(t *testing.T) {
	cfg := testConfig()
	cfg.Delay = 30 * time.Second
	cfg.MaxDelay = time.Minute
	s := taskpkg.NewSpectatorDelay(cfg)
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	if err := s.SetDelay("viewer-1", -time.Second); !errors.Is(err, taskpkg.ErrDelayRange) {
		t.Fatalf("negative delay must yield ErrDelayRange, got %v", err)
	}
	if err := s.SetDelay("viewer-1", 61*time.Second); !errors.Is(err, taskpkg.ErrDelayRange) {
		t.Fatalf("delay above MaxDelay must yield ErrDelayRange, got %v", err)
	}
	if err := s.SetDelay("viewer-1", 0); err != nil {
		t.Fatalf("SetDelay(0): %v", err)
	}
	publish(t, s, event(1, "e", base))
	got, err := s.Next("viewer-1", base, 10)
	if err != nil {
		t.Fatalf("Next: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("packet with the new zero delay not released: got %d", len(got))
	}
}

// A restart keeps the cursor, the backlog and the delay table.
func TestSnapshotRestorePreservesCursorAndPending(t *testing.T) {
	cfg := testConfig()
	s := taskpkg.NewSpectatorDelay(cfg)
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	publish(t, s, event(1, "a", base))
	publish(t, s, event(2, "b", base))
	publish(t, s, event(3, "c", base))
	first, err := s.Next("viewer-1", base, 1)
	if err != nil || len(first) != 1 || first[0].Seq != 1 {
		t.Fatalf("first Next = %+v, err %v", first, err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap) == 0 {
		t.Fatal("Snapshot returned no bytes")
	}
	restored, err := taskpkg.NewSpectatorDelayFromSnapshot(cfg, snap)
	if err != nil {
		t.Fatalf("NewSpectatorDelayFromSnapshot: %v", err)
	}
	if pending, err := restored.Pending("viewer-1"); err != nil || pending != 2 {
		t.Fatalf("restored pending = %d, err %v, want 2", pending, err)
	}
	next, err := restored.Next("viewer-1", base, 10)
	if err != nil {
		t.Fatalf("Next after restore: %v", err)
	}
	if len(next) != 2 || next[0].Seq != 2 || next[1].Seq != 3 {
		t.Fatalf("restored stream = %+v, want seq 2,3", next)
	}
	publish(t, restored, event(4, "d", base))
	last, err := restored.Next("viewer-1", base, 10)
	if err != nil || len(last) != 1 || last[0].Seq != 4 {
		t.Fatalf("stream after restore = %+v, err %v, want seq 4", last, err)
	}
	if _, err := taskpkg.NewSpectatorDelayFromSnapshot(cfg, []byte("{broken")); err == nil {
		t.Fatal("corrupt snapshot was accepted")
	}
}

// Concurrent reads of one viewer must partition the backlog, never duplicate.
func TestConcurrentNextDoesNotDuplicate(t *testing.T) {
	s := taskpkg.NewSpectatorDelay(testConfig())
	base := time.Unix(1700000000, 0)
	register(t, s, "m1", "viewer-1", base)
	for i := int64(1); i <= 10; i++ {
		publish(t, s, event(i, "e", base))
	}
	const workers = 10
	var wg sync.WaitGroup
	batches := make([][]taskpkg.Packet, workers)
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			batches[idx], errs[idx] = s.Next("viewer-1", base, 5)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Next %d: %v", i, err)
		}
	}
	seen := make(map[int64]bool)
	for _, batch := range batches {
		for _, p := range batch {
			if seen[p.Seq] {
				t.Fatalf("sequence %d delivered twice", p.Seq)
			}
			seen[p.Seq] = true
		}
	}
	if len(seen) != 10 {
		t.Fatalf("delivered %d distinct sequences, want 10", len(seen))
	}
}
