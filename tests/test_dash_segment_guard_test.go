package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-h-02"
)

func dashConfig() taskpkg.Config {
	return taskpkg.Config{MaxSegmentsPerRep: 10, MaxSegmentsPerQuery: 4}
}

func rep720() taskpkg.Representation {
	return taskpkg.Representation{
		ID:                     "v720",
		Bandwidth:              2400000,
		Timescale:              90000,
		SegmentDuration:        360000,
		StartNumber:            1,
		PresentationTimeOffset: 900000,
		MaxSegments:            6,
	}
}

func rep1080() taskpkg.Representation {
	return taskpkg.Representation{
		ID:                     "v1080",
		Bandwidth:              5200000,
		Timescale:              1000,
		SegmentDuration:        6000,
		StartNumber:            1,
		PresentationTimeOffset: 10000,
		MaxSegments:            10,
	}
}

func newDash(t *testing.T) *taskpkg.DashSegment {
	t.Helper()
	d := taskpkg.NewDashSegment(dashConfig())
	if err := d.AddRepresentation(rep720()); err != nil {
		t.Fatalf("AddRepresentation(v720): %v", err)
	}
	if err := d.AddRepresentation(rep1080()); err != nil {
		t.Fatalf("AddRepresentation(v1080): %v", err)
	}
	return d
}

func mustRef(t *testing.T, d *taskpkg.DashSegment, repID string, number uint64) taskpkg.SegmentRef {
	t.Helper()
	ref, err := d.SegmentAt(repID, number)
	if err != nil {
		t.Fatalf("SegmentAt(%s, %d): %v", repID, number, err)
	}
	return ref
}

// Malformed timelines are refused, including a segment count above the
// configured cap and duplicate IDs.
func TestAddRepresentationValidation(t *testing.T) {
	d := taskpkg.NewDashSegment(dashConfig())
	bad := []taskpkg.Representation{
		{ID: "", Bandwidth: 100, Timescale: 1000, SegmentDuration: 2000, StartNumber: 1, MaxSegments: 2},
		{ID: "zero-ts", Bandwidth: 100, Timescale: 0, SegmentDuration: 2000, StartNumber: 1, MaxSegments: 2},
		{ID: "zero-dur", Bandwidth: 100, Timescale: 1000, SegmentDuration: 0, StartNumber: 1, MaxSegments: 2},
		{ID: "zero-start", Bandwidth: 100, Timescale: 1000, SegmentDuration: 2000, StartNumber: 0, MaxSegments: 2},
		{ID: "too-many", Bandwidth: 100, Timescale: 1000, SegmentDuration: 2000, StartNumber: 1, MaxSegments: 11},
	}
	for _, rep := range bad {
		if err := d.AddRepresentation(rep); !errors.Is(err, taskpkg.ErrInvalidRepresentation) {
			t.Fatalf("AddRepresentation(%q) err = %v, want ErrInvalidRepresentation", rep.ID, err)
		}
	}
	if err := d.AddRepresentation(rep720()); err != nil {
		t.Fatalf("AddRepresentation(v720): %v", err)
	}
	if err := d.AddRepresentation(rep720()); !errors.Is(err, taskpkg.ErrDuplicateRepresentation) {
		t.Fatalf("duplicate rep err = %v, want ErrDuplicateRepresentation", err)
	}
	if d.RepresentationCount() != 1 {
		t.Fatalf("RepresentationCount = %d, want 1", d.RepresentationCount())
	}
}

// Segment numbers resolve from the presentation time offset and the declared
// segment duration.
func TestSegmentAtComputesTimeline(t *testing.T) {
	d := newDash(t)
	first := mustRef(t, d, "v720", 1)
	if first.Start != 10*time.Second || first.End != 14*time.Second {
		t.Fatalf("segment 1 = [%v, %v), want [10s, 14s)", first.Start, first.End)
	}
	if first.URI != "v720/1.m4s" {
		t.Fatalf("segment 1 URI = %q, want v720/1.m4s", first.URI)
	}
	third := mustRef(t, d, "v720", 3)
	if third.Start != 18*time.Second || third.End != 22*time.Second {
		t.Fatalf("segment 3 = [%v, %v), want [18s, 22s)", third.Start, third.End)
	}
	if third.RepresentationID != "v720" {
		t.Fatalf("segment 3 representation = %q, want v720", third.RepresentationID)
	}
}

