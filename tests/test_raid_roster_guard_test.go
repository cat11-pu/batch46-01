package task

import (
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-b-10"
)

var raidNow = time.Date(2026, 7, 4, 21, 0, 0, 0, time.UTC)

func raidCfg() taskpkg.Config {
	return taskpkg.Config{
		Size:       8,
		MaxTanks:   2,
		MaxHealers: 2,
		MaxDPS:     4,
		BenchSize:  2,
		SwapWindow: 2 * time.Minute,
	}
}

func newRoster(t *testing.T) *taskpkg.RaidRoster {
	t.Helper()
	r := taskpkg.NewRaidRoster(raidCfg(), "raid-1")
	if r == nil {
		t.Fatalf("NewRaidRoster returned nil")
	}
	return r
}

func raidAdd(t *testing.T, r *taskpkg.RaidRoster, name string, role taskpkg.Role, gear int) {
	t.Helper()
	if err := r.Add(taskpkg.Member{Name: name, Role: role, GearScore: gear}, raidNow); err != nil {
		t.Fatalf("Add(%s/%s): %v", name, role, err)
	}
}

func findSlot(slots []taskpkg.Slot, name string) (taskpkg.Slot, bool) {
	for _, s := range slots {
		if s.Name == name {
			return s, true
		}
	}
	return taskpkg.Slot{}, false
}

func TestAddRequiresUnlockedRoster(t *testing.T) {
	r := newRoster(t)
	if err := r.Lock(raidNow); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if !r.Locked() {
		t.Fatalf("Locked() = false right after Lock")
	}
	err := r.Add(taskpkg.Member{Name: "bob", Role: taskpkg.RoleTank, GearScore: 620}, raidNow.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrRosterLocked) {
		t.Fatalf("add on a locked roster must yield ErrRosterLocked, got %v", err)
	}
	if got := len(r.Roster()); got != 0 {
		t.Fatalf("rejected add changed the roster: %d slots", got)
	}
}

func TestRaidFullRejected(t *testing.T) {
	cfg := raidCfg()
	cfg.Size = 3
	cfg.MaxTanks = 1
	cfg.MaxDPS = 2
	cfg.BenchSize = 1
	r := taskpkg.NewRaidRoster(cfg, "raid-small")
	raidAdd(t, r, "t1", taskpkg.RoleTank, 620)
	raidAdd(t, r, "h1", taskpkg.RoleHealer, 610)
	raidAdd(t, r, "d1", taskpkg.RoleDPS, 600)
	err := r.Add(taskpkg.Member{Name: "h2", Role: taskpkg.RoleHealer, GearScore: 615}, raidNow)
	if !errors.Is(err, taskpkg.ErrRaidFull) {
		t.Fatalf("fourth member must yield ErrRaidFull, got %v", err)
	}
}

func TestRoleQuotaRejected(t *testing.T) {
	r := newRoster(t)
	raidAdd(t, r, "h1", taskpkg.RoleHealer, 610)
	raidAdd(t, r, "h2", taskpkg.RoleHealer, 615)
	err := r.Add(taskpkg.Member{Name: "h3", Role: taskpkg.RoleHealer, GearScore: 620}, raidNow)
	if !errors.Is(err, taskpkg.ErrRoleQuota) {
		t.Fatalf("third healer must yield ErrRoleQuota, got %v", err)
	}
	if got := r.Count(taskpkg.RoleHealer); got != 2 {
		t.Fatalf("healer count = %d, want 2", got)
	}
}

func TestDuplicateMemberRejected(t *testing.T) {
	r := newRoster(t)
	raidAdd(t, r, "bob", taskpkg.RoleTank, 620)
	err := r.Add(taskpkg.Member{Name: "bob", Role: taskpkg.RoleDPS, GearScore: 600}, raidNow)
	if !errors.Is(err, taskpkg.ErrDuplicateMember) {
		t.Fatalf("duplicate member must yield ErrDuplicateMember, got %v", err)
	}
}

