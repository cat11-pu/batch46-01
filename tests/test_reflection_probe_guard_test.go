package task

import (
	"errors"
	"math"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-d-04"
)

func newManager(t *testing.T, maxProbes, faceSize int) *taskpkg.ReflectionProbe {
	t.Helper()
	r, err := taskpkg.NewReflectionProbe(maxProbes, faceSize)
	if err != nil {
		t.Fatalf("NewReflectionProbe(%d,%d) returned error: %v", maxProbes, faceSize, err)
	}
	if r == nil {
		t.Fatal("NewReflectionProbe returned a nil manager")
	}
	return r
}

func unitBox() taskpkg.BoxBounds {
	return taskpkg.BoxBounds{Min: taskpkg.Vec3{X: -1, Y: -1, Z: -1}, Max: taskpkg.Vec3{X: 1, Y: 1, Z: 1}}
}

func addUnitProbe(t *testing.T, r *taskpkg.ReflectionProbe, id string, priority int) {
	t.Helper()
	if err := r.AddProbe(id, unitBox(), priority); err != nil {
		t.Fatalf("AddProbe(%q) returned error: %v", id, err)
	}
}

func TestConstructorValidation(t *testing.T) {
	if _, err := taskpkg.NewReflectionProbe(0, 8); !errors.Is(err, taskpkg.ErrBadConfig) {
		t.Fatalf("maxProbes 0 error = %v, want ErrBadConfig", err)
	}
	if _, err := taskpkg.NewReflectionProbe(65, 8); !errors.Is(err, taskpkg.ErrBadConfig) {
		t.Fatalf("maxProbes 65 error = %v, want ErrBadConfig", err)
	}
	if _, err := taskpkg.NewReflectionProbe(4, 6); !errors.Is(err, taskpkg.ErrBadConfig) {
		t.Fatalf("faceSize 6 error = %v, want ErrBadConfig", err)
	}
	if _, err := taskpkg.NewReflectionProbe(4, 4096); !errors.Is(err, taskpkg.ErrBadConfig) {
		t.Fatalf("faceSize 4096 error = %v, want ErrBadConfig", err)
	}
	if _, err := taskpkg.NewReflectionProbe(4, 4); err != nil {
		t.Fatalf("NewReflectionProbe(4,4) returned error: %v", err)
	}
}

func TestAddProbeValidation(t *testing.T) {
	r := newManager(t, 4, 8)
	if err := r.AddProbe("", unitBox(), 1); !errors.Is(err, taskpkg.ErrBadProbe) {
		t.Fatalf("empty id error = %v, want ErrBadProbe", err)
	}
	flat := taskpkg.BoxBounds{Min: taskpkg.Vec3{}, Max: taskpkg.Vec3{X: 0, Y: 1, Z: 1}}
	if err := r.AddProbe("flat", flat, 1); !errors.Is(err, taskpkg.ErrBadProbe) {
		t.Fatalf("degenerate bounds error = %v, want ErrBadProbe", err)
	}
	nan := taskpkg.BoxBounds{Min: taskpkg.Vec3{X: math.NaN()}, Max: taskpkg.Vec3{X: 1, Y: 1, Z: 1}}
	if err := r.AddProbe("nan", nan, 1); !errors.Is(err, taskpkg.ErrBadProbe) {
		t.Fatalf("NaN bounds error = %v, want ErrBadProbe", err)
	}
	addUnitProbe(t, r, "a", 1)
	if err := r.AddProbe("a", unitBox(), 2); !errors.Is(err, taskpkg.ErrDuplicateProbe) {
		t.Fatalf("duplicate id error = %v, want ErrDuplicateProbe", err)
	}
}

func TestAddProbeBudget(t *testing.T) {
	r := newManager(t, 2, 8)
	addUnitProbe(t, r, "a", 1)
	addUnitProbe(t, r, "b", 1)
	if err := r.AddProbe("c", unitBox(), 1); !errors.Is(err, taskpkg.ErrTooManyProbes) {
		t.Fatalf("third probe error = %v, want ErrTooManyProbes", err)
	}
	if got := len(r.ProbeIDs()); got != 2 {
		t.Fatalf("ProbeIDs length = %d, want 2", got)
	}
}

