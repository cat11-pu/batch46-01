package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-d-03"
)

func newGrid(t *testing.T, counts [3]int) *taskpkg.LightProbe {
	t.Helper()
	g, err := taskpkg.NewLightProbe(taskpkg.Vec3{}, 1, counts)
	if err != nil {
		t.Fatalf("NewLightProbe(%v) returned error: %v", counts, err)
	}
	if g == nil {
		t.Fatal("NewLightProbe returned a nil grid")
	}
	return g
}

func fillGrid(t *testing.T, g *taskpkg.LightProbe, counts [3]int, fn func(x, y, z int) taskpkg.SH9) {
	t.Helper()
	for z := 0; z < counts[2]; z++ {
		for y := 0; y < counts[1]; y++ {
			for x := 0; x < counts[0]; x++ {
				if err := g.SetProbe(x, y, z, fn(x, y, z)); err != nil {
					t.Fatalf("SetProbe(%d,%d,%d) returned error: %v", x, y, z, err)
				}
			}
		}
	}
}

func shNear(a, b taskpkg.SH9, eps float64) bool {
	for i := range a.C {
		if math.Abs(a.C[i].X-b.C[i].X) > eps ||
			math.Abs(a.C[i].Y-b.C[i].Y) > eps ||
			math.Abs(a.C[i].Z-b.C[i].Z) > eps {
			return false
		}
	}
	return true
}

func TestConstructorValidation(t *testing.T) {
	if _, err := taskpkg.NewLightProbe(taskpkg.Vec3{}, 0, [3]int{2, 2, 2}); !errors.Is(err, taskpkg.ErrBadSpacing) {
		t.Fatalf("spacing 0 error = %v, want ErrBadSpacing", err)
	}
	if _, err := taskpkg.NewLightProbe(taskpkg.Vec3{}, math.NaN(), [3]int{2, 2, 2}); !errors.Is(err, taskpkg.ErrBadSpacing) {
		t.Fatalf("spacing NaN error = %v, want ErrBadSpacing", err)
	}
	if _, err := taskpkg.NewLightProbe(taskpkg.Vec3{}, 1, [3]int{1, 4, 4}); !errors.Is(err, taskpkg.ErrBadGrid) {
		t.Fatalf("counts {1,4,4} error = %v, want ErrBadGrid", err)
	}
	if _, err := taskpkg.NewLightProbe(taskpkg.Vec3{}, 1, [3]int{17, 4, 4}); !errors.Is(err, taskpkg.ErrBadGrid) {
		t.Fatalf("counts {17,4,4} error = %v, want ErrBadGrid", err)
	}
	if _, err := taskpkg.NewLightProbe(taskpkg.Vec3{}, 1, [3]int{16, 16, 16}); err != nil {
		t.Fatalf("counts {16,16,16} returned error: %v", err)
	}
}

func TestProbeCountAndIndex(t *testing.T) {
	g := newGrid(t, [3]int{4, 3, 2})
	if got := g.ProbeCount(); got != 24 {
		t.Fatalf("ProbeCount = %d, want 24", got)
	}
	index, err := g.Index(1, 2, 1)
	if err != nil {
		t.Fatalf("Index(1,2,1) returned error: %v", err)
	}
	if index != 21 {
		t.Fatalf("Index(1,2,1) = %d, want 21", index)
	}
	coord, err := g.Coordinates(21)
	if err != nil {
		t.Fatalf("Coordinates(21) returned error: %v", err)
	}
	if coord != [3]int{1, 2, 1} {
		t.Fatalf("Coordinates(21) = %v, want [1 2 1]", coord)
	}
	if _, err := g.Index(4, 0, 0); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("Index(4,0,0) error = %v, want ErrIndexRange", err)
	}
	if _, err := g.Coordinates(24); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("Coordinates(24) error = %v, want ErrIndexRange", err)
	}
}

