package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-h-05"
)

var abrT0 = time.Unix(1700000000, 0)

func abrConfig() taskpkg.Config {
	return taskpkg.Config{
		MinRungs:       2,
		SettleWindow:   10 * time.Second,
		EWMAlpha:       0.5,
		UpSwitchBuffer: 5 * time.Second,
		MaxUpStep:      2,
	}
}

func abrRungs() []taskpkg.Rung {
	return []taskpkg.Rung{
		{ID: 1, Height: 360, Bandwidth: 800000},
		{ID: 2, Height: 720, Bandwidth: 2400000},
		{ID: 3, Height: 1080, Bandwidth: 5200000},
		{ID: 4, Height: 2160, Bandwidth: 8000000},
	}
}

func newAbr(t *testing.T) *taskpkg.AbrLadder {
	t.Helper()
	a := taskpkg.NewAbrLadder(abrConfig())
	if err := a.SetLadder(abrRungs()); err != nil {
		t.Fatalf("SetLadder: %v", err)
	}
	return a
}

// The ladder rejects malformed, duplicated, unordered and too-short input.
func TestSetLadderValidation(t *testing.T) {
	a := taskpkg.NewAbrLadder(abrConfig())
	bad := [][]taskpkg.Rung{
		{{ID: 0, Height: 360, Bandwidth: 800000}, {ID: 2, Height: 720, Bandwidth: 2400000}},
		{{ID: 1, Height: 0, Bandwidth: 800000}, {ID: 2, Height: 720, Bandwidth: 2400000}},
		{{ID: 1, Height: 360, Bandwidth: 0}, {ID: 2, Height: 720, Bandwidth: 2400000}},
		{{ID: 1, Height: 360, Bandwidth: 800000}, {ID: 1, Height: 720, Bandwidth: 2400000}},
		{{ID: 1, Height: 720, Bandwidth: 2400000}, {ID: 2, Height: 360, Bandwidth: 800000}},
		{{ID: 1, Height: 360, Bandwidth: 800000}},
	}
	want := []error{
		taskpkg.ErrInvalidRung,
		taskpkg.ErrInvalidRung,
		taskpkg.ErrInvalidRung,
		taskpkg.ErrDuplicateRung,
		taskpkg.ErrLadderOrder,
		taskpkg.ErrTooFewRungs,
	}
	for i, rungs := range bad {
		if err := a.SetLadder(rungs); !errors.Is(err, want[i]) {
			t.Fatalf("SetLadder(%d) err = %v, want %v", i, err, want[i])
		}
	}
	if len(a.Rungs()) != 0 {
		t.Fatalf("ladder kept %d rungs after rejected input", len(a.Rungs()))
	}
	if err := a.SetLadder(abrRungs()); err != nil {
		t.Fatalf("valid SetLadder: %v", err)
	}
	got := a.Rungs()
	if len(got) != 4 || got[0].ID != 1 || got[3].ID != 4 {
		t.Fatalf("ladder = %+v", got)
	}
	if st := a.Stats(); st.Rungs != 4 {
		t.Fatalf("Stats.Rungs = %d, want 4", st.Rungs)
	}
}

// Bandwidth lookup takes the highest rung that fits and never over-promises.
func TestRungForBandwidth(t *testing.T) {
	a := newAbr(t)
	r, err := a.RungForBandwidth(4000000)
	if err != nil || r.ID != 2 {
		t.Fatalf("RungForBandwidth(4M) = (%+v, %v), want rung 2", r, err)
	}
	r, err = a.RungForBandwidth(2400000)
	if err != nil || r.ID != 2 {
		t.Fatalf("RungForBandwidth(exact) = (%+v, %v), want rung 2", r, err)
	}
	r, err = a.RungForBandwidth(2399999)
	if err != nil || r.ID != 1 {
		t.Fatalf("RungForBandwidth(just under) = (%+v, %v), want rung 1", r, err)
	}
	if _, err := a.RungForBandwidth(600000); !errors.Is(err, taskpkg.ErrNoRungFits) {
		t.Fatalf("RungForBandwidth(below ladder) err = %v, want ErrNoRungFits", err)
	}
}

