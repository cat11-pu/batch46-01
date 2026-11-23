package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-d-05"
)

func newVolume(t *testing.T) *taskpkg.IrradianceVolume {
	t.Helper()
	v, err := taskpkg.NewIrradianceVolume(taskpkg.VoxelGrid{X: 4, Y: 4, Z: 4}, 1)
	if err != nil {
		t.Fatalf("NewIrradianceVolume returned error: %v", err)
	}
	if v == nil {
		t.Fatal("NewIrradianceVolume returned a nil volume")
	}
	return v
}

func readIrradiance(t *testing.T, v *taskpkg.IrradianceVolume, coord [3]int) taskpkg.Vec3 {
	t.Helper()
	got, err := v.Irradiance(coord)
	if err != nil {
		t.Fatalf("Irradiance(%v) returned error: %v", coord, err)
	}
	return got
}

func TestConstructorValidation(t *testing.T) {
	if _, err := taskpkg.NewIrradianceVolume(taskpkg.VoxelGrid{X: 4, Y: 4, Z: 4}, 0); !errors.Is(err, taskpkg.ErrBadSpacing) {
		t.Fatalf("spacing 0 error = %v, want ErrBadSpacing", err)
	}
	if _, err := taskpkg.NewIrradianceVolume(taskpkg.VoxelGrid{X: 4, Y: 4, Z: 4}, math.NaN()); !errors.Is(err, taskpkg.ErrBadSpacing) {
		t.Fatalf("spacing NaN error = %v, want ErrBadSpacing", err)
	}
	if _, err := taskpkg.NewIrradianceVolume(taskpkg.VoxelGrid{X: 0, Y: 4, Z: 4}, 1); !errors.Is(err, taskpkg.ErrBadGrid) {
		t.Fatalf("grid X=0 error = %v, want ErrBadGrid", err)
	}
	if _, err := taskpkg.NewIrradianceVolume(taskpkg.VoxelGrid{X: 65, Y: 1, Z: 1}, 1); !errors.Is(err, taskpkg.ErrBadGrid) {
		t.Fatalf("grid X=65 error = %v, want ErrBadGrid", err)
	}
	if _, err := taskpkg.NewIrradianceVolume(taskpkg.VoxelGrid{X: 64, Y: 64, Z: 64}, 1); err != nil {
		t.Fatalf("grid 64^3 returned error: %v", err)
	}
}

func TestVoxelIndexRowMajor(t *testing.T) {
	v := newVolume(t)
	if got := v.VoxelCount(); got != 64 {
		t.Fatalf("VoxelCount = %d, want 64", got)
	}
	index, err := v.VoxelIndex([3]int{1, 2, 3})
	if err != nil {
		t.Fatalf("VoxelIndex returned error: %v", err)
	}
	if index != 57 {
		t.Fatalf("VoxelIndex(1,2,3) = %d, want 57", index)
	}
	coord, err := v.VoxelCoord(57)
	if err != nil {
		t.Fatalf("VoxelCoord returned error: %v", err)
	}
	if coord != [3]int{1, 2, 3} {
		t.Fatalf("VoxelCoord(57) = %v, want [1 2 3]", coord)
	}
	if _, err := v.VoxelIndex([3]int{-1, 0, 0}); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("VoxelIndex(-1,0,0) error = %v, want ErrIndexRange", err)
	}
	if _, err := v.VoxelCoord(64); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("VoxelCoord(64) error = %v, want ErrIndexRange", err)
	}
}

