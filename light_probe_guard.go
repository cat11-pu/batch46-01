package task

import (
	"errors"
	"sync"
)

// Errors returned by the probe grid.
var (
	ErrBadSpacing     = errors.New("light_probe: spacing must be positive and finite")
	ErrBadGrid        = errors.New("light_probe: axis count must be in [2,16] and the grid at most "+"4096 probes")
	ErrIndexRange     = errors.New("light_probe: probe index out of range")
	ErrOutsideVolume  = errors.New("light_probe: position outside the probe volume")
	ErrSnapshot       = errors.New("light_probe: corrupt probe snapshot")
	ErrNotImplemented = errors.New("light_probe: not implemented")
)

// MaxProbes is the largest probe count one grid may hold.
const MaxProbes = 4096

// Vec3 is a position, direction or RGB colour.
type Vec3 struct{ X, Y, Z float64 }

// SH9 holds nine RGB spherical-harmonic coefficients. Index 0 is the DC term,
// 1..3 are the linear Y, Z, X terms and 4..8 are the quadratic terms
// XY, YZ, 3Z^2-1, XZ, X^2-Y^2.
type SH9 struct {
	C [9]Vec3
}

// LightProbe is a regular 3D lattice of irradiance probes. Probe (0,0,0)
// sits at Origin and probe (x,y,z) at Origin + (x,y,z)*Spacing. All methods are
// safe for concurrent use.
type LightProbe struct {
	mu      sync.RWMutex
	origin  Vec3
	spacing float64
	counts  [3]int
	probes  []SH9
	dirty   []bool
}

// NewLightProbe validates the lattice and allocates storage. Spacing must
// be positive and finite (ErrBadSpacing); every axis must hold 2..16 probes and
// the product must not exceed MaxProbes (ErrBadGrid).
func NewLightProbe(origin Vec3, spacing float64, counts [3]int) (*LightProbe, error) {
	return &LightProbe{origin: origin, spacing: spacing, counts: counts}, nil
}

// ProbeCount returns the number of probes in the lattice.
func (g *LightProbe) ProbeCount() int {
	return 0
}

// Index maps an axis coordinate to its row-major slot:
// x + counts[0]*(y + counts[1]*z). Coordinates outside the lattice return
// ErrIndexRange.
func (g *LightProbe) Index(x, y, z int) (int, error) {
	return 0, ErrNotImplemented
}

// Coordinates maps a slot back to its axis coordinate. A slot outside
// [0,ProbeCount) returns ErrIndexRange.
func (g *LightProbe) Coordinates(index int) ([3]int, error) {
	return [3]int{}, ErrNotImplemented
}

// SetProbe stores the coefficients of one probe and clears its dirty flag.
// A coordinate outside the lattice returns ErrIndexRange.
func (g *LightProbe) SetProbe(x, y, z int, sh SH9) error {
	return ErrNotImplemented
}

// GetProbe returns the coefficients of one probe. A probe that has never been
// set returns the zero SH9 with a nil error. A coordinate outside the lattice
// returns ErrIndexRange.
func (g *LightProbe) GetProbe(x, y, z int) (SH9, error) {
	return SH9{}, ErrNotImplemented
}

// CellForPosition locates a world position inside a lattice cell. It returns
// the lower cell coordinate (each axis clamped to [0,counts-2]) and the local
// coordinate in [0,1] within that cell; the far face of the lattice maps to
// the last cell with local 1. Positions outside the lattice, including NaN,
// return ErrOutsideVolume.
func (g *LightProbe) CellForPosition(p Vec3) ([3]int, Vec3, error) {
	return [3]int{}, Vec3{}, ErrNotImplemented
}

// Trilinear interpolates the eight probes around p. A position outside the
// lattice returns ErrOutsideVolume.
func (g *LightProbe) Trilinear(p Vec3) (SH9, error) {
	return SH9{}, ErrNotImplemented
}

// Tetrahedral interpolates the four probes of the tetrahedron that contains p,
// using the six-tetrahedron subdivision of the cell. A position outside the
// lattice returns ErrOutsideVolume.
func (g *LightProbe) Tetrahedral(p Vec3) (SH9, error) {
	return SH9{}, ErrNotImplemented
}

// Irradiance evaluates the reconstructed diffuse irradiance along n with
// cosine-lobe convolution: pi*c0 + (2*pi/3)*(c1*y+c2*z+c3*x) +
// (pi/4)*(c4*x*y+c5*y*z+c6*(3z^2-1)+c7*x*z+c8*(x^2-y^2)). n is normalised
// internally; a zero-length n returns the zero vector.
func (g *LightProbe) Irradiance(sh SH9, n Vec3) Vec3 {
	return Vec3{}
}

// Blend linearly interpolates two SH sets; w is clamped to [0,1].
func (g *LightProbe) Blend(a, b SH9, w float64) SH9 {
	return SH9{}
}

// Invalidate marks one probe as needing a rebake. Marking an already dirty
// probe is idempotent; a coordinate outside the lattice returns ErrIndexRange.
func (g *LightProbe) Invalidate(x, y, z int) error {
	return ErrNotImplemented
}

// DirtyCount returns the number of probes currently marked for rebake.
func (g *LightProbe) DirtyCount() int {
	return 0
}

// Snapshot serialises the lattice, every probe and the dirty set for a warm
// restart.
func (g *LightProbe) Snapshot() ([]byte, error) {
	return nil, ErrNotImplemented
}

// Restore replaces probes and the dirty set from a snapshot taken by Snapshot.
// Corrupt data, or a snapshot whose origin, spacing or counts differ from this
// grid, returns ErrSnapshot and leaves the grid untouched.
func (g *LightProbe) Restore(data []byte) error {
	return ErrNotImplemented
}
