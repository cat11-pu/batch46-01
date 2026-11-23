package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-d-01"
)

func newFog(t *testing.T) *taskpkg.VolumetricFog {
	t.Helper()
	f, err := taskpkg.NewVolumetricFog(
		taskpkg.FroxelGrid{Width: 8, Height: 8, Depth: 16},
		taskpkg.Frustum{Near: 0.1, Far: 1000, TanHalfFovY: 0.5, Aspect: 16.0 / 9.0},
	)
	if err != nil {
		t.Fatalf("NewVolumetricFog returned error: %v", err)
	}
	if f == nil {
		t.Fatal("NewVolumetricFog returned a nil volume")
	}
	return f
}

func fogBoundary(t *testing.T, f *taskpkg.VolumetricFog, slice int) float64 {
	t.Helper()
	v, err := f.SliceBoundary(slice)
	if err != nil {
		t.Fatalf("SliceBoundary(%d) returned error: %v", slice, err)
	}
	return v
}

func TestReconstructViewZEndpoints(t *testing.T) {
	f := newFog(t)
	near, err := f.ReconstructViewZ(0)
	if err != nil {
		t.Fatalf("ReconstructViewZ(0) returned error: %v", err)
	}
	if math.Abs(near-0.1) > 1e-12 {
		t.Fatalf("ReconstructViewZ(0) = %v, want 0.1", near)
	}
	far, err := f.ReconstructViewZ(1)
	if err != nil {
		t.Fatalf("ReconstructViewZ(1) returned error: %v", err)
	}
	if math.Abs(far-1000) > 1e-9 {
		t.Fatalf("ReconstructViewZ(1) = %v, want 1000", far)
	}
	mid, err := f.ReconstructViewZ(0.5)
	if err != nil {
		t.Fatalf("ReconstructViewZ(0.5) returned error: %v", err)
	}
	want := 0.1 * 1000 / (1000 - 0.5*(1000-0.1))
	if math.Abs(mid-want) > 1e-9 {
		t.Fatalf("ReconstructViewZ(0.5) = %v, want %v", mid, want)
	}
}

func TestReconstructViewZRejectsInvalidDepth(t *testing.T) {
	f := newFog(t)
	for _, depth := range []float64{-0.001, 1.0001, math.NaN()} {
		if _, err := f.ReconstructViewZ(depth); !errors.Is(err, taskpkg.ErrInvalidDepth) {
			t.Fatalf("ReconstructViewZ(%v) error = %v, want ErrInvalidDepth", depth, err)
		}
	}
}

func TestSliceBoundaryIsExponential(t *testing.T) {
	f := newFog(t)
	if b := fogBoundary(t, f, 0); math.Abs(b-0.1) > 1e-12 {
		t.Fatalf("SliceBoundary(0) = %v, want 0.1", b)
	}
	if b := fogBoundary(t, f, 16); math.Abs(b-1000) > 1e-9 {
		t.Fatalf("SliceBoundary(16) = %v, want 1000", b)
	}
	wantHalf := 0.1 * math.Pow(1000.0/0.1, 0.5)
	if b := fogBoundary(t, f, 8); math.Abs(b-wantHalf) > 1e-9 {
		t.Fatalf("SliceBoundary(8) = %v, want %v", b, wantHalf)
	}
	prev := fogBoundary(t, f, 0)
	for i := 1; i <= 16; i++ {
		cur := fogBoundary(t, f, i)
		if cur <= prev {
			t.Fatalf("SliceBoundary not increasing at %d: %v after %v", i, cur, prev)
		}
		prev = cur
	}
}

func TestSliceBoundaryRangeErrors(t *testing.T) {
	f := newFog(t)
	for _, slice := range []int{-1, 17} {
		if _, err := f.SliceBoundary(slice); !errors.Is(err, taskpkg.ErrSliceRange) {
			t.Fatalf("SliceBoundary(%d) error = %v, want ErrSliceRange", slice, err)
		}
	}
}