// Height lookup is used by the settings screen and must not round up.
func TestRungForHeight(t *testing.T) {
	a := newAbr(t)
	r, err := a.RungForHeight(800)
	if err != nil || r.ID != 2 {
		t.Fatalf("RungForHeight(800) = (%+v, %v), want rung 2", r, err)
	}
	r, err = a.RungForHeight(1080)
	if err != nil || r.ID != 3 {
		t.Fatalf("RungForHeight(1080) = (%+v, %v), want rung 3", r, err)
	}
	r, err = a.RungForHeight(9999)
	if err != nil || r.ID != 4 {
		t.Fatalf("RungForHeight(9999) = (%+v, %v), want rung 4", r, err)
	}
	if _, err := a.RungForHeight(100); !errors.Is(err, taskpkg.ErrNoRungFits) {
		t.Fatalf("RungForHeight(100) err = %v, want ErrNoRungFits", err)
	}
}

// The first sample seeds the estimate, later samples move it by the configured
// weight.
func TestObserveSampleMovingAverage(t *testing.T) {
	a := newAbr(t)
	if err := a.ObserveSample(3000000, abrT0); err != nil {
		t.Fatalf("first sample: %v", err)
	}
	if got := a.Estimate(); got != 3000000 {
		t.Fatalf("estimate after first sample = %d, want 3000000", got)
	}
	if err := a.ObserveSample(1000000, abrT0.Add(time.Second)); err != nil {
		t.Fatalf("second sample: %v", err)
	}
	if got := a.Estimate(); got != 2000000 {
		t.Fatalf("estimate after second sample = %d, want 2000000", got)
	}
	if st := a.Stats(); st.Samples != 2 {
		t.Fatalf("Stats.Samples = %d, want 2", st.Samples)
	}
}

// Zero or negative measurements are refused and do not move the estimate.
func TestObserveSampleRejectsNonPositive(t *testing.T) {
	a := newAbr(t)
	if err := a.ObserveSample(0, abrT0); !errors.Is(err, taskpkg.ErrInvalidSample) {
		t.Fatalf("zero sample err = %v, want ErrInvalidSample", err)
	}
	if err := a.ObserveSample(-1200, abrT0); !errors.Is(err, taskpkg.ErrInvalidSample) {
		t.Fatalf("negative sample err = %v, want ErrInvalidSample", err)
	}
	if st := a.Stats(); st.Samples != 0 || st.Estimate != 0 {
		t.Fatalf("Stats = %+v after rejected samples", st)
	}
}

// Deciding without a ladder is an error rather than a silent no-op.
func TestDecideWithoutLadder(t *testing.T) {
	a := taskpkg.NewAbrLadder(abrConfig())
	if _, _, err := a.Decide(10*time.Second, abrT0); !errors.Is(err, taskpkg.ErrLadderNotSet) {
		t.Fatalf("Decide without ladder err = %v, want ErrLadderNotSet", err)
	}
	if _, ok := a.Current(); ok {
		t.Fatalf("Current reported a rung without a ladder")
	}
}

// Climbing needs a filled buffer; the very same estimate is held back when the
// buffer is short.
func TestDecideUpSwitchNeedsBuffer(t *testing.T) {
	a := newAbr(t)
	if err := a.ObserveSample(5200000, abrT0); err != nil {
		t.Fatalf("ObserveSample: %v", err)
	}
	r, switched, err := a.Decide(3*time.Second, abrT0)
	if err != nil {
		t.Fatalf("Decide(short buffer): %v", err)
	}
	if switched || r.ID != 1 {
		t.Fatalf("Decide(short buffer) = (%+v, %v), want to stay on rung 1", r, switched)
	}
	r, switched, err = a.Decide(10*time.Second, abrT0)
	if err != nil {
		t.Fatalf("Decide(full buffer): %v", err)
	}
	if !switched || r.ID != 3 {
		t.Fatalf("Decide(full buffer) = (%+v, %v), want rung 3", r, switched)
	}
	if st := a.Stats(); st.Switches != 1 {
		t.Fatalf("Stats.Switches = %d, want 1", st.Switches)
	}
}

// A collapse in measured bandwidth downgrades right away, without waiting for
// the buffer.
func TestDecideDowngradesImmediately(t *testing.T) {
	a := newAbr(t)
	if err := a.ForceRung(3, abrT0.Add(-20*time.Second)); err != nil {
		t.Fatalf("ForceRung: %v", err)
	}
	if err := a.ObserveSample(900000, abrT0.Add(-19*time.Second)); err != nil {
		t.Fatalf("ObserveSample: %v", err)
	}
	r, switched, err := a.Decide(0, abrT0)
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if !switched || r.ID != 1 {
		t.Fatalf("Decide(downgrade) = (%+v, %v), want rung 1", r, switched)
	}
}