func TestProbeIDsOrder(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 5)
	addUnitProbe(t, r, "b", 9)
	addUnitProbe(t, r, "c", 5)
	ids := r.ProbeIDs()
	want := []string{"b", "a", "c"}
	if len(ids) != len(want) {
		t.Fatalf("ProbeIDs = %v, want %v", ids, want)
	}
	for i := range want {
		if ids[i] != want[i] {
			t.Fatalf("ProbeIDs = %v, want %v", ids, want)
		}
	}
}

func TestScheduleCaptureOrder(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 1)
	addUnitProbe(t, r, "b", 2)
	jobs, err := r.ScheduleCapture(100)
	if err != nil {
		t.Fatalf("ScheduleCapture returned error: %v", err)
	}
	if len(jobs) != taskpkg.MaxJobsPerFrame {
		t.Fatalf("ScheduleCapture returned %d jobs, want %d", len(jobs), taskpkg.MaxJobsPerFrame)
	}
	if jobs[0] != (taskpkg.CaptureJob{ProbeID: "b", Face: taskpkg.FacePosX, Mip: 0}) {
		t.Fatalf("first job = %+v, want b/PosX/mip0", jobs[0])
	}
	if jobs[4] != (taskpkg.CaptureJob{ProbeID: "b", Face: taskpkg.FaceNegX, Mip: 0}) {
		t.Fatalf("fifth job = %+v, want b/NegX/mip0", jobs[4])
	}
	if jobs[5] != (taskpkg.CaptureJob{ProbeID: "b", Face: taskpkg.FaceNegX, Mip: 1}) {
		t.Fatalf("sixth job = %+v, want b/NegX/mip1", jobs[5])
	}
	if jobs[6] != (taskpkg.CaptureJob{ProbeID: "b", Face: taskpkg.FaceNegX, Mip: 2}) {
		t.Fatalf("seventh job = %+v, want b/NegX/mip2", jobs[6])
	}
	if jobs[7] != (taskpkg.CaptureJob{ProbeID: "b", Face: taskpkg.FaceNegX, Mip: 3}) {
		t.Fatalf("eighth job = %+v, want b/NegX/mip3", jobs[7])
	}
	for i, job := range jobs {
		if job.ProbeID != "b" {
			t.Fatalf("job %d belongs to %q, want the higher priority probe b", i, job.ProbeID)
		}
	}
}

func TestScheduleBatchesAreIdempotent(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 1)
	addUnitProbe(t, r, "b", 1)
	seen := map[taskpkg.CaptureJob]bool{}
	total := 0
	for pass := 0; pass < 6; pass++ {
		jobs, err := r.ScheduleCapture(10)
		if err != nil {
			t.Fatalf("ScheduleCapture pass %d returned error: %v", pass, err)
		}
		if len(jobs) != taskpkg.MaxJobsPerFrame {
			t.Fatalf("pass %d returned %d jobs, want %d", pass, len(jobs), taskpkg.MaxJobsPerFrame)
		}
		for _, job := range jobs {
			if seen[job] {
				t.Fatalf("job %+v was scheduled twice", job)
			}
			seen[job] = true
		}
		total += len(jobs)
	}
	if total != 48 {
		t.Fatalf("scheduled %d jobs, want 48 (2 probes * 6 faces * 4 mips)", total)
	}
	if got := r.PendingCount(); got != 48 {
		t.Fatalf("PendingCount = %d, want 48", got)
	}
	more, err := r.ScheduleCapture(10)
	if err != nil {
		t.Fatalf("seventh ScheduleCapture returned error: %v", err)
	}
	if len(more) != 0 {
		t.Fatalf("seventh ScheduleCapture returned %d jobs, want 0", len(more))
	}
	if got := r.PendingCount(); got != 48 {
		t.Fatalf("PendingCount after seventh pass = %d, want 48", got)
	}
}

