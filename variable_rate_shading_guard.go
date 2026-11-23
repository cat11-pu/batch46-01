package task

import "errors"

// errNotImplemented is returned by every stub until the task is implemented.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the guard.
var (
	ErrInvalidConfig   = errors.New("vrs: invalid config")
	ErrInvalidRate     = errors.New("vrs: invalid shading rate")
	ErrSurfaceExists   = errors.New("vrs: surface already registered")
	ErrSurfaceUnknown  = errors.New("vrs: unknown surface")
	ErrInvalidSurface  = errors.New("vrs: invalid surface or rectangle")
	ErrRateBudget      = errors.New("vrs: rate change budget exceeded")
	ErrVrsUnsupported  = errors.New("vrs: shading rate control unsupported")
	ErrCorruptSnapshot = errors.New("vrs: corrupt snapshot")
)

// VrsConfig configures a variable rate shading guard. TileSize is the shading
// rate tile edge in pixels and MaxRateChangesPerFrame caps how many tiles may
// differ from the committed state inside one frame.
type VrsConfig struct {
	TileSize               int
	MaxRateChangesPerFrame int
}

// Surface is a registered shading rate image.
type Surface struct {
	Name   string
	Width  int
	Height int
	Cols   int
	Rows   int
}

// RateRect asks for one shading rate over a pixel rectangle. Rate 1 means
// 1x1, 2 means 2x2 and 4 means 4x4 shading.
type RateRect struct {
	X    int
	Y    int
	W    int
	H    int
	Rate int
}

// ChangeSet is the outcome of one SetRates call.
type ChangeSet struct {
	Surface      string
	ChangedTiles int
}

// CommitResult reports what a frame commit flushed.
type CommitResult struct {
	Surfaces     []string
	ChangedTiles int
}

// DegradeResult reports the fallback applied on hardware without VRS.
type DegradeResult struct {
	Surfaces   int
	TilesReset int
}

// VrsStats is a snapshot of guard counters.
type VrsStats struct {
	Surfaces    int
	Tiles       int
	Pending     int
	CommitTiles int
	Degraded    bool
}

// VrsGuard tracks committed and pending per-tile shading rates for a set of
// surfaces.
type VrsGuard struct {
	cfg VrsConfig
}

// NewVrsGuard returns a guard configured by cfg.
func NewVrsGuard(cfg VrsConfig) (*VrsGuard, error) {
	return &VrsGuard{cfg: cfg}, nil
}

// NewVrsGuardFromSnapshot restores committed rates, pending rates and the
// degraded flag from data produced by Snapshot.
func NewVrsGuardFromSnapshot(cfg VrsConfig, data []byte) (*VrsGuard, error) {
	return nil, errNotImplemented
}

// SetSurface registers a shading rate image. The grid is ceil(width/tile) by
// ceil(height/tile) and every tile starts at 1x1.
func (g *VrsGuard) SetSurface(name string, width, height int) error {
	return errNotImplemented
}

// SurfaceInfo returns the registered surface description.
func (g *VrsGuard) SurfaceInfo(name string) (Surface, error) {
	return Surface{}, errNotImplemented
}

// RateAt returns the committed rate of the tile containing pixel (x, y).
func (g *VrsGuard) RateAt(name string, x, y int) (int, error) {
	return 0, errNotImplemented
}

// PendingRateAt returns the rate the tile would have after CommitFrame.
func (g *VrsGuard) PendingRateAt(name string, x, y int) (int, error) {
	return 0, errNotImplemented
}

// SetRectRate sets one rectangle and returns how many tiles this call changed
// in the pending state. It shares the per-frame change budget with SetRates.
func (g *VrsGuard) SetRectRate(name string, x, y, w, h int, rate int) (int, error) {
	return 0, errNotImplemented
}

// SetRates applies a batch of rectangles on one surface transactionally. The
// effective rate of a tile is the coarsest rate of all rectangles covering
// it.
func (g *VrsGuard) SetRates(name string, rects []RateRect) (ChangeSet, error) {
	return ChangeSet{}, errNotImplemented
}

// CombineRates returns the coarser of two shading rates.
func CombineRates(a, b int) int {
	return 0
}

// PendingChanges returns how many tiles differ from the committed state.
func (g *VrsGuard) PendingChanges(name string) (int, error) {
	return 0, errNotImplemented
}

// CommitFrame makes pending rates the committed state and resets the frame
// budget.
func (g *VrsGuard) CommitFrame() (CommitResult, error) {
	return CommitResult{}, errNotImplemented
}

// ClearRates sets every tile of a surface back to 1x1 in the pending state and
// returns how many tiles that changes.
func (g *VrsGuard) ClearRates(name string) (int, error) {
	return 0, errNotImplemented
}

// DegradeTo1x1 switches the guard to the fallback path used on devices without
// variable rate shading support.
func (g *VrsGuard) DegradeTo1x1() (DegradeResult, error) {
	return DegradeResult{}, errNotImplemented
}

// Snapshot serialises committed rates, pending rates and the degraded flag.
func (g *VrsGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// Stats returns a copy of the current counters.
func (g *VrsGuard) Stats() VrsStats {
	return VrsStats{}
}
