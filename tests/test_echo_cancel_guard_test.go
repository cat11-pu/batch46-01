package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-g-04"
)

func echoTestConfig() taskpkg.Config {
	return taskpkg.Config{
		SampleRate:         16000,
		MaxDelayMS:         200,
		DoubleTalkMarginDB: 6,
		MaxBlockSamples:    480,
	}
}

func echoSignal(n int, seed uint32, amp float64) []float64 {
	v := seed
	out := make([]float64, n)
	for i := range out {
		v ^= v << 13
		v ^= v >> 17
		v ^= v << 5
		out[i] = (float64(v)/float64(uint64(1)<<32)*2 - 1) * amp
	}
	return out
}

func echoPath(ref []float64, delay int, gain float64) []float64 {
	out := make([]float64, len(ref))
	for i := delay; i < len(ref); i++ {
		out[i] = gain * ref[i-delay]
	}
	return out
}

func sineBlock(n, start int, freq, amp, sampleRate float64) []float64 {
	out := make([]float64, n)
	for i := 0; i < n; i++ {
		t := float64(start+i) / sampleRate
		out[i] = amp * math.Sin(2*math.Pi*freq*t)
	}
	return out
}

func echoRMS(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	sum := 0.0
	for _, x := range xs {
		sum += x * x
	}
	return math.Sqrt(sum / float64(len(xs)))
}

func echoCorrelation(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	ma, mb := 0.0, 0.0
	for i := range a {
		ma += a[i]
		mb += b[i]
	}
	ma /= float64(len(a))
	mb /= float64(len(b))
	num, da, db := 0.0, 0.0, 0.0
	for i := range a {
		x := a[i] - ma
		y := b[i] - mb
		num += x * y
		da += x * x
		db += y * y
	}
	if da == 0 || db == 0 {
		return 0
	}
	return num / math.Sqrt(da*db)
}

func newEcho(t *testing.T, cfg taskpkg.Config) *taskpkg.EchoCancel {
	t.Helper()
	e, err := taskpkg.NewEchoCancel("call-7712", cfg)
	if err != nil {
		t.Fatalf("NewEchoCancel returned error: %v", err)
	}
	return e
}

func runEchoBlocks(t *testing.T, e *taskpkg.EchoCancel, ref, mic []float64, block int) []taskpkg.Result {
	t.Helper()
	var out []taskpkg.Result
	for start := 0; start+block <= len(mic); start += block {
		if err := e.PushReference(ref[start : start+block]); err != nil {
			t.Fatalf("PushReference returned error: %v", err)
		}
		res, err := e.Process(mic[start:start+block], true)
		if err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
		out = append(out, res)
	}
	return out
}

func lockedEcho(t *testing.T) (*taskpkg.EchoCancel, []float64, []float64) {
	t.Helper()
	ref := echoSignal(4800, 7, 0.5)
	mic := echoPath(ref, 150, 0.5)
	e := newEcho(t, echoTestConfig())
	results := runEchoBlocks(t, e, ref, mic, 480)
	if len(results) == 0 {
		t.Fatal("no echo blocks were processed")
	}
	return e, ref, mic
}

func TestConfigRejectsExcessiveMaxDelay(t *testing.T) {
	cfg := echoTestConfig()
	cfg.MaxDelayMS = 600
	if _, err := taskpkg.NewEchoCancel("call-1", cfg); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewEchoCancel(max delay 600) error = %v, want ErrInvalidConfig", err)
	}
}

func TestConfigRejectsBadSampleRate(t *testing.T) {
	cfg := echoTestConfig()
	cfg.SampleRate = 4000
	if _, err := taskpkg.NewEchoCancel("call-1", cfg); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewEchoCancel(sample rate 4000) error = %v, want ErrInvalidConfig", err)
	}
	cfg = echoTestConfig()
	cfg.MaxBlockSamples = 100
	if _, err := taskpkg.NewEchoCancel("call-1", cfg); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewEchoCancel(block 100) error = %v, want ErrInvalidConfig", err)
	}
}

func TestBlockTooLargeRejected(t *testing.T) {
	e := newEcho(t, echoTestConfig())
	if err := e.PushReference(make([]float64, 1000)); !errors.Is(err, taskpkg.ErrBlockTooLarge) {
		t.Fatalf("PushReference(1000) error = %v, want ErrBlockTooLarge", err)
	}
	if _, err := e.Process(make([]float64, 1000), true); !errors.Is(err, taskpkg.ErrBlockTooLarge) {
		t.Fatalf("Process(1000) error = %v, want ErrBlockTooLarge", err)
	}
}

func TestEstimateDelayFromEcho(t *testing.T) {
	e, _, _ := lockedEcho(t)
	if got := e.EstimateDelay(); got != 150 {
		t.Fatalf("EstimateDelay = %d, want 150", got)
	}
}

