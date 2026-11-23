package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-g-05"
)

const vsync60 = 16666666 * time.Nanosecond

func pacerTestConfig() taskpkg.Config {
	return taskpkg.Config{
		RefreshHz:              60,
		MaxPending:             8,
		LateToleranceMS:        8,
		AudioResyncThresholdMS: 40,
	}
}

func newPacer(t *testing.T, cfg taskpkg.Config) *taskpkg.FramePacer {
	t.Helper()
	p, err := taskpkg.NewFramePacer("stream-9001", cfg)
	if err != nil {
		t.Fatalf("NewFramePacer returned error: %v", err)
	}
	return p
}

func submitFrame(t *testing.T, p *taskpkg.FramePacer, id int64, pts, dur time.Duration) {
	t.Helper()
	if err := p.Submit(taskpkg.Frame{ID: id, PTS: pts, Duration: dur}); err != nil {
		t.Fatalf("Submit(%d) returned error: %v", id, err)
	}
}

func TestConfigRejectsBadRefreshAndQueue(t *testing.T) {
	cfg := pacerTestConfig()
	cfg.RefreshHz = 0
	if _, err := taskpkg.NewFramePacer("s", cfg); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewFramePacer(refresh 0) error = %v, want ErrInvalidConfig", err)
	}
	cfg = pacerTestConfig()
	cfg.RefreshHz = 500
	if _, err := taskpkg.NewFramePacer("s", cfg); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewFramePacer(refresh 500) error = %v, want ErrInvalidConfig", err)
	}
	cfg = pacerTestConfig()
	cfg.MaxPending = 100
	if _, err := taskpkg.NewFramePacer("s", cfg); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewFramePacer(max pending 100) error = %v, want ErrInvalidConfig", err)
	}
}

func TestFrameIntervalForRefresh(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	got := p.FrameInterval()
	if got < vsync60-1000 || got > vsync60+1000 {
		t.Fatalf("FrameInterval at 60 Hz = %v, want about %v", got, vsync60)
	}
	cfg := pacerTestConfig()
	cfg.RefreshHz = 120
	p = newPacer(t, cfg)
	want := time.Second / 120
	if got := p.FrameInterval(); got < want-1000 || got > want+1000 {
		t.Fatalf("FrameInterval at 120 Hz = %v, want about %v", got, want)
	}
}

func TestNotReadyBeforeVsync(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 1, time.Millisecond, 10*time.Millisecond)
	if _, err := p.Next(0); !errors.Is(err, taskpkg.ErrNotReady) {
		t.Fatalf("Next(0) error = %v, want ErrNotReady", err)
	}
	if got := p.Pending(); got != 1 {
		t.Fatalf("Pending after early Next = %d, want 1", got)
	}
}

func TestPresentAtVsyncBoundary(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 7, time.Millisecond, 10*time.Millisecond)
	pres, err := p.Next(vsync60)
	if err != nil {
		t.Fatalf("Next(vsync) returned error: %v", err)
	}
	if pres.Frame.ID != 7 {
		t.Fatalf("presented frame %d, want 7", pres.Frame.ID)
	}
	if pres.PresentAt < vsync60-1000 || pres.PresentAt > vsync60+1000 {
		t.Fatalf("PresentAt = %v, want about %v", pres.PresentAt, vsync60)
	}
	if p.PresentedCount() != 1 || p.Pending() != 0 {
		t.Fatalf("counters after presentation = presented %d pending %d", p.PresentedCount(), p.Pending())
	}
}

func TestPresentsInPTSOrder(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 3, 50*time.Millisecond, 10*time.Millisecond)
	submitFrame(t, p, 1, 10*time.Millisecond, 10*time.Millisecond)
	submitFrame(t, p, 2, 30*time.Millisecond, 10*time.Millisecond)
	for _, step := range []struct {
		now  time.Duration
		want int64
	}{
		{20 * time.Millisecond, 1},
		{40 * time.Millisecond, 2},
		{55 * time.Millisecond, 3},
	} {
		pres, err := p.Next(step.now)
		if err != nil {
			t.Fatalf("Next(%v) returned error: %v", step.now, err)
		}
		if pres.Frame.ID != step.want {
			t.Fatalf("Next(%v) presented %d, want %d", step.now, pres.Frame.ID, step.want)
		}
	}
	if p.PresentedCount() != 3 {
		t.Fatalf("PresentedCount = %d, want 3", p.PresentedCount())
	}
}

