package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by TickScheduler.
var (
	ErrClockRewound    = errors.New("tick: elapsed time is negative")
	ErrTaskExists      = errors.New("tick: task already registered")
	ErrUnknownTask     = errors.New("tick: unknown task")
	ErrTaskLimit       = errors.New("tick: task limit reached")
	ErrInvalidTask     = errors.New("tick: invalid task")
	ErrInvalidInterval = errors.New("tick: interval must be positive")
	ErrAlreadyPaused   = errors.New("tick: scheduler already paused")
	ErrNotPaused       = errors.New("tick: scheduler is not paused")
)

// Config controls the fixed timestep loop of one server process.
type Config struct {
	// TickRate is the number of simulation ticks per second; zero means 20.
	TickRate int
	// MaxCatchUpTicks is the maximum number of ticks executed after one
	// stalled frame; zero means 8. Extra ticks are dropped.
	MaxCatchUpTicks int
	// BudgetMicros is the per-tick CPU budget; zero means unlimited.
	BudgetMicros int64
	// StarveAfterTicks is the skip streak that marks a task as starved; zero
	// means 3.
	StarveAfterTicks int
	// MaxTasks caps registered tasks; zero means 64.
	MaxTasks int
}

// Task is one registered periodic job.
type Task struct {
	ID         string
	Priority   int
	Interval   uint32
	NextTick   uint32
	SkipStreak int
}

// TickPlan is the outcome of advancing the wall clock.
type TickPlan struct {
	Ticks           []uint32
	DroppedTicks    int
	RemainderMicros int64
}

// TickResult is the outcome of executing one tick.
type TickResult struct {
	Tick       uint32
	Ran        []string
	Skipped    []string
	OverBudget bool
	CostMicros int64
}

// TickScheduler turns elapsed real time into fixed simulation ticks and runs
// due tasks in a deterministic order. All exported methods are safe for
// concurrent use.
type TickScheduler struct {
	cfg Config
}

// NewTickScheduler returns an empty scheduler configured by cfg.
func NewTickScheduler(cfg Config) *TickScheduler {
	return &TickScheduler{cfg: cfg}
}

// NewTickSchedulerFromSnapshot restores the tick counter, remainder, pause
// flag and every task's schedule from bytes produced by Snapshot.
func NewTickSchedulerFromSnapshot(data []byte) (*TickScheduler, error) {
	return nil, errNotImplemented
}

// Register adds a periodic task. Interval is measured in ticks and must be
// positive; a new task first becomes due at tick zero.
func (s *TickScheduler) Register(id string, priority int, interval uint32) error {
	return errNotImplemented
}

// Unregister removes a task.
func (s *TickScheduler) Unregister(id string) error {
	return errNotImplemented
}

// Advance converts elapsed microseconds into ticks. The fractional remainder
// is carried into the next call; at most MaxCatchUpTicks are returned and the
// rest are counted in DroppedTicks. While paused no ticks are produced and no
// time is accumulated.
func (s *TickScheduler) Advance(elapsedMicros int64) (TickPlan, error) {
	return TickPlan{}, errNotImplemented
}

// RunTick executes the tasks due at tick in priority-descending order, with
// equal priorities in registration order. exec returns the cost in
// microseconds; a task whose cost would exceed Config.BudgetMicros is skipped
// together with every later task, and the tick is marked OverBudget.
func (s *TickScheduler) RunTick(tick uint32, exec func(id string) int64) (TickResult, error) {
	return TickResult{}, errNotImplemented
}

// DueTasks returns the tasks due at tick in execution order.
func (s *TickScheduler) DueTasks(tick uint32) []string {
	return nil
}

// TickCount reports how many ticks have elapsed.
func (s *TickScheduler) TickCount() uint32 {
	return 0
}

// Pause stops tick production.
func (s *TickScheduler) Pause() error {
	return errNotImplemented
}

// Resume restarts tick production after Pause.
func (s *TickScheduler) Resume() error {
	return errNotImplemented
}

// Starved returns the ids whose skip streak reached StarveAfterTicks, sorted
// lexicographically.
func (s *TickScheduler) Starved() []string {
	return nil
}

// Snapshot serialises the scheduler state.
func (s *TickScheduler) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
