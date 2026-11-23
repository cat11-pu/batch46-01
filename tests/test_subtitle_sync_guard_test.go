package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-h-03"
)

func subConfig() taskpkg.Config {
	return taskpkg.Config{MaxCues: 8, MinDuration: 500 * time.Millisecond}
}

func subCue(startMS, endMS int, text string) taskpkg.Cue {
	return taskpkg.Cue{
		Start: time.Duration(startMS) * time.Millisecond,
		End:   time.Duration(endMS) * time.Millisecond,
		Text:  text,
	}
}

func newSub(t *testing.T) *taskpkg.SubtitleSync {
	t.Helper()
	return taskpkg.NewSubtitleSync(subConfig())
}

// Loading unsorted cues stores them in playback order.
func TestLoadCuesSortsByStart(t *testing.T) {
	s := newSub(t)
	err := s.LoadCues([]taskpkg.Cue{
		subCue(5000, 6000, "third"),
		subCue(1000, 2000, "first"),
		subCue(3000, 4000, "second"),
	})
	if err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	cues := s.Cues()
	if len(cues) != 3 {
		t.Fatalf("Cues returned %d cues, want 3", len(cues))
	}
	if cues[0].Text != "first" || cues[1].Text != "second" || cues[2].Text != "third" {
		t.Fatalf("cue order = %q, %q, %q", cues[0].Text, cues[1].Text, cues[2].Text)
	}
	if cues[0].Start != time.Second || cues[2].Start != 5*time.Second {
		t.Fatalf("cue starts = %v and %v, want 1s and 5s", cues[0].Start, cues[2].Start)
	}
	st := s.Stats()
	if st.Cues != 3 || st.TotalDuration != 3*time.Second {
		t.Fatalf("Stats = %+v, want 3 cues and 3s total", st)
	}
}

// A cue that ends at or before it starts is rejected.
func TestLoadCuesRejectsInvalidCue(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{subCue(1000, 1000, "empty")}); !errors.Is(err, taskpkg.ErrInvalidCue) {
		t.Fatalf("zero-length cue err = %v, want ErrInvalidCue", err)
	}
	if err := s.LoadCues([]taskpkg.Cue{subCue(2000, 1500, "reversed")}); !errors.Is(err, taskpkg.ErrInvalidCue) {
		t.Fatalf("reversed cue err = %v, want ErrInvalidCue", err)
	}
	if len(s.Cues()) != 0 {
		t.Fatalf("track kept %d cues after a rejected load", len(s.Cues()))
	}
}

// Overlapping cues are refused and the previous track survives untouched.
func TestLoadCuesRejectsOverlapAtomically(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{subCue(0, 2000, "a"), subCue(1500, 2500, "b")}); !errors.Is(err, taskpkg.ErrCueOverlap) {
		t.Fatalf("overlapping load err = %v, want ErrCueOverlap", err)
	}
	if len(s.Cues()) != 0 {
		t.Fatalf("track kept %d cues after a rejected load", len(s.Cues()))
	}
	if err := s.LoadCues([]taskpkg.Cue{subCue(0, 1000, "a")}); err != nil {
		t.Fatalf("valid load after rejection: %v", err)
	}
	if len(s.Cues()) != 1 {
		t.Fatalf("valid load produced %d cues, want 1", len(s.Cues()))
	}
}

// The configured cue count is a hard ceiling.
func TestLoadCuesRejectsTooMany(t *testing.T) {
	cfg := subConfig()
	cfg.MaxCues = 3
	s := taskpkg.NewSubtitleSync(cfg)
	cues := []taskpkg.Cue{
		subCue(0, 900, "a"),
		subCue(1000, 1900, "b"),
		subCue(2000, 2900, "c"),
		subCue(3000, 3900, "d"),
	}
	if err := s.LoadCues(cues); !errors.Is(err, taskpkg.ErrTooManyCues) {
		t.Fatalf("over-cap load err = %v, want ErrTooManyCues", err)
	}
	if len(s.Cues()) != 0 {
		t.Fatalf("track kept cues after an over-cap load")
	}
	if err := s.LoadCues(cues[:3]); err != nil {
		t.Fatalf("load at the cap: %v", err)
	}
	if len(s.Cues()) != 3 {
		t.Fatalf("track has %d cues, want 3", len(s.Cues()))
	}
}

