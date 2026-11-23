package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-h-01"
)

const (
	hlsTarget = 4 * time.Second
	hlsT0     = 1700000000 // seconds since epoch
)

func hlsConfig() taskpkg.Config {
	return taskpkg.Config{
		TargetDuration: hlsTarget,
		MaxWindow:      6,
		PDTTolerance:   250 * time.Millisecond,
	}
}

// newHls builds a playlist with three renditions and returns it.
func newHls(t *testing.T) *taskpkg.HlsPlaylist {
	t.Helper()
	p := taskpkg.NewHlsPlaylist(hlsConfig())
	for _, v := range []taskpkg.Variant{
		{URI: "480p/index.m3u8", Bandwidth: 800000, Height: 360},
		{URI: "720p/index.m3u8", Bandwidth: 2400000, Height: 720},
		{URI: "1080p/index.m3u8", Bandwidth: 5200000, Height: 1080},
	} {
		if err := p.AddVariant(v); err != nil {
			t.Fatalf("AddVariant(%s): %v", v.URI, err)
		}
	}
	return p
}

func hlsSeg(seq uint64, pdt time.Time) taskpkg.Segment {
	return taskpkg.Segment{
		MediaSequence: seq,
		URI:           "seg-" + itoa(seq) + ".ts",
		Duration:      hlsTarget,
		PDT:           pdt,
	}
}

func itoa(v uint64) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func mustAppend(t *testing.T, p *taskpkg.HlsPlaylist, seg taskpkg.Segment) {
	t.Helper()
	if err := p.AppendSegment(seg); err != nil {
		t.Fatalf("AppendSegment(%d): %v", seg.MediaSequence, err)
	}
}

// The selected rendition is the highest bandwidth that fits under the cap.
func TestSelectVariantHighestWithinCap(t *testing.T) {
	p := newHls(t)
	got, err := p.SelectVariant(4000000, 0)
	if err != nil {
		t.Fatalf("SelectVariant: %v", err)
	}
	if got.URI != "720p/index.m3u8" {
		t.Fatalf("SelectVariant(4Mbps) picked %q, want 720p", got.URI)
	}
	if got.Bandwidth != 2400000 {
		t.Fatalf("SelectVariant bandwidth = %d, want 2400000", got.Bandwidth)
	}
	if p.VariantCount() != 3 {
		t.Fatalf("VariantCount = %d, want 3", p.VariantCount())
	}
}

// A height cap must remove renditions above it even when bandwidth allows them.
func TestSelectVariantRespectsHeightCap(t *testing.T) {
	p := newHls(t)
	got, err := p.SelectVariant(10000000, 720)
	if err != nil {
		t.Fatalf("SelectVariant with height cap: %v", err)
	}
	if got.URI != "720p/index.m3u8" {
		t.Fatalf("height-capped choice = %q, want 720p", got.URI)
	}
	got, err = p.SelectVariant(10000000, 0)
	if err != nil {
		t.Fatalf("SelectVariant without cap: %v", err)
	}
	if got.URI != "1080p/index.m3u8" {
		t.Fatalf("uncapped choice = %q, want 1080p", got.URI)
	}
}

// No rendition below the measured bandwidth is a reported failure, not a
// silent downgrade to the lowest one.
func TestSelectVariantNoFit(t *testing.T) {
	p := newHls(t)
	if _, err := p.SelectVariant(600000, 0); !errors.Is(err, taskpkg.ErrNoVariantFit) {
		t.Fatalf("SelectVariant(600kbps) err = %v, want ErrNoVariantFit", err)
	}
	empty := taskpkg.NewHlsPlaylist(hlsConfig())
	if _, err := empty.SelectVariant(9000000, 0); !errors.Is(err, taskpkg.ErrNoVariantFit) {
		t.Fatalf("empty ladder err = %v, want ErrNoVariantFit", err)
	}
}

