package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-d-02"
)

func newWaves(t *testing.T) *taskpkg.WaterWaves {
	t.Helper()
	w, err := taskpkg.NewWaterWaves(64, 64)
	if err != nil {
		t.Fatalf("NewWaterWaves returned error: %v", err)
	}
	if w == nil {
		t.Fatal("NewWaterWaves returned a nil surface")
	}
	return w
}

func mustAddWave(t *testing.T, w *taskpkg.WaterWaves, p taskpkg.WaveParam) {
	t.Helper()
	if err := w.AddWave(p); err != nil {
		t.Fatalf("AddWave(%+v) returned error: %v", p, err)
	}
}

func TestConstructorValidation(t *testing.T) {
	if _, err := taskpkg.NewWaterWaves(-4, 64); !errors.Is(err, taskpkg.ErrBadExtent) {
		t.Fatalf("NewWaterWaves(-4) error = %v, want ErrBadExtent", err)
	}
	if _, err := taskpkg.NewWaterWaves(math.Inf(1), 64); !errors.Is(err, taskpkg.ErrBadExtent) {
		t.Fatalf("NewWaterWaves(+Inf) error = %v, want ErrBadExtent", err)
	}
	if _, err := taskpkg.NewWaterWaves(64, 6); !errors.Is(err, taskpkg.ErrBadResolution) {
		t.Fatalf("NewWaterWaves(res 6) error = %v, want ErrBadResolution", err)
	}
	if _, err := taskpkg.NewWaterWaves(64, 1024); !errors.Is(err, taskpkg.ErrBadResolution) {
		t.Fatalf("NewWaterWaves(res 1024) error = %v, want ErrBadResolution", err)
	}
	if _, err := taskpkg.NewWaterWaves(64, 64); err != nil {
		t.Fatalf("NewWaterWaves(64, 64) returned error: %v", err)
	}
}

func TestAddWaveRejectsInvalidParams(t *testing.T) {
	w := newWaves(t)
	bad := []taskpkg.WaveParam{
		{Dir: taskpkg.Vec2{X: 1}, Amplitude: 0, Wavelength: 10, Speed: 1, Steepness: 0.2},
		{Dir: taskpkg.Vec2{X: 1}, Amplitude: math.NaN(), Wavelength: 10, Speed: 1, Steepness: 0.2},
		{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1, Wavelength: 0, Speed: 1, Steepness: 0.2},
		{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1, Wavelength: -5, Speed: 1, Steepness: 0.2},
		{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1, Wavelength: 10, Speed: 1, Steepness: -0.1},
		{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1, Wavelength: 10, Speed: 1, Steepness: 1.5},
		{Dir: taskpkg.Vec2{}, Amplitude: 1, Wavelength: 10, Speed: 1, Steepness: 0.2},
		{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1, Wavelength: 10, Speed: math.NaN(), Steepness: 0.2},
	}
	for _, p := range bad {
		if err := w.AddWave(p); !errors.Is(err, taskpkg.ErrBadWave) {
			t.Fatalf("AddWave(%+v) error = %v, want ErrBadWave", p, err)
		}
	}
	if w.WaveCount() != 0 {
		t.Fatalf("WaveCount after rejected waves = %d, want 0", w.WaveCount())
	}
}

func TestAddWaveLimit(t *testing.T) {
	w := newWaves(t)
	for i := 0; i < taskpkg.MaxWaves; i++ {
		mustAddWave(t, w, taskpkg.WaveParam{
			Dir:        taskpkg.Vec2{X: 1},
			Amplitude:  0.1,
			Wavelength: 8,
			Speed:      1,
			Steepness:  0.1,
		})
	}
	if got := w.WaveCount(); got != taskpkg.MaxWaves {
		t.Fatalf("WaveCount = %d, want %d", got, taskpkg.MaxWaves)
	}
	extra := taskpkg.WaveParam{Dir: taskpkg.Vec2{X: 1}, Amplitude: 0.1, Wavelength: 8, Speed: 1, Steepness: 0.1}
	if err := w.AddWave(extra); !errors.Is(err, taskpkg.ErrTooManyWaves) {
		t.Fatalf("ninth AddWave error = %v, want ErrTooManyWaves", err)
	}
	if got := w.WaveCount(); got != taskpkg.MaxWaves {
		t.Fatalf("WaveCount after rejected ninth wave = %d, want %d", got, taskpkg.MaxWaves)
	}
}