func TestDuplicateFrameRejected(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 5, 10*time.Millisecond, 10*time.Millisecond)
	err := p.Submit(taskpkg.Frame{ID: 5, PTS: 20 * time.Millisecond, Duration: 10 * time.Millisecond})
	if !errors.Is(err, taskpkg.ErrDuplicateFrame) {
		t.Fatalf("duplicate Submit error = %v, want ErrDuplicateFrame", err)
	}
	if got := p.Pending(); got != 1 {
		t.Fatalf("Pending after duplicate = %d, want 1", got)
	}
}

func TestQueueFullRejected(t *testing.T) {
	cfg := pacerTestConfig()
	cfg.MaxPending = 2
	p := newPacer(t, cfg)
	submitFrame(t, p, 1, 0, 10*time.Millisecond)
	submitFrame(t, p, 2, 10*time.Millisecond, 10*time.Millisecond)
	err := p.Submit(taskpkg.Frame{ID: 3, PTS: 20 * time.Millisecond, Duration: 10 * time.Millisecond})
	if !errors.Is(err, taskpkg.ErrQueueFull) {
		t.Fatalf("third Submit error = %v, want ErrQueueFull", err)
	}
	if got := p.Pending(); got != 2 {
		t.Fatalf("Pending after overflow = %d, want 2", got)
	}
}

func TestInvalidFrameRejected(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	bad := []taskpkg.Frame{
		{ID: 0, PTS: 0, Duration: 10 * time.Millisecond},
		{ID: 1, PTS: -time.Millisecond, Duration: 10 * time.Millisecond},
		{ID: 2, PTS: 0, Duration: 0},
	}
	for _, f := range bad {
		if err := p.Submit(f); !errors.Is(err, taskpkg.ErrInvalidFrame) {
			t.Fatalf("Submit(%+v) error = %v, want ErrInvalidFrame", f, err)
		}
	}
	if got := p.Pending(); got != 0 {
		t.Fatalf("Pending after invalid frames = %d, want 0", got)
	}
}

func TestLateFrameDropped(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 1, 0, 10*time.Millisecond)
	if _, err := p.Next(100 * time.Millisecond); !errors.Is(err, taskpkg.ErrNotReady) {
		t.Fatalf("Next after deadline error = %v, want ErrNotReady", err)
	}
	if got := p.DroppedCount(); got != 1 {
		t.Fatalf("DroppedCount = %d, want 1", got)
	}
	if got := p.PresentedCount(); got != 0 {
		t.Fatalf("PresentedCount = %d, want 0", got)
	}
}

func TestWithinLateToleranceStillPresents(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 2, 0, 10*time.Millisecond)
	pres, err := p.Next(5 * time.Millisecond)
	if err != nil {
		t.Fatalf("Next inside tolerance returned error: %v", err)
	}
	if pres.Frame.ID != 2 {
		t.Fatalf("presented frame %d, want 2", pres.Frame.ID)
	}
	if p.DroppedCount() != 0 {
		t.Fatalf("DroppedCount = %d, want 0 inside the tolerance", p.DroppedCount())
	}
}

func TestAudioClockResyncsVideo(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 4, 10*time.Millisecond, 10*time.Millisecond)
	if err := p.SetAudioClock(200 * time.Millisecond); err != nil {
		t.Fatalf("SetAudioClock returned error: %v", err)
	}
	pres, err := p.Next(200 * time.Millisecond)
	if err != nil {
		t.Fatalf("Next(200ms) returned error: %v", err)
	}
	if !pres.Resynced {
		t.Fatalf("presentation was not marked resynced: %+v", pres)
	}
	if pres.PresentAt < 199*time.Millisecond || pres.PresentAt > 201*time.Millisecond {
		t.Fatalf("resynced PresentAt = %v, want near the 200 ms audio clock", pres.PresentAt)
	}
	if got := p.AudioClock(); got != 200*time.Millisecond {
		t.Fatalf("AudioClock = %v, want 200ms", got)
	}
}