func TestSliceCenterIsGeometricMean(t *testing.T) {
	f := newFog(t)
	b0 := fogBoundary(t, f, 0)
	b1 := fogBoundary(t, f, 1)
	center, err := f.SliceCenter(0)
	if err != nil {
		t.Fatalf("SliceCenter(0) returned error: %v", err)
	}
	want := math.Sqrt(b0 * b1)
	if math.Abs(center-want) > 1e-12 {
		t.Fatalf("SliceCenter(0) = %v, want geometric mean %v", center, want)
	}
	if _, err := f.SliceCenter(16); !errors.Is(err, taskpkg.ErrSliceRange) {
		t.Fatalf("SliceCenter(16) error = %v, want ErrSliceRange", err)
	}
}

func TestFroxelForDepthMapsSlices(t *testing.T) {
	f := newFog(t)
	z, err := f.FroxelForDepth(0.1)
	if err != nil {
		t.Fatalf("FroxelForDepth(near) returned error: %v", err)
	}
	if z != 0 {
		t.Fatalf("FroxelForDepth(near) = %d, want 0", z)
	}
	b10 := fogBoundary(t, f, 10)
	z, err = f.FroxelForDepth(b10)
	if err != nil {
		t.Fatalf("FroxelForDepth(boundary 10) returned error: %v", err)
	}
	if z != 10 {
		t.Fatalf("FroxelForDepth(boundary 10) = %d, want 10", z)
	}
	z, err = f.FroxelForDepth(1000 * 0.9999)
	if err != nil {
		t.Fatalf("FroxelForDepth(near far) returned error: %v", err)
	}
	if z != 15 {
		t.Fatalf("FroxelForDepth(near far) = %d, want 15", z)
	}
}

func TestFroxelForDepthRejectsOutsideRange(t *testing.T) {
	f := newFog(t)
	for _, viewZ := range []float64{0.05, 1000, -1, math.NaN()} {
		if _, err := f.FroxelForDepth(viewZ); !errors.Is(err, taskpkg.ErrDepthRange) {
			t.Fatalf("FroxelForDepth(%v) error = %v, want ErrDepthRange", viewZ, err)
		}
	}
}

func TestAddSampleAccumulatesOpticalDepth(t *testing.T) {
	f := newFog(t)
	b3 := fogBoundary(t, f, 3)
	b4 := fogBoundary(t, f, 4)
	if err := f.AddSample(taskpkg.FogSample{Slice: 3, Density: 0.25}); err != nil {
		t.Fatalf("AddSample returned error: %v", err)
	}
	got, err := f.OpticalDepth(3)
	if err != nil {
		t.Fatalf("OpticalDepth(3) returned error: %v", err)
	}
	want := 0.25 * (b4 - b3)
	if math.Abs(got-want) > 1e-12 {
		t.Fatalf("OpticalDepth(3) = %v, want %v", got, want)
	}
	if earlier, err := f.OpticalDepth(2); err != nil || earlier != 0 {
		t.Fatalf("OpticalDepth(2) = %v, %v, want 0, nil", earlier, err)
	}
}

func TestNegativeDensityRejected(t *testing.T) {
	f := newFog(t)
	if err := f.AddSample(taskpkg.FogSample{Slice: 3, Density: -1}); !errors.Is(err, taskpkg.ErrNegativeDensity) {
		t.Fatalf("AddSample(-1) error = %v, want ErrNegativeDensity", err)
	}
	if err := f.AddSample(taskpkg.FogSample{Slice: 3, Density: math.NaN()}); !errors.Is(err, taskpkg.ErrNegativeDensity) {
		t.Fatalf("AddSample(NaN) error = %v, want ErrNegativeDensity", err)
	}
	if got, err := f.OpticalDepth(3); err != nil || got != 0 {
		t.Fatalf("OpticalDepth(3) after rejected samples = %v, %v, want 0, nil", got, err)
	}
}

