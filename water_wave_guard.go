package task

import (
	"errors"
	"sync"
)

// Errors returned by the water surface. Validation errors are sentinels so a
// caller can tell a bad wave authoring value from a corrupt cached snapshot.
var (
	ErrBadResolution  = errors.New("water_waves: resolution must be a power of two in [8,512]")
	ErrBadExtent      = errors.New("water_waves: extent must be positive and finite")
	ErrBadWave        = errors.New("water_waves: invalid wave parameters")
	ErrTooManyWaves   = errors.New("water_waves: at most 8 waves")
	ErrBadTime        = errors.New("water_waves: time must be finite and non-negative")
	ErrBadLOD         = errors.New("water_waves: lod out of range")
	ErrBadShoreFade   = errors.New("water_waves: shore fade outside (0, extent/2]")
	ErrSnapshot       = errors.New("water_waves: corrupt wave snapshot")
	ErrNotImplemented = errors.New("water_waves: not implemented")
)

const (
	// MaxWaves is the number of Gerstner waves one surface may carry.
	MaxWaves = 8
	// MinResolution and MaxResolution bound the LOD0 vertex grid per axis.
	MinResolution = 8
	MaxResolution = 512
)

// Vec2 is a horizontal direction or offset in the XZ plane.
type Vec2 struct{ X, Z float64 }

// Vec3 is a position or normal in surface space.
type Vec3 struct{ X, Y, Z float64 }

// WaveParam describes one travelling Gerstner wave. Dir must be a non-zero
// direction in the XZ plane, Wavelength is in metres, Speed is metres per
// second and Steepness is the Gerstner Q factor in [0,1].
type WaveParam struct {
	Dir        Vec2
	Amplitude  float64
	Wavelength float64
	Speed      float64
	Steepness  float64
}

// Vertex is one displaced surface vertex.
type Vertex struct {
	Position Vec3
	Normal   Vec3
}

// MeshInfo describes the grid BuildMesh produces for one LOD.
type MeshInfo struct {
	Rows      int
	Cols      int
	Triangles int
	Spacing   float64
}

// WaterWaves owns the wave stack of one water patch. The patch spans
// [-extent/2, extent/2] on both X and Z. All methods are safe for concurrent
// use.
type WaterWaves struct {
	mu     sync.RWMutex
	extent float64
	res    int
	waves  []WaveParam
	shore  float64
}

// NewWaterWaves validates the patch size and the LOD0 resolution. Extent must
// be positive and finite (ErrBadExtent); resolution must be a power of two in
// [MinResolution, MaxResolution] (ErrBadResolution). The default shore fade
// distance is extent/8.
func NewWaterWaves(extent float64, resolution int) (*WaterWaves, error) {
	return &WaterWaves{extent: extent, res: resolution, shore: extent / 8}, nil
}

// AddWave appends one wave after validation. Dir must be non-zero, amplitude
// and wavelength positive, speed finite, steepness in [0,1]; violations return
// ErrBadWave. Adding a ninth wave returns ErrTooManyWaves.
func (w *WaterWaves) AddWave(p WaveParam) error {
	return ErrNotImplemented
}

// Waves returns a copy of the current wave stack in insertion order.
func (w *WaterWaves) Waves() []WaveParam {
	return nil
}

// WaveCount returns the number of waves currently applied.
func (w *WaterWaves) WaveCount() int {
	return 0
}

// Height returns the vertical Gerstner displacement at (x,z) and time t:
// sum of Amplitude*sin(k*(Dir . (x,z)) - k*Speed*t) with k = 2*pi/Wavelength.
// NaN or negative t returns ErrBadTime.
func (w *WaterWaves) Height(x, z, t float64) (float64, error) {
	return 0, ErrNotImplemented
}

// Displacement returns the horizontal Gerstner offset at (x,z) and time t.
// Each wave adds Steepness*Amplitude*Dir*cos(phase) on X and Z; Y is always 0
// because the vertical part is reported by Height.
func (w *WaterWaves) Displacement(x, z, t float64) (Vec3, error) {
	return Vec3{}, ErrNotImplemented
}

// Normal returns the unit surface normal at (x,z) and time t. A surface with
// no waves returns (0,1,0).
func (w *WaterWaves) Normal(x, z, t float64) (Vec3, error) {
	return Vec3{}, ErrNotImplemented
}

// BuildMesh displaces the grid at the given LOD and returns it row by row:
// rows advance along Z from -extent/2 to extent/2, columns advance along X
// from -extent/2 to extent/2. Each vertex position is the base grid point plus
// Displacement, with Height as Y, and its normal comes from Normal. lod must be
// in [0, log2(resolution)] or ErrBadLOD is returned; an invalid time returns
// ErrBadTime.
func (w *WaterWaves) BuildMesh(t float64, lod int) ([]Vertex, error) {
	return nil, ErrNotImplemented
}

// MeshInfo reports the vertex grid of one LOD: (n+1)^2 vertices, 2*n^2
// triangles and spacing extent/n, where n = resolution >> lod. An out-of-range
// LOD returns ErrBadLOD.
func (w *WaterWaves) MeshInfo(lod int) (MeshInfo, error) {
	return MeshInfo{}, ErrNotImplemented
}

// MaxHeight returns the sum of all wave amplitudes, the largest vertical
// displacement the surface can ever reach.
func (w *WaterWaves) MaxHeight() float64 {
	return 0
}

// ShoreFactor returns the shore attenuation in [0,1] at (x,z): the distance to
// the nearest patch border divided by the shore fade distance, clamped to 1.
// The centre of the patch is 1 and the border is 0.
func (w *WaterWaves) ShoreFactor(x, z float64) (float64, error) {
	return 0, ErrNotImplemented
}

// SetShoreFade sets the fade distance used by ShoreFactor. Values outside
// (0, extent/2] return ErrBadShoreFade.
func (w *WaterWaves) SetShoreFade(distance float64) error {
	return ErrNotImplemented
}

// Reset removes every wave and restores the default shore fade.
func (w *WaterWaves) Reset() {
}

// Snapshot serialises the wave stack and shore fade for a warm restart.
func (w *WaterWaves) Snapshot() ([]byte, error) {
	return nil, ErrNotImplemented
}

// Restore replaces the wave stack from a snapshot taken by Snapshot. Corrupt
// data, or a snapshot produced for a different extent or resolution, returns
// ErrSnapshot and leaves the surface unchanged.
func (w *WaterWaves) Restore(data []byte) error {
	return ErrNotImplemented
}
