package task

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-c-04"
)

func schedConfig() taskpkg.Config {
	return taskpkg.Config{
		TickRate:         10,
		MaxCatchUpTicks:  8,
		BudgetMicros:     1_000_000,
		StarveAfterTicks: 3,
		MaxTasks:         32,
	}
}

func newScheduler(t *testing.T) *taskpkg.TickScheduler {
	t.Helper()
	return taskpkg.NewTickScheduler(schedConfig())
}

func mustRegister(t *testing.T, s *taskpkg.TickScheduler, id string, priority int, interval uint32) {
	t.Helper()
	if err := s.Register(id, priority, interval); err != nil {
		t.Fatalf("Register(%q): %v", id, err)
	}
}

// Advance must turn elapsed time into the right number of ticks.
func TestAdvanceExactTicks(t *testing.T) {
	s := newScheduler(t)
	plan, err := s.Advance(300_000)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if !reflect.DeepEqual(plan.Ticks, []uint32{0, 1, 2}) {
		t.Fatalf("plan ticks = %v, want [0 1 2]", plan.Ticks)
	}
	if plan.DroppedTicks != 0 {
		t.Fatalf("dropped ticks = %d, want 0", plan.DroppedTicks)
	}
	if plan.RemainderMicros != 0 {
		t.Fatalf("remainder = %d, want 0", plan.RemainderMicros)
	}
	if got := s.TickCount(); got != 3 {
		t.Fatalf("TickCount = %d, want 3", got)
	}
}

// Fractional time must be carried into the next Advance call.
func TestRemainderCarried(t *testing.T) {
	s := newScheduler(t)
	plan, err := s.Advance(150_000)
	if err != nil {
		t.Fatalf("first Advance: %v", err)
	}
	if !reflect.DeepEqual(plan.Ticks, []uint32{0}) {
		t.Fatalf("first plan ticks = %v, want [0]", plan.Ticks)
	}
	if plan.RemainderMicros != 50_000 {
		t.Fatalf("first remainder = %d, want 50000", plan.RemainderMicros)
	}
	plan, err = s.Advance(60_000)
	if err != nil {
		t.Fatalf("second Advance: %v", err)
	}
	if !reflect.DeepEqual(plan.Ticks, []uint32{1}) {
		t.Fatalf("second plan ticks = %v, want [1]", plan.Ticks)
	}
	if plan.RemainderMicros != 10_000 {
		t.Fatalf("second remainder = %d, want 10000", plan.RemainderMicros)
	}
	if got := s.TickCount(); got != 2 {
		t.Fatalf("TickCount = %d, want 2", got)
	}
}

// A long stall must be capped so the loop does not spiral.
func TestCatchUpCapped(t *testing.T) {
	cfg := schedConfig()
	cfg.MaxCatchUpTicks = 3
	s := taskpkg.NewTickScheduler(cfg)
	plan, err := s.Advance(1_000_000)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if !reflect.DeepEqual(plan.Ticks, []uint32{7, 8, 9}) {
		t.Fatalf("capped ticks = %v, want [7 8 9]", plan.Ticks)
	}
	if plan.DroppedTicks != 7 {
		t.Fatalf("dropped ticks = %d, want 7", plan.DroppedTicks)
	}
	if got := s.TickCount(); got != 10 {
		t.Fatalf("TickCount = %d, want 10", got)
	}
}

// Negative elapsed time must be rejected.
func TestNegativeElapsedRejected(t *testing.T) {
	s := newScheduler(t)
	if _, err := s.Advance(-1); !errors.Is(err, taskpkg.ErrClockRewound) {
		t.Fatalf("negative Advance error = %v, want ErrClockRewound", err)
	}
	if got := s.TickCount(); got != 0 {
		t.Fatalf("TickCount changed to %d", got)
	}
}

// Duplicate task ids must be rejected.
func TestRegisterDuplicate(t *testing.T) {
	s := newScheduler(t)
	mustRegister(t, s, "physics", 10, 1)
	if err := s.Register("physics", 20, 2); !errors.Is(err, taskpkg.ErrTaskExists) {
		t.Fatalf("duplicate Register error = %v, want ErrTaskExists", err)
	}
}

// A zero interval is not a valid schedule.
func TestRegisterInvalidInterval(t *testing.T) {
	s := newScheduler(t)
	if err := s.Register("physics", 10, 0); !errors.Is(err, taskpkg.ErrInvalidInterval) {
		t.Fatalf("zero interval error = %v, want ErrInvalidInterval", err)
	}
}