func TestAddSamplesIsAtomic(t *testing.T) {
	f := newFog(t)
	err := f.AddSamples([]taskpkg.FogSample{
		{Slice: 0, Density: 0.5},
		{Slice: 1, Density: -0.5},
	})
	if !errors.Is(err, taskpkg.ErrNegativeDensity) {
		t.Fatalf("AddSamples with one bad sample error = %v, want ErrNegativeDensity", err)
	}
	if got, err := f.OpticalDepth(0); err != nil || got != 0 {
		t.Fatalf("OpticalDepth(0) after rejected batch = %v, %v, want 0, nil", got, err)
	}
	b0 := fogBoundary(t, f, 0)
	b1 := fogBoundary(t, f, 1)
	if err := f.AddSamples([]taskpkg.FogSample{{Slice: 0, Density: 0.5}}); err != nil {
		t.Fatalf("AddSamples valid batch returned error: %v", err)
	}
	if got, err := f.OpticalDepth(0); err != nil || math.Abs(got-0.5*(b1-b0)) > 1e-12 {
		t.Fatalf("OpticalDepth(0) = %v, %v, want %v", got, err, 0.5*(b1-b0))
	}
}

func TestSliceRangeErrors(t *testing.T) {
	f := newFog(t)
	if err := f.AddSample(taskpkg.FogSample{Slice: 16, Density: 1}); !errors.Is(err, taskpkg.ErrSliceRange) {
		t.Fatalf("AddSample(slice 16) error = %v, want ErrSliceRange", err)
	}
	if _, err := f.OpticalDepth(-1); !errors.Is(err, taskpkg.ErrSliceRange) {
		t.Fatalf("OpticalDepth(-1) error = %v, want ErrSliceRange", err)
	}
	if _, err := f.Transmittance(99); !errors.Is(err, taskpkg.ErrSliceRange) {
		t.Fatalf("Transmittance(99) error = %v, want ErrSliceRange", err)
	}
	if _, err := f.InScatter(16); !errors.Is(err, taskpkg.ErrSliceRange) {
		t.Fatalf("InScatter(16) error = %v, want ErrSliceRange", err)
	}
}

func TestTransmittanceAndInScatter(t *testing.T) {
	f := newFog(t)
	seg0 := fogBoundary(t, f, 1) - fogBoundary(t, f, 0)
	seg5 := fogBoundary(t, f, 6) - fogBoundary(t, f, 5)
	if err := f.AddSample(taskpkg.FogSample{Slice: 0, Density: 1.0}); err != nil {
		t.Fatalf("AddSample returned error: %v", err)
	}
	if err := f.AddSample(taskpkg.FogSample{Slice: 5, Density: 0.5}); err != nil {
		t.Fatalf("AddSample returned error: %v", err)
	}
	if err := f.AddSample(taskpkg.FogSample{Slice: 12, Density: 0.25}); err != nil {
		t.Fatalf("AddSample returned error: %v", err)
	}
	trans, err := f.Transmittance(0)
	if err != nil {
		t.Fatalf("Transmittance(0) returned error: %v", err)
	}
	want := math.Exp(-seg0)
	if math.Abs(trans-want) > 1e-12 {
		t.Fatalf("Transmittance(0) = %v, want %v", trans, want)
	}
	scatter, err := f.InScatter(0)
	if err != nil {
		t.Fatalf("InScatter(0) returned error: %v", err)
	}
	if math.Abs(scatter-(1-trans)) > 1e-12 {
		t.Fatalf("InScatter(0) = %v, want %v", scatter, 1-trans)
	}
	through4, err := f.Transmittance(4)
	if err != nil {
		t.Fatalf("Transmittance(4) returned error: %v", err)
	}
	through5, err := f.Transmittance(5)
	if err != nil {
		t.Fatalf("Transmittance(5) returned error: %v", err)
	}
	if math.Abs(through5-math.Exp(-(seg0+0.5*seg5))) > 1e-12 {
		t.Fatalf("Transmittance(5) = %v, want %v", through5, math.Exp(-(seg0+0.5*seg5)))
	}
	if through5 >= through4 {
		t.Fatalf("Transmittance(5) = %v is not below Transmittance(4) = %v", through5, through4)
	}
	through12, err := f.Transmittance(12)
	if err != nil {
		t.Fatalf("Transmittance(12) returned error: %v", err)
	}
	if through12 >= through5 {
		t.Fatalf("Transmittance(12) = %v is not below Transmittance(5) = %v", through12, through5)
	}
	last, err := f.Transmittance(15)
	if err != nil {
		t.Fatalf("Transmittance(15) returned error: %v", err)
	}
	if math.Abs(last-through12) > 1e-12 {
		t.Fatalf("Transmittance(15) = %v, want %v (no density after slice 12)", last, through12)
	}
}