func TestWorldToVoxel(t *testing.T) {
	v, err := taskpkg.NewIrradianceVolume(taskpkg.VoxelGrid{X: 4, Y: 4, Z: 4}, 2)
	if err != nil {
		t.Fatalf("NewIrradianceVolume returned error: %v", err)
	}
	coord, fraction, err := v.WorldToVoxel(taskpkg.Vec3{X: 3, Y: 5, Z: 5})
	if err != nil {
		t.Fatalf("WorldToVoxel returned error: %v", err)
	}
	if coord != [3]int{1, 2, 2} {
		t.Fatalf("WorldToVoxel coord = %v, want [1 2 2]", coord)
	}
	if math.Abs(fraction.X-0.5) > 1e-12 || math.Abs(fraction.Y-0.5) > 1e-12 || math.Abs(fraction.Z-0.5) > 1e-12 {
		t.Fatalf("WorldToVoxel fraction = %+v, want (0.5,0.5,0.5)", fraction)
	}
	if _, _, err := v.WorldToVoxel(taskpkg.Vec3{X: -0.1}); !errors.Is(err, taskpkg.ErrOutsideVolume) {
		t.Fatalf("WorldToVoxel(-0.1,0,0) error = %v, want ErrOutsideVolume", err)
	}
	if _, _, err := v.WorldToVoxel(taskpkg.Vec3{X: 8}); !errors.Is(err, taskpkg.ErrOutsideVolume) {
		t.Fatalf("WorldToVoxel(8,0,0) error = %v, want ErrOutsideVolume", err)
	}
	if _, _, err := v.WorldToVoxel(taskpkg.Vec3{Y: math.NaN()}); !errors.Is(err, taskpkg.ErrOutsideVolume) {
		t.Fatalf("WorldToVoxel(NaN) error = %v, want ErrOutsideVolume", err)
	}
}

func TestFrameLifecycle(t *testing.T) {
	v := newVolume(t)
	first, err := v.BeginFrame()
	if err != nil {
		t.Fatalf("BeginFrame returned error: %v", err)
	}
	if first != 1 {
		t.Fatalf("first frame = %d, want 1", first)
	}
	if _, err := v.BeginFrame(); !errors.Is(err, taskpkg.ErrFrameActive) {
		t.Fatalf("nested BeginFrame error = %v, want ErrFrameActive", err)
	}
	if got := v.StagedCount(); got != 0 {
		t.Fatalf("StagedCount after begin = %d, want 0", got)
	}
	if err := v.AbortFrame(); err != nil {
		t.Fatalf("AbortFrame returned error: %v", err)
	}
	if got := v.StagedCount(); got != 0 {
		t.Fatalf("StagedCount after abort = %d, want 0", got)
	}
	second, err := v.BeginFrame()
	if err != nil {
		t.Fatalf("second BeginFrame returned error: %v", err)
	}
	if second != 2 {
		t.Fatalf("second frame = %d, want 2", second)
	}
}

func TestCommitFirstWriteAdoptsValue(t *testing.T) {
	v := newVolume(t)
	if _, err := v.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame returned error: %v", err)
	}
	if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{1, 2, 3}, Irradiance: taskpkg.Vec3{X: 4, Y: 5, Z: 6}, Samples: 2}); err != nil {
		t.Fatalf("Stage returned error: %v", err)
	}
	if got := v.StagedCount(); got != 1 {
		t.Fatalf("StagedCount = %d, want 1", got)
	}
	stats, err := v.CommitFrame()
	if err != nil {
		t.Fatalf("CommitFrame returned error: %v", err)
	}
	if stats != (taskpkg.FrameStats{Frame: 1, Staged: 1, Applied: 1, Skipped: 0}) {
		t.Fatalf("CommitFrame stats = %+v, want frame 1 staged 1 applied 1 skipped 0", stats)
	}
	if got := readIrradiance(t, v, [3]int{1, 2, 3}); got != (taskpkg.Vec3{X: 4, Y: 5, Z: 6}) {
		t.Fatalf("stored irradiance = %+v, want (4,5,6)", got)
	}
	samples, err := v.Samples([3]int{1, 2, 3})
	if err != nil {
		t.Fatalf("Samples returned error: %v", err)
	}
	if samples != 2 {
		t.Fatalf("Samples = %d, want 2", samples)
	}
	if got := v.StagedCount(); got != 0 {
		t.Fatalf("StagedCount after commit = %d, want 0", got)
	}
}