func TestHeightSingleWave(t *testing.T) {
	w := newWaves(t)
	mustAddWave(t, w, taskpkg.WaveParam{
		Dir:        taskpkg.Vec2{X: 1},
		Amplitude:  2,
		Wavelength: 20,
		Speed:      0,
		Steepness:  0.2,
	})
	got, err := w.Height(5, 0, 0)
	if err != nil {
		t.Fatalf("Height returned error: %v", err)
	}
	if math.Abs(got-2) > 1e-9 {
		t.Fatalf("Height at crest = %v, want 2", got)
	}
	trough, err := w.Height(15, 0, 0)
	if err != nil {
		t.Fatalf("Height returned error: %v", err)
	}
	if math.Abs(trough+2) > 1e-9 {
		t.Fatalf("Height at trough = %v, want -2", trough)
	}
}

func TestHeightSuperposition(t *testing.T) {
	w := newWaves(t)
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1, Wavelength: 20, Speed: 0, Steepness: 0})
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{Z: 1}, Amplitude: 1.5, Wavelength: 40, Speed: 0, Steepness: 0})
	got, err := w.Height(5, 10, 0)
	if err != nil {
		t.Fatalf("Height returned error: %v", err)
	}
	if math.Abs(got-2.5) > 1e-9 {
		t.Fatalf("superposed Height = %v, want 2.5", got)
	}
}

func TestHeightRejectsBadTime(t *testing.T) {
	w := newWaves(t)
	if _, err := w.Height(0, 0, -1); !errors.Is(err, taskpkg.ErrBadTime) {
		t.Fatalf("Height(t=-1) error = %v, want ErrBadTime", err)
	}
	if _, err := w.Height(0, 0, math.NaN()); !errors.Is(err, taskpkg.ErrBadTime) {
		t.Fatalf("Height(t=NaN) error = %v, want ErrBadTime", err)
	}
}

func TestDisplacementGerstner(t *testing.T) {
	w := newWaves(t)
	mustAddWave(t, w, taskpkg.WaveParam{
		Dir:        taskpkg.Vec2{X: 1},
		Amplitude:  2,
		Wavelength: 20,
		Speed:      0,
		Steepness:  0.5,
	})
	crest, err := w.Displacement(0, 0, 0)
	if err != nil {
		t.Fatalf("Displacement returned error: %v", err)
	}
	if math.Abs(crest.X-1) > 1e-9 || math.Abs(crest.Y) > 1e-12 || math.Abs(crest.Z) > 1e-12 {
		t.Fatalf("Displacement at origin = %+v, want (1,0,0)", crest)
	}
	zero, err := w.Displacement(5, 0, 0)
	if err != nil {
		t.Fatalf("Displacement returned error: %v", err)
	}
	if math.Abs(zero.X) > 1e-9 {
		t.Fatalf("Displacement at wave crest = %+v, want zero horizontal offset", zero)
	}
}

func TestNormalIsUnitAndFlatWhenStill(t *testing.T) {
	w := newWaves(t)
	flat, err := w.Normal(3, 4, 0)
	if err != nil {
		t.Fatalf("Normal on still water returned error: %v", err)
	}
	if flat.X != 0 || flat.Y != 1 || flat.Z != 0 {
		t.Fatalf("Normal on still water = %+v, want (0,1,0)", flat)
	}
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1.2, Wavelength: 12, Speed: 2, Steepness: 0.4})
	n, err := w.Normal(1.5, -2.5, 0.7)
	if err != nil {
		t.Fatalf("Normal returned error: %v", err)
	}
	length := math.Sqrt(n.X*n.X + n.Y*n.Y + n.Z*n.Z)
	if math.Abs(length-1) > 1e-9 {
		t.Fatalf("Normal length = %v, want 1", length)
	}
	if n.Y <= 0 {
		t.Fatalf("Normal %+v points away from the camera", n)
	}
}