// Switches respect the settle window and report the remaining cooldown.
func TestDecideRespectsSettleWindow(t *testing.T) {
	a := newAbr(t)
	if err := a.ObserveSample(6000000, abrT0); err != nil {
		t.Fatalf("ObserveSample: %v", err)
	}
	if _, switched, err := a.Decide(10*time.Second, abrT0); err != nil || !switched {
		t.Fatalf("first climb = (%v, %v), want a switch", switched, err)
	}
	if err := a.ObserveSample(500000, abrT0.Add(time.Second)); err != nil {
		t.Fatalf("second sample: %v", err)
	}
	r, switched, err := a.Decide(10*time.Second, abrT0.Add(5*time.Second))
	if err != nil {
		t.Fatalf("Decide during cooldown: %v", err)
	}
	if switched {
		t.Fatalf("Decide switched during the settle window to %+v", r)
	}
	if cd := a.Cooldown(abrT0.Add(5 * time.Second)); cd != 5*time.Second {
		t.Fatalf("Cooldown = %v, want 5s", cd)
	}
	r, switched, err = a.Decide(10*time.Second, abrT0.Add(11*time.Second))
	if err != nil {
		t.Fatalf("Decide after cooldown: %v", err)
	}
	if !switched || r.ID != 2 {
		t.Fatalf("Decide after cooldown = (%+v, %v), want rung 2", r, switched)
	}
	if cd := a.Cooldown(abrT0.Add(11 * time.Second)); cd != 10*time.Second {
		t.Fatalf("Cooldown right after a switch = %v, want a full 10s", cd)
	}
	if cd := a.Cooldown(abrT0.Add(21 * time.Second)); cd != 0 {
		t.Fatalf("Cooldown one window after the switch = %v, want 0", cd)
	}
}

// One switch climbs at most MaxUpStep rungs even when the link allows more.
func TestDecideClimbsOneStepAtATime(t *testing.T) {
	a := newAbr(t)
	if err := a.ObserveSample(9000000, abrT0); err != nil {
		t.Fatalf("ObserveSample: %v", err)
	}
	r, switched, err := a.Decide(10*time.Second, abrT0)
	if err != nil || !switched || r.ID != 3 {
		t.Fatalf("first climb = (%+v, %v, %v), want rung 3", r, switched, err)
	}
	r, switched, err = a.Decide(10*time.Second, abrT0.Add(11*time.Second))
	if err != nil || !switched || r.ID != 4 {
		t.Fatalf("second climb = (%+v, %v, %v), want rung 4", r, switched, err)
	}
}

// Forcing a rung validates the id and does not double-count a repeated pin.
func TestForceRungValidation(t *testing.T) {
	a := newAbr(t)
	if err := a.ForceRung(99, abrT0); !errors.Is(err, taskpkg.ErrUnknownRung) {
		t.Fatalf("ForceRung(99) err = %v, want ErrUnknownRung", err)
	}
	if st := a.Stats(); st.Switches != 0 {
		t.Fatalf("Stats.Switches = %d after a rejected pin", st.Switches)
	}
	if err := a.ForceRung(3, abrT0); err != nil {
		t.Fatalf("ForceRung(3): %v", err)
	}
	r, ok := a.Current()
	if !ok || r.ID != 3 {
		t.Fatalf("Current = (%+v, %v), want rung 3", r, ok)
	}
	if err := a.ForceRung(3, abrT0.Add(time.Second)); err != nil {
		t.Fatalf("repeat ForceRung(3): %v", err)
	}
	if st := a.Stats(); st.Switches != 1 {
		t.Fatalf("Stats.Switches = %d after repeat pin, want 1", st.Switches)
	}
}

