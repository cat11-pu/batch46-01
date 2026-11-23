package task

import "errors"

// errNotImplemented is returned by every stub until the task is implemented.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the prepass guard.
var (
	ErrInvalidConfig    = errors.New("prepass: invalid config")
	ErrBatchExists      = errors.New("prepass: batch already registered")
	ErrBatchUnknown     = errors.New("prepass: unknown batch")
	ErrInvalidDepth     = errors.New("prepass: invalid depth")
	ErrDrawBudget       = errors.New("prepass: draw budget exceeded")
	ErrDuplicateBatch   = errors.New("prepass: duplicate batch in submission")
	ErrAlreadySubmitted = errors.New("prepass: batch already submitted this frame")
	ErrOrderViolation   = errors.New("prepass: submission order violates the plan")
	ErrBatchOccluded    = errors.New("prepass: batch is occluded")
	ErrCorruptSnapshot  = errors.New("prepass: corrupt snapshot")
)

// DepthConfig configures a depth prepass. MaxDrawCalls is the per-frame draw
// call budget for the prepass.
type DepthConfig struct {
	MaxDrawCalls int
}

// DrawBatch is one opaque draw call considered by the prepass.
type DrawBatch struct {
	ID         string
	Depth      float32
	IndexCount int
	CastsDepth bool
	Occluded   bool
}

// SubmitResult reports the work one Submit call admitted.
type SubmitResult struct {
	DrawCalls int
	Triangles int
}

// PrepassStats is a snapshot of guard counters.
type PrepassStats struct {
	Batches            int
	PlanDraws          int
	FrameDraws         int
	SubmittedTriangles int
}

// DepthPrepass tracks draw batches, their depth sorting and the per-frame
// submission state.
type DepthPrepass struct {
	cfg DepthConfig
}

// NewDepthPrepass returns a prepass guard configured by cfg.
func NewDepthPrepass(cfg DepthConfig) (*DepthPrepass, error) {
	return &DepthPrepass{cfg: cfg}, nil
}

// NewDepthPrepassFromSnapshot restores the batch table, occlusion flags and
// the current frame submission set from data produced by Snapshot.
func NewDepthPrepassFromSnapshot(cfg DepthConfig, data []byte) (*DepthPrepass, error) {
	return nil, errNotImplemented
}

// AddBatch registers a draw batch.
func (g *DepthPrepass) AddBatch(b DrawBatch) error {
	return errNotImplemented
}

// RemoveBatch unregisters a draw batch.
func (g *DepthPrepass) RemoveBatch(id string) error {
	return errNotImplemented
}

// Batch returns one registered batch.
func (g *DepthPrepass) Batch(id string) (DrawBatch, error) {
	return DrawBatch{}, errNotImplemented
}

// Plan returns the batches the prepass would draw, sorted front to back and
// with ties broken by ID. Non-casting and occluded batches are excluded.
func (g *DepthPrepass) Plan() ([]DrawBatch, error) {
	return nil, errNotImplemented
}

// SetOccluded records that a batch was rejected by an earlier depth test.
func (g *DepthPrepass) SetOccluded(id string, occluded bool) error {
	return errNotImplemented
}

// Submit admits batches for the current frame in plan order and within the
// draw call budget.
func (g *DepthPrepass) Submit(ids []string) (SubmitResult, error) {
	return SubmitResult{}, errNotImplemented
}

// BeginFrame clears the per-frame submission state.
func (g *DepthPrepass) BeginFrame() error {
	return errNotImplemented
}

// FrameDraws returns how many draw calls the current frame has admitted.
func (g *DepthPrepass) FrameDraws() int {
	return 0
}

// Stats returns a copy of the current counters.
func (g *DepthPrepass) Stats() PrepassStats {
	return PrepassStats{}
}

// Snapshot serialises the batch table, occlusion flags, submission set and
// counters.
func (g *DepthPrepass) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