// The task table must respect Config.MaxTasks.
func TestTaskLimit(t *testing.T) {
	cfg := schedConfig()
	cfg.MaxTasks = 2
	s := taskpkg.NewTickScheduler(cfg)
	mustRegister(t, s, "a", 1, 1)
	mustRegister(t, s, "b", 1, 1)
	if err := s.Register("c", 1, 1); !errors.Is(err, taskpkg.ErrTaskLimit) {
		t.Fatalf("third task error = %v, want ErrTaskLimit", err)
	}
}

// Interval arithmetic must decide which tasks are due.
func TestDueTasksInterval(t *testing.T) {
	s := newScheduler(t)
	mustRegister(t, s, "physics", 10, 3)
	if got := s.DueTasks(0); !reflect.DeepEqual(got, []string{"physics"}) {
		t.Fatalf("DueTasks(0) = %v, want [physics]", got)
	}
	if got := s.DueTasks(1); len(got) != 0 {
		t.Fatalf("DueTasks(1) = %v, want empty", got)
	}
	if got := s.DueTasks(3); !reflect.DeepEqual(got, []string{"physics"}) {
		t.Fatalf("DueTasks(3) = %v, want [physics]", got)
	}
	if got := s.DueTasks(6); !reflect.DeepEqual(got, []string{"physics"}) {
		t.Fatalf("DueTasks(6) = %v, want [physics]", got)
	}
}

// Tasks must run by priority and then registration order.
func TestRunTickOrder(t *testing.T) {
	s := newScheduler(t)
	mustRegister(t, s, "net", 5, 1)
	mustRegister(t, s, "ai", 9, 1)
	mustRegister(t, s, "audio", 5, 1)
	res, err := s.RunTick(0, func(id string) int64 { return 1 })
	if err != nil {
		t.Fatalf("RunTick: %v", err)
	}
	if !reflect.DeepEqual(res.Ran, []string{"ai", "net", "audio"}) {
		t.Fatalf("run order = %v, want [ai net audio]", res.Ran)
	}
	if res.OverBudget {
		t.Fatalf("tick incorrectly marked over budget")
	}
}

// The per-tick budget must skip the tail of the task list.
func TestBudgetSkipsRemaining(t *testing.T) {
	cfg := schedConfig()
	cfg.BudgetMicros = 100
	s := taskpkg.NewTickScheduler(cfg)
	for _, id := range []string{"a", "b", "c"} {
		mustRegister(t, s, id, 5, 1)
	}
	res, err := s.RunTick(0, func(id string) int64 { return 60 })
	if err != nil {
		t.Fatalf("RunTick: %v", err)
	}
	if !reflect.DeepEqual(res.Ran, []string{"a"}) {
		t.Fatalf("ran = %v, want [a]", res.Ran)
	}
	if !reflect.DeepEqual(res.Skipped, []string{"b", "c"}) {
		t.Fatalf("skipped = %v, want [b c]", res.Skipped)
	}
	if !res.OverBudget {
		t.Fatalf("tick not marked over budget")
	}
	if res.CostMicros != 60 {
		t.Fatalf("cost = %d, want 60", res.CostMicros)
	}
}

// Repeated skips must mark a task as starved and clear after a successful run.
func TestStarvedAfterRepeatedSkips(t *testing.T) {
	cfg := schedConfig()
	cfg.BudgetMicros = 10
	cfg.StarveAfterTicks = 2
	s := taskpkg.NewTickScheduler(cfg)
	mustRegister(t, s, "cheap", 9, 1)
	mustRegister(t, s, "expensive", 1, 1)
	cost := func(id string) int64 {
		if id == "expensive" {
			return 100
		}
		return 1
	}
	if _, err := s.RunTick(0, cost); err != nil {
		t.Fatalf("RunTick(0): %v", err)
	}
	if _, err := s.RunTick(1, cost); err != nil {
		t.Fatalf("RunTick(1): %v", err)
	}
	if got := s.Starved(); !reflect.DeepEqual(got, []string{"expensive"}) {
		t.Fatalf("Starved = %v, want [expensive]", got)
	}
	if _, err := s.RunTick(2, func(id string) int64 { return 1 }); err != nil {
		t.Fatalf("RunTick(2): %v", err)
	}
	if got := s.Starved(); len(got) != 0 {
		t.Fatalf("Starved after recovery = %v, want empty", got)
	}
}