func TestScreenToFroxelMapsUV(t *testing.T) {
	f := newFog(t)
	center, err := f.ScreenToFroxel(taskpkg.Vec2{X: 0.5, Y: 0.5}, 0.1)
	if err != nil {
		t.Fatalf("ScreenToFroxel(center) returned error: %v", err)
	}
	if center != [3]int{4, 4, 0} {
		t.Fatalf("ScreenToFroxel(center) = %v, want [4 4 0]", center)
	}
	topLeft, err := f.ScreenToFroxel(taskpkg.Vec2{X: 0, Y: 0}, 0.1)
	if err != nil {
		t.Fatalf("ScreenToFroxel(top-left) returned error: %v", err)
	}
	if topLeft != [3]int{0, 7, 0} {
		t.Fatalf("ScreenToFroxel(top-left) = %v, want [0 7 0]", topLeft)
	}
	bottomRight, err := f.ScreenToFroxel(taskpkg.Vec2{X: 1, Y: 1}, 0.1)
	if err != nil {
		t.Fatalf("ScreenToFroxel(bottom-right) returned error: %v", err)
	}
	if bottomRight != [3]int{7, 0, 0} {
		t.Fatalf("ScreenToFroxel(bottom-right) = %v, want [7 0 0]", bottomRight)
	}
	if _, err := f.ScreenToFroxel(taskpkg.Vec2{X: 1.2, Y: 0.5}, 0.1); !errors.Is(err, taskpkg.ErrBadUV) {
		t.Fatalf("ScreenToFroxel(uv 1.2) error = %v, want ErrBadUV", err)
	}
}

func TestSnapshotRestoreRoundTrip(t *testing.T) {
	f := newFog(t)
	if err := f.AddSample(taskpkg.FogSample{Slice: 3, Density: 0.5}); err != nil {
		t.Fatalf("AddSample returned error: %v", err)
	}
	if err := f.AddSample(taskpkg.FogSample{Slice: 7, Density: 0.25}); err != nil {
		t.Fatalf("AddSample returned error: %v", err)
	}
	before, err := f.OpticalDepth(7)
	if err != nil {
		t.Fatalf("OpticalDepth(7) returned error: %v", err)
	}
	snap, err := f.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	if len(snap) == 0 {
		t.Fatal("Snapshot returned an empty payload")
	}
	f.Reset()
	if got, err := f.OpticalDepth(7); err != nil || got != 0 {
		t.Fatalf("OpticalDepth(7) after Reset = %v, %v, want 0, nil", got, err)
	}
	if err := f.Restore(snap); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	after, err := f.OpticalDepth(7)
	if err != nil {
		t.Fatalf("OpticalDepth(7) after restore returned error: %v", err)
	}
	if math.Abs(after-before) > 1e-12 {
		t.Fatalf("OpticalDepth(7) after restore = %v, want %v", after, before)
	}
	if err := f.Restore([]byte("densities: nope")); !errors.Is(err, taskpkg.ErrSnapshot) {
		t.Fatalf("Restore(garbage) error = %v, want ErrSnapshot", err)
	}
}

func TestConcurrentAddSampleKeepsEveryContribution(t *testing.T) {
	f := newFog(t)
	seg := fogBoundary(t, f, 6) - fogBoundary(t, f, 5)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = f.AddSample(taskpkg.FogSample{Slice: 5, Density: 0.01})
		}()
	}
	wg.Wait()
	got, err := f.OpticalDepth(5)
	if err != nil {
		t.Fatalf("OpticalDepth(5) returned error: %v", err)
	}
	want := 2.0 * seg
	if math.Abs(got-want) > 1e-9 {
		t.Fatalf("OpticalDepth(5) = %v, want %v after 200 concurrent adds", got, want)
	}
}