// Cues shorter than the minimum duration are rejected.
func TestLoadCuesRejectsTooShort(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{subCue(0, 300, "blink")}); !errors.Is(err, taskpkg.ErrCueTooShort) {
		t.Fatalf("short cue err = %v, want ErrCueTooShort", err)
	}
	if err := s.LoadCues([]taskpkg.Cue{subCue(0, 900, "ok"), subCue(1000, 1300, "short")}); !errors.Is(err, taskpkg.ErrCueTooShort) {
		t.Fatalf("short cue in batch err = %v, want ErrCueTooShort", err)
	}
	if len(s.Cues()) != 0 {
		t.Fatalf("track kept %d cues after a rejected load", len(s.Cues()))
	}
}

// Appending keeps the track sorted no matter what order the cues arrive in.
func TestAppendCueInsertsSorted(t *testing.T) {
	s := newSub(t)
	for _, c := range []taskpkg.Cue{
		subCue(3000, 4000, "third"),
		subCue(1000, 2000, "first"),
		subCue(2000, 3000, "second"),
	} {
		if err := s.AppendCue(c); err != nil {
			t.Fatalf("AppendCue(%q): %v", c.Text, err)
		}
	}
	cues := s.Cues()
	if len(cues) != 3 {
		t.Fatalf("track has %d cues, want 3", len(cues))
	}
	if cues[0].Start != time.Second || cues[1].Start != 2*time.Second || cues[2].Start != 3*time.Second {
		t.Fatalf("appended cue starts = %v, %v, %v", cues[0].Start, cues[1].Start, cues[2].Start)
	}
	got, ok := s.At(2500 * time.Millisecond)
	if !ok || got.Text != "second" {
		t.Fatalf("At(2.5s) = (%q, %v), want second", got.Text, ok)
	}
}

// Duplicate and overlapping appends are reported separately and change
// nothing.
func TestAppendCueRejectsOverlapAndDuplicate(t *testing.T) {
	s := newSub(t)
	if err := s.AppendCue(subCue(1000, 2000, "hello")); err != nil {
		t.Fatalf("first AppendCue: %v", err)
	}
	if err := s.AppendCue(subCue(1000, 2000, "hello")); !errors.Is(err, taskpkg.ErrDuplicateCue) {
		t.Fatalf("duplicate append err = %v, want ErrDuplicateCue", err)
	}
	if err := s.AppendCue(subCue(1500, 2500, "late")); !errors.Is(err, taskpkg.ErrCueOverlap) {
		t.Fatalf("overlapping append err = %v, want ErrCueOverlap", err)
	}
	if len(s.Cues()) != 1 {
		t.Fatalf("track has %d cues after rejected appends, want 1", len(s.Cues()))
	}
}

// Lookup uses half-open intervals, so touching cues hand over at the boundary.
func TestAtUsesHalfOpenIntervals(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{subCue(1000, 2000, "one"), subCue(2000, 3000, "two")}); err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	if got, ok := s.At(1999 * time.Millisecond); !ok || got.Text != "one" {
		t.Fatalf("At(1.999s) = (%q, %v), want one", got.Text, ok)
	}
	if got, ok := s.At(2 * time.Second); !ok || got.Text != "two" {
		t.Fatalf("At(2s) = (%q, %v), want two", got.Text, ok)
	}
	if _, ok := s.At(3 * time.Second); ok {
		t.Fatalf("At(3s) matched a cue ending at 3s")
	}
	if _, ok := s.At(500 * time.Millisecond); ok {
		t.Fatalf("At(0.5s) matched a cue starting at 1s")
	}
}

