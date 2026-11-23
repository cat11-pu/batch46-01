package task

import (
	"errors"
	"fmt"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-g-01"
)

func audioNear(a, b, eps float64) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func spatialTestConfig() taskpkg.Config {
	return taskpkg.Config{
		MinDistance:   1,
		MaxDistance:   50,
		OcclusionGain: 0.35,
		MaxVoices:     8,
		MaxSources:    256,
		SpeedOfSound:  343,
	}
}

func spatialTestListener() taskpkg.AudienceListener {
	return taskpkg.AudienceListener{
		Pos:     taskpkg.Vec3{},
		Forward: taskpkg.Vec3{Z: -1},
		Up:      taskpkg.Vec3{Y: 1},
	}
}

func setupAudio(t *testing.T, maxVoices int) *taskpkg.SpatialAudio {
	t.Helper()
	cfg := spatialTestConfig()
	cfg.MaxVoices = maxVoices
	a := taskpkg.NewSpatialAudio("level-04", cfg)
	if err := a.SetListener(spatialTestListener()); err != nil {
		t.Fatalf("SetListener returned error: %v", err)
	}
	return a
}

func TestPanRightSourceIsFullyRight(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "footstep", Pos: taskpkg.Vec3{X: 2}, Gain: 1, Priority: 1}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	g, err := a.Pan("footstep")
	if err != nil {
		t.Fatalf("Pan returned error: %v", err)
	}
	if g.Left > 0.01 || !audioNear(g.Right, 1, 0.01) {
		t.Fatalf("Pan(right) = left %v right %v, want 0/1", g.Left, g.Right)
	}
	if !audioNear(g.AzimuthDeg, 90, 0.5) {
		t.Fatalf("azimuth = %v, want 90", g.AzimuthDeg)
	}
	if g.Rear {
		t.Fatal("source to the right reported as rear")
	}
}

func TestPanLeftSourceIsFullyLeft(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "radio", Pos: taskpkg.Vec3{X: -3}, Gain: 1, Priority: 1}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	g, err := a.Pan("radio")
	if err != nil {
		t.Fatalf("Pan returned error: %v", err)
	}
	if !audioNear(g.Left, 1, 0.01) || g.Right > 0.01 {
		t.Fatalf("Pan(left) = left %v right %v, want 1/0", g.Left, g.Right)
	}
	if !audioNear(g.AzimuthDeg, -90, 0.5) {
		t.Fatalf("azimuth = %v, want -90", g.AzimuthDeg)
	}
}

func TestPanRearSourceIsDiffuse(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "hover", Pos: taskpkg.Vec3{Z: 2}, Gain: 1, Priority: 1}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	g, err := a.Pan("hover")
	if err != nil {
		t.Fatalf("Pan returned error: %v", err)
	}
	if !g.Rear {
		t.Fatalf("source behind the listener not flagged rear: %+v", g)
	}
	want := 1 / math.Sqrt2
	if !audioNear(g.Left, want, 1e-6) || !audioNear(g.Right, want, 1e-6) {
		t.Fatalf("rear gains = %v/%v, want %v/%v", g.Left, g.Right, want, want)
	}
}

func TestElevationDegrees(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "drone", Pos: taskpkg.Vec3{Y: 4}, Gain: 1, Priority: 1}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	g, err := a.Pan("drone")
	if err != nil {
		t.Fatalf("Pan returned error: %v", err)
	}
	if !audioNear(g.ElevationDeg, 90, 0.5) {
		t.Fatalf("elevation = %v, want 90", g.ElevationDeg)
	}
}

func TestAttenuationInverseDistance(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "engine", Pos: taskpkg.Vec3{Z: -2}, Gain: 1, Priority: 1}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	got, err := a.Attenuation("engine")
	if err != nil {
		t.Fatalf("Attenuation returned error: %v", err)
	}
	if !audioNear(got, 0.5, 1e-9) {
		t.Fatalf("attenuation at 2 m = %v, want 0.5", got)
	}
	if err := a.AddSource(taskpkg.Source{ID: "near", Pos: taskpkg.Vec3{Z: -0.25}, Gain: 1, Priority: 1}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	got, err = a.Attenuation("near")
	if err != nil {
		t.Fatalf("Attenuation returned error: %v", err)
	}
	if !audioNear(got, 1, 1e-9) {
		t.Fatalf("attenuation inside MinDistance = %v, want 1", got)
	}
}