func TestCompleteFlow(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 1)
	var all []taskpkg.CaptureJob
	for {
		jobs, err := r.ScheduleCapture(1234)
		if err != nil {
			t.Fatalf("ScheduleCapture returned error: %v", err)
		}
		if len(jobs) == 0 {
			break
		}
		all = append(all, jobs...)
	}
	if len(all) != 24 {
		t.Fatalf("scheduled %d jobs, want 24", len(all))
	}
	for _, job := range all {
		if err := r.Complete(job); err != nil {
			t.Fatalf("Complete(%+v) returned error: %v", job, err)
		}
	}
	if got := r.PendingCount(); got != 0 {
		t.Fatalf("PendingCount = %d, want 0", got)
	}
	info, err := r.Probe("a")
	if err != nil {
		t.Fatalf("Probe returned error: %v", err)
	}
	if !info.Captured {
		t.Fatal("probe is not captured after all jobs completed")
	}
	if info.CapturedAt != 1234 {
		t.Fatalf("CapturedAt = %d, want 1234", info.CapturedAt)
	}
	if err := r.Complete(all[0]); !errors.Is(err, taskpkg.ErrUnknownJob) {
		t.Fatalf("second Complete error = %v, want ErrUnknownJob", err)
	}
}

func TestCompleteUnknownJob(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 1)
	if err := r.Complete(taskpkg.CaptureJob{ProbeID: "a", Face: taskpkg.FacePosX, Mip: 0}); !errors.Is(err, taskpkg.ErrUnknownJob) {
		t.Fatalf("Complete unsigned job error = %v, want ErrUnknownJob", err)
	}
	if err := r.Complete(taskpkg.CaptureJob{ProbeID: "ghost", Face: taskpkg.FaceNegZ, Mip: 2}); !errors.Is(err, taskpkg.ErrUnknownJob) {
		t.Fatalf("Complete unknown probe error = %v, want ErrUnknownJob", err)
	}
}

func TestFailRetriesThenGivesUp(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 2)
	jobs, err := r.ScheduleCapture(50)
	if err != nil {
		t.Fatalf("ScheduleCapture returned error: %v", err)
	}
	if len(jobs) == 0 {
		t.Fatal("ScheduleCapture returned no jobs")
	}
	target := jobs[0]
	if err := r.Fail(target); err != nil {
		t.Fatalf("first Fail returned error: %v", err)
	}
	if err := r.Fail(target); err != nil {
		t.Fatalf("second Fail returned error: %v", err)
	}
	if err := r.Fail(target); !errors.Is(err, taskpkg.ErrProbeFailed) {
		t.Fatalf("third Fail error = %v, want ErrProbeFailed", err)
	}
	if got := r.PendingCount(); got != len(jobs)-1 {
		t.Fatalf("PendingCount after giving up = %d, want %d", got, len(jobs)-1)
	}
	remaining, err := r.ScheduleCapture(51)
	if err != nil {
		t.Fatalf("ScheduleCapture returned error: %v", err)
	}
	for _, job := range remaining {
		if job == target {
			t.Fatalf("given-up job %+v was scheduled again", target)
		}
	}
}

func TestMipCountValues(t *testing.T) {
	r := newManager(t, 4, 8)
	cases := []struct {
		size int
		want int
	}{
		{1, 1},
		{2, 2},
		{4, 3},
		{256, 9},
		{0, 0},
		{-4, 0},
	}
	for _, tc := range cases {
		if got := r.MipCount(tc.size); got != tc.want {
			t.Fatalf("MipCount(%d) = %d, want %d", tc.size, got, tc.want)
		}
	}
}