// Registering the same URI twice must be refused and must not grow the ladder.
func TestAddVariantRejectsDuplicateAndInvalid(t *testing.T) {
	p := taskpkg.NewHlsPlaylist(hlsConfig())
	if err := p.AddVariant(taskpkg.Variant{URI: "a/index.m3u8", Bandwidth: 1200000, Height: 540}); err != nil {
		t.Fatalf("first AddVariant: %v", err)
	}
	if err := p.AddVariant(taskpkg.Variant{URI: "a/index.m3u8", Bandwidth: 1800000, Height: 720}); !errors.Is(err, taskpkg.ErrDuplicateVariant) {
		t.Fatalf("duplicate URI err = %v, want ErrDuplicateVariant", err)
	}
	if err := p.AddVariant(taskpkg.Variant{URI: "", Bandwidth: 900000}); !errors.Is(err, taskpkg.ErrInvalidVariant) {
		t.Fatalf("empty URI err = %v, want ErrInvalidVariant", err)
	}
	if err := p.AddVariant(taskpkg.Variant{URI: "b/index.m3u8", Bandwidth: 0}); !errors.Is(err, taskpkg.ErrInvalidVariant) {
		t.Fatalf("zero bandwidth err = %v, want ErrInvalidVariant", err)
	}
	if p.VariantCount() != 1 {
		t.Fatalf("VariantCount = %d, want 1", p.VariantCount())
	}
}

// A segment longer than the target duration is rejected before it enters the
// window, and the next expected sequence stays unchanged.
func TestAppendSegmentRejectsDurationOverTarget(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	mustAppend(t, p, hlsSeg(100, t0))
	tooLong := hlsSeg(101, t0.Add(hlsTarget))
	tooLong.Duration = 4500 * time.Millisecond
	if err := p.AppendSegment(tooLong); !errors.Is(err, taskpkg.ErrSegmentDuration) {
		t.Fatalf("overlong segment err = %v, want ErrSegmentDuration", err)
	}
	zero := hlsSeg(101, t0.Add(hlsTarget))
	zero.Duration = 0
	if err := p.AppendSegment(zero); !errors.Is(err, taskpkg.ErrSegmentDuration) {
		t.Fatalf("zero-length segment err = %v, want ErrSegmentDuration", err)
	}
	if len(p.Window()) != 1 {
		t.Fatalf("window grew to %d after rejected appends", len(p.Window()))
	}
	next, err := p.MediaSequence()
	if err != nil {
		t.Fatalf("MediaSequence: %v", err)
	}
	if next != 101 {
		t.Fatalf("MediaSequence = %d, want 101", next)
	}
}

// A gap in the media sequence must be refused; strictly the next sequence is
// accepted.
func TestAppendSegmentRejectsSequenceGap(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	mustAppend(t, p, hlsSeg(100, t0))
	if err := p.AppendSegment(hlsSeg(102, t0.Add(2*hlsTarget))); !errors.Is(err, taskpkg.ErrSegmentOrder) {
		t.Fatalf("sequence gap err = %v, want ErrSegmentOrder", err)
	}
	rewound := hlsSeg(100, t0.Add(hlsTarget))
	rewound.URI = "seg-100-repack.ts"
	if err := p.AppendSegment(rewound); !errors.Is(err, taskpkg.ErrSegmentOrder) {
		t.Fatalf("rewound sequence err = %v, want ErrSegmentOrder", err)
	}
	if len(p.Window()) != 1 {
		t.Fatalf("window has %d segments after rejected appends", len(p.Window()))
	}
}

// Re-appending the newest segment verbatim is reported as a duplicate, which
// callers use to acknowledge a reload that returned the same playlist.
func TestAppendSegmentRejectsDuplicate(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	mustAppend(t, p, hlsSeg(100, t0))
	if err := p.AppendSegment(hlsSeg(100, t0)); !errors.Is(err, taskpkg.ErrDuplicateSegment) {
		t.Fatalf("duplicate segment err = %v, want ErrDuplicateSegment", err)
	}
	if len(p.Window()) != 1 {
		t.Fatalf("duplicate append grew the window to %d", len(p.Window()))
	}
}

// The window keeps only the newest MaxWindow segments and the statistics
// describe exactly that window.
func TestWindowKeepsNewestWithinLimit(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	for i := uint64(0); i < 5; i++ {
		mustAppend(t, p, hlsSeg(100+i, t0.Add(time.Duration(i)*hlsTarget)))
	}
	w := p.Window()
	if len(w) != 5 {
		t.Fatalf("window length = %d, want 5", len(w))
	}
	if w[0].MediaSequence != 100 || w[4].MediaSequence != 104 {
		t.Fatalf("window sequences = %d..%d, want 100..104", w[0].MediaSequence, w[4].MediaSequence)
	}
	st := p.Stats()
	if st.Segments != 5 || st.Gaps != 0 {
		t.Fatalf("Stats = %+v, want 5 segments and no gaps", st)
	}
	if st.FirstSequence != 100 || st.LastSequence != 104 {
		t.Fatalf("Stats sequences = %d..%d, want 100..104", st.FirstSequence, st.LastSequence)
	}
	if st.Duration != 5*hlsTarget {
		t.Fatalf("Stats duration = %v, want %v", st.Duration, 5*hlsTarget)
	}
}

