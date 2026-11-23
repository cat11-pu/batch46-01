package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by LagCompensation.
var (
	ErrTickOutOfOrder  = errors.New("lag: tick is not newer than the last frame")
	ErrFrameTooLarge   = errors.New("lag: frame exceeds the entity limit")
	ErrDuplicateEntity = errors.New("lag: duplicate entity in frame")
	ErrInvalidEntity   = errors.New("lag: invalid entity state")
	ErrTickExpired     = errors.New("lag: tick outside the history window")
	ErrTickInFuture    = errors.New("lag: tick newer than recorded history")
	ErrNoFrames        = errors.New("lag: no recorded frames")
	ErrEntityNotFound  = errors.New("lag: entity not found in frame")
	ErrNoHit           = errors.New("lag: ray hit nothing")
	ErrInvalidRay      = errors.New("lag: invalid ray")
)

// Vec3 is a position or direction in world space.
type Vec3 struct {
	X float64
	Y float64
	Z float64
}

// EntityState is one entity captured in a world frame.
type EntityState struct {
	ID     string
	Pos    Vec3
	Radius float64
	Alive  bool
}

// WorldFrame is the authoritative world at one server tick.
type WorldFrame struct {
	Tick     uint32
	Entities []EntityState
}

// RaycastHit is the nearest entity crossed by a lag compensated ray.
type RaycastHit struct {
	EntityID string
	Distance float64
	Point    Vec3
}

// Config controls the history ring of one match.
type Config struct {
	// HistoryFrames is how many recent frames are kept; zero means 64.
	HistoryFrames int
	// MaxRewindTicks is how far behind the newest tick a client may rewind.
	MaxRewindTicks uint32
	// MaxEntitiesPerFrame caps the entities accepted in one frame; zero means
	// 256.
	MaxEntitiesPerFrame int
}

// LagCompensation stores recent world frames so a shot can be validated
// against the world the shooter actually saw. All exported methods are safe
// for concurrent use.
type LagCompensation struct {
	cfg Config
}

// NewLagCompensation returns an empty history configured by cfg.
func NewLagCompensation(cfg Config) *LagCompensation {
	return &LagCompensation{cfg: cfg}
}

// NewLagCompensationFromSnapshot restores the frame history from bytes
// produced by Snapshot.
func NewLagCompensationFromSnapshot(data []byte) (*LagCompensation, error) {
	return nil, errNotImplemented
}

// RecordFrame appends the world state for tick. Ticks must be strictly larger
// than the newest recorded tick; when the ring is full the oldest frame is
// evicted.
func (l *LagCompensation) RecordFrame(tick uint32, entities []EntityState) error {
	return errNotImplemented
}

// Rewind returns a copy of the exact frame recorded for tick. Ticks older than
// the oldest retained frame yield ErrTickExpired; ticks newer than the newest
// frame yield ErrTickInFuture.
func (l *LagCompensation) Rewind(tick uint32) (WorldFrame, error) {
	return WorldFrame{}, errNotImplemented
}

// ClampClientTick clamps a client supplied tick into
// [newest-MaxRewindTicks, newest]. With no frames it returns ErrNoFrames.
func (l *LagCompensation) ClampClientTick(clientTick uint32) (uint32, error) {
	return 0, errNotImplemented
}

// EntityAt returns one entity from the frame recorded for tick, including dead
// entities.
func (l *LagCompensation) EntityAt(tick uint32, id string) (EntityState, error) {
	return EntityState{}, errNotImplemented
}

// Raycast intersects the ray origin+t*dir (t in [0, maxDistance]) with the
// alive entities of the frame recorded for tick, and returns the nearest hit.
// Equidistant hits are resolved by the lexicographically smallest entity id.
func (l *LagCompensation) Raycast(tick uint32, origin, dir Vec3, maxDistance float64) (RaycastHit, error) {
	return RaycastHit{}, errNotImplemented
}

// FrameCount reports how many frames are currently retained.
func (l *LagCompensation) FrameCount() int {
	return 0
}

// OldestTick returns the oldest retained tick and whether any frame exists.
func (l *LagCompensation) OldestTick() (uint32, bool) {
	return 0, false
}

// NewestTick returns the newest recorded tick and whether any frame exists.
func (l *LagCompensation) NewestTick() (uint32, bool) {
	return 0, false
}

// MemoryBytes reports the approximate memory held by the history ring.
func (l *LagCompensation) MemoryBytes() int {
	return 0
}

// Prune drops every frame older than beforeTick and returns how many frames
// were removed.
func (l *LagCompensation) Prune(beforeTick uint32) int {
	return 0
}

// Snapshot serialises the retained frames and configuration.
func (l *LagCompensation) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