func TestEchoCancellationERLE(t *testing.T) {
	e, _, _ := lockedEcho(t)
	if got := e.ERLE(); got < 30 {
		t.Fatalf("ERLE = %v dB, want at least 30", got)
	}
}

func TestResidualBelowEchoLevel(t *testing.T) {
	_, _, mic := lockedEcho(t)
	e := newEcho(t, echoTestConfig())
	ref := echoSignal(4800, 7, 0.5)
	results := runEchoBlocks(t, e, ref, mic, 480)
	last := results[len(results)-1]
	micRMS := echoRMS(mic[len(mic)-480:])
	resRMS := echoRMS(last.Samples)
	if resRMS >= 0.05*micRMS {
		t.Fatalf("residual RMS = %v, echo mic RMS = %v; want residual under 5%%", resRMS, micRMS)
	}
}

func TestGainEstimateMatchesEchoPath(t *testing.T) {
	e, _, _ := lockedEcho(t)
	if got := e.Gain(); math.Abs(got-0.5) > 0.02 {
		t.Fatalf("Gain = %v, want about 0.5", got)
	}
}

func TestDoubleTalkDetected(t *testing.T) {
	ref := echoSignal(1440, 11, 0.5)
	mic := echoPath(ref, 150, 0.5)
	speech := sineBlock(480, 960, 440, 0.8, 16000)
	e := newEcho(t, echoTestConfig())
	var last taskpkg.Result
	for b := 0; b < 3; b++ {
		if err := e.PushReference(ref[b*480 : (b+1)*480]); err != nil {
			t.Fatalf("PushReference returned error: %v", err)
		}
		block := append([]float64(nil), mic[b*480:(b+1)*480]...)
		if b == 2 {
			for i := range block {
				block[i] += speech[i]
			}
		}
		res, err := e.Process(block, true)
		if err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
		last = res
	}
	if !last.DoubleTalk {
		t.Fatal("double talk not detected")
	}
	if got := e.DoubleTalkFrames(); got != 1 {
		t.Fatalf("DoubleTalkFrames = %d, want 1", got)
	}
}

func TestNearEndPreservedDuringDoubleTalk(t *testing.T) {
	ref := echoSignal(1440, 11, 0.5)
	mic := echoPath(ref, 150, 0.5)
	speech := sineBlock(480, 960, 440, 0.8, 16000)
	e := newEcho(t, echoTestConfig())
	var last taskpkg.Result
	for b := 0; b < 3; b++ {
		if err := e.PushReference(ref[b*480 : (b+1)*480]); err != nil {
			t.Fatalf("PushReference returned error: %v", err)
		}
		block := append([]float64(nil), mic[b*480:(b+1)*480]...)
		if b == 2 {
			for i := range block {
				block[i] += speech[i]
			}
		}
		res, err := e.Process(block, true)
		if err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
		last = res
	}
	if got := echoCorrelation(last.Samples, speech); got < 0.95 {
		t.Fatalf("near end correlation after cancellation = %v, want at least 0.95", got)
	}
	if got := echoRMS(last.Samples); got < 0.3 {
		t.Fatalf("near end RMS after cancellation = %v, want near end kept", got)
	}
}

func TestDoubleTalkFreezesGain(t *testing.T) {
	ref := echoSignal(1440, 11, 0.5)
	mic := echoPath(ref, 150, 0.5)
	speech := sineBlock(480, 960, 440, 0.8, 16000)
	e := newEcho(t, echoTestConfig())
	var before, after float64
	for b := 0; b < 3; b++ {
		if err := e.PushReference(ref[b*480 : (b+1)*480]); err != nil {
			t.Fatalf("PushReference returned error: %v", err)
		}
		block := append([]float64(nil), mic[b*480:(b+1)*480]...)
		if b == 2 {
			for i := range block {
				block[i] += speech[i]
			}
		}
		before = e.Gain()
		if _, err := e.Process(block, true); err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
		after = e.Gain()
	}
	if math.Abs(after-before) > 0.02 {
		t.Fatalf("gain moved during double talk: %v -> %v", before, after)
	}
}

func TestFarEndSilentPassesMic(t *testing.T) {
	e := newEcho(t, echoTestConfig())
	mic := sineBlock(480, 0, 330, 0.4, 16000)
	if err := e.PushReference(make([]float64, 480)); err != nil {
		t.Fatalf("PushReference returned error: %v", err)
	}
	res, err := e.Process(mic, false)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	if len(res.Samples) != len(mic) {
		t.Fatalf("Process returned %d samples, want %d", len(res.Samples), len(mic))
	}
	for i := range mic {
		if res.Samples[i] != mic[i] {
			t.Fatalf("sample %d changed with the far end silent: %v vs %v", i, res.Samples[i], mic[i])
		}
	}
	if res.ERLE != 0 || res.DoubleTalk {
		t.Fatalf("silent far end result = %+v, want ERLE 0 and no double talk", res)
	}
	if got := e.EstimateDelay(); got != -1 {
		t.Fatalf("EstimateDelay with no echo = %d, want -1", got)
	}
}