func TestSourcesBeyondMaxDistanceCulled(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "waterfall", Pos: taskpkg.Vec3{Z: -60}, Gain: 1, Priority: 1}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	if got, err := a.Attenuation("waterfall"); err != nil || got != 0 {
		t.Fatalf("Attenuation beyond MaxDistance = %v, %v; want 0, nil", got, err)
	}
	mix, err := a.Mix()
	if err != nil {
		t.Fatalf("Mix returned error: %v", err)
	}
	if len(mix.Voices) != 0 || mix.Culled != 1 {
		t.Fatalf("Mix = %d voices, %d culled; want 0 and 1", len(mix.Voices), mix.Culled)
	}
}

func TestOccludedSourceGetsOcclusionGain(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "generator", Pos: taskpkg.Vec3{Z: -1}, Gain: 0.8, Priority: 1, Occluded: true}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	mix, err := a.Mix()
	if err != nil {
		t.Fatalf("Mix returned error: %v", err)
	}
	if len(mix.Voices) != 1 {
		t.Fatalf("Mix returned %d voices, want 1", len(mix.Voices))
	}
	if !audioNear(mix.Voices[0].Gain, 0.28, 1e-9) {
		t.Fatalf("occluded gain = %v, want 0.28", mix.Voices[0].Gain)
	}
}

func TestDopplerApproachAndRecede(t *testing.T) {
	a := setupAudio(t, 8)
	src := taskpkg.Source{ID: "car", Pos: taskpkg.Vec3{Z: -10}, Vel: taskpkg.Vec3{Z: -34.3}, Gain: 1, Priority: 1}
	if err := a.AddSource(src); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	got, err := a.Doppler("car")
	if err != nil {
		t.Fatalf("Doppler returned error: %v", err)
	}
	if !audioNear(got, 343.0/377.3, 1e-4) {
		t.Fatalf("receding doppler = %v, want %v", got, 343.0/377.3)
	}
	src.Vel = taskpkg.Vec3{Z: 34.3}
	if err := a.UpdateSource(src); err != nil {
		t.Fatalf("UpdateSource returned error: %v", err)
	}
	got, err = a.Doppler("car")
	if err != nil {
		t.Fatalf("Doppler returned error: %v", err)
	}
	if !audioNear(got, 343.0/308.7, 1e-4) {
		t.Fatalf("approaching doppler = %v, want %v", got, 343.0/308.7)
	}
}

func TestDopplerClampedToRange(t *testing.T) {
	a := setupAudio(t, 8)
	src := taskpkg.Source{ID: "jet", Pos: taskpkg.Vec3{Z: -10}, Vel: taskpkg.Vec3{Z: -1000}, Gain: 1, Priority: 1}
	if err := a.AddSource(src); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	got, err := a.Doppler("jet")
	if err != nil {
		t.Fatalf("Doppler returned error: %v", err)
	}
	if got != 0.5 {
		t.Fatalf("fast receding doppler = %v, want clamp 0.5", got)
	}
	src.Vel = taskpkg.Vec3{Z: 1000}
	if err := a.UpdateSource(src); err != nil {
		t.Fatalf("UpdateSource returned error: %v", err)
	}
	got, err = a.Doppler("jet")
	if err != nil {
		t.Fatalf("Doppler returned error: %v", err)
	}
	if got != 2.0 {
		t.Fatalf("fast approaching doppler = %v, want clamp 2.0", got)
	}
}

func TestVoiceLimitKeepsHighestPriority(t *testing.T) {
	a := setupAudio(t, 2)
	for _, s := range []taskpkg.Source{
		{ID: "ambience", Pos: taskpkg.Vec3{Z: -1}, Gain: 1, Priority: 1},
		{ID: "radio", Pos: taskpkg.Vec3{Z: -1}, Gain: 1, Priority: 5},
		{ID: "gunshot", Pos: taskpkg.Vec3{Z: -1}, Gain: 1, Priority: 9},
	} {
		if err := a.AddSource(s); err != nil {
			t.Fatalf("AddSource(%s) returned error: %v", s.ID, err)
		}
	}
	mix, err := a.Mix()
	if err != nil {
		t.Fatalf("Mix returned error: %v", err)
	}
	if len(mix.Voices) != 2 {
		t.Fatalf("Mix returned %d voices, want 2", len(mix.Voices))
	}
	if mix.Voices[0].ID != "gunshot" || mix.Voices[1].ID != "radio" {
		t.Fatalf("voice order = %s,%s; want gunshot,radio", mix.Voices[0].ID, mix.Voices[1].ID)
	}
	if len(mix.Dropped) != 1 || mix.Dropped[0] != "ambience" {
		t.Fatalf("dropped = %v, want [ambience]", mix.Dropped)
	}
}

