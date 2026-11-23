package task

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-g-02"
)

func limiterNear(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func limiterTestConfig() taskpkg.Config {
	return taskpkg.Config{
		SampleRate:      48000,
		Channels:        1,
		LookaheadMS:     1.5,
		ReleaseMS:       50,
		CeilingDBFS:     -0.3,
		MaxBlockSamples: 4096,
	}
}

func linearCeiling() float64 {
	return math.Pow(10, -0.3/20)
}

func filledBlock(n int, v float64) []float64 {
	b := make([]float64, n)
	for i := range b {
		b[i] = v
	}
	return b
}

func maxAbs(xs []float64) float64 {
	m := 0.0
	for _, x := range xs {
		if a := math.Abs(x); a > m {
			m = a
		}
	}
	return m
}

func processOne(t *testing.T, l *taskpkg.DspLimiter, in []float64) []float64 {
	t.Helper()
	out := make([]float64, len(in))
	if err := l.ProcessBlock([][]float64{in}, [][]float64{out}); err != nil {
		t.Fatalf("ProcessBlock returned error: %v", err)
	}
	return out
}

func TestLatencySamplesFromLookahead(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	if got := l.LatencySamples(); got != 72 {
		t.Fatalf("LatencySamples = %d, want 72", got)
	}
}

func TestConfigValidatedAndLookaheadClamped(t *testing.T) {
	bad := limiterTestConfig()
	bad.SampleRate = 0
	if _, err := taskpkg.NewDspLimiter(bad); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewDspLimiter(sample rate 0) error = %v, want ErrInvalidConfig", err)
	}
	cfg := limiterTestConfig()
	cfg.LookaheadMS = 10
	l, err := taskpkg.NewDspLimiter(cfg)
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	if got := l.LatencySamples(); got != 240 {
		t.Fatalf("clamped LatencySamples = %d, want 240", got)
	}
}

func TestBelowCeilingPassesAfterLatency(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	out := processOne(t, l, filledBlock(300, 0.5))
	for i := 0; i < 72; i++ {
		if out[i] != 0 {
			t.Fatalf("output[%d] = %v, want lookahead silence", i, out[i])
		}
	}
	for _, i := range []int{72, 150, 299} {
		if !limiterNear(out[i], 0.5, 1e-12) {
			t.Fatalf("output[%d] = %v, want 0.5", i, out[i])
		}
	}
}

func TestConstantOverloadReducedToCeiling(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	out := processOne(t, l, filledBlock(300, 2.0))
	if !limiterNear(out[72], linearCeiling(), 1e-9) {
		t.Fatalf("limited sample = %v, want ceiling %v", out[72], linearCeiling())
	}
	if !limiterNear(out[299], linearCeiling(), 1e-9) {
		t.Fatalf("steady limited sample = %v, want ceiling %v", out[299], linearCeiling())
	}
}

func TestPeakNeverExceedsCeiling(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	in := filledBlock(500, 0.1)
	in[0] = 4.0
	out := processOne(t, l, in)
	peak := maxAbs(out)
	if peak > linearCeiling()+1e-9 {
		t.Fatalf("peak output = %v, exceeds ceiling %v", peak, linearCeiling())
	}
	if peak < 0.9*linearCeiling() {
		t.Fatalf("peak output = %v, want the impulse limited close to %v", peak, linearCeiling())
	}
}

func TestGainReductionAfterOverload(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	_ = processOne(t, l, filledBlock(300, 4.0))
	want := 20 * math.Log10(linearCeiling()/4.0)
	got := l.GainReductionDB()
	if !limiterNear(got, want, 0.05) {
		t.Fatalf("GainReductionDB = %v, want %v", got, want)
	}
}

func TestMaxGainReductionTracksWorstCase(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	_ = processOne(t, l, filledBlock(300, 4.0))
	want := 20 * math.Log10(linearCeiling()/4.0)
	if got := l.MaxGainReductionDB(); !limiterNear(got, want, 0.05) {
		t.Fatalf("MaxGainReductionDB = %v, want %v", got, want)
	}
	_ = processOne(t, l, filledBlock(400, 0.2))
	if got := l.MaxGainReductionDB(); !limiterNear(got, want, 0.05) {
		t.Fatalf("MaxGainReductionDB forgot the earlier peak: %v, want %v", got, want)
	}
}

func TestReleaseRecoversGain(t *testing.T) {
	cfg := limiterTestConfig()
	cfg.ReleaseMS = 50
	l, err := taskpkg.NewDspLimiter(cfg)
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	_ = processOne(t, l, filledBlock(300, 4.0))
	if gr := l.GainReductionDB(); gr > -10 {
		t.Fatalf("gain reduction before release = %v, want below -10 dB", gr)
	}
	for i := 0; i < 12; i++ {
		_ = processOne(t, l, filledBlock(4000, 0))
	}
	if gr := l.GainReductionDB(); gr < -0.5 {
		t.Fatalf("gain reduction after one second = %v, want above -0.5 dB", gr)
	}
}