// Shifting moves the whole track or nothing at all.
func TestShiftIsAtomic(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{subCue(1000, 2000, "a"), subCue(3000, 4000, "b")}); err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	if err := s.Shift(500 * time.Millisecond); err != nil {
		t.Fatalf("Shift(+0.5s): %v", err)
	}
	cues := s.Cues()
	if cues[0].Start != 1500*time.Millisecond || cues[1].Start != 3500*time.Millisecond {
		t.Fatalf("shifted starts = %v, %v", cues[0].Start, cues[1].Start)
	}
	if err := s.Shift(-3 * time.Second); !errors.Is(err, taskpkg.ErrNegativeTime) {
		t.Fatalf("negative shift err = %v, want ErrNegativeTime", err)
	}
	cues = s.Cues()
	if cues[0].Start != 1500*time.Millisecond || cues[1].Start != 3500*time.Millisecond {
		t.Fatalf("track changed after rejected shift: %v, %v", cues[0].Start, cues[1].Start)
	}
}

// Frame rate conversion scales every timecode.
func TestRescaleScalesTimecodes(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{subCue(4000, 6000, "line")}); err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	if err := s.Rescale(30, 24); err != nil {
		t.Fatalf("Rescale(30,24): %v", err)
	}
	got := s.Cues()[0]
	if got.Start != 5*time.Second || got.End != 7500*time.Millisecond {
		t.Fatalf("rescaled cue = [%v, %v), want [5s, 7.5s)", got.Start, got.End)
	}
	if err := s.Rescale(0, 24); !errors.Is(err, taskpkg.ErrInvalidFPS) {
		t.Fatalf("zero fps err = %v, want ErrInvalidFPS", err)
	}
}

// A conversion that would shrink a cue below the minimum is refused without
// touching the track.
func TestRescaleRejectsShortResultAtomically(t *testing.T) {
	s := taskpkg.NewSubtitleSync(subConfig())
	if err := s.LoadCues([]taskpkg.Cue{subCue(1000, 1900, "brief")}); err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	if err := s.Rescale(25, 50); !errors.Is(err, taskpkg.ErrCueTooShort) {
		t.Fatalf("collapsing rescale err = %v, want ErrCueTooShort", err)
	}
	if got := s.Cues()[0]; got.Start != time.Second || got.End != 1900*time.Millisecond {
		t.Fatalf("track changed after rejected rescale: [%v, %v)", got.Start, got.End)
	}
	if err := s.Rescale(30, 24); err != nil {
		t.Fatalf("valid rescale: %v", err)
	}
	if got := s.Cues()[0]; got.Start != 1250*time.Millisecond || got.End != 2375*time.Millisecond {
		t.Fatalf("rescaled cue = [%v, %v), want [1.25s, 2.375s)", got.Start, got.End)
	}
}

// Merging joins cues separated by at most the given gap and concatenates text.
func TestMergeJoinsNeighbouringCues(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{
		subCue(0, 1000, "first"),
		subCue(1200, 2000, "second"),
		subCue(5000, 6000, "third"),
	}); err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	if n := s.Merge(300 * time.Millisecond); n != 1 {
		t.Fatalf("Merge(300ms) merged %d cues, want 1", n)
	}
	cues := s.Cues()
	if len(cues) != 2 {
		t.Fatalf("merge left %d cues, want 2", len(cues))
	}
	if cues[0].Start != 0 || cues[0].End != 2*time.Second {
		t.Fatalf("merged cue = [%v, %v), want [0s, 2s)", cues[0].Start, cues[0].End)
	}
	if cues[0].Text != "first second" {
		t.Fatalf("merged text = %q, want %q", cues[0].Text, "first second")
	}
	if n := s.Merge(300 * time.Millisecond); n != 0 {
		t.Fatalf("second Merge merged %d cues, want 0", n)
	}
}