// Switching rendition advances the discontinuity sequence once and every later
// segment carries it; re-selecting the same rendition does not advance again.
func TestSetVariantBumpsDiscontinuity(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	mustAppend(t, p, hlsSeg(100, t0))
	if err := p.SetVariant("720p/index.m3u8", t0); err != nil {
		t.Fatalf("SetVariant(720p): %v", err)
	}
	if p.Current() != "720p/index.m3u8" {
		t.Fatalf("Current = %q, want 720p", p.Current())
	}
	if p.DiscontinuitySequence() != 1 {
		t.Fatalf("discontinuity = %d, want 1 after a switch", p.DiscontinuitySequence())
	}
	mustAppend(t, p, hlsSeg(101, t0.Add(hlsTarget)))
	w := p.Window()
	if w[0].DiscontinuitySeq != 0 {
		t.Fatalf("pre-switch segment discontinuity = %d, want 0", w[0].DiscontinuitySeq)
	}
	if w[len(w)-1].DiscontinuitySeq != 1 {
		t.Fatalf("post-switch segment discontinuity = %d, want 1", w[len(w)-1].DiscontinuitySeq)
	}
	if err := p.SetVariant("720p/index.m3u8", t0.Add(2*hlsTarget)); err != nil {
		t.Fatalf("re-select same variant: %v", err)
	}
	if p.DiscontinuitySequence() != 1 {
		t.Fatalf("idempotent switch changed discontinuity to %d", p.DiscontinuitySequence())
	}
}

// Switching to an unregistered rendition leaves the whole playlist untouched.
func TestSetVariantUnknownKeepsState(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	if err := p.SetVariant("1080p/index.m3u8", t0); err != nil {
		t.Fatalf("SetVariant(1080p): %v", err)
	}
	if err := p.SetVariant("2160p/index.m3u8", t0.Add(hlsTarget)); !errors.Is(err, taskpkg.ErrUnknownVariant) {
		t.Fatalf("unknown variant err = %v, want ErrUnknownVariant", err)
	}
	if p.Current() != "1080p/index.m3u8" {
		t.Fatalf("Current = %q after failed switch, want 1080p", p.Current())
	}
	if p.DiscontinuitySequence() != 1 {
		t.Fatalf("discontinuity = %d after failed switch, want 1", p.DiscontinuitySequence())
	}
	mustAppend(t, p, hlsSeg(100, t0))
	w := p.Window()
	if len(w) != 1 || w[0].DiscontinuitySeq != 1 {
		t.Fatalf("segment after failed switch = %+v, want discontinuity 1", w)
	}
}

// Program date time that jumps backwards beyond the tolerance is rejected; a
// small overlap inside the tolerance is accepted.
func TestPDTRegressionRejectedWithinTolerance(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	mustAppend(t, p, hlsSeg(100, t0))
	back := hlsSeg(101, t0.Add(3500*time.Millisecond))
	if err := p.AppendSegment(back); !errors.Is(err, taskpkg.ErrPDTRegression) {
		t.Fatalf("regressed PDT err = %v, want ErrPDTRegression", err)
	}
	if len(p.Window()) != 1 {
		t.Fatalf("window grew to %d after rejected PDT", len(p.Window()))
	}
	mustAppend(t, p, hlsSeg(101, t0.Add(3900*time.Millisecond)))
	if len(p.Window()) != 2 {
		t.Fatalf("window length = %d, want 2", len(p.Window()))
	}
}

// The live edge delay is measured from the end of the newest segment and never
// goes negative.
func TestLiveEdgeDelay(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	if d := p.LiveEdgeDelay(t0); d != 0 {
		t.Fatalf("LiveEdgeDelay on empty playlist = %v, want 0", d)
	}
	mustAppend(t, p, hlsSeg(100, t0))
	if d := p.LiveEdgeDelay(t0.Add(10 * time.Second)); d != 6*time.Second {
		t.Fatalf("LiveEdgeDelay(+10s) = %v, want 6s", d)
	}
	if d := p.LiveEdgeDelay(t0.Add(2 * time.Second)); d != 0 {
		t.Fatalf("LiveEdgeDelay(ahead of edge) = %v, want 0", d)
	}
}