func TestBoxProjectDirections(t *testing.T) {
	r := newManager(t, 4, 8)
	bounds := taskpkg.BoxBounds{Min: taskpkg.Vec3{}, Max: taskpkg.Vec3{X: 2, Y: 4, Z: 6}}
	center := taskpkg.Vec3{X: 1, Y: 2, Z: 3}
	right, err := r.BoxProject(center, taskpkg.Vec3{X: 1}, bounds)
	if err != nil {
		t.Fatalf("BoxProject(+X) returned error: %v", err)
	}
	if right != (taskpkg.Vec3{X: 2, Y: 2, Z: 3}) {
		t.Fatalf("BoxProject(+X) = %+v, want (2,2,3)", right)
	}
	up, err := r.BoxProject(center, taskpkg.Vec3{Y: 1}, bounds)
	if err != nil {
		t.Fatalf("BoxProject(+Y) returned error: %v", err)
	}
	if up != (taskpkg.Vec3{X: 1, Y: 4, Z: 3}) {
		t.Fatalf("BoxProject(+Y) = %+v, want (1,4,3)", up)
	}
	diagonal, err := r.BoxProject(center, taskpkg.Vec3{X: 1, Y: 1}, bounds)
	if err != nil {
		t.Fatalf("BoxProject(diagonal) returned error: %v", err)
	}
	if diagonal != (taskpkg.Vec3{X: 2, Y: 3, Z: 3}) {
		t.Fatalf("BoxProject(diagonal) = %+v, want (2,3,3)", diagonal)
	}
	if _, err := r.BoxProject(taskpkg.Vec3{X: -1}, taskpkg.Vec3{X: 1}, bounds); !errors.Is(err, taskpkg.ErrOutsideBox) {
		t.Fatalf("BoxProject outside box error = %v, want ErrOutsideBox", err)
	}
	if _, err := r.BoxProject(center, taskpkg.Vec3{}, bounds); !errors.Is(err, taskpkg.ErrBadDirection) {
		t.Fatalf("BoxProject zero direction error = %v, want ErrBadDirection", err)
	}
}

func TestBlendWeightsFalloff(t *testing.T) {
	r := newManager(t, 4, 8)
	if err := r.AddProbe("a", taskpkg.BoxBounds{Min: taskpkg.Vec3{X: -1, Y: -1, Z: -1}, Max: taskpkg.Vec3{X: 1, Y: 1, Z: 1}}, 1); err != nil {
		t.Fatalf("AddProbe(a) returned error: %v", err)
	}
	if err := r.AddProbe("b", taskpkg.BoxBounds{Min: taskpkg.Vec3{X: 0, Y: -1, Z: -1}, Max: taskpkg.Vec3{X: 2, Y: 1, Z: 1}}, 1); err != nil {
		t.Fatalf("AddProbe(b) returned error: %v", err)
	}
	weights, err := r.BlendWeights(taskpkg.Vec3{X: 0.4})
	if err != nil {
		t.Fatalf("BlendWeights returned error: %v", err)
	}
	if len(weights) != 2 {
		t.Fatalf("BlendWeights returned %v, want two probes", weights)
	}
	if math.Abs(weights["a"]-0.6) > 1e-9 || math.Abs(weights["b"]-0.4) > 1e-9 {
		t.Fatalf("BlendWeights = %v, want a=0.6 b=0.4", weights)
	}
	if _, err := r.BlendWeights(taskpkg.Vec3{X: 10}); !errors.Is(err, taskpkg.ErrOutsideProbes) {
		t.Fatalf("BlendWeights outside probes error = %v, want ErrOutsideProbes", err)
	}
	single := newManager(t, 4, 8)
	addUnitProbe(t, single, "solo", 3)
	only, err := single.BlendWeights(taskpkg.Vec3{})
	if err != nil {
		t.Fatalf("BlendWeights at probe centre returned error: %v", err)
	}
	if len(only) != 1 || math.Abs(only["solo"]-1) > 1e-12 {
		t.Fatalf("BlendWeights at probe centre = %v, want solo=1", only)
	}
}

