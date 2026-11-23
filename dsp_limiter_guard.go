package task

import (
	"errors"
	"sync"
)

// errNotImplemented marks every method that still has to be written.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by the limiter.
var (
	ErrInvalidConfig    = errors.New("limiter: invalid config")
	ErrChannelMismatch  = errors.New("limiter: channel mismatch")
	ErrBlockTooLarge    = errors.New("limiter: block too large")
	ErrInvalidParameter = errors.New("limiter: invalid parameter")
	ErrBadSnapshot      = errors.New("limiter: invalid snapshot")
)

// Fixed limits of the limiter.
const (
	// DefaultMaxBlockSamples is used when Config.MaxBlockSamples is zero.
	DefaultMaxBlockSamples = 4096
	// MaxLookaheadMS is the largest lookahead the limiter may use.
	MaxLookaheadMS = 5.0
)

// Config controls one limiter instance.
type Config struct {
	SampleRate      int     // Hz, 8000..192000
	Channels        int     // 1..8
	LookaheadMS     float64 // 0..50, values above 5 are clamped to 5
	ReleaseMS       float64 // 1..5000
	CeilingDBFS     float64 // -24..0
	StereoLink      bool    // apply one gain to every channel
	MaxBlockSamples int     // zero means DefaultMaxBlockSamples
}

// DspLimiter is a lookahead peak limiter with a shared gain envelope.
type DspLimiter struct {
	cfg Config
	mu  sync.Mutex
	// TODO: per channel delay lines, the shared sliding peak window and the
	// smoothed gain state belong here.
}

// NewDspLimiter validates cfg and returns a limiter at unity gain.
func NewDspLimiter(cfg Config) (*DspLimiter, error) {
	return &DspLimiter{cfg: cfg}, nil
}

// NewDspLimiterFromSnapshot rebuilds a limiter, including delay lines and gain
// envelope, from Snapshot bytes.
func NewDspLimiterFromSnapshot(cfg Config, data []byte) (*DspLimiter, error) {
	return nil, errNotImplemented
}

// ProcessBlock limits in and writes the delayed, limited signal to out.
//
// Both slices must have cfg.Channels rows, equal row lengths within
// (0, MaxBlockSamples]. Because the limiter looks ahead, the first
// LatencySamples output frames are zero. Attack is immediate, release follows a
// one pole curve, and no output sample may exceed the linear ceiling.
func (l *DspLimiter) ProcessBlock(in, out [][]float64) error {
	return errNotImplemented
}

// SetCeiling changes the output ceiling in dBFS; it must be within [-24, 0].
func (l *DspLimiter) SetCeiling(dbfs float64) error {
	return errNotImplemented
}

// SetRelease changes the release time in milliseconds; it must be in [1, 5000].
func (l *DspLimiter) SetRelease(ms float64) error {
	return errNotImplemented
}

// LatencySamples returns the lookahead delay in samples.
func (l *DspLimiter) LatencySamples() int {
	return 0
}

// GainReductionDB returns the current gain in dB (0 means no reduction).
func (l *DspLimiter) GainReductionDB() float64 {
	return 0
}

// MaxGainReductionDB returns the lowest gain in dB since the last Reset.
func (l *DspLimiter) MaxGainReductionDB() float64 {
	return 0
}

// Reset clears delay lines, gain reduction and counters.
func (l *DspLimiter) Reset() {
}

// Snapshot serialises the delay lines and gain envelope.
func (l *DspLimiter) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// snapshotState is the on-the-wire form of DspLimiter.
type snapshotState struct {
	SampleRate int         `json:"sample_rate"`
	Channels   int         `json:"channels"`
	Gain       float64     `json:"gain"`
	MaxGain    float64     `json:"max_gain"`
	Delay      [][]float64 `json:"delay"`
	Window     []float64   `json:"window"`
	WindowPos  int         `json:"window_pos"`
}