// Trimming obeys the configured window ceiling and reports how many segments
// left the window.
func TestTrimWindowEnforcesConfiguredCap(t *testing.T) {
	cfg := hlsConfig()
	cfg.MaxWindow = 5
	p := taskpkg.NewHlsPlaylist(cfg)
	t0 := time.Unix(hlsT0, 0)
	for i := uint64(0); i < 5; i++ {
		mustAppend(t, p, hlsSeg(100+i, t0.Add(time.Duration(i)*hlsTarget)))
	}
	if _, err := p.TrimWindow(6); !errors.Is(err, taskpkg.ErrWindowTooLarge) {
		t.Fatalf("TrimWindow(6) err = %v, want ErrWindowTooLarge", err)
	}
	if _, err := p.TrimWindow(0); !errors.Is(err, taskpkg.ErrWindowTooLarge) {
		t.Fatalf("TrimWindow(0) err = %v, want ErrWindowTooLarge", err)
	}
	removed, err := p.TrimWindow(2)
	if err != nil {
		t.Fatalf("TrimWindow(2): %v", err)
	}
	if removed != 3 {
		t.Fatalf("TrimWindow(2) removed %d, want 3", removed)
	}
	w := p.Window()
	if len(w) != 2 || w[0].MediaSequence != 103 || w[1].MediaSequence != 104 {
		t.Fatalf("window after trim = %d segments starting at %d", len(w), w[0].MediaSequence)
	}
	removed, err = p.TrimWindow(2)
	if err != nil || removed != 0 {
		t.Fatalf("second TrimWindow(2) = (%d, %v), want (0, nil)", removed, err)
	}
}

// Readers racing with each other always observe a complete, trimmed window.
func TestConcurrentWindowReadsAreConsistent(t *testing.T) {
	cfg := hlsConfig()
	cfg.MaxWindow = 8
	p := taskpkg.NewHlsPlaylist(cfg)
	t0 := time.Unix(hlsT0, 0)
	for i := uint64(0); i < 20; i++ {
		mustAppend(t, p, hlsSeg(100+i, t0.Add(time.Duration(i)*hlsTarget)))
	}
	var wg sync.WaitGroup
	errs := make(chan string, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w := p.Window()
			if len(w) != 8 {
				errs <- "window length changed during concurrent reads"
				return
			}
			if w[7].MediaSequence != 119 {
				errs <- "newest segment missing during concurrent reads"
				return
			}
			if st := p.Stats(); st.Segments != 8 || st.LastSequence != 119 {
				errs <- "stats disagreed with the window during concurrent reads"
				return
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatalf("concurrent read: %s", msg)
	}
}

// A restart must preserve the window, the discontinuity numbering and duplicate
// rejection.
func TestSnapshotRestoreKeepsState(t *testing.T) {
	p := newHls(t)
	t0 := time.Unix(hlsT0, 0)
	for i := uint64(0); i < 3; i++ {
		mustAppend(t, p, hlsSeg(100+i, t0.Add(time.Duration(i)*hlsTarget)))
	}
	if err := p.SetVariant("1080p/index.m3u8", t0.Add(3*hlsTarget)); err != nil {
		t.Fatalf("SetVariant: %v", err)
	}
	snap, err := p.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewHlsPlaylistFromSnapshot(hlsConfig(), snap)
	if err != nil {
		t.Fatalf("NewHlsPlaylistFromSnapshot: %v", err)
	}
	next, err := restored.MediaSequence()
	if err != nil || next != 103 {
		t.Fatalf("restored MediaSequence = (%d, %v), want (103, nil)", next, err)
	}
	if restored.DiscontinuitySequence() != 1 {
		t.Fatalf("restored discontinuity = %d, want 1", restored.DiscontinuitySequence())
	}
	if w := restored.Window(); len(w) != 3 || w[0].MediaSequence != 100 {
		t.Fatalf("restored window = %d segments, want 3 starting at 100", len(w))
	}
	if err := restored.AppendSegment(hlsSeg(102, t0.Add(2*hlsTarget))); !errors.Is(err, taskpkg.ErrDuplicateSegment) {
		t.Fatalf("duplicate after restore err = %v, want ErrDuplicateSegment", err)
	}
	post := hlsSeg(103, t0.Add(3*hlsTarget))
	if err := restored.AppendSegment(post); err != nil {
		t.Fatalf("append after restore: %v", err)
	}
	w := restored.Window()
	if w[len(w)-1].DiscontinuitySeq != 1 {
		t.Fatalf("segment after restore discontinuity = %d, want 1", w[len(w)-1].DiscontinuitySeq)
	}
}