func TestSetGetProbeRoundTrip(t *testing.T) {
	g := newGrid(t, [3]int{4, 4, 4})
	want := taskpkg.SH9{C: [9]taskpkg.Vec3{{X: 0.5, Y: 1.5, Z: 2.5}, {}, {}, {}, {}, {}, {}, {X: 9, Y: 8, Z: 7}}}
	if err := g.SetProbe(1, 2, 1, want); err != nil {
		t.Fatalf("SetProbe returned error: %v", err)
	}
	got, err := g.GetProbe(1, 2, 1)
	if err != nil {
		t.Fatalf("GetProbe returned error: %v", err)
	}
	if got != want {
		t.Fatalf("GetProbe = %+v, want %+v", got, want)
	}
	unset, err := g.GetProbe(0, 0, 0)
	if err != nil {
		t.Fatalf("GetProbe on unset probe returned error: %v", err)
	}
	if unset != (taskpkg.SH9{}) {
		t.Fatalf("GetProbe on unset probe = %+v, want zero SH9", unset)
	}
}

func TestProbeIndexRangeErrors(t *testing.T) {
	g := newGrid(t, [3]int{4, 4, 4})
	if err := g.SetProbe(-1, 0, 0, taskpkg.SH9{}); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("SetProbe(-1,0,0) error = %v, want ErrIndexRange", err)
	}
	if err := g.SetProbe(0, 4, 0, taskpkg.SH9{}); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("SetProbe(0,4,0) error = %v, want ErrIndexRange", err)
	}
	if err := g.Invalidate(0, 0, 9); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("Invalidate(0,0,9) error = %v, want ErrIndexRange", err)
	}
	if _, err := g.GetProbe(9, 9, 9); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("GetProbe(9,9,9) error = %v, want ErrIndexRange", err)
	}
}

func TestCellForPositionMapsLocalCoords(t *testing.T) {
	g, err := taskpkg.NewLightProbe(taskpkg.Vec3{}, 2, [3]int{4, 4, 4})
	if err != nil {
		t.Fatalf("NewLightProbe returned error: %v", err)
	}
	cell, local, err := g.CellForPosition(taskpkg.Vec3{X: 3, Y: 5, Z: 5})
	if err != nil {
		t.Fatalf("CellForPosition returned error: %v", err)
	}
	if cell != [3]int{1, 2, 2} {
		t.Fatalf("cell = %v, want [1 2 2]", cell)
	}
	if math.Abs(local.X-0.5) > 1e-12 || math.Abs(local.Y-0.5) > 1e-12 || math.Abs(local.Z-0.5) > 1e-12 {
		t.Fatalf("local = %+v, want (0.5,0.5,0.5)", local)
	}
	if _, _, err := g.CellForPosition(taskpkg.Vec3{X: -1}); !errors.Is(err, taskpkg.ErrOutsideVolume) {
		t.Fatalf("CellForPosition(-1,0,0) error = %v, want ErrOutsideVolume", err)
	}
	farCell, farLocal, err := g.CellForPosition(taskpkg.Vec3{X: 6})
	if err != nil {
		t.Fatalf("CellForPosition(far face) returned error: %v", err)
	}
	if farCell != [3]int{2, 0, 0} {
		t.Fatalf("far face cell = %v, want [2 0 0]", farCell)
	}
	if math.Abs(farLocal.X-1) > 1e-12 {
		t.Fatalf("far face local.X = %v, want 1", farLocal.X)
	}
	if _, _, err := g.CellForPosition(taskpkg.Vec3{X: 6.5}); !errors.Is(err, taskpkg.ErrOutsideVolume) {
		t.Fatalf("CellForPosition(6.5,0,0) error = %v, want ErrOutsideVolume", err)
	}
}

func TestTrilinearConstantField(t *testing.T) {
	counts := [3]int{3, 3, 3}
	g := newGrid(t, counts)
	constant := taskpkg.SH9{C: [9]taskpkg.Vec3{{X: 2, Y: 3, Z: 4}, {}, {}, {}, {X: 1, Y: 1, Z: 1}}}
	fillGrid(t, g, counts, func(x, y, z int) taskpkg.SH9 { return constant })
	got, err := g.Trilinear(taskpkg.Vec3{X: 1.3, Y: 0.7, Z: 1.1})
	if err != nil {
		t.Fatalf("Trilinear returned error: %v", err)
	}
	if !shNear(got, constant, 1e-9) {
		t.Fatalf("Trilinear on constant field = %+v, want %+v", got, constant)
	}
}

