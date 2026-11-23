package task

import (
	"errors"
	"math"
	"sync"
)

// errNotImplemented marks every method that still has to be written.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by the spatial mixer.
var (
	ErrUnknownSource   = errors.New("spatial: unknown source")
	ErrDuplicateSource = errors.New("spatial: duplicate source id")
	ErrSourceTableFull = errors.New("spatial: source table full")
	ErrInvalidSource   = errors.New("spatial: invalid source")
	ErrInvalidBasis    = errors.New("spatial: invalid listener basis")
	ErrNoListener      = errors.New("spatial: listener not configured")
	ErrBadSnapshot     = errors.New("spatial: invalid snapshot")
)

// Vec3 is a right handed world space vector in metres.
type Vec3 struct {
	X float64
	Y float64
	Z float64
}

// Sub returns v-o.
func (v Vec3) Sub(o Vec3) Vec3 { return Vec3{v.X - o.X, v.Y - o.Y, v.Z - o.Z} }

// Dot returns the dot product of v and o.
func (v Vec3) Dot(o Vec3) float64 { return v.X*o.X + v.Y*o.Y + v.Z*o.Z }

// Len returns the Euclidean length of v.
func (v Vec3) Len() float64 { return math.Sqrt(v.Dot(v)) }

// Normalized returns the unit vector along v; ok is false for a zero vector.
func (v Vec3) Normalized() (Vec3, bool) {
	l := v.Len()
	if l == 0 || math.IsNaN(l) {
		return Vec3{}, false
	}
	return Vec3{v.X / l, v.Y / l, v.Z / l}, true
}

// Config holds per scene mixing limits and defaults.
type Config struct {
	MinDistance   float64 // distance below which attenuation is 1; default 1 m
	MaxDistance   float64 // distance at or beyond which a source is culled; default 50 m
	OcclusionGain float64 // linear multiplier for occluded sources; default 0.35
	MaxVoices     int     // audible voices per frame; default 32
	MaxSources    int     // registered source cap; default 256
	SpeedOfSound  float64 // metres per second; default 343
}

// AudienceListener is the listener pose used for panning and Doppler.
type AudienceListener struct {
	Pos     Vec3
	Forward Vec3
	Up      Vec3
	Vel     Vec3
}

// Source is one positional emitter registered in a scene.
type Source struct {
	ID       string
	Pos      Vec3
	Vel      Vec3
	Gain     float64 // linear [0,1]
	Priority int     // higher wins when the voice budget is exceeded
	Occluded bool
}

// PanGains carries equal power stereo gains and the measured direction.
type PanGains struct {
	Left         float64
	Right        float64
	AzimuthDeg   float64 // -180..180, positive to the listener right
	ElevationDeg float64 // -90..90
	Rear         bool
}

// Voice is one mixed source in a rendered frame.
type Voice struct {
	ID       string
	Left     float64
	Right    float64
	Gain     float64
	Pitch    float64
	Priority int
}

// FrameMix is the result of one mixing pass.
type FrameMix struct {
	Voices  []Voice
	Dropped []string // audible sources that lost the voice budget
	Culled  int      // sources beyond MaxDistance
}

// SpatialAudio mixes positional emitters for one scene.
type SpatialAudio struct {
	sceneID string
	cfg     Config

	mu          sync.Mutex
	listener    AudienceListener
	hasListener bool
	sources     map[string]Source
}

// NewSpatialAudio creates a mixer for a scene. Zero value Config fields take
// their documented defaults.
func NewSpatialAudio(sceneID string, cfg Config) *SpatialAudio {
	return &SpatialAudio{sceneID: sceneID, cfg: cfg, sources: make(map[string]Source)}
}

// NewSpatialAudioFromSnapshot restores a mixer, including listener pose and
// registered sources, from Snapshot bytes.
func NewSpatialAudioFromSnapshot(cfg Config, data []byte) (*SpatialAudio, error) {
	return nil, errNotImplemented
}

// SceneID returns the scene the mixer was created for.
func (s *SpatialAudio) SceneID() string { return s.sceneID }

// SetListener installs the listener pose. Forward must be non-zero and must not
// be parallel to Up; the basis is normalised internally.
func (s *SpatialAudio) SetListener(l AudienceListener) error {
	return errNotImplemented
}

// AddSource registers a new emitter. Duplicate ids and a full table are
// rejected and leave the existing state untouched.
func (s *SpatialAudio) AddSource(src Source) error {
	return errNotImplemented
}

// UpdateSource replaces pose and gain of a registered emitter.
func (s *SpatialAudio) UpdateSource(src Source) error {
	return errNotImplemented
}

// RemoveSource drops an emitter and reports whether it existed.
func (s *SpatialAudio) RemoveSource(id string) bool {
	return false
}

// Source returns a copy of one registered emitter.
func (s *SpatialAudio) Source(id string) (Source, error) {
	return Source{}, errNotImplemented
}

// ActiveSources returns the number of registered emitters.
func (s *SpatialAudio) ActiveSources() int {
	return 0
}

// Pan returns equal power stereo gains for a registered emitter. Sources behind
// the listener are diffuse: left and right are both 1/sqrt(2).
func (s *SpatialAudio) Pan(id string) (PanGains, error) {
	return PanGains{}, errNotImplemented
}

// Attenuation returns inverse distance gain clamped by MinDistance and
// MaxDistance.
func (s *SpatialAudio) Attenuation(id string) (float64, error) {
	return 0, errNotImplemented
}

// Doppler returns the pitch factor for an emitter, clamped to [0.5, 2.0].
func (s *SpatialAudio) Doppler(id string) (float64, error) {
	return 0, errNotImplemented
}

// Mix builds the audible voice list: attenuation, source gain and occlusion are
// folded together, the voice budget keeps the highest priorities and the
// remainder is reported as dropped. Mix does not change the source table.
func (s *SpatialAudio) Mix() (FrameMix, error) {
	return FrameMix{}, errNotImplemented
}

// MixOrder returns every registered id ordered by priority descending and id
// ascending, independent of insertion order.
func (s *SpatialAudio) MixOrder() []string {
	return nil
}

// Snapshot serialises listener pose and sources for a scene reload.
func (s *SpatialAudio) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// snapshotState is the on-the-wire form of SpatialAudio.
type snapshotState struct {
	SceneID  string            `json:"scene"`
	Listener AudienceListener  `json:"listener"`
	HasSet   bool              `json:"has_listener"`
	Sources  map[string]Source `json:"sources"`
}
