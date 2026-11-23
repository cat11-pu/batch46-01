package task

import "errors"

// errNotImplemented is returned by every stub until the task is implemented.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the impostor guard.
var (
	ErrInvalidConfig   = errors.New("impostor: invalid config")
	ErrInvalidRequest  = errors.New("impostor: invalid request")
	ErrAssetConflict   = errors.New("impostor: asset already baked with other parameters")
	ErrAssetUnknown    = errors.New("impostor: unknown asset")
	ErrAtlasFull       = errors.New("impostor: atlas is full")
	ErrBakeBudget      = errors.New("impostor: bake budget exceeded")
	ErrCorruptSnapshot = errors.New("impostor: corrupt snapshot")
)

// ImpostorConfig configures the impostor atlas. AtlasCells is the total number
// of cells, MaxBakeOpsPerFrame caps how many cells may be baked in one frame,
// LODDistance is where the mesh ends and FadeBand is the blend width.
type ImpostorConfig struct {
	AtlasCells         int
	MaxBakeOpsPerFrame int
	LODDistance        float32
	FadeBand           float32
}

// ImpostorRequest asks for a contiguous run of cells for one asset.
type ImpostorRequest struct {
	AssetID  string
	Frames   int
	CellSize int
}

// BakeResult describes the allocation. Allocated is zero for a cache hit.
type BakeResult struct {
	AssetID   string
	FirstCell int
	Allocated int
	Frames    int
}

// Billboard is one impostor instance resolved for a view angle and distance.
type Billboard struct {
	AssetID string
	Frame   int
	Mode    string
	Blend   float32
	U       float32
}

// Usage reports atlas occupancy.
type Usage struct {
	Used   int
	Total  int
	Assets int
}

// ImpostorStats is a snapshot of guard counters.
type ImpostorStats struct {
	Assets     int
	Bakes      int
	CacheHits  int
	Frees      int
	FrameBakes int
}

// ImpostorGuard tracks impostor atlas allocations and per-frame bake budgets.
type ImpostorGuard struct {
	cfg ImpostorConfig
}

// NewImpostorGuard returns a guard configured by cfg.
func NewImpostorGuard(cfg ImpostorConfig) (*ImpostorGuard, error) {
	return &ImpostorGuard{cfg: cfg}, nil
}

// NewImpostorGuardFromSnapshot restores allocations, free cells and counters
// from data produced by Snapshot.
func NewImpostorGuardFromSnapshot(cfg ImpostorConfig, data []byte) (*ImpostorGuard, error) {
	return nil, errNotImplemented
}

// Bake allocates a contiguous cell run for an asset. Repeating the same
// request is a cache hit with Allocated zero.
func (g *ImpostorGuard) Bake(req ImpostorRequest) (BakeResult, error) {
	return BakeResult{}, errNotImplemented
}

// Instance resolves an asset for the given view angle and camera distance.
func (g *ImpostorGuard) Instance(assetID string, angle, distance float32) (Billboard, error) {
	return Billboard{}, errNotImplemented
}

// FrameFor maps a view angle in radians to a baked frame, wrapping at 2*pi.
func (g *ImpostorGuard) FrameFor(assetID string, angle float32) (int, error) {
	return 0, errNotImplemented
}

// SelectMode chooses between "mesh", "blend" and "impostor" for a distance and
// computes the blend factor.
func (g *ImpostorGuard) SelectMode(distance float32) (string, error) {
	return "", errNotImplemented
}

// BeginFrame resets the per-frame bake budget.
func (g *ImpostorGuard) BeginFrame() error {
	return errNotImplemented
}

// FreeAsset releases an asset's cells back to the atlas.
func (g *ImpostorGuard) FreeAsset(assetID string) error {
	return errNotImplemented
}

// AtlasUsage returns the current atlas occupancy.
func (g *ImpostorGuard) AtlasUsage() (Usage, error) {
	return Usage{}, errNotImplemented
}

// Stats returns a copy of the current counters.
func (g *ImpostorGuard) Stats() ImpostorStats {
	return ImpostorStats{}
}

// Snapshot serialises allocations, free cells, counters and the frame budget
// state.
func (g *ImpostorGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