func TestTrilinearGradient(t *testing.T) {
	counts := [3]int{2, 2, 2}
	g := newGrid(t, counts)
	fillGrid(t, g, counts, func(x, y, z int) taskpkg.SH9 {
		return taskpkg.SH9{C: [9]taskpkg.Vec3{{X: 10*float64(x) + 100*float64(y) + 1000*float64(z)}}}
	})
	got, err := g.Trilinear(taskpkg.Vec3{X: 0.5, Y: 0.5, Z: 0.5})
	if err != nil {
		t.Fatalf("Trilinear returned error: %v", err)
	}
	if math.Abs(got.C[0].X-555) > 1e-9 {
		t.Fatalf("Trilinear gradient = %v, want 555", got.C[0].X)
	}
}

func TestTrilinearOutsideVolume(t *testing.T) {
	g := newGrid(t, [3]int{2, 2, 2})
	if _, err := g.Trilinear(taskpkg.Vec3{X: -0.1}); !errors.Is(err, taskpkg.ErrOutsideVolume) {
		t.Fatalf("Trilinear(-0.1,0,0) error = %v, want ErrOutsideVolume", err)
	}
	if _, err := g.Trilinear(taskpkg.Vec3{X: 1.5}); !errors.Is(err, taskpkg.ErrOutsideVolume) {
		t.Fatalf("Trilinear(1.5,0,0) error = %v, want ErrOutsideVolume", err)
	}
}

func TestTetrahedralConstantField(t *testing.T) {
	counts := [3]int{3, 3, 3}
	g := newGrid(t, counts)
	constant := taskpkg.SH9{C: [9]taskpkg.Vec3{{X: -1, Y: 0.5, Z: 8}}}
	fillGrid(t, g, counts, func(x, y, z int) taskpkg.SH9 { return constant })
	got, err := g.Tetrahedral(taskpkg.Vec3{X: 1.2, Y: 0.3, Z: 0.8})
	if err != nil {
		t.Fatalf("Tetrahedral returned error: %v", err)
	}
	if !shNear(got, constant, 1e-9) {
		t.Fatalf("Tetrahedral on constant field = %+v, want %+v", got, constant)
	}
}

func TestTetrahedralCornerExact(t *testing.T) {
	counts := [3]int{2, 2, 2}
	g := newGrid(t, counts)
	fillGrid(t, g, counts, func(x, y, z int) taskpkg.SH9 {
		return taskpkg.SH9{C: [9]taskpkg.Vec3{{X: float64(x + 2*y + 4*z)}}}
	})
	origin, err := g.Tetrahedral(taskpkg.Vec3{})
	if err != nil {
		t.Fatalf("Tetrahedral at origin returned error: %v", err)
	}
	if math.Abs(origin.C[0].X) > 1e-12 {
		t.Fatalf("Tetrahedral at origin = %v, want 0", origin.C[0].X)
	}
	far, err := g.Tetrahedral(taskpkg.Vec3{X: 1, Y: 1, Z: 1})
	if err != nil {
		t.Fatalf("Tetrahedral at far corner returned error: %v", err)
	}
	if math.Abs(far.C[0].X-7) > 1e-12 {
		t.Fatalf("Tetrahedral at far corner = %v, want 7", far.C[0].X)
	}
}

func TestIrradianceDC(t *testing.T) {
	g := newGrid(t, [3]int{2, 2, 2})
	sh := taskpkg.SH9{C: [9]taskpkg.Vec3{{X: 1, Y: 2, Z: 3}}}
	got := g.Irradiance(sh, taskpkg.Vec3{Y: 1})
	if math.Abs(got.X-math.Pi) > 1e-9 || math.Abs(got.Y-2*math.Pi) > 1e-9 || math.Abs(got.Z-3*math.Pi) > 1e-9 {
		t.Fatalf("Irradiance(DC) = %+v, want (%v,%v,%v)", got, math.Pi, 2*math.Pi, 3*math.Pi)
	}
}

func TestIrradianceLinearAndQuadratic(t *testing.T) {
	g := newGrid(t, [3]int{2, 2, 2})
	linear := taskpkg.SH9{C: [9]taskpkg.Vec3{{}, {}, {}, {Y: 2}}}
	gotLinear := g.Irradiance(linear, taskpkg.Vec3{X: 1})
	if math.Abs(gotLinear.Y-4*math.Pi/3) > 1e-9 {
		t.Fatalf("Irradiance(linear) = %+v, want Y = 4pi/3", gotLinear)
	}
	quad := taskpkg.SH9{C: [9]taskpkg.Vec3{{}, {}, {}, {}, {}, {}, {Z: 3}}}
	gotQuad := g.Irradiance(quad, taskpkg.Vec3{Z: 1})
	if math.Abs(gotQuad.Z-3*math.Pi/2) > 1e-9 {
		t.Fatalf("Irradiance(quadratic) = %+v, want Z = 3pi/2", gotQuad)
	}
}