func TestBenchFullRejected(t *testing.T) {
	cfg := raidCfg()
	cfg.BenchSize = 1
	r := taskpkg.NewRaidRoster(cfg, "raid-bench")
	raidAdd(t, r, "t1", taskpkg.RoleTank, 620)
	raidAdd(t, r, "h1", taskpkg.RoleHealer, 610)
	raidAdd(t, r, "d1", taskpkg.RoleDPS, 600)
	raidAdd(t, r, "d2", taskpkg.RoleDPS, 601)
	if err := r.Bench("d2", raidNow); err != nil {
		t.Fatalf("first bench: %v", err)
	}
	err := r.Bench("d1", raidNow)
	if !errors.Is(err, taskpkg.ErrIllegalBench) {
		t.Fatalf("benching into a full bench must yield ErrIllegalBench, got %v", err)
	}
	if got := len(r.BenchList()); got != 1 {
		t.Fatalf("bench size = %d, want 1", got)
	}
}

func TestBenchFreesRoleQuota(t *testing.T) {
	r := newRoster(t)
	raidAdd(t, r, "h1", taskpkg.RoleHealer, 610)
	raidAdd(t, r, "h2", taskpkg.RoleHealer, 615)
	err := r.Add(taskpkg.Member{Name: "h3", Role: taskpkg.RoleHealer, GearScore: 620}, raidNow)
	if !errors.Is(err, taskpkg.ErrRoleQuota) {
		t.Fatalf("third healer must be rejected, got %v", err)
	}
	if err := r.Bench("h1", raidNow); err != nil {
		t.Fatalf("Bench: %v", err)
	}
	if err := r.Add(taskpkg.Member{Name: "h3", Role: taskpkg.RoleHealer, GearScore: 620}, raidNow); err != nil {
		t.Fatalf("benching a healer must free the quota, got %v", err)
	}
	if got := r.Count(taskpkg.RoleHealer); got != 2 {
		t.Fatalf("healer count = %d, want 2", got)
	}
	if bench := r.BenchList(); len(bench) != 1 || bench[0] != "h1" {
		t.Fatalf("bench = %v, want [h1]", bench)
	}
}

func smallLockedRoster(t *testing.T) (*taskpkg.RaidRoster, time.Time) {
	t.Helper()
	cfg := raidCfg()
	cfg.Size = 4
	cfg.MaxTanks = 1
	cfg.MaxHealers = 1
	cfg.MaxDPS = 2
	cfg.BenchSize = 2
	r := taskpkg.NewRaidRoster(cfg, "raid-lock")
	raidAdd(t, r, "t1", taskpkg.RoleTank, 620)
	raidAdd(t, r, "h1", taskpkg.RoleHealer, 610)
	raidAdd(t, r, "d1", taskpkg.RoleDPS, 600)
	raidAdd(t, r, "d2", taskpkg.RoleDPS, 601)
	if err := r.Bench("d2", raidNow); err != nil {
		t.Fatalf("Bench d2: %v", err)
	}
	if err := r.Lock(raidNow); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	return r, raidNow
}

func TestSwapWithinWindow(t *testing.T) {
	r, lockedAt := smallLockedRoster(t)
	if err := r.Swap("d2", "d1", lockedAt.Add(time.Minute)); err != nil {
		t.Fatalf("Swap inside the window: %v", err)
	}
	slots := r.Roster()
	if s, ok := findSlot(slots, "d2"); !ok || s.Bench {
		t.Fatalf("d2 must be active after the swap")
	}
	if s, ok := findSlot(slots, "d1"); !ok || !s.Bench {
		t.Fatalf("d1 must be benched after the swap")
	}
	if bench := r.BenchList(); len(bench) != 1 || bench[0] != "d1" {
		t.Fatalf("bench = %v, want [d1]", bench)
	}
}

func TestSwapAfterWindowClosed(t *testing.T) {
	r, lockedAt := smallLockedRoster(t)
	err := r.Swap("d2", "d1", lockedAt.Add(3*time.Minute))
	if !errors.Is(err, taskpkg.ErrSwapWindowClosed) {
		t.Fatalf("swap after the window must yield ErrSwapWindowClosed, got %v", err)
	}
	if bench := r.BenchList(); len(bench) != 1 || bench[0] != "d2" {
		t.Fatalf("rejected swap changed the bench: %v", bench)
	}
}