// Cooldown is zero before any switch and expires with the settle window.
func TestCooldownWindow(t *testing.T) {
	a := newAbr(t)
	if cd := a.Cooldown(abrT0); cd != 0 {
		t.Fatalf("Cooldown before any switch = %v, want 0", cd)
	}
	if err := a.ForceRung(2, abrT0); err != nil {
		t.Fatalf("ForceRung: %v", err)
	}
	if cd := a.Cooldown(abrT0.Add(2 * time.Second)); cd != 8*time.Second {
		t.Fatalf("Cooldown(+2s) = %v, want 8s", cd)
	}
	if cd := a.Cooldown(abrT0.Add(10 * time.Second)); cd != 0 {
		t.Fatalf("Cooldown(at window end) = %v, want 0", cd)
	}
	if cd := a.Cooldown(abrT0.Add(30 * time.Second)); cd != 0 {
		t.Fatalf("Cooldown(long after) = %v, want 0", cd)
	}
}

// Stats reports the ladder, the pinned rung, the estimate and both counters.
func TestStatsReflectsState(t *testing.T) {
	a := newAbr(t)
	if err := a.ObserveSample(1000000, abrT0); err != nil {
		t.Fatalf("first sample: %v", err)
	}
	if err := a.ObserveSample(3000000, abrT0.Add(time.Second)); err != nil {
		t.Fatalf("second sample: %v", err)
	}
	if err := a.ForceRung(2, abrT0.Add(2*time.Second)); err != nil {
		t.Fatalf("ForceRung: %v", err)
	}
	st := a.Stats()
	if st.Rungs != 4 || st.Current != 2 || st.Estimate != 2000000 || st.Samples != 2 || st.Switches != 1 {
		t.Fatalf("Stats = %+v", st)
	}
	if got := a.Estimate(); got != 2000000 {
		t.Fatalf("Estimate = %d, want 2000000", got)
	}
}

// Samples arriving concurrently are all counted and leave the estimate at the
// measurement value.
func TestConcurrentSamplesAndReads(t *testing.T) {
	a := newAbr(t)
	var wg sync.WaitGroup
	errs := make(chan string, 40)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.ObserveSample(2000000, abrT0); err != nil {
				errs <- "sample failed: " + err.Error()
			}
		}()
	}
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if est := a.Estimate(); est != 0 && est != 2000000 {
				errs <- "estimate took a partial value"
				return
			}
			if st := a.Stats(); st.Rungs != 4 {
				errs <- "stats lost the ladder"
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatalf("concurrent sample: %s", msg)
	}
	st := a.Stats()
	if st.Samples != 100 {
		t.Fatalf("Stats.Samples = %d, want 100", st.Samples)
	}
	if st.Estimate != 2000000 {
		t.Fatalf("Stats.Estimate = %d, want 2000000", st.Estimate)
	}
}

// A restart keeps the estimate, the current rung, the counters and the
// remaining cooldown.
func TestSnapshotRestoreKeepsState(t *testing.T) {
	a := newAbr(t)
	if err := a.ObserveSample(1000000, abrT0); err != nil {
		t.Fatalf("first sample: %v", err)
	}
	if err := a.ObserveSample(3000000, abrT0.Add(time.Second)); err != nil {
		t.Fatalf("second sample: %v", err)
	}
	if err := a.ForceRung(3, abrT0); err != nil {
		t.Fatalf("ForceRung: %v", err)
	}
	snap, err := a.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewAbrLadderFromSnapshot(abrConfig(), snap)
	if err != nil {
		t.Fatalf("NewAbrLadderFromSnapshot: %v", err)
	}
	st := restored.Stats()
	if st.Rungs != 4 || st.Current != 3 || st.Estimate != 2000000 || st.Samples != 2 || st.Switches != 1 {
		t.Fatalf("restored Stats = %+v", st)
	}
	r, ok := restored.Current()
	if !ok || r.ID != 3 {
		t.Fatalf("restored Current = (%+v, %v), want rung 3", r, ok)
	}
	if cd := restored.Cooldown(abrT0.Add(5 * time.Second)); cd != 5*time.Second {
		t.Fatalf("restored Cooldown(+5s) = %v, want 5s", cd)
	}
	if err := restored.ObserveSample(5000000, abrT0.Add(6*time.Second)); err != nil {
		t.Fatalf("sample after restore: %v", err)
	}
	if got := restored.Estimate(); got != 3500000 {
		t.Fatalf("estimate after restore = %d, want 3500000", got)
	}
	if _, err := taskpkg.NewAbrLadderFromSnapshot(abrConfig(), []byte("]]")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot err = %v, want ErrCorruptSnapshot", err)
	}
}
