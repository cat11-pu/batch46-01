package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-c-03"
)

func lagConfig() taskpkg.Config {
	return taskpkg.Config{HistoryFrames: 8, MaxRewindTicks: 100, MaxEntitiesPerFrame: 16}
}

func newLag(t *testing.T) *taskpkg.LagCompensation {
	t.Helper()
	return taskpkg.NewLagCompensation(lagConfig())
}

func body(id string, x, y, z, radius float64, alive bool) taskpkg.EntityState {
	return taskpkg.EntityState{ID: id, Pos: taskpkg.Vec3{X: x, Y: y, Z: z}, Radius: radius, Alive: alive}
}

func near(a, b float64) bool {
	return math.Abs(a-b) < 1e-6
}

// Strictly increasing frames must be retained.
func TestRecordFramesMonotonic(t *testing.T) {
	l := newLag(t)
	for _, tick := range []uint32{100, 101, 102} {
		if err := l.RecordFrame(tick, []taskpkg.EntityState{body("p1", float64(tick), 0, 0, 1, true)}); err != nil {
			t.Fatalf("RecordFrame(%d): %v", tick, err)
		}
	}
	if got := l.FrameCount(); got != 3 {
		t.Fatalf("FrameCount = %d, want 3", got)
	}
	if oldest, ok := l.OldestTick(); !ok || oldest != 100 {
		t.Fatalf("OldestTick = %d, %v, want 100, true", oldest, ok)
	}
	if newest, ok := l.NewestTick(); !ok || newest != 102 {
		t.Fatalf("NewestTick = %d, %v, want 102, true", newest, ok)
	}
}

// A repeated or older tick must be rejected.
func TestOutOfOrderFrameRejected(t *testing.T) {
	l := newLag(t)
	if err := l.RecordFrame(100, []taskpkg.EntityState{body("p1", 0, 0, 0, 1, true)}); err != nil {
		t.Fatalf("first frame: %v", err)
	}
	if err := l.RecordFrame(100, []taskpkg.EntityState{body("p1", 1, 0, 0, 1, true)}); !errors.Is(err, taskpkg.ErrTickOutOfOrder) {
		t.Fatalf("repeated tick error = %v, want ErrTickOutOfOrder", err)
	}
	if err := l.RecordFrame(99, []taskpkg.EntityState{body("p1", 2, 0, 0, 1, true)}); !errors.Is(err, taskpkg.ErrTickOutOfOrder) {
		t.Fatalf("older tick error = %v, want ErrTickOutOfOrder", err)
	}
	if got := l.FrameCount(); got != 1 {
		t.Fatalf("rejected frames changed the history: %d", got)
	}
}

// One entity id may appear only once per frame.
func TestDuplicateEntityRejected(t *testing.T) {
	l := newLag(t)
	err := l.RecordFrame(10, []taskpkg.EntityState{
		body("p1", 0, 0, 0, 1, true),
		body("p1", 5, 0, 0, 1, true),
	})
	if !errors.Is(err, taskpkg.ErrDuplicateEntity) {
		t.Fatalf("duplicate entity error = %v, want ErrDuplicateEntity", err)
	}
	if got := l.FrameCount(); got != 0 {
		t.Fatalf("rejected frame was stored: %d", got)
	}
}

// A frame may not exceed the configured entity cap.
func TestFrameEntityLimit(t *testing.T) {
	cfg := lagConfig()
	cfg.MaxEntitiesPerFrame = 2
	l := taskpkg.NewLagCompensation(cfg)
	err := l.RecordFrame(10, []taskpkg.EntityState{
		body("p1", 0, 0, 0, 1, true),
		body("p2", 1, 0, 0, 1, true),
		body("p3", 2, 0, 0, 1, true),
	})
	if !errors.Is(err, taskpkg.ErrFrameTooLarge) {
		t.Fatalf("oversized frame error = %v, want ErrFrameTooLarge", err)
	}
}

// The history ring must evict old frames and bounded memory.
func TestRingEvictionBounded(t *testing.T) {
	cfg := lagConfig()
	cfg.HistoryFrames = 3
	cfg.MaxEntitiesPerFrame = 2
	l := taskpkg.NewLagCompensation(cfg)
	for tick := uint32(10); tick < 20; tick++ {
		entities := []taskpkg.EntityState{
			body("p1", float64(tick), 0, 0, 1, true),
			body("p2", float64(tick), 1, 0, 1, true),
		}
		if err := l.RecordFrame(tick, entities); err != nil {
			t.Fatalf("RecordFrame(%d): %v", tick, err)
		}
	}
	if got := l.FrameCount(); got != 3 {
		t.Fatalf("FrameCount = %d, want 3", got)
	}
	if oldest, ok := l.OldestTick(); !ok || oldest != 17 {
		t.Fatalf("OldestTick = %d, %v, want 17, true", oldest, ok)
	}
	if mem := l.MemoryBytes(); mem <= 0 || mem > 768 {
		t.Fatalf("MemoryBytes = %d, want in (0,768]", mem)
	}
	if _, err := l.Rewind(10); !errors.Is(err, taskpkg.ErrTickExpired) {
		t.Fatalf("evicted tick error = %v, want ErrTickExpired", err)
	}
}

