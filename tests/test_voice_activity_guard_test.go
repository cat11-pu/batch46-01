package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-g-03"
)

func vadTestConfig() taskpkg.Config {
	return taskpkg.Config{
		SampleRate:        16000,
		FrameMS:           30,
		EnergyThresholdDB: -40,
		MinSpeechFrames:   1,
		HangoverFrames:    0,
	}
}

func vadBlock(amp float64) []float64 {
	b := make([]float64, 480)
	for i := range b {
		b[i] = amp
	}
	return b
}

func newVAD(t *testing.T, cfg taskpkg.Config) *taskpkg.VoiceActivity {
	t.Helper()
	v, err := taskpkg.NewVoiceActivity(cfg)
	if err != nil {
		t.Fatalf("NewVoiceActivity returned error: %v", err)
	}
	return v
}

func mustPush(t *testing.T, v *taskpkg.VoiceActivity, amp float64) taskpkg.FrameInfo {
	t.Helper()
	info, err := v.Push(vadBlock(amp))
	if err != nil {
		t.Fatalf("Push returned error: %v", err)
	}
	return info
}

func TestInvalidFrameLengthConfig(t *testing.T) {
	cfg := vadTestConfig()
	cfg.FrameMS = 5
	if _, err := taskpkg.NewVoiceActivity(cfg); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewVoiceActivity(frame 5 ms) error = %v, want ErrInvalidConfig", err)
	}
}

func TestInvalidSampleRateConfig(t *testing.T) {
	cfg := vadTestConfig()
	cfg.SampleRate = 0
	if _, err := taskpkg.NewVoiceActivity(cfg); !errors.Is(err, taskpkg.ErrInvalidConfig) {
		t.Fatalf("NewVoiceActivity(sample rate 0) error = %v, want ErrInvalidConfig", err)
	}
}

func TestFrameSizeMismatchRejected(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	if got := v.FrameSamples(); got != 480 {
		t.Fatalf("FrameSamples = %d, want 480", got)
	}
	if _, err := v.Push(make([]float64, 240)); !errors.Is(err, taskpkg.ErrFrameSize) {
		t.Fatalf("Push(240 samples) error = %v, want ErrFrameSize", err)
	}
	if got := v.TotalFrames(); got != 0 {
		t.Fatalf("TotalFrames after rejected push = %d, want 0", got)
	}
}

func TestLoudFrameIsVoice(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	info := mustPush(t, v, 0.5)
	if !info.Voice || !info.Speech {
		t.Fatalf("loud frame = voice %v speech %v, want true/true", info.Voice, info.Speech)
	}
	if math.Abs(info.EnergyDB-(-6.02)) > 0.1 {
		t.Fatalf("loud frame energy = %v dBFS, want about -6.02", info.EnergyDB)
	}
	if !v.IsSpeech() {
		t.Fatal("IsSpeech false right after a loud frame")
	}
}

func TestQuietFrameIsNotVoice(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	info := mustPush(t, v, 0.001)
	if info.Voice || info.Speech {
		t.Fatalf("quiet frame = voice %v speech %v, want false/false", info.Voice, info.Speech)
	}
	if math.Abs(info.EnergyDB-(-60)) > 0.1 {
		t.Fatalf("quiet frame energy = %v dBFS, want about -60", info.EnergyDB)
	}
	if v.IsSpeech() {
		t.Fatal("IsSpeech true after a single quiet frame")
	}
}

func TestMinSpeechFramesDelaysOnset(t *testing.T) {
	cfg := vadTestConfig()
	cfg.MinSpeechFrames = 3
	v := newVAD(t, cfg)
	for i := 0; i < 2; i++ {
		info := mustPush(t, v, 0.5)
		if !info.Voice {
			t.Fatalf("loud frame %d not marked voice", i)
		}
		if info.Speech {
			t.Fatalf("speech started after %d frames, want 3", i+1)
		}
	}
	info := mustPush(t, v, 0.5)
	if !info.Speech {
		t.Fatal("speech did not start on the third consecutive loud frame")
	}
}

func TestHangoverHoldsSpeech(t *testing.T) {
	cfg := vadTestConfig()
	cfg.HangoverFrames = 3
	v := newVAD(t, cfg)
	if info := mustPush(t, v, 0.5); !info.Speech {
		t.Fatal("speech did not start on the loud frame")
	}
	for i := 0; i < 3; i++ {
		info := mustPush(t, v, 0.001)
		if !info.Speech {
			t.Fatalf("hangover frame %d not reported as speech", i+1)
		}
	}
	info := mustPush(t, v, 0.001)
	if info.Speech || v.IsSpeech() {
		t.Fatal("speech still active after the hangover expired")
	}
}