func TestReferenceHistoryBounded(t *testing.T) {
	e := newEcho(t, echoTestConfig())
	ref := echoSignal(4800, 3, 0.4)
	for start := 0; start+480 <= len(ref); start += 480 {
		if err := e.PushReference(ref[start : start+480]); err != nil {
			t.Fatalf("PushReference returned error: %v", err)
		}
	}
	want := 3200 + 480
	if got := e.ReferenceLen(); got != want {
		t.Fatalf("ReferenceLen = %d, want the capped %d", got, want)
	}
}

func TestSnapshotRestoreReproducesOutput(t *testing.T) {
	ref := echoSignal(2400, 5, 0.5)
	mic := echoPath(ref, 150, 0.5)
	e := newEcho(t, echoTestConfig())
	for b := 0; b < 2; b++ {
		if err := e.PushReference(ref[b*480 : (b+1)*480]); err != nil {
			t.Fatalf("PushReference returned error: %v", err)
		}
		if _, err := e.Process(mic[b*480:(b+1)*480], true); err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
	}
	data, err := e.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	nextRef := ref[960:1440]
	nextMic := mic[960:1440]
	if err := e.PushReference(nextRef); err != nil {
		t.Fatalf("PushReference returned error: %v", err)
	}
	first, err := e.Process(nextMic, true)
	if err != nil {
		t.Fatalf("Process returned error: %v", err)
	}
	restored, err := taskpkg.NewEchoCancelFromSnapshot(echoTestConfig(), data)
	if err != nil {
		t.Fatalf("NewEchoCancelFromSnapshot returned error: %v", err)
	}
	if got := restored.EstimateDelay(); got != 150 {
		t.Fatalf("restored EstimateDelay = %d, want 150", got)
	}
	if err := restored.PushReference(nextRef); err != nil {
		t.Fatalf("restored PushReference returned error: %v", err)
	}
	second, err := restored.Process(nextMic, true)
	if err != nil {
		t.Fatalf("restored Process returned error: %v", err)
	}
	if len(first.Samples) != len(second.Samples) {
		t.Fatalf("restored output length = %d, want %d", len(second.Samples), len(first.Samples))
	}
	for i := range first.Samples {
		if math.Abs(first.Samples[i]-second.Samples[i]) > 1e-9 {
			t.Fatalf("sample %d after restore = %v, want %v", i, second.Samples[i], first.Samples[i])
		}
	}
}

func TestSessionIsolation(t *testing.T) {
	ref := echoSignal(960, 19, 0.5)
	mic := echoPath(ref, 150, 0.5)
	used := newEcho(t, echoTestConfig())
	for b := 0; b < 2; b++ {
		if err := used.PushReference(ref[b*480 : (b+1)*480]); err != nil {
			t.Fatalf("PushReference returned error: %v", err)
		}
		if _, err := used.Process(mic[b*480:(b+1)*480], true); err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
	}
	other := newEcho(t, echoTestConfig())
	res, err := other.Process(mic[480:960], true)
	if err != nil {
		t.Fatalf("isolated Process returned error: %v", err)
	}
	if res.ERLE != 0 {
		t.Fatalf("second session ERLE = %v, want 0 with no reference", res.ERLE)
	}
	if got := other.EstimateDelay(); got != -1 {
		t.Fatalf("second session EstimateDelay = %d, want -1", got)
	}
	if got := other.Gain(); got != 0 {
		t.Fatalf("second session Gain = %v, want 0", got)
	}
	for i := range res.Samples {
		if res.Samples[i] != mic[480+i] {
			t.Fatalf("second session altered sample %d", i)
		}
	}
}

func TestConcurrentSnapshotDuringProcess(t *testing.T) {
	ref := echoSignal(2400, 23, 0.5)
	mic := echoPath(ref, 150, 0.5)
	e := newEcho(t, echoTestConfig())
	for b := 0; b < 3; b++ {
		if err := e.PushReference(ref[b*480 : (b+1)*480]); err != nil {
			t.Fatalf("PushReference returned error: %v", err)
		}
		if _, err := e.Process(mic[b*480:(b+1)*480], true); err != nil {
			t.Fatalf("Process returned error: %v", err)
		}
	}
	block := mic[1440:1920]
	var wg sync.WaitGroup
	errs := make(chan error, 60)
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			if _, err := e.Process(block, true); err != nil {
				errs <- err
				return
			}
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 30; i++ {
			if _, err := e.Snapshot(); err != nil {
				errs <- err
				return
			}
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent call returned error: %v", err)
	}
}