// Rewind must return the frozen frame, not the latest state.
func TestRewindReturnsFrozenFrame(t *testing.T) {
	l := newLag(t)
	if err := l.RecordFrame(100, []taskpkg.EntityState{body("p1", 0, 0, 0, 1, true)}); err != nil {
		t.Fatalf("RecordFrame(100): %v", err)
	}
	if err := l.RecordFrame(101, []taskpkg.EntityState{body("p1", 10, 0, 0, 1, true)}); err != nil {
		t.Fatalf("RecordFrame(101): %v", err)
	}
	frame, err := l.Rewind(100)
	if err != nil {
		t.Fatalf("Rewind(100): %v", err)
	}
	if len(frame.Entities) != 1 || frame.Entities[0].Pos.X != 0 {
		t.Fatalf("rewind returned %+v, want p1 at x=0", frame.Entities)
	}
	frame.Entities[0].Pos.X = 999
	again, err := l.Rewind(100)
	if err != nil {
		t.Fatalf("second Rewind(100): %v", err)
	}
	if again.Entities[0].Pos.X != 0 {
		t.Fatalf("caller mutation leaked into the history: %v", again.Entities[0].Pos.X)
	}
}

// Ticks that fell out of the history window must be expired.
func TestRewindExpiredTick(t *testing.T) {
	cfg := lagConfig()
	cfg.HistoryFrames = 2
	l := taskpkg.NewLagCompensation(cfg)
	for tick := uint32(100); tick <= 104; tick++ {
		if err := l.RecordFrame(tick, []taskpkg.EntityState{body("p1", float64(tick), 0, 0, 1, true)}); err != nil {
			t.Fatalf("RecordFrame(%d): %v", tick, err)
		}
	}
	if _, err := l.Rewind(100); !errors.Is(err, taskpkg.ErrTickExpired) {
		t.Fatalf("expired tick error = %v, want ErrTickExpired", err)
	}
}

// A tick newer than the newest frame must be rejected.
func TestRewindFutureTick(t *testing.T) {
	l := newLag(t)
	if err := l.RecordFrame(100, []taskpkg.EntityState{body("p1", 0, 0, 0, 1, true)}); err != nil {
		t.Fatalf("RecordFrame: %v", err)
	}
	if _, err := l.Rewind(200); !errors.Is(err, taskpkg.ErrTickInFuture) {
		t.Fatalf("future tick error = %v, want ErrTickInFuture", err)
	}
}

// EntityAt must read the requested historical frame.
func TestEntityAtHistoricalFrame(t *testing.T) {
	l := newLag(t)
	if err := l.RecordFrame(100, []taskpkg.EntityState{body("p1", 5, 0, 0, 1, true)}); err != nil {
		t.Fatalf("RecordFrame(100): %v", err)
	}
	if err := l.RecordFrame(101, []taskpkg.EntityState{body("p1", 9, 0, 0, 1, true)}); err != nil {
		t.Fatalf("RecordFrame(101): %v", err)
	}
	old, err := l.EntityAt(100, "p1")
	if err != nil {
		t.Fatalf("EntityAt(100): %v", err)
	}
	if old.Pos.X != 5 {
		t.Fatalf("historical x = %v, want 5", old.Pos.X)
	}
	newer, err := l.EntityAt(101, "p1")
	if err != nil {
		t.Fatalf("EntityAt(101): %v", err)
	}
	if newer.Pos.X != 9 {
		t.Fatalf("newest x = %v, want 9", newer.Pos.X)
	}
	if _, err := l.EntityAt(100, "ghost"); !errors.Is(err, taskpkg.ErrEntityNotFound) {
		t.Fatalf("missing entity error = %v, want ErrEntityNotFound", err)
	}
}

// Raycast must report the nearest alive entity and its distance.
func TestRaycastNearestAlive(t *testing.T) {
	l := newLag(t)
	if err := l.RecordFrame(50, []taskpkg.EntityState{
		body("far", 20, 0, 0, 1, true),
		body("near", 10, 0, 0, 1, true),
	}); err != nil {
		t.Fatalf("RecordFrame: %v", err)
	}
	hit, err := l.Raycast(50, taskpkg.Vec3{}, taskpkg.Vec3{X: 1}, 100)
	if err != nil {
		t.Fatalf("Raycast: %v", err)
	}
	if hit.EntityID != "near" {
		t.Fatalf("raycast hit %q, want near", hit.EntityID)
	}
	if !near(hit.Distance, 9) {
		t.Fatalf("hit distance = %v, want 9", hit.Distance)
	}
	if !near(hit.Point.X, 9) {
		t.Fatalf("hit point x = %v, want 9", hit.Point.X)
	}
}