// Numbers below the start or past the declared count are out of range.
func TestSegmentAtOutOfRange(t *testing.T) {
	d := newDash(t)
	if _, err := d.SegmentAt("v720", 0); !errors.Is(err, taskpkg.ErrSegmentOutOfRange) {
		t.Fatalf("number 0 err = %v, want ErrSegmentOutOfRange", err)
	}
	last := mustRef(t, d, "v720", 6)
	if last.Start != 30*time.Second || last.End != 34*time.Second {
		t.Fatalf("last segment = [%v, %v), want [30s, 34s)", last.Start, last.End)
	}
	if _, err := d.SegmentAt("v720", 7); !errors.Is(err, taskpkg.ErrSegmentOutOfRange) {
		t.Fatalf("number 7 err = %v, want ErrSegmentOutOfRange", err)
	}
	if _, err := d.SegmentAt("v1080", 11); !errors.Is(err, taskpkg.ErrSegmentOutOfRange) {
		t.Fatalf("v1080 number 11 err = %v, want ErrSegmentOutOfRange", err)
	}
}

// A wall-clock instant maps to the segment that contains it.
func TestSegmentForTimeContainment(t *testing.T) {
	d := newDash(t)
	cases := []struct {
		at   time.Duration
		want uint64
	}{
		{10 * time.Second, 1},
		{13999 * time.Millisecond, 1},
		{14 * time.Second, 2},
		{25500 * time.Millisecond, 4},
	}
	for _, c := range cases {
		ref, err := d.SegmentForTime("v720", c.at)
		if err != nil {
			t.Fatalf("SegmentForTime(%v): %v", c.at, err)
		}
		if ref.Number != c.want {
			t.Fatalf("SegmentForTime(%v) = segment %d, want %d", c.at, ref.Number, c.want)
		}
		if c.at < ref.Start || c.at >= ref.End {
			t.Fatalf("SegmentForTime(%v) returned [%v, %v) which does not contain it", c.at, ref.Start, ref.End)
		}
	}
}

// Instants before the presentation start and past the last segment are
// rejected with distinct sentinels.
func TestSegmentForTimeOutsideTimeline(t *testing.T) {
	d := newDash(t)
	if _, err := d.SegmentForTime("v720", 9*time.Second); !errors.Is(err, taskpkg.ErrBeforeStart) {
		t.Fatalf("before start err = %v, want ErrBeforeStart", err)
	}
	if _, err := d.SegmentForTime("v720", 34*time.Second); !errors.Is(err, taskpkg.ErrSegmentOutOfRange) {
		t.Fatalf("past end err = %v, want ErrSegmentOutOfRange", err)
	}
	if _, err := d.SegmentNumberAt("v720", 34*time.Second); !errors.Is(err, taskpkg.ErrSegmentOutOfRange) {
		t.Fatalf("SegmentNumberAt past end err = %v, want ErrSegmentOutOfRange", err)
	}
}

// SegmentNumberAt reports the same numbering as SegmentForTime.
func TestSegmentNumberAt(t *testing.T) {
	d := newDash(t)
	n, err := d.SegmentNumberAt("v720", 13500*time.Millisecond)
	if err != nil {
		t.Fatalf("SegmentNumberAt: %v", err)
	}
	if n != 1 {
		t.Fatalf("SegmentNumberAt(13.5s) = %d, want 1", n)
	}
	n, err = d.SegmentNumberAt("v720", 14*time.Second)
	if err != nil || n != 2 {
		t.Fatalf("SegmentNumberAt(14s) = (%d, %v), want (2, nil)", n, err)
	}
	if _, err := d.SegmentNumberAt("v720", 5*time.Second); !errors.Is(err, taskpkg.ErrBeforeStart) {
		t.Fatalf("SegmentNumberAt(5s) err = %v, want ErrBeforeStart", err)
	}
}

// Non-integer segment lengths are converted with the timeline timescale.
func TestSegmentDurationUsesTimescale(t *testing.T) {
	d := taskpkg.NewDashSegment(dashConfig())
	rep := taskpkg.Representation{
		ID:              "audio",
		Bandwidth:       128000,
		Timescale:       1000,
		SegmentDuration: 1500,
		StartNumber:     1,
		MaxSegments:     4,
	}
	if err := d.AddRepresentation(rep); err != nil {
		t.Fatalf("AddRepresentation: %v", err)
	}
	dur, err := d.SegmentDuration("audio")
	if err != nil {
		t.Fatalf("SegmentDuration: %v", err)
	}
	if dur != 1500*time.Millisecond {
		t.Fatalf("SegmentDuration = %v, want 1.5s", dur)
	}
	second := mustRef(t, d, "audio", 2)
	if second.Start != 1500*time.Millisecond || second.End != 3*time.Second {
		t.Fatalf("segment 2 = [%v, %v), want [1.5s, 3s)", second.Start, second.End)
	}
	if _, err := d.SegmentDuration("missing"); !errors.Is(err, taskpkg.ErrUnknownRepresentation) {
		t.Fatalf("SegmentDuration(missing) err = %v, want ErrUnknownRepresentation", err)
	}
}