// Splitting cuts a cue in two and refuses fragments below the minimum.
func TestSplitCue(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{subCue(1000, 3000, "hello")}); err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	if err := s.Split(0, 1500*time.Millisecond); err != nil {
		t.Fatalf("Split: %v", err)
	}
	cues := s.Cues()
	if len(cues) != 2 {
		t.Fatalf("split produced %d cues, want 2", len(cues))
	}
	if cues[0].End != 1500*time.Millisecond || cues[1].Start != 1500*time.Millisecond {
		t.Fatalf("split boundary = %v / %v, want 1.5s", cues[0].End, cues[1].Start)
	}
	if got, ok := s.At(1200 * time.Millisecond); !ok || got.Text != "hello" {
		t.Fatalf("At(1.2s) after split = (%q, %v)", got.Text, ok)
	}
	if err := s.Split(0, 1100*time.Millisecond); !errors.Is(err, taskpkg.ErrCueTooShort) {
		t.Fatalf("tiny fragment err = %v, want ErrCueTooShort", err)
	}
	if err := s.Split(1, 3000*time.Millisecond); !errors.Is(err, taskpkg.ErrSplitOutsideCue) {
		t.Fatalf("boundary split err = %v, want ErrSplitOutsideCue", err)
	}
	if err := s.Split(9, 2000*time.Millisecond); !errors.Is(err, taskpkg.ErrIndexOutOfRange) {
		t.Fatalf("bad index err = %v, want ErrIndexOutOfRange", err)
	}
	if len(s.Cues()) != 2 {
		t.Fatalf("track has %d cues after rejected splits, want 2", len(s.Cues()))
	}
}

// Readers racing with a shift always see a complete, ordered track.
func TestConcurrentReadsDuringShift(t *testing.T) {
	s := newSub(t)
	var cues []taskpkg.Cue
	for i := 0; i < 8; i++ {
		cues = append(cues, subCue(i*1000, i*1000+900, "line"))
	}
	if err := s.LoadCues(cues); err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan string, 32)
	start := make(chan struct{})
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			got := s.Cues()
			if len(got) != 8 {
				errs <- "reader saw a partial track"
				return
			}
			for j := 1; j < len(got); j++ {
				if got[j].Start <= got[j-1].Start {
					errs <- "reader saw an unordered track"
					return
				}
				if got[j].Start < got[j-1].End {
					errs <- "reader saw an overlapping track"
					return
				}
			}
			if got[0].End-got[0].Start <= 0 {
				errs <- "reader saw a cue with a non-positive duration"
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		if err := s.Shift(500 * time.Millisecond); err != nil {
			errs <- "shift failed: " + err.Error()
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatalf("concurrent reader: %s", msg)
	}
	if got := s.Cues()[0].Start; got != 500*time.Millisecond {
		t.Fatalf("track start after shift = %v, want 500ms", got)
	}
}

// A restart keeps every cue, their order, and the duplicate and overlap rules.
func TestSnapshotRestoreKeepsTrack(t *testing.T) {
	s := newSub(t)
	if err := s.LoadCues([]taskpkg.Cue{
		subCue(0, 900, "a"),
		subCue(1000, 1900, "b"),
		subCue(2000, 2900, "c"),
	}); err != nil {
		t.Fatalf("LoadCues: %v", err)
	}
	if err := s.Shift(500 * time.Millisecond); err != nil {
		t.Fatalf("Shift: %v", err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewSubtitleSyncFromSnapshot(subConfig(), snap)
	if err != nil {
		t.Fatalf("NewSubtitleSyncFromSnapshot: %v", err)
	}
	cues := restored.Cues()
	if len(cues) != 3 || cues[0].Start != 500*time.Millisecond {
		t.Fatalf("restored track = %d cues starting at %v", len(cues), cues[0].Start)
	}
	if got, ok := restored.At(1200 * time.Millisecond); !ok || got.Text != "a" {
		t.Fatalf("restored At(1.2s) = (%q, %v), want a", got.Text, ok)
	}
	if err := restored.AppendCue(subCue(1000, 2000, "late")); !errors.Is(err, taskpkg.ErrCueOverlap) {
		t.Fatalf("overlap after restore err = %v, want ErrCueOverlap", err)
	}
	if _, err := taskpkg.NewSubtitleSyncFromSnapshot(subConfig(), []byte("garbage")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot err = %v, want ErrCorruptSnapshot", err)
	}
}