func TestStageAndCommitWithoutFrame(t *testing.T) {
	v := newVolume(t)
	if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{}, Samples: 1}); !errors.Is(err, taskpkg.ErrNoFrame) {
		t.Fatalf("Stage without frame error = %v, want ErrNoFrame", err)
	}
	if _, err := v.CommitFrame(); !errors.Is(err, taskpkg.ErrNoFrame) {
		t.Fatalf("CommitFrame without frame error = %v, want ErrNoFrame", err)
	}
	if err := v.AbortFrame(); !errors.Is(err, taskpkg.ErrNoFrame) {
		t.Fatalf("AbortFrame without frame error = %v, want ErrNoFrame", err)
	}
}

func TestStageValidation(t *testing.T) {
	v := newVolume(t)
	if _, err := v.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame returned error: %v", err)
	}
	if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{0, 0, 4}, Samples: 1}); !errors.Is(err, taskpkg.ErrIndexRange) {
		t.Fatalf("Stage out-of-range coord error = %v, want ErrIndexRange", err)
	}
	if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{0, 0, 0}, Samples: -1}); !errors.Is(err, taskpkg.ErrBadUpdate) {
		t.Fatalf("Stage negative samples error = %v, want ErrBadUpdate", err)
	}
	if got := v.StagedCount(); got != 0 {
		t.Fatalf("StagedCount after rejected stages = %d, want 0", got)
	}
	stats, err := v.CommitFrame()
	if err != nil {
		t.Fatalf("CommitFrame returned error: %v", err)
	}
	if stats != (taskpkg.FrameStats{Frame: 1, Staged: 0, Applied: 0, Skipped: 0}) {
		t.Fatalf("CommitFrame stats = %+v, want empty frame", stats)
	}
}

func TestDuplicateStageRejected(t *testing.T) {
	v := newVolume(t)
	if _, err := v.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame returned error: %v", err)
	}
	if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{0, 0, 0}, Irradiance: taskpkg.Vec3{X: 1, Y: 1, Z: 1}, Samples: 1}); err != nil {
		t.Fatalf("first Stage returned error: %v", err)
	}
	if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{0, 0, 0}, Irradiance: taskpkg.Vec3{X: 2, Y: 2, Z: 2}, Samples: 1}); !errors.Is(err, taskpkg.ErrDuplicate) {
		t.Fatalf("duplicate Stage error = %v, want ErrDuplicate", err)
	}
	if got := v.StagedCount(); got != 1 {
		t.Fatalf("StagedCount = %d, want 1", got)
	}
	if _, err := v.CommitFrame(); err != nil {
		t.Fatalf("CommitFrame returned error: %v", err)
	}
	if got := readIrradiance(t, v, [3]int{0, 0, 0}); got != (taskpkg.Vec3{X: 1, Y: 1, Z: 1}) {
		t.Fatalf("stored irradiance = %+v, want the first staged value (1,1,1)", got)
	}
}

func TestHysteresisBlend(t *testing.T) {
	v := newVolume(t)
	if err := v.SetBlendAlpha(0.25); err != nil {
		t.Fatalf("SetBlendAlpha returned error: %v", err)
	}
	commit := func(u taskpkg.VoxelUpdate) {
		t.Helper()
		if _, err := v.BeginFrame(); err != nil {
			t.Fatalf("BeginFrame returned error: %v", err)
		}
		if err := v.Stage(u); err != nil {
			t.Fatalf("Stage returned error: %v", err)
		}
		if _, err := v.CommitFrame(); err != nil {
			t.Fatalf("CommitFrame returned error: %v", err)
		}
	}
	coord := [3]int{2, 2, 2}
	commit(taskpkg.VoxelUpdate{Coord: coord, Irradiance: taskpkg.Vec3{X: 10}, Samples: 1})
	if got := readIrradiance(t, v, coord); math.Abs(got.X-10) > 1e-12 {
		t.Fatalf("first bake = %+v, want X=10", got)
	}
	commit(taskpkg.VoxelUpdate{Coord: coord, Irradiance: taskpkg.Vec3{X: 2}, Samples: 1})
	if got := readIrradiance(t, v, coord); math.Abs(got.X-8) > 1e-12 {
		t.Fatalf("second bake = %+v, want X=8 (0.25*2 + 0.75*10)", got)
	}
	commit(taskpkg.VoxelUpdate{Coord: coord, Irradiance: taskpkg.Vec3{X: 6}, Samples: 2})
	if got := readIrradiance(t, v, coord); math.Abs(got.X-7.5) > 1e-12 {
		t.Fatalf("third bake = %+v, want X=7.5 (0.25*6 + 0.75*8)", got)
	}
	samples, err := v.Samples(coord)
	if err != nil {
		t.Fatalf("Samples returned error: %v", err)
	}
	if samples != 4 {
		t.Fatalf("Samples = %d, want 4", samples)
	}
}

