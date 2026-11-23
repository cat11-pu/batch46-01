package task

import "errors"

// errNotImplemented is returned by every stub until the task is implemented.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the guard. Callers distinguish a bad request
// from a full target table or a resolve that blew its byte budget.
var (
	ErrInvalidConfig      = errors.New("msaa: invalid config")
	ErrInvalidTarget      = errors.New("msaa: invalid target")
	ErrUnsupportedSamples = errors.New("msaa: unsupported sample count")
	ErrTargetExists       = errors.New("msaa: target already registered")
	ErrTargetUnknown      = errors.New("msaa: unknown target")
	ErrNoResolveNeeded    = errors.New("msaa: single sample target needs no resolve")
	ErrBudgetExceeded     = errors.New("msaa: resolve budget exceeded")
	ErrRegionOutside      = errors.New("msaa: region outside target")
	ErrTargetLimit        = errors.New("msaa: target limit reached")
	ErrCorruptSnapshot    = errors.New("msaa: corrupt snapshot")
)

// MsaaConfig configures a resolve guard. TileSize is the dirty-region tile
// edge in pixels, MaxTargets caps the target table, MaxResolveBytes is the
// per-frame resolve byte budget.
type MsaaConfig struct {
	TileSize        int
	MaxTargets      int
	MaxResolveBytes int64
}

// RenderTarget is a registered multisample surface.
type RenderTarget struct {
	Name    string
	Width   int
	Height  int
	Samples int
}

// ResolveResult describes one resolve. Bytes is the number of bytes written
// by a real resolve and zero for a cache hit.
type ResolveResult struct {
	Target    string
	Samples   int
	Bytes     int64
	FromCache bool
}

// FrameResult is the outcome of resolving several targets in one frame.
type FrameResult struct {
	Results    []ResolveResult
	TotalBytes int64
}

// MsaaStats is a snapshot of guard counters.
type MsaaStats struct {
	Targets        int
	DirtyTiles     int
	Resolves       int
	CachedResolves int
	BytesResolved  int64
}

// MsaaResolve tracks multisample targets, their dirty tiles and their resolve
// cache state.
type MsaaResolve struct {
	cfg MsaaConfig
}

// NewMsaaResolve returns a guard configured by cfg.
func NewMsaaResolve(cfg MsaaConfig) (*MsaaResolve, error) {
	return &MsaaResolve{cfg: cfg}, nil
}

// NewMsaaResolveFromSnapshot restores a guard, including registered targets,
// dirty tiles and resolve cache state, from data produced by Snapshot.
func NewMsaaResolveFromSnapshot(cfg MsaaConfig, data []byte) (*MsaaResolve, error) {
	return nil, errNotImplemented
}

// RegisterTarget registers a multisample target. The new target starts fully
// dirty and unresolved.
func (g *MsaaResolve) RegisterTarget(name string, width, height, samples int) error {
	return errNotImplemented
}

// ResizeTarget changes the size of an existing target, marks the whole new
// surface dirty and drops its resolve cache.
func (g *MsaaResolve) ResizeTarget(name string, width, height int) error {
	return errNotImplemented
}

// Target returns a copy of the registered target description.
func (g *MsaaResolve) Target(name string) (RenderTarget, error) {
	return RenderTarget{}, errNotImplemented
}

// MarkDirty marks the given pixel rectangle dirty. Regions that stick out of
// the target are clipped; a region fully outside is rejected.
func (g *MsaaResolve) MarkDirty(name string, x, y, w, h int) error {
	return errNotImplemented
}

// DirtyTiles returns the number of tiles that currently need a resolve.
func (g *MsaaResolve) DirtyTiles(name string) (int, error) {
	return 0, errNotImplemented
}

// Resolve resolves one target. A target without dirty tiles is served from
// the cache with FromCache set and zero bytes written.
func (g *MsaaResolve) Resolve(name string) (ResolveResult, error) {
	return ResolveResult{}, errNotImplemented
}

// ResolveFrame resolves the named targets in the given order. It validates
// every name and the total byte budget before writing anything.
func (g *MsaaResolve) ResolveFrame(names []string) (FrameResult, error) {
	return FrameResult{}, errNotImplemented
}

// Snapshot serialises targets, dirty tiles and resolve cache state so that a
// restarted process keeps its bookkeeping.
func (g *MsaaResolve) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// Stats returns a copy of the current counters.
func (g *MsaaResolve) Stats() MsaaStats {
	return MsaaStats{}
}