func TestPullRequiresLock(t *testing.T) {
	r := newRoster(t)
	raidAdd(t, r, "t1", taskpkg.RoleTank, 620)
	raidAdd(t, r, "h1", taskpkg.RoleHealer, 610)
	raidAdd(t, r, "d1", taskpkg.RoleDPS, 600)
	if _, err := r.Pull("boss-1", raidNow); !errors.Is(err, taskpkg.ErrRosterUnlocked) {
		t.Fatalf("pull before lock must yield ErrRosterUnlocked, got %v", err)
	}
}

func TestPullRecordsRoster(t *testing.T) {
	r, _ := smallLockedRoster(t)
	rec, err := r.Pull("boss-1", raidNow.Add(time.Minute))
	if err != nil {
		t.Fatalf("Pull: %v", err)
	}
	want := []string{"d1", "h1", "t1"}
	if len(rec.Members) != len(want) {
		t.Fatalf("pull members = %v, want %v", rec.Members, want)
	}
	for i := range want {
		if rec.Members[i] != want[i] {
			t.Fatalf("pull members = %v, want %v", rec.Members, want)
		}
	}
	got, err := r.Attendance("boss-1")
	if err != nil {
		t.Fatalf("Attendance: %v", err)
	}
	if len(got) != 3 || got[0] != "d1" || got[1] != "h1" || got[2] != "t1" {
		t.Fatalf("attendance = %v, want [d1 h1 t1]", got)
	}
	if _, err := r.Attendance("boss-unknown"); !errors.Is(err, taskpkg.ErrUnknownBoss) {
		t.Fatalf("unknown boss must yield ErrUnknownBoss, got %v", err)
	}
}

func TestBossLockoutRepeatedPull(t *testing.T) {
	r, _ := smallLockedRoster(t)
	if _, err := r.Pull("boss-1", raidNow.Add(time.Minute)); err != nil {
		t.Fatalf("first pull: %v", err)
	}
	if _, err := r.Pull("boss-1", raidNow.Add(2*time.Minute)); !errors.Is(err, taskpkg.ErrLockoutActive) {
		t.Fatalf("repeated pull must yield ErrLockoutActive, got %v", err)
	}
	if err := r.UnlockBoss("boss-1"); err != nil {
		t.Fatalf("UnlockBoss: %v", err)
	}
	if _, err := r.Pull("boss-1", raidNow.Add(3*time.Minute)); err != nil {
		t.Fatalf("pull after UnlockBoss: %v", err)
	}
}

func TestSwapEnforcesRoleQuota(t *testing.T) {
	r := newRoster(t)
	raidAdd(t, r, "h1", taskpkg.RoleHealer, 610)
	raidAdd(t, r, "h2", taskpkg.RoleHealer, 615)
	raidAdd(t, r, "d1", taskpkg.RoleDPS, 600)
	if err := r.Bench("h1", raidNow); err != nil {
		t.Fatalf("Bench h1: %v", err)
	}
	raidAdd(t, r, "h3", taskpkg.RoleHealer, 620)
	err := r.Swap("h1", "d1", raidNow)
	if !errors.Is(err, taskpkg.ErrRoleQuota) {
		t.Fatalf("swap that would exceed the healer quota must yield ErrRoleQuota, got %v", err)
	}
	if got := r.Count(taskpkg.RoleHealer); got != 2 {
		t.Fatalf("healer count = %d, want 2", got)
	}
	slots := r.Roster()
	if s, ok := findSlot(slots, "h1"); !ok || !s.Bench {
		t.Fatalf("h1 must stay benched after the rejected swap")
	}
	if s, ok := findSlot(slots, "d1"); !ok || s.Bench {
		t.Fatalf("d1 must stay active after the rejected swap")
	}
}