// Dead entities must be ignored, and an all-dead frame is a clean miss.
func TestRaycastSkipsDeadEntities(t *testing.T) {
	l := newLag(t)
	if err := l.RecordFrame(50, []taskpkg.EntityState{
		body("corpse", 10, 0, 0, 5, false),
	}); err != nil {
		t.Fatalf("RecordFrame: %v", err)
	}
	if _, err := l.Raycast(50, taskpkg.Vec3{}, taskpkg.Vec3{X: 1}, 100); !errors.Is(err, taskpkg.ErrNoHit) {
		t.Fatalf("all-dead raycast error = %v, want ErrNoHit", err)
	}
}

// Equidistant hits must resolve to the smaller entity id.
func TestRaycastDeterministicTieBreak(t *testing.T) {
	l := newLag(t)
	if err := l.RecordFrame(50, []taskpkg.EntityState{
		body("zombie-b", 10, 1, 0, 1, true),
		body("zombie-a", 10, -1, 0, 1, true),
	}); err != nil {
		t.Fatalf("RecordFrame: %v", err)
	}
	hit, err := l.Raycast(50, taskpkg.Vec3{}, taskpkg.Vec3{X: 1}, 100)
	if err != nil {
		t.Fatalf("Raycast: %v", err)
	}
	if hit.EntityID != "zombie-a" {
		t.Fatalf("tie broken to %q, want zombie-a", hit.EntityID)
	}
	if !near(hit.Distance, 10) {
		t.Fatalf("tie distance = %v, want 10", hit.Distance)
	}
}

// ClampClientTick must clamp both ends of the rewind window.
func TestClampClientTick(t *testing.T) {
	cfg := lagConfig()
	cfg.MaxRewindTicks = 100
	l := taskpkg.NewLagCompensation(cfg)
	if err := l.RecordFrame(1000, []taskpkg.EntityState{body("p1", 0, 0, 0, 1, true)}); err != nil {
		t.Fatalf("RecordFrame: %v", err)
	}
	clamped, err := l.ClampClientTick(850)
	if err != nil {
		t.Fatalf("ClampClientTick(850): %v", err)
	}
	if clamped != 900 {
		t.Fatalf("clamped old tick = %d, want 900", clamped)
	}
	clamped, err = l.ClampClientTick(1200)
	if err != nil {
		t.Fatalf("ClampClientTick(1200): %v", err)
	}
	if clamped != 1000 {
		t.Fatalf("clamped future tick = %d, want 1000", clamped)
	}
	clamped, err = l.ClampClientTick(950)
	if err != nil {
		t.Fatalf("ClampClientTick(950): %v", err)
	}
	if clamped != 950 {
		t.Fatalf("in-window tick changed to %d, want 950", clamped)
	}
}

// Recording and reading concurrently must stay consistent.
func TestConcurrentRecordAndRewind(t *testing.T) {
	cfg := lagConfig()
	cfg.HistoryFrames = 3
	l := taskpkg.NewLagCompensation(cfg)
	var wg sync.WaitGroup
	stop := make(chan struct{})
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				l.FrameCount()
				if newest, ok := l.NewestTick(); ok {
					l.EntityAt(newest, "p1")
				}
			}
		}()
	}
	var recordErr error
	for tick := uint32(1); tick <= 60; tick++ {
		if err := l.RecordFrame(tick, []taskpkg.EntityState{body("p1", float64(tick), 0, 0, 1, true)}); err != nil {
			recordErr = err
			break
		}
	}
	close(stop)
	wg.Wait()
	if recordErr != nil {
		t.Fatalf("concurrent RecordFrame: %v", recordErr)
	}
	if got := l.FrameCount(); got != 3 {
		t.Fatalf("FrameCount after concurrent run = %d, want 3", got)
	}
	newest, ok := l.NewestTick()
	if !ok || newest != 60 {
		t.Fatalf("NewestTick = %d, %v, want 60, true", newest, ok)
	}
	if _, err := l.Rewind(60); err != nil {
		t.Fatalf("Rewind(newest): %v", err)
	}
}

// A restarted process must keep the same rewind history.
func TestSnapshotRestoresHistory(t *testing.T) {
	l := newLag(t)
	for _, tick := range []uint32{100, 101, 102} {
		if err := l.RecordFrame(tick, []taskpkg.EntityState{body("p1", float64(tick)*3, 0, 0, 1, true)}); err != nil {
			t.Fatalf("RecordFrame(%d): %v", tick, err)
		}
	}
	snap, err := l.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewLagCompensationFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewLagCompensationFromSnapshot: %v", err)
	}
	if got := restored.FrameCount(); got != 3 {
		t.Fatalf("restored FrameCount = %d, want 3", got)
	}
	frame, err := restored.Rewind(101)
	if err != nil {
		t.Fatalf("Rewind(101) on restored: %v", err)
	}
	if len(frame.Entities) != 1 || frame.Entities[0].Pos.X != 303 {
		t.Fatalf("restored frame = %+v, want p1 at x=303", frame.Entities)
	}
	if oldest, ok := restored.OldestTick(); !ok || oldest != 100 {
		t.Fatalf("restored OldestTick = %d, %v, want 100, true", oldest, ok)
	}
}