func TestSnapshotRestorePendingJobs(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 2)
	addUnitProbe(t, r, "b", 1)
	jobs, err := r.ScheduleCapture(777)
	if err != nil {
		t.Fatalf("ScheduleCapture returned error: %v", err)
	}
	if len(jobs) != taskpkg.MaxJobsPerFrame {
		t.Fatalf("ScheduleCapture returned %d jobs, want %d", len(jobs), taskpkg.MaxJobsPerFrame)
	}
	if err := r.Complete(jobs[0]); err != nil {
		t.Fatalf("Complete returned error: %v", err)
	}
	snap, err := r.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot returned error: %v", err)
	}
	restored := newManager(t, 4, 8)
	if err := restored.Restore(snap); err != nil {
		t.Fatalf("Restore returned error: %v", err)
	}
	if got := restored.PendingCount(); got != len(jobs)-1 {
		t.Fatalf("restored PendingCount = %d, want %d", got, len(jobs)-1)
	}
	next, err := restored.ScheduleCapture(1)
	if err != nil {
		t.Fatalf("ScheduleCapture after restore returned error: %v", err)
	}
	if len(next) != taskpkg.MaxJobsPerFrame {
		t.Fatalf("ScheduleCapture after restore returned %d jobs, want %d", len(next), taskpkg.MaxJobsPerFrame)
	}
	if err := restored.Restore([]byte("captures: nope")); !errors.Is(err, taskpkg.ErrSnapshot) {
		t.Fatalf("Restore(garbage) error = %v, want ErrSnapshot", err)
	}
	other := newManager(t, 4, 16)
	if err := other.Restore(snap); !errors.Is(err, taskpkg.ErrSnapshot) {
		t.Fatalf("Restore into different config error = %v, want ErrSnapshot", err)
	}
}

func TestProbeInfoAndRemove(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 3)
	info, err := r.Probe("a")
	if err != nil {
		t.Fatalf("Probe(a) returned error: %v", err)
	}
	if info.ID != "a" || info.Priority != 3 || info.Captured {
		t.Fatalf("Probe(a) = %+v, want id a priority 3 uncaptured", info)
	}
	if _, err := r.Probe("ghost"); !errors.Is(err, taskpkg.ErrUnknownProbe) {
		t.Fatalf("Probe(ghost) error = %v, want ErrUnknownProbe", err)
	}
	if _, err := r.ScheduleCapture(1); err != nil {
		t.Fatalf("ScheduleCapture returned error: %v", err)
	}
	if got := r.PendingCount(); got != taskpkg.MaxJobsPerFrame {
		t.Fatalf("PendingCount = %d, want %d", got, taskpkg.MaxJobsPerFrame)
	}
	if err := r.RemoveProbe("a"); err != nil {
		t.Fatalf("RemoveProbe(a) returned error: %v", err)
	}
	if got := len(r.ProbeIDs()); got != 0 {
		t.Fatalf("ProbeIDs length after remove = %d, want 0", got)
	}
	if got := r.PendingCount(); got != 0 {
		t.Fatalf("PendingCount after remove = %d, want 0", got)
	}
	if err := r.RemoveProbe("a"); !errors.Is(err, taskpkg.ErrUnknownProbe) {
		t.Fatalf("second RemoveProbe error = %v, want ErrUnknownProbe", err)
	}
}

func TestConcurrentScheduling(t *testing.T) {
	r := newManager(t, 4, 8)
	addUnitProbe(t, r, "a", 1)
	addUnitProbe(t, r, "b", 1)
	var wg sync.WaitGroup
	results := make([][]taskpkg.CaptureJob, 8)
	errs := make([]error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = r.ScheduleCapture(9)
		}(i)
	}
	wg.Wait()
	seen := map[taskpkg.CaptureJob]bool{}
	for i := 0; i < 8; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent ScheduleCapture[%d] returned error: %v", i, errs[i])
		}
		for _, job := range results[i] {
			if seen[job] {
				t.Fatalf("job %+v was handed out by two schedulers", job)
			}
			seen[job] = true
		}
	}
	if len(seen) != 48 {
		t.Fatalf("scheduled %d distinct jobs, want 48", len(seen))
	}
	if got := r.PendingCount(); got != 48 {
		t.Fatalf("PendingCount = %d, want 48", got)
	}
}