func TestRosterSortedAndCopied(t *testing.T) {
	r := newRoster(t)
	raidAdd(t, r, "zoe", taskpkg.RoleDPS, 600)
	raidAdd(t, r, "amy", taskpkg.RoleHealer, 610)
	raidAdd(t, r, "bob", taskpkg.RoleTank, 620)
	slots := r.Roster()
	if len(slots) != 3 {
		t.Fatalf("roster size = %d, want 3", len(slots))
	}
	if slots[0].Name != "bob" || slots[0].Role != taskpkg.RoleTank {
		t.Fatalf("slot 0 = %+v, want bob the tank", slots[0])
	}
	if slots[1].Name != "amy" || slots[1].Role != taskpkg.RoleHealer {
		t.Fatalf("slot 1 = %+v, want amy the healer", slots[1])
	}
	if slots[2].Name != "zoe" || slots[2].Role != taskpkg.RoleDPS {
		t.Fatalf("slot 2 = %+v, want zoe the dps", slots[2])
	}
	slots[0].Name = "mallory"
	again := r.Roster()
	if again[0].Name != "bob" {
		t.Fatalf("caller mutation leaked into the roster: %+v", again[0])
	}
}

func TestSnapshotRestoresLockState(t *testing.T) {
	r, lockedAt := smallLockedRoster(t)
	if _, err := r.Pull("boss-9", lockedAt.Add(time.Minute)); err != nil {
		t.Fatalf("Pull boss-9: %v", err)
	}
	before, err := r.Attendance("boss-9")
	if err != nil {
		t.Fatalf("Attendance before snapshot: %v", err)
	}
	snap, err := r.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewRaidRosterFromSnapshot(func() taskpkg.Config {
		cfg := raidCfg()
		cfg.Size = 4
		cfg.MaxTanks = 1
		cfg.MaxHealers = 1
		cfg.MaxDPS = 2
		cfg.BenchSize = 2
		return cfg
	}(), snap)
	if err != nil {
		t.Fatalf("NewRaidRosterFromSnapshot: %v", err)
	}
	if !restored.Locked() {
		t.Fatalf("restored roster must still be locked")
	}
	if err := restored.Add(taskpkg.Member{Name: "late", Role: taskpkg.RoleDPS, GearScore: 600}, lockedAt.Add(30*time.Second)); !errors.Is(err, taskpkg.ErrRosterLocked) {
		t.Fatalf("restored lock must reject adds, got %v", err)
	}
	after, err := restored.Attendance("boss-9")
	if err != nil {
		t.Fatalf("Attendance after restore: %v", err)
	}
	if len(after) != len(before) || len(after) != 3 {
		t.Fatalf("attendance after restore = %v, want %v", after, before)
	}
	if _, err := restored.Pull("boss-9", lockedAt.Add(2*time.Minute)); !errors.Is(err, taskpkg.ErrLockoutActive) {
		t.Fatalf("restored lockout must reject a repeated pull, got %v", err)
	}
	if err := restored.Swap("d2", "d1", lockedAt.Add(3*time.Minute)); !errors.Is(err, taskpkg.ErrSwapWindowClosed) {
		t.Fatalf("restored swap deadline not enforced, got %v", err)
	}
	if err := restored.Swap("d2", "d1", lockedAt.Add(time.Minute)); err != nil {
		t.Fatalf("swap inside the restored window: %v", err)
	}
}

func TestConcurrentAddsRespectSize(t *testing.T) {
	cfg := raidCfg()
	cfg.Size = 5
	cfg.MaxTanks = 2
	cfg.MaxHealers = 2
	cfg.MaxDPS = 5
	cfg.BenchSize = 1
	r := taskpkg.NewRaidRoster(cfg, "raid-race")
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs[i] = r.Add(taskpkg.Member{
				Name:      "dps-" + strconv.Itoa(i),
				Role:      taskpkg.RoleDPS,
				GearScore: 600 + i,
			}, raidNow)
		}(i)
	}
	wg.Wait()
	successes, full := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, taskpkg.ErrRaidFull) {
			full++
		}
	}
	if successes != 5 || full != 5 {
		t.Fatalf("successes=%d full=%d, want 5 and 5", successes, full)
	}
	if got := r.Count(taskpkg.RoleDPS); got != 5 {
		t.Fatalf("active dps count = %d, want 5", got)
	}
	if got := len(r.Roster()); got != 5 {
		t.Fatalf("roster size = %d, want 5", got)
	}
}