func TestResetClearsGainReduction(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	_ = processOne(t, l, filledBlock(300, 4.0))
	l.Reset()
	if gr := l.GainReductionDB(); gr != 0 {
		t.Fatalf("GainReductionDB after Reset = %v, want 0", gr)
	}
	if mgr := l.MaxGainReductionDB(); mgr != 0 {
		t.Fatalf("MaxGainReductionDB after Reset = %v, want 0", mgr)
	}
	if got := l.LatencySamples(); got != 72 {
		t.Fatalf("LatencySamples after Reset = %d, want 72", got)
	}
}

func TestStereoLinkAppliesSharedGain(t *testing.T) {
	cfg := limiterTestConfig()
	cfg.Channels = 2
	cfg.StereoLink = true
	l, err := taskpkg.NewDspLimiter(cfg)
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	left := filledBlock(300, 4.0)
	right := filledBlock(300, 0.5)
	outL := make([]float64, 300)
	outR := make([]float64, 300)
	if err := l.ProcessBlock([][]float64{left, right}, [][]float64{outL, outR}); err != nil {
		t.Fatalf("ProcessBlock returned error: %v", err)
	}
	if !limiterNear(outL[299], linearCeiling(), 1e-9) {
		t.Fatalf("linked loud channel = %v, want %v", outL[299], linearCeiling())
	}
	wantR := 0.5 * linearCeiling() / 4.0
	if !limiterNear(outR[299], wantR, 1e-9) {
		t.Fatalf("linked quiet channel = %v, want %v", outR[299], wantR)
	}
}

func TestStereoUnlinkedLeavesQuietChannel(t *testing.T) {
	cfg := limiterTestConfig()
	cfg.Channels = 2
	cfg.StereoLink = false
	l, err := taskpkg.NewDspLimiter(cfg)
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	outL := make([]float64, 300)
	outR := make([]float64, 300)
	if err := l.ProcessBlock(
		[][]float64{filledBlock(300, 4.0), filledBlock(300, 0.5)},
		[][]float64{outL, outR},
	); err != nil {
		t.Fatalf("ProcessBlock returned error: %v", err)
	}
	if !limiterNear(outL[299], linearCeiling(), 1e-9) {
		t.Fatalf("unlinked loud channel = %v, want %v", outL[299], linearCeiling())
	}
	if !limiterNear(outR[299], 0.5, 1e-12) {
		t.Fatalf("unlinked quiet channel = %v, want 0.5", outR[299])
	}
}

func TestChannelMismatchRejected(t *testing.T) {
	cfg := limiterTestConfig()
	cfg.Channels = 2
	l, err := taskpkg.NewDspLimiter(cfg)
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	err = l.ProcessBlock([][]float64{filledBlock(64, 0.1)}, [][]float64{make([]float64, 64)})
	if !errors.Is(err, taskpkg.ErrChannelMismatch) {
		t.Fatalf("ProcessBlock with one row error = %v, want ErrChannelMismatch", err)
	}
}

func TestBlockTooLargeRejected(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	err = l.ProcessBlock([][]float64{filledBlock(5000, 0.1)}, [][]float64{make([]float64, 5000)})
	if !errors.Is(err, taskpkg.ErrBlockTooLarge) {
		t.Fatalf("oversized ProcessBlock error = %v, want ErrBlockTooLarge", err)
	}
}

func TestSnapshotRestoreContinuesOutput(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	_ = processOne(t, l, filledBlock(400, 3.0))
	data, err := l.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	next := filledBlock(200, 3.0)
	first := processOne(t, l, next)
	restored, err := taskpkg.NewDspLimiterFromSnapshot(limiterTestConfig(), data)
	if err != nil {
		t.Fatalf("NewDspLimiterFromSnapshot returned error: %v", err)
	}
	second := processOne(t, restored, next)
	for i := range first {
		if !limiterNear(first[i], second[i], 1e-9) {
			t.Fatalf("sample %d after restore = %v, want %v", i, second[i], first[i])
		}
	}
}

func TestConcurrentParameterUpdates(t *testing.T) {
	l, err := taskpkg.NewDspLimiter(limiterTestConfig())
	if err != nil {
		t.Fatalf("NewDspLimiter returned error: %v", err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := l.SetRelease(float64(10 + i)); err != nil {
				errs <- fmt.Errorf("SetRelease: %w", err)
				return
			}
			in := filledBlock(64, 0.2+float64(i%3))
			out := make([]float64, 64)
			if err := l.ProcessBlock([][]float64{in}, [][]float64{out}); err != nil {
				errs <- fmt.Errorf("ProcessBlock: %w", err)
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent limiter call returned error: %v", err)
	}
	if gr := l.GainReductionDB(); math.IsNaN(gr) || math.IsInf(gr, 0) {
		t.Fatalf("GainReductionDB = %v, want a finite value", gr)
	}
}