func TestSnapshotRestoreKeepsQueue(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 1, 10*time.Millisecond, 10*time.Millisecond)
	submitFrame(t, p, 2, 30*time.Millisecond, 10*time.Millisecond)
	submitFrame(t, p, 3, 50*time.Millisecond, 10*time.Millisecond)
	data, err := p.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	restored, err := taskpkg.NewFramePacerFromSnapshot(pacerTestConfig(), data)
	if err != nil {
		t.Fatalf("NewFramePacerFromSnapshot returned error: %v", err)
	}
	if got := restored.Pending(); got != 3 {
		t.Fatalf("restored Pending = %d, want 3", got)
	}
	pres, err := restored.Next(20 * time.Millisecond)
	if err != nil {
		t.Fatalf("restored Next returned error: %v", err)
	}
	if pres.Frame.ID != 1 {
		t.Fatalf("restored first frame = %d, want 1", pres.Frame.ID)
	}
	if got := restored.Pending(); got != 2 {
		t.Fatalf("restored Pending after one present = %d, want 2", got)
	}
	if got := restored.PresentedCount(); got != 1 {
		t.Fatalf("restored PresentedCount = %d, want 1", got)
	}
}

func TestResetClearsPacer(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 1, 0, 10*time.Millisecond)
	submitFrame(t, p, 2, 20*time.Millisecond, 10*time.Millisecond)
	if _, err := p.Next(0); err != nil {
		t.Fatalf("Next returned error: %v", err)
	}
	if err := p.SetAudioClock(50 * time.Millisecond); err != nil {
		t.Fatalf("SetAudioClock returned error: %v", err)
	}
	p.Reset()
	if p.Pending() != 0 || p.PresentedCount() != 0 || p.DroppedCount() != 0 {
		t.Fatalf("Reset left counters queue=%d presented=%d dropped=%d", p.Pending(), p.PresentedCount(), p.DroppedCount())
	}
	if got := p.AudioClock(); got != 0 {
		t.Fatalf("AudioClock after Reset = %v, want 0", got)
	}
	if _, err := p.Next(0); !errors.Is(err, taskpkg.ErrNotReady) {
		t.Fatalf("Next after Reset error = %v, want ErrNotReady", err)
	}
}

func TestConcurrentSubmitAndDrain(t *testing.T) {
	cfg := pacerTestConfig()
	cfg.MaxPending = 64
	p := newPacer(t, cfg)
	var wg sync.WaitGroup
	errs := make(chan error, 60)
	for i := 0; i < 60; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f := taskpkg.Frame{ID: int64(i + 1), PTS: time.Duration(i) * time.Millisecond, Duration: 10 * time.Millisecond}
			if err := p.Submit(f); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Submit returned error: %v", err)
	}
	for now := time.Duration(0); now <= 100*time.Millisecond; now += 100 * time.Microsecond {
		_, _ = p.Next(now)
	}
	if got := p.PresentedCount(); got != 60 {
		t.Fatalf("PresentedCount after drain = %d, want 60", got)
	}
	if got := p.DroppedCount(); got != 0 {
		t.Fatalf("DroppedCount after drain = %d, want 0", got)
	}
	if got := p.Pending(); got != 0 {
		t.Fatalf("Pending after drain = %d, want 0", got)
	}
}

func TestStaleFrameAfterPresentationDropped(t *testing.T) {
	p := newPacer(t, pacerTestConfig())
	submitFrame(t, p, 10, 100*time.Millisecond, 10*time.Millisecond)
	if _, err := p.Next(100 * time.Millisecond); err != nil {
		t.Fatalf("Next returned error: %v", err)
	}
	submitFrame(t, p, 11, 50*time.Millisecond, 10*time.Millisecond)
	if _, err := p.Next(110 * time.Millisecond); !errors.Is(err, taskpkg.ErrNotReady) {
		t.Fatalf("Next after stale submit error = %v, want ErrNotReady", err)
	}
	if got := p.DroppedCount(); got != 1 {
		t.Fatalf("DroppedCount = %d, want 1", got)
	}
	if got := p.Pending(); got != 0 {
		t.Fatalf("Pending after stale drop = %d, want 0", got)
	}
}