// Pause must stop tick production and resume must not back-fill the gap.
func TestPauseStopsTicks(t *testing.T) {
	s := newScheduler(t)
	if err := s.Pause(); err != nil {
		t.Fatalf("Pause: %v", err)
	}
	plan, err := s.Advance(1_000_000)
	if err != nil {
		t.Fatalf("paused Advance: %v", err)
	}
	if len(plan.Ticks) != 0 {
		t.Fatalf("paused Advance produced ticks %v", plan.Ticks)
	}
	if got := s.TickCount(); got != 0 {
		t.Fatalf("TickCount while paused = %d, want 0", got)
	}
	if err := s.Resume(); err != nil {
		t.Fatalf("Resume: %v", err)
	}
	plan, err = s.Advance(100_000)
	if err != nil {
		t.Fatalf("resumed Advance: %v", err)
	}
	if !reflect.DeepEqual(plan.Ticks, []uint32{0}) {
		t.Fatalf("after resume ticks = %v, want [0]", plan.Ticks)
	}
	if got := s.TickCount(); got != 1 {
		t.Fatalf("TickCount after resume = %d, want 1", got)
	}
}

// Unregistered tasks must disappear from the schedule.
func TestUnregisterStopsDue(t *testing.T) {
	s := newScheduler(t)
	mustRegister(t, s, "net", 5, 1)
	if err := s.Unregister("net"); err != nil {
		t.Fatalf("Unregister: %v", err)
	}
	if got := s.DueTasks(0); len(got) != 0 {
		t.Fatalf("DueTasks after Unregister = %v, want empty", got)
	}
	if err := s.Unregister("net"); !errors.Is(err, taskpkg.ErrUnknownTask) {
		t.Fatalf("second Unregister error = %v, want ErrUnknownTask", err)
	}
}

// A restarted process must resume the same tick and schedule.
func TestSnapshotRestoresSchedule(t *testing.T) {
	s := newScheduler(t)
	mustRegister(t, s, "physics", 10, 1)
	mustRegister(t, s, "net", 5, 2)
	plan, err := s.Advance(300_000)
	if err != nil {
		t.Fatalf("Advance: %v", err)
	}
	if len(plan.Ticks) != 3 {
		t.Fatalf("expected 3 ticks before snapshot, got %v", plan.Ticks)
	}
	if _, err := s.RunTick(0, func(id string) int64 { return 1 }); err != nil {
		t.Fatalf("RunTick: %v", err)
	}
	snap, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewTickSchedulerFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewTickSchedulerFromSnapshot: %v", err)
	}
	if got := restored.TickCount(); got != 3 {
		t.Fatalf("restored TickCount = %d, want 3", got)
	}
	if got, want := restored.DueTasks(3), s.DueTasks(3); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored DueTasks(3) = %v, want %v", got, want)
	}
	origPlan, err := s.Advance(100_000)
	if err != nil {
		t.Fatalf("original Advance: %v", err)
	}
	restoredPlan, err := restored.Advance(100_000)
	if err != nil {
		t.Fatalf("restored Advance: %v", err)
	}
	if !reflect.DeepEqual(origPlan, restoredPlan) {
		t.Fatalf("plans diverged after restart: %+v != %+v", origPlan, restoredPlan)
	}
	if got := restored.TickCount(); got != 4 {
		t.Fatalf("restored TickCount after advance = %d, want 4", got)
	}
}

// Concurrent registration and advancing must not corrupt the schedule.
func TestConcurrentRegisterAndAdvance(t *testing.T) {
	cfg := schedConfig()
	cfg.MaxTasks = 64
	s := taskpkg.NewTickScheduler(cfg)
	var wg sync.WaitGroup
	errs := make(chan error, 20)
	for i := 0; i < 10; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 2; j++ {
				id := fmt.Sprintf("task-%d-%d", i, j)
				if err := s.Register(id, i, 1); err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		if _, err := s.Advance(1_000_000); err != nil {
			errs <- err
		}
	}()
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent operation: %v", err)
	}
	if got := s.TickCount(); got != 10 {
		t.Fatalf("TickCount = %d, want 10", got)
	}
	if due := s.DueTasks(10); len(due) != 20 {
		t.Fatalf("due tasks after concurrent registration = %d, want 20", len(due))
	}
}