func TestBuildMeshFlatGrid(t *testing.T) {
	w, err := taskpkg.NewWaterWaves(32, 64)
	if err != nil {
		t.Fatalf("NewWaterWaves returned error: %v", err)
	}
	vertices, err := w.BuildMesh(0, 0)
	if err != nil {
		t.Fatalf("BuildMesh returned error: %v", err)
	}
	if len(vertices) != 65*65 {
		t.Fatalf("BuildMesh returned %d vertices, want %d", len(vertices), 65*65)
	}
	first := vertices[0]
	if math.Abs(first.Position.X+16) > 1e-9 || math.Abs(first.Position.Y) > 1e-12 || math.Abs(first.Position.Z+16) > 1e-9 {
		t.Fatalf("first vertex = %+v, want (-16,0,-16)", first.Position)
	}
	if first.Normal != (taskpkg.Vec3{Y: 1}) {
		t.Fatalf("first normal = %+v, want (0,1,0)", first.Normal)
	}
	last := vertices[len(vertices)-1]
	if math.Abs(last.Position.X-16) > 1e-9 || math.Abs(last.Position.Z-16) > 1e-9 {
		t.Fatalf("last vertex = %+v, want (16,*,16)", last.Position)
	}
	info, err := w.MeshInfo(0)
	if err != nil {
		t.Fatalf("MeshInfo returned error: %v", err)
	}
	if info.Rows != 65 || info.Cols != 65 || info.Triangles != 8192 || math.Abs(info.Spacing-0.5) > 1e-12 {
		t.Fatalf("MeshInfo(0) = %+v, want rows 65 cols 65 tris 8192 spacing 0.5", info)
	}
}

func TestBuildMeshLODSelection(t *testing.T) {
	w := newWaves(t)
	vertices, err := w.BuildMesh(0.25, 3)
	if err != nil {
		t.Fatalf("BuildMesh(lod 3) returned error: %v", err)
	}
	if len(vertices) != 81 {
		t.Fatalf("BuildMesh(lod 3) returned %d vertices, want 81", len(vertices))
	}
	info, err := w.MeshInfo(3)
	if err != nil {
		t.Fatalf("MeshInfo(3) returned error: %v", err)
	}
	if info.Rows != 9 || info.Cols != 9 || info.Triangles != 128 || math.Abs(info.Spacing-8) > 1e-12 {
		t.Fatalf("MeshInfo(3) = %+v, want rows 9 cols 9 tris 128 spacing 8", info)
	}
	coarsest, err := w.MeshInfo(6)
	if err != nil {
		t.Fatalf("MeshInfo(6) returned error: %v", err)
	}
	if coarsest.Rows != 2 || coarsest.Triangles != 2 || math.Abs(coarsest.Spacing-64) > 1e-12 {
		t.Fatalf("MeshInfo(6) = %+v, want rows 2 tris 2 spacing 64", coarsest)
	}
	if _, err := w.MeshInfo(7); !errors.Is(err, taskpkg.ErrBadLOD) {
		t.Fatalf("MeshInfo(7) error = %v, want ErrBadLOD", err)
	}
	if _, err := w.BuildMesh(0, -1); !errors.Is(err, taskpkg.ErrBadLOD) {
		t.Fatalf("BuildMesh(lod -1) error = %v, want ErrBadLOD", err)
	}
}

func TestMaxHeightIsAmplitudeSum(t *testing.T) {
	w := newWaves(t)
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1, Wavelength: 10, Speed: 0, Steepness: 0})
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{Z: 1}, Amplitude: 2, Wavelength: 20, Speed: 0, Steepness: 0})
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{X: 0.6, Z: 0.8}, Amplitude: 0.5, Wavelength: 5, Speed: 0, Steepness: 0})
	if got := w.MaxHeight(); math.Abs(got-3.5) > 1e-9 {
		t.Fatalf("MaxHeight = %v, want 3.5", got)
	}
}