// A ranged read starts at the segment containing the requested instant.
func TestAlignFromCoversRange(t *testing.T) {
	d := newDash(t)
	refs, err := d.AlignFrom("v720", 12*time.Second, 3)
	if err != nil {
		t.Fatalf("AlignFrom: %v", err)
	}
	if len(refs) != 3 {
		t.Fatalf("AlignFrom returned %d segments, want 3", len(refs))
	}
	for i, ref := range refs {
		want := uint64(i + 1)
		if ref.Number != want {
			t.Fatalf("refs[%d].Number = %d, want %d", i, ref.Number, want)
		}
	}
	if refs[2].Start != 18*time.Second || refs[2].End != 22*time.Second {
		t.Fatalf("third ref = [%v, %v), want [18s, 22s)", refs[2].Start, refs[2].End)
	}
}

// Ranged reads obey the per-query segment budget and the timeline end.
func TestAlignFromRejectsOverBudget(t *testing.T) {
	d := newDash(t)
	if _, err := d.AlignFrom("v720", 10*time.Second, 5); !errors.Is(err, taskpkg.ErrQueryTooLarge) {
		t.Fatalf("count 5 err = %v, want ErrQueryTooLarge", err)
	}
	if _, err := d.AlignFrom("v720", 10*time.Second, 0); !errors.Is(err, taskpkg.ErrQueryTooLarge) {
		t.Fatalf("count 0 err = %v, want ErrQueryTooLarge", err)
	}
	if _, err := d.AlignFrom("v720", 30*time.Second, 2); !errors.Is(err, taskpkg.ErrSegmentOutOfRange) {
		t.Fatalf("range past end err = %v, want ErrSegmentOutOfRange", err)
	}
	if _, err := d.AlignFrom("v720", 8*time.Second, 1); !errors.Is(err, taskpkg.ErrBeforeStart) {
		t.Fatalf("range before start err = %v, want ErrBeforeStart", err)
	}
}

// A switch starts at the first segment boundary of the target at or after the
// requested instant.
func TestSwitchRepresentationAlignsForward(t *testing.T) {
	d := newDash(t)
	ref, err := d.SwitchRepresentation("v720", "v1080", 13*time.Second)
	if err != nil {
		t.Fatalf("SwitchRepresentation: %v", err)
	}
	if ref.RepresentationID != "v1080" || ref.Number != 2 {
		t.Fatalf("switch target = %s/%d, want v1080/2", ref.RepresentationID, ref.Number)
	}
	if ref.Start != 16*time.Second || ref.End != 22*time.Second {
		t.Fatalf("switch target = [%v, %v), want [16s, 22s)", ref.Start, ref.End)
	}
	if d.Current() != "v1080" {
		t.Fatalf("Current = %q, want v1080", d.Current())
	}
	if _, err := d.SwitchRepresentation("v1080", "v1080", 20*time.Second); !errors.Is(err, taskpkg.ErrSameRepresentation) {
		t.Fatalf("same representation err = %v, want ErrSameRepresentation", err)
	}
	if d.Current() != "v1080" {
		t.Fatalf("Current = %q after refused switch, want v1080", d.Current())
	}
}

// Unknown representations and switch points outside the source timeline leave
// the selection untouched.
func TestSwitchRepresentationRejectsUnknownAndEarly(t *testing.T) {
	d := newDash(t)
	if _, err := d.SwitchRepresentation("v720", "v2160", 13*time.Second); !errors.Is(err, taskpkg.ErrUnknownRepresentation) {
		t.Fatalf("unknown target err = %v, want ErrUnknownRepresentation", err)
	}
	if _, err := d.SwitchRepresentation("v2160", "v1080", 13*time.Second); !errors.Is(err, taskpkg.ErrUnknownRepresentation) {
		t.Fatalf("unknown source err = %v, want ErrUnknownRepresentation", err)
	}
	if _, err := d.SwitchRepresentation("v720", "v1080", 9*time.Second); !errors.Is(err, taskpkg.ErrBeforeStart) {
		t.Fatalf("early switch err = %v, want ErrBeforeStart", err)
	}
	if d.Current() != "" {
		t.Fatalf("Current = %q after refused switches, want empty", d.Current())
	}
	if st := d.Stats(); st.Served != 0 {
		t.Fatalf("Stats.Served = %d after refused switches, want 0", st.Served)
	}
}