func TestMixOrderIsDeterministic(t *testing.T) {
	a := setupAudio(t, 8)
	for _, s := range []taskpkg.Source{
		{ID: "ambience", Pos: taskpkg.Vec3{Z: -1}, Gain: 1, Priority: 1},
		{ID: "gunshot", Pos: taskpkg.Vec3{Z: -1}, Gain: 1, Priority: 9},
		{ID: "radio", Pos: taskpkg.Vec3{Z: -1}, Gain: 1, Priority: 5},
	} {
		if err := a.AddSource(s); err != nil {
			t.Fatalf("AddSource(%s) returned error: %v", s.ID, err)
		}
	}
	got := a.MixOrder()
	want := []string{"gunshot", "radio", "ambience"}
	if len(got) != len(want) {
		t.Fatalf("MixOrder returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MixOrder = %v, want %v", got, want)
		}
	}
	again := a.MixOrder()
	if len(again) != len(want) || again[0] != want[0] || again[2] != want[2] {
		t.Fatalf("MixOrder changed between calls: %v then %v", got, again)
	}
}

func TestDuplicateSourceRejectedKeepsOriginal(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "alarm", Pos: taskpkg.Vec3{Z: -1}, Gain: 1, Priority: 1}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	err := a.AddSource(taskpkg.Source{ID: "alarm", Pos: taskpkg.Vec3{Z: -9}, Gain: 0.2, Priority: 9})
	if !errors.Is(err, taskpkg.ErrDuplicateSource) {
		t.Fatalf("duplicate AddSource error = %v, want ErrDuplicateSource", err)
	}
	got, err := a.Source("alarm")
	if err != nil {
		t.Fatalf("Source returned error: %v", err)
	}
	if got.Pos.Z != -1 || got.Gain != 1 || got.Priority != 1 {
		t.Fatalf("duplicate add mutated the source: %+v", got)
	}
}

func TestInvalidListenerBasisRejected(t *testing.T) {
	a := taskpkg.NewSpatialAudio("level-04", spatialTestConfig())
	err := a.SetListener(taskpkg.AudienceListener{Forward: taskpkg.Vec3{}, Up: taskpkg.Vec3{Y: 1}})
	if !errors.Is(err, taskpkg.ErrInvalidBasis) {
		t.Fatalf("zero forward error = %v, want ErrInvalidBasis", err)
	}
	err = a.SetListener(taskpkg.AudienceListener{Forward: taskpkg.Vec3{Y: 1}, Up: taskpkg.Vec3{Y: 1}})
	if !errors.Is(err, taskpkg.ErrInvalidBasis) {
		t.Fatalf("parallel basis error = %v, want ErrInvalidBasis", err)
	}
}

func TestSnapshotRestoreKeepsPose(t *testing.T) {
	a := setupAudio(t, 8)
	if err := a.AddSource(taskpkg.Source{ID: "footstep", Pos: taskpkg.Vec3{X: 2}, Gain: 0.9, Priority: 3}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	if err := a.AddSource(taskpkg.Source{ID: "alarm", Pos: taskpkg.Vec3{Z: 2}, Gain: 0.4, Priority: 7, Occluded: true}); err != nil {
		t.Fatalf("AddSource returned error: %v", err)
	}
	data, err := a.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	restored, err := taskpkg.NewSpatialAudioFromSnapshot(spatialTestConfig(), data)
	if err != nil {
		t.Fatalf("NewSpatialAudioFromSnapshot returned error: %v", err)
	}
	if restored.ActiveSources() != 2 {
		t.Fatalf("restored source count = %d, want 2", restored.ActiveSources())
	}
	before, err := a.Pan("footstep")
	if err != nil {
		t.Fatalf("Pan returned error: %v", err)
	}
	after, err := restored.Pan("footstep")
	if err != nil {
		t.Fatalf("restored Pan returned error: %v", err)
	}
	if !audioNear(before.Right, after.Right, 1e-9) || !audioNear(before.AzimuthDeg, after.AzimuthDeg, 1e-9) {
		t.Fatalf("pan changed after restore: %+v vs %+v", before, after)
	}
}

func TestConcurrentAddSources(t *testing.T) {
	a := setupAudio(t, 8)
	var wg sync.WaitGroup
	errs := make(chan error, 64)
	for i := 0; i < 64; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			id := fmt.Sprintf("prop-%02d", i)
			if err := a.AddSource(taskpkg.Source{ID: id, Pos: taskpkg.Vec3{Z: -1}, Gain: 1, Priority: i}); err != nil {
				errs <- err
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent AddSource returned error: %v", err)
	}
	if got := a.ActiveSources(); got != 64 {
		t.Fatalf("ActiveSources after concurrent adds = %d, want 64", got)
	}
	mix, err := a.Mix()
	if err != nil {
		t.Fatalf("Mix returned error: %v", err)
	}
	if len(mix.Voices) != 8 {
		t.Fatalf("Mix voices = %d, want the configured 8", len(mix.Voices))
	}
}