func TestShoreFactorFadesToBorder(t *testing.T) {
	w := newWaves(t)
	center, err := w.ShoreFactor(0, 0)
	if err != nil {
		t.Fatalf("ShoreFactor(center) returned error: %v", err)
	}
	if center != 1 {
		t.Fatalf("ShoreFactor(center) = %v, want 1", center)
	}
	border, err := w.ShoreFactor(32, 0)
	if err != nil {
		t.Fatalf("ShoreFactor(border) returned error: %v", err)
	}
	if border != 0 {
		t.Fatalf("ShoreFactor(border) = %v, want 0", border)
	}
	half, err := w.ShoreFactor(28, 0)
	if err != nil {
		t.Fatalf("ShoreFactor returned error: %v", err)
	}
	if math.Abs(half-0.5) > 1e-12 {
		t.Fatalf("ShoreFactor(28,0) = %v, want 0.5", half)
	}
	quarter, err := w.ShoreFactor(0, -30)
	if err != nil {
		t.Fatalf("ShoreFactor returned error: %v", err)
	}
	if math.Abs(quarter-0.25) > 1e-12 {
		t.Fatalf("ShoreFactor(0,-30) = %v, want 0.25", quarter)
	}
}

func TestSetShoreFadeValidation(t *testing.T) {
	w := newWaves(t)
	if err := w.SetShoreFade(0); !errors.Is(err, taskpkg.ErrBadShoreFade) {
		t.Fatalf("SetShoreFade(0) error = %v, want ErrBadShoreFade", err)
	}
	if err := w.SetShoreFade(33); !errors.Is(err, taskpkg.ErrBadShoreFade) {
		t.Fatalf("SetShoreFade(33) error = %v, want ErrBadShoreFade", err)
	}
	if err := w.SetShoreFade(16); err != nil {
		t.Fatalf("SetShoreFade(16) returned error: %v", err)
	}
	got, err := w.ShoreFactor(28, 0)
	if err != nil {
		t.Fatalf("ShoreFactor returned error: %v", err)
	}
	if math.Abs(got-0.25) > 1e-12 {
		t.Fatalf("ShoreFactor(28,0) with fade 16 = %v, want 0.25", got)
	}
}

func TestSnapshotRestoreKeepsWaves(t *testing.T) {
	w := newWaves(t)
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{X: 1}, Amplitude: 2, Wavelength: 20, Speed: 0, Steepness: 0.3})
	before, err := w.Height(5, 0, 0)
	if err != nil {
		t.Fatalf("Height returned error: %v", err)
	}
	snap, err := w.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(snap) == 0 {
		t.Fatal("Snapshot returned an empty payload")
	}
	w.Reset()
	if got := w.WaveCount(); got != 0 {
		t.Fatalf("WaveCount after Reset = %d, want 0", got)
	}
	if err := w.Restore(snap); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if got := w.WaveCount(); got != 1 {
		t.Fatalf("WaveCount after Restore = %d, want 1", got)
	}
	after, err := w.Height(5, 0, 0)
	if err != nil {
		t.Fatalf("Height after restore returned error: %v", err)
	}
	if math.Abs(after-before) > 1e-9 {
		t.Fatalf("Height after restore = %v, want %v", after, before)
	}
	if err := w.Restore([]byte("waves: nope")); !errors.Is(err, taskpkg.ErrSnapshot) {
		t.Fatalf("Restore(garbage) error = %v, want ErrSnapshot", err)
	}
}

func TestConcurrentMeshBuild(t *testing.T) {
	w := newWaves(t)
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{X: 1}, Amplitude: 1.5, Wavelength: 16, Speed: 1.5, Steepness: 0.4})
	mustAddWave(t, w, taskpkg.WaveParam{Dir: taskpkg.Vec2{Z: 1}, Amplitude: 0.8, Wavelength: 9, Speed: 0.7, Steepness: 0.3})
	var wg sync.WaitGroup
	errs := make([]error, 8)
	sizes := make([]int, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			vertices, err := w.BuildMesh(0.5, 0)
			errs[i] = err
			sizes[i] = len(vertices)
		}(i)
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent BuildMesh[%d] returned error: %v", i, errs[i])
		}
		if sizes[i] != 65*65 {
			t.Fatalf("concurrent BuildMesh[%d] returned %d vertices, want %d", i, sizes[i], 65*65)
		}
	}
}