func TestBlendClampAndMidpoint(t *testing.T) {
	g := newGrid(t, [3]int{2, 2, 2})
	a := taskpkg.SH9{C: [9]taskpkg.Vec3{{}}}
	b := taskpkg.SH9{C: [9]taskpkg.Vec3{{X: 2, Y: 4, Z: 6}}}
	mid := g.Blend(a, b, 0.5)
	if math.Abs(mid.C[0].X-1) > 1e-12 || math.Abs(mid.C[0].Y-2) > 1e-12 || math.Abs(mid.C[0].Z-3) > 1e-12 {
		t.Fatalf("Blend midpoint = %+v, want (1,2,3)", mid.C[0])
	}
	if got := g.Blend(a, b, -1); got != a {
		t.Fatalf("Blend(w=-1) = %+v, want a", got)
	}
	if got := g.Blend(a, b, 2); got != b {
		t.Fatalf("Blend(w=2) = %+v, want b", got)
	}
}

func TestInvalidateTracksDirtyProbes(t *testing.T) {
	g := newGrid(t, [3]int{2, 2, 2})
	if err := g.Invalidate(0, 0, 0); err != nil {
		t.Fatalf("Invalidate returned error: %v", err)
	}
	if err := g.Invalidate(1, 0, 0); err != nil {
		t.Fatalf("Invalidate returned error: %v", err)
	}
	if got := g.DirtyCount(); got != 2 {
		t.Fatalf("DirtyCount = %d, want 2", got)
	}
	if err := g.Invalidate(0, 0, 0); err != nil {
		t.Fatalf("repeat Invalidate returned error: %v", err)
	}
	if got := g.DirtyCount(); got != 2 {
		t.Fatalf("DirtyCount after repeat invalidate = %d, want 2", got)
	}
	if err := g.SetProbe(0, 0, 0, taskpkg.SH9{}); err != nil {
		t.Fatalf("SetProbe returned error: %v", err)
	}
	if got := g.DirtyCount(); got != 1 {
		t.Fatalf("DirtyCount after SetProbe = %d, want 1", got)
	}
}

func TestSnapshotRestoreAndConcurrentReads(t *testing.T) {
	g := newGrid(t, [3]int{2, 2, 2})
	sh := taskpkg.SH9{C: [9]taskpkg.Vec3{{X: 4, Y: 5, Z: 6}}}
	if err := g.SetProbe(1, 1, 1, sh); err != nil {
		t.Fatalf("SetProbe returned error: %v", err)
	}
	if err := g.Invalidate(0, 0, 0); err != nil {
		t.Fatalf("Invalidate returned error: %v", err)
	}
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	restored := newGrid(t, [3]int{2, 2, 2})
	if err := restored.Restore(snap); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	got, err := restored.GetProbe(1, 1, 1)
	if err != nil {
		t.Fatalf("GetProbe after restore returned error: %v", err)
	}
	if got != sh {
		t.Fatalf("restored probe = %+v, want %+v", got, sh)
	}
	if got := restored.DirtyCount(); got != 1 {
		t.Fatalf("restored DirtyCount = %d, want 1", got)
	}
	if err := restored.Restore([]byte("sh: nope")); !errors.Is(err, taskpkg.ErrSnapshot) {
		t.Fatalf("Restore(garbage) error = %v, want ErrSnapshot", err)
	}
	other := newGrid(t, [3]int{3, 2, 2})
	if err := other.Restore(snap); !errors.Is(err, taskpkg.ErrSnapshot) {
		t.Fatalf("Restore into mismatched grid error = %v, want ErrSnapshot", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 32)
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := restored.Trilinear(taskpkg.Vec3{X: 0.5, Y: 0.5, Z: 0.5}); err != nil {
				errs[i] = err
				return
			}
			_, errs[i] = restored.GetProbe(1, 1, 1)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent read %d returned error: %v", i, err)
		}
	}
}