func TestNoiseFloorTracksQuietFrames(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	for i := 0; i < 100; i++ {
		if _, err := v.Push(vadBlock(0.0001)); err != nil {
			t.Fatalf("Push returned error: %v", err)
		}
	}
	floor := v.NoiseFloorDB()
	if floor > -75 || floor < -85 {
		t.Fatalf("NoiseFloorDB after 100 quiet frames = %v, want about -80", floor)
	}
}

func TestPushFramesBatch(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	batch := make([]float64, 0, 1440)
	for i := 0; i < 3; i++ {
		batch = append(batch, vadBlock(0.5)...)
	}
	infos, err := v.PushFrames(batch)
	if err != nil {
		t.Fatalf("PushFrames returned error: %v", err)
	}
	if len(infos) != 3 {
		t.Fatalf("PushFrames returned %d frames, want 3", len(infos))
	}
	for i, info := range infos {
		if info.Index != i {
			t.Fatalf("frame %d index = %d, want %d", i, info.Index, i)
		}
		if !info.Speech {
			t.Fatalf("frame %d not reported as speech", i)
		}
	}
	if got := v.TotalFrames(); got != 3 {
		t.Fatalf("TotalFrames = %d, want 3", got)
	}
}

func TestPushFramesRejectsPartialBatch(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	if _, err := v.PushFrames(make([]float64, 1200)); !errors.Is(err, taskpkg.ErrFrameSize) {
		t.Fatalf("PushFrames(partial frame) error = %v, want ErrFrameSize", err)
	}
	if got := v.TotalFrames(); got != 0 {
		t.Fatalf("Partial batch consumed %d frames, want 0", got)
	}
}

func TestSpeechRatio(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	mustPush(t, v, 0.5)
	for i := 0; i < 3; i++ {
		mustPush(t, v, 0.001)
	}
	if got := v.SpeechFrames(); got != 1 {
		t.Fatalf("SpeechFrames = %d, want 1", got)
	}
	if got := v.SpeechRatio(); math.Abs(got-0.25) > 1e-9 {
		t.Fatalf("SpeechRatio = %v, want 0.25", got)
	}
}

func TestSetEnergyThreshold(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	if err := v.SetEnergyThreshold(10); !errors.Is(err, taskpkg.ErrInvalidThreshold) {
		t.Fatalf("SetEnergyThreshold(10) error = %v, want ErrInvalidThreshold", err)
	}
	if err := v.SetEnergyThreshold(-200); !errors.Is(err, taskpkg.ErrInvalidThreshold) {
		t.Fatalf("SetEnergyThreshold(-200) error = %v, want ErrInvalidThreshold", err)
	}
	if err := v.SetEnergyThreshold(-30); err != nil {
		t.Fatalf("SetEnergyThreshold(-30) returned error: %v", err)
	}
	info := mustPush(t, v, 0.01)
	if info.Voice {
		t.Fatalf("frame at %v dBFS marked voice with a -30 dBFS threshold", info.EnergyDB)
	}
}

func TestSnapshotRestoreKeepsHangover(t *testing.T) {
	cfg := vadTestConfig()
	cfg.HangoverFrames = 3
	v := newVAD(t, cfg)
	mustPush(t, v, 0.5)
	data, err := v.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	restored, err := taskpkg.NewVoiceActivityFromSnapshot(cfg, data)
	if err != nil {
		t.Fatalf("NewVoiceActivityFromSnapshot returned error: %v", err)
	}
	info := mustPush(t, restored, 0.001)
	if !info.Speech {
		t.Fatal("restored detector lost the hangover state")
	}
	if got := restored.TotalFrames(); got != 2 {
		t.Fatalf("restored TotalFrames = %d, want 2", got)
	}
}

func TestConcurrentPushCountsFrames(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	var wg sync.WaitGroup
	errs := make(chan error, 100)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := v.Push(vadBlock(0.5)); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent Push returned error: %v", err)
	}
	if got := v.TotalFrames(); got != 100 {
		t.Fatalf("TotalFrames after concurrent pushes = %d, want 100", got)
	}
	if got := v.SpeechFrames(); got != 100 {
		t.Fatalf("SpeechFrames after concurrent pushes = %d, want 100", got)
	}
}

func TestResetClearsCounters(t *testing.T) {
	v := newVAD(t, vadTestConfig())
	for i := 0; i < 10; i++ {
		mustPush(t, v, 0.5)
	}
	v.Reset()
	if got := v.TotalFrames(); got != 0 {
		t.Fatalf("TotalFrames after Reset = %d, want 0", got)
	}
	if v.IsSpeech() {
		t.Fatal("IsSpeech true after Reset")
	}
	if got := v.SpeechRatio(); got != 0 {
		t.Fatalf("SpeechRatio after Reset = %v, want 0", got)
	}
	if got := v.NoiseFloorDB(); got != -40 {
		t.Fatalf("NoiseFloorDB after Reset = %v, want -40", got)
	}
}