func TestZeroSamplesAreSkipped(t *testing.T) {
	v := newVolume(t)
	if _, err := v.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame returned error: %v", err)
	}
	if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{3, 3, 3}, Irradiance: taskpkg.Vec3{X: 9, Y: 9, Z: 9}, Samples: 0}); err != nil {
		t.Fatalf("Stage returned error: %v", err)
	}
	stats, err := v.CommitFrame()
	if err != nil {
		t.Fatalf("CommitFrame returned error: %v", err)
	}
	if stats.Applied != 0 || stats.Skipped != 1 {
		t.Fatalf("CommitFrame stats = %+v, want 0 applied and 1 skipped", stats)
	}
	if got := readIrradiance(t, v, [3]int{3, 3, 3}); got != (taskpkg.Vec3{}) {
		t.Fatalf("skipped voxel = %+v, want zero", got)
	}
	samples, err := v.Samples([3]int{3, 3, 3})
	if err != nil {
		t.Fatalf("Samples returned error: %v", err)
	}
	if samples != 0 {
		t.Fatalf("skipped voxel Samples = %d, want 0", samples)
	}
}

func TestAbortDiscardsStagedUpdates(t *testing.T) {
	v := newVolume(t)
	if _, err := v.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame returned error: %v", err)
	}
	if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{1, 1, 1}, Irradiance: taskpkg.Vec3{X: 5}, Samples: 1}); err != nil {
		t.Fatalf("Stage returned error: %v", err)
	}
	if err := v.AbortFrame(); err != nil {
		t.Fatalf("AbortFrame returned error: %v", err)
	}
	if got := readIrradiance(t, v, [3]int{1, 1, 1}); got != (taskpkg.Vec3{}) {
		t.Fatalf("voxel after abort = %+v, want zero", got)
	}
	if _, err := v.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame after abort returned error: %v", err)
	}
	if got := v.StagedCount(); got != 0 {
		t.Fatalf("StagedCount in new frame = %d, want 0", got)
	}
}

func TestBlendAlphaValidation(t *testing.T) {
	v := newVolume(t)
	if got := v.BlendAlpha(); math.Abs(got-0.2) > 1e-12 {
		t.Fatalf("default BlendAlpha = %v, want 0.2", got)
	}
	for _, alpha := range []float64{0, -0.5, 1.5, math.NaN()} {
		if err := v.SetBlendAlpha(alpha); !errors.Is(err, taskpkg.ErrBadAlpha) {
			t.Fatalf("SetBlendAlpha(%v) error = %v, want ErrBadAlpha", alpha, err)
		}
	}
	if err := v.SetBlendAlpha(1); err != nil {
		t.Fatalf("SetBlendAlpha(1) returned error: %v", err)
	}
	if got := v.BlendAlpha(); got != 1 {
		t.Fatalf("BlendAlpha = %v, want 1", got)
	}
}

func TestWrapIndex(t *testing.T) {
	v := newVolume(t)
	cases := []struct {
		i, n int
		want int
	}{
		{-1, 4, 3},
		{9, 4, 1},
		{3, 0, 0},
		{-5, 3, 1},
		{8, 4, 0},
	}
	for _, tc := range cases {
		if got := v.WrapIndex(tc.i, tc.n); got != tc.want {
			t.Fatalf("WrapIndex(%d,%d) = %d, want %d", tc.i, tc.n, got, tc.want)
		}
	}
}

