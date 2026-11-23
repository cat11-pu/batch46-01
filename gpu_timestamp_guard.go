package task

import "errors"

// Errors reported by GpuTimestamp. A caller must be able to tell an exhausted
// pool from an ordering mistake, because the renderer drops a pass on the
// former and fixes its submission order on the latter.
var (
	ErrPoolExhausted = errors.New("gpu timestamp: query pool exhausted")
	ErrUnknownQuery  = errors.New("gpu timestamp: query index not allocated")
	ErrQueryInFlight = errors.New("gpu timestamp: query already begun")
	ErrNoBegin       = errors.New("gpu timestamp: query was never begun")
	ErrBadRange      = errors.New("gpu timestamp: end stamp precedes begin stamp")
	ErrFrameMismatch = errors.New("gpu timestamp: query belongs to another frame")
	ErrNotReady      = errors.New("gpu timestamp: timestamp result not available")
	ErrReleaseBusy   = errors.New("gpu timestamp: query still in flight")
	ErrBadPoolSize   = errors.New("gpu timestamp: pool size must be positive")
	ErrBadTickPeriod = errors.New("gpu timestamp: tick period must be positive")

	errNotImplemented = errors.New("not implemented")
)

// QueryState is the lifecycle state of one timestamp query slot.
type QueryState int

const (
	// QueryIdle means the slot is allocated but no timestamp was written.
	QueryIdle QueryState = iota
	// QueryBegun means a begin timestamp was recorded and no end yet.
	QueryBegun
	// QueryResolved means both stamps are present and the duration is valid.
	QueryResolved
	// QueryUnavailable means the driver refused to return a result this frame.
	QueryUnavailable
)

// QueryInfo is the observable view of one query slot.
type QueryInfo struct {
	Index      int
	Label      string
	State      QueryState
	Frame      uint64
	DurationMs float64
}

// GpuTimestamp owns one fixed pool of timestamp queries shared by all passes
// of a frame. Slots are recycled by index, labels are free-form pass names.
type GpuTimestamp struct {
	poolSize     int
	tickPeriodNs float64
}

// NewGpuTimestamp creates a pool of poolSize query slots where one GPU tick
// lasts tickPeriodNs nanoseconds. Both arguments must be positive.
func NewGpuTimestamp(poolSize int, tickPeriodNs float64) (*GpuTimestamp, error) {
	return &GpuTimestamp{poolSize: poolSize, tickPeriodNs: tickPeriodNs}, nil
}

// NewGpuTimestampFromSnapshot restores the pass history and budgets saved by
// Snapshot so that a device-loss restart keeps its per-frame accounting.
func NewGpuTimestampFromSnapshot(poolSize int, tickPeriodNs float64, data []byte) (*GpuTimestamp, error) {
	return nil, errNotImplemented
}

// AllocQuery reserves the lowest free slot for label and returns its index.
// Labels do not have to be unique; totals aggregate by label.
func (g *GpuTimestamp) AllocQuery(label string) (int, error) {
	return 0, errNotImplemented
}

// ActiveCount reports how many slots are currently allocated.
func (g *GpuTimestamp) ActiveCount() int {
	return 0
}

// BeginQuery records the begin stamp of index for frame.
func (g *GpuTimestamp) BeginQuery(index int, frame, ts uint64) error {
	return errNotImplemented
}

// EndQuery records the end stamp of index for frame and resolves the result.
func (g *GpuTimestamp) EndQuery(index int, frame, ts uint64) error {
	return errNotImplemented
}

// DurationMs returns the resolved duration of index in milliseconds.
func (g *GpuTimestamp) DurationMs(index int) (float64, error) {
	return 0, errNotImplemented
}

// Ready reports whether index has a usable resolved result.
func (g *GpuTimestamp) Ready(index int) bool {
	return false
}

// MarkUnavailable flags index as having no usable result for this frame; its
// duration must be excluded from every aggregate.
func (g *GpuTimestamp) MarkUnavailable(index int, reason string) error {
	return errNotImplemented
}

// Release returns index to the pool. A query that is still begun cannot be
// released.
func (g *GpuTimestamp) Release(index int) error {
	return errNotImplemented
}

// ResolvedQueries returns the resolved slots of frame ordered by index.
func (g *GpuTimestamp) ResolvedQueries(frame uint64) []QueryInfo {
	return nil
}

// PassMs returns the summed duration of every resolved query of frame whose
// label equals pass.
func (g *GpuTimestamp) PassMs(frame uint64, pass string) float64 {
	return 0
}

// FrameMs returns the summed duration of all resolved queries of frame.
func (g *GpuTimestamp) FrameMs(frame uint64) float64 {
	return 0
}

// SetBudget records the per-frame millisecond budget of one pass label.
func (g *GpuTimestamp) SetBudget(pass string, budgetMs float64) {
}

// OverBudget returns, in ascending label order, the passes whose totals in
// frame exceed their configured budget.
func (g *GpuTimestamp) OverBudget(frame uint64) []string {
	return nil
}

// Snapshot serialises the pass totals and budgets. Query slots themselves are
// not retained; a restored pool starts with an empty allocation table.
func (g *GpuTimestamp) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
