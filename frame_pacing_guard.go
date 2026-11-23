package task

import (
	"errors"
	"sync"
	"time"
)

// errNotImplemented marks every method that still has to be written.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by the pacer.
var (
	ErrInvalidConfig  = errors.New("pacer: invalid config")
	ErrInvalidFrame   = errors.New("pacer: invalid frame")
	ErrDuplicateFrame = errors.New("pacer: duplicate frame id")
	ErrQueueFull      = errors.New("pacer: queue full")
	ErrNotReady       = errors.New("pacer: no frame ready")
	ErrBadSnapshot    = errors.New("pacer: invalid snapshot")
)

// Config controls one frame pacer.
type Config struct {
	RefreshHz              float64 // display refresh, 24..240
	MaxPending             int     // 1..64, zero means 64
	LateToleranceMS        float64 // 0..100, zero means 8 ms
	AudioResyncThresholdMS float64 // 10..500, zero means 40 ms
}

// Frame is one decoded video frame.
type Frame struct {
	ID       int64
	PTS      time.Duration
	Duration time.Duration
}

// Presentation is the frame handed to the compositor.
type Presentation struct {
	Frame     Frame
	PresentAt time.Duration
	Resynced  bool
}

// FramePacer schedules frames onto display vsync boundaries.
type FramePacer struct {
	streamID string
	cfg      Config
	mu       sync.Mutex
	// TODO: pending frames ordered by PTS, counters, last presented PTS and the
	// audio clock belong here.
}

// NewFramePacer validates cfg and returns an empty pacer.
func NewFramePacer(streamID string, cfg Config) (*FramePacer, error) {
	return &FramePacer{streamID: streamID, cfg: cfg}, nil
}

// NewFramePacerFromSnapshot restores the pending queue, counters and audio
// clock from Snapshot bytes.
func NewFramePacerFromSnapshot(cfg Config, data []byte) (*FramePacer, error) {
	return nil, errNotImplemented
}

// StreamID returns the stream the pacer was created for.
func (p *FramePacer) StreamID() string { return p.streamID }

// Submit queues a frame. IDs must be unique, PTS must not be negative and
// Duration must be positive; at most MaxPending frames may wait.
func (p *FramePacer) Submit(f Frame) error {
	return errNotImplemented
}

// Next presents the earliest frame that is due at now.
//
// A frame is due when now has reached its vsync aligned presentation time.
// Frames older than the late tolerance and frames whose PTS is before the last
// presented PTS are dropped instead of shown. When nothing is due the call
// returns ErrNotReady and leaves the queue unchanged, apart from dropped late
// frames.
func (p *FramePacer) Next(now time.Duration) (Presentation, error) {
	return Presentation{}, errNotImplemented
}

// Pending returns the number of queued frames.
func (p *FramePacer) Pending() int {
	return 0
}

// PresentedCount returns the number of frames shown.
func (p *FramePacer) PresentedCount() int64 {
	return 0
}

// DroppedCount returns the number of frames discarded as late or stale.
func (p *FramePacer) DroppedCount() int64 {
	return 0
}

// SetAudioClock installs the master audio clock used to re-anchor drifting
// video timestamps.
func (p *FramePacer) SetAudioClock(d time.Duration) error {
	return errNotImplemented
}

// AudioClock returns the master audio clock, or 0 when it was never set.
func (p *FramePacer) AudioClock() time.Duration {
	return 0
}

// Drift returns last presented PTS minus the audio clock.
func (p *FramePacer) Drift() time.Duration {
	return 0
}

// FrameInterval returns one display refresh interval.
func (p *FramePacer) FrameInterval() time.Duration {
	return 0
}

// Reset clears the queue, counters and audio clock.
func (p *FramePacer) Reset() {
}

// Snapshot serialises the pending queue and counters.
func (p *FramePacer) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// snapshotState is the on-the-wire form of FramePacer.
type snapshotState struct {
	StreamID     string        `json:"stream"`
	Queue        []Frame       `json:"queue"`
	Presented    int64         `json:"presented"`
	Dropped      int64         `json:"dropped"`
	LastPTS      time.Duration `json:"last_pts"`
	AudioClock   time.Duration `json:"audio_clock"`
	AudioClockOK bool          `json:"audio_clock_set"`
}