func TestSnapshotRestoreAndFrameCounter(t *testing.T) {
	v := newVolume(t)
	if err := v.SetBlendAlpha(0.4); err != nil {
		t.Fatalf("SetBlendAlpha returned error: %v", err)
	}
	for pass := 0; pass < 2; pass++ {
		if _, err := v.BeginFrame(); err != nil {
			t.Fatalf("BeginFrame returned error: %v", err)
		}
		if err := v.Stage(taskpkg.VoxelUpdate{Coord: [3]int{0, 1, 2}, Irradiance: taskpkg.Vec3{X: float64(pass + 1), Y: 2, Z: 3}, Samples: 1}); err != nil {
			t.Fatalf("Stage returned error: %v", err)
		}
		if _, err := v.CommitFrame(); err != nil {
			t.Fatalf("CommitFrame returned error: %v", err)
		}
	}
	before := readIrradiance(t, v, [3]int{0, 1, 2})
	snap, err := v.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	restored := newVolume(t)
	if err := restored.Restore(snap); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if got := readIrradiance(t, restored, [3]int{0, 1, 2}); got != before {
		t.Fatalf("restored irradiance = %+v, want %+v", got, before)
	}
	if got := restored.BlendAlpha(); math.Abs(got-0.4) > 1e-12 {
		t.Fatalf("restored BlendAlpha = %v, want 0.4", got)
	}
	frame, err := restored.BeginFrame()
	if err != nil {
		t.Fatalf("BeginFrame after restore returned error: %v", err)
	}
	if frame != 3 {
		t.Fatalf("frame after restore = %d, want 3", frame)
	}
	if err := restored.Restore([]byte("voxels: nope")); !errors.Is(err, taskpkg.ErrSnapshot) {
		t.Fatalf("Restore(garbage) error = %v, want ErrSnapshot", err)
	}
	small := newVolume2x2(t)
	if err := small.Restore(snap); !errors.Is(err, taskpkg.ErrSnapshot) {
		t.Fatalf("Restore into different grid error = %v, want ErrSnapshot", err)
	}
}

func newVolume2x2(t *testing.T) *taskpkg.IrradianceVolume {
	t.Helper()
	v, err := taskpkg.NewIrradianceVolume(taskpkg.VoxelGrid{X: 2, Y: 2, Z: 2}, 1)
	if err != nil {
		t.Fatalf("NewIrradianceVolume(2,2,2) returned error: %v", err)
	}
	return v
}

func TestConcurrentStaging(t *testing.T) {
	v := newVolume(t)
	if _, err := v.BeginFrame(); err != nil {
		t.Fatalf("BeginFrame returned error: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 64)
	for i := 0; i < 64; i++ {
		coord, err := v.VoxelCoord(i)
		if err != nil {
			t.Fatalf("VoxelCoord(%d) returned error: %v", i, err)
		}
		wg.Add(1)
		go func(i int, coord [3]int) {
			defer wg.Done()
			errs[i] = v.Stage(taskpkg.VoxelUpdate{Coord: coord, Irradiance: taskpkg.Vec3{X: 1}, Samples: 1})
		}(i, coord)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Stage[%d] returned error: %v", i, err)
		}
	}
	stats, err := v.CommitFrame()
	if err != nil {
		t.Fatalf("CommitFrame returned error: %v", err)
	}
	if stats.Applied != 64 {
		t.Fatalf("applied %d updates, want 64", stats.Applied)
	}
	for i := 0; i < 64; i++ {
		coord, err := v.VoxelCoord(i)
		if err != nil {
			t.Fatalf("VoxelCoord(%d) returned error: %v", i, err)
		}
		samples, err := v.Samples(coord)
		if err != nil {
			t.Fatalf("Samples(%v) returned error: %v", coord, err)
		}
		if samples != 1 {
			t.Fatalf("voxel %v samples = %d, want 1", coord, samples)
		}
	}
}