// Stats counts registered timelines, successful lookups and the selection.
func TestStatsCountsServed(t *testing.T) {
	d := newDash(t)
	st := d.Stats()
	if st.Representations != 2 || st.Current != "" || st.Served != 0 {
		t.Fatalf("initial Stats = %+v, want 2 reps, no current, 0 served", st)
	}
	mustRef(t, d, "v720", 1)
	if _, err := d.SegmentForTime("v720", 13*time.Second); err != nil {
		t.Fatalf("SegmentForTime: %v", err)
	}
	if _, err := d.SegmentAt("v720", 99); err == nil {
		t.Fatalf("SegmentAt(99) unexpectedly succeeded")
	}
	if st := d.Stats(); st.Served != 2 {
		t.Fatalf("Stats.Served = %d, want 2 after two successful lookups", st.Served)
	}
	if _, err := d.SwitchRepresentation("v720", "v1080", 13*time.Second); err != nil {
		t.Fatalf("SwitchRepresentation: %v", err)
	}
	if st := d.Stats(); st.Current != "v1080" {
		t.Fatalf("Stats.Current = %q, want v1080", st.Current)
	}
}

// Concurrent lookups during a switch always return a segment that really
// contains the requested instant.
func TestConcurrentLookupsDuringSwitch(t *testing.T) {
	d := newDash(t)
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	start := make(chan struct{})
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			rep := "v720"
			at := time.Duration(11+i%8) * time.Second
			if i%2 == 0 {
				rep = "v1080"
				at = time.Duration(11+i%20) * time.Second
			}
			ref, err := d.SegmentForTime(rep, at)
			if err != nil {
				errs <- "lookup failed: " + err.Error()
				return
			}
			if at < ref.Start || at >= ref.End {
				errs <- "segment does not contain the requested instant"
				return
			}
			if ref.RepresentationID != rep {
				errs <- "lookup returned a segment from another representation"
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		if _, err := d.SwitchRepresentation("v720", "v1080", 13*time.Second); err != nil {
			errs <- "switch failed: " + err.Error()
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatalf("concurrent lookup: %s", msg)
	}
}

// A restart keeps the selected representation and every segment boundary.
func TestSnapshotRestoreKeepsTimeline(t *testing.T) {
	d := newDash(t)
	if _, err := d.SwitchRepresentation("v720", "v1080", 13*time.Second); err != nil {
		t.Fatalf("SwitchRepresentation: %v", err)
	}
	snap, err := d.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewDashSegmentFromSnapshot(dashConfig(), snap)
	if err != nil {
		t.Fatalf("NewDashSegmentFromSnapshot: %v", err)
	}
	if restored.Current() != "v1080" {
		t.Fatalf("restored Current = %q, want v1080", restored.Current())
	}
	if restored.RepresentationCount() != 2 {
		t.Fatalf("restored RepresentationCount = %d, want 2", restored.RepresentationCount())
	}
	n, err := restored.SegmentNumberAt("v1080", 17*time.Second)
	if err != nil || n != 2 {
		t.Fatalf("restored SegmentNumberAt(17s) = (%d, %v), want (2, nil)", n, err)
	}
	if _, err := taskpkg.NewDashSegmentFromSnapshot(dashConfig(), []byte("{not json")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot err = %v, want ErrCorruptSnapshot", err)
	}
}

// Switching back to the coarser timeline picks the next common boundary
// instead of reusing the old position.
func TestSwitchBackRealigns(t *testing.T) {
	d := newDash(t)
	if _, err := d.SwitchRepresentation("v720", "v1080", 13*time.Second); err != nil {
		t.Fatalf("first switch: %v", err)
	}
	ref, err := d.SwitchRepresentation("v1080", "v720", 25*time.Second)
	if err != nil {
		t.Fatalf("switch back: %v", err)
	}
	if ref.RepresentationID != "v720" || ref.Number != 5 {
		t.Fatalf("switch back = %s/%d, want v720/5", ref.RepresentationID, ref.Number)
	}
	if ref.Start != 26*time.Second || ref.End != 30*time.Second {
		t.Fatalf("switch back segment = [%v, %v), want [26s, 30s)", ref.Start, ref.End)
	}
	if d.Current() != "v720" {
		t.Fatalf("Current = %q, want v720", d.Current())
	}
}
