package task

import (
	"errors"
	"sync"
)

// errNotImplemented marks every method that still has to be written.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by the detector.
var (
	ErrInvalidConfig    = errors.New("vad: invalid config")
	ErrFrameSize        = errors.New("vad: frame size mismatch")
	ErrInvalidThreshold = errors.New("vad: invalid threshold")
	ErrBadSnapshot      = errors.New("vad: invalid snapshot")
)

// Config controls one voice activity detector.
type Config struct {
	SampleRate        int     // Hz, 8000..192000
	FrameMS           float64 // milliseconds, 10..40
	EnergyThresholdDB float64 // dBFS, -90..0
	MinSpeechFrames   int     // consecutive active frames needed to start speech, >= 1
	HangoverFrames    int     // voiced frames kept after the last active frame, 0..100
}

// FrameInfo is the decision for one analysis frame.
type FrameInfo struct {
	Index    int
	EnergyDB float64
	Voice    bool // frame energy crossed the threshold
	Speech   bool // gated decision after MinSpeechFrames and hangover
}

// VoiceActivity classifies a mono stream frame by frame.
type VoiceActivity struct {
	cfg Config
	mu  sync.Mutex
	// TODO: frame counter, active run length, hangover state and noise floor
	// tracking belong here.
}

// NewVoiceActivity validates cfg and returns a detector with no frames seen.
func NewVoiceActivity(cfg Config) (*VoiceActivity, error) {
	return &VoiceActivity{cfg: cfg}, nil
}

// NewVoiceActivityFromSnapshot restores frame counters, hangover state and the
// noise floor estimate from Snapshot bytes.
func NewVoiceActivityFromSnapshot(cfg Config, data []byte) (*VoiceActivity, error) {
	return nil, errNotImplemented
}

// FrameSamples returns the exact number of samples one frame must contain.
func (v *VoiceActivity) FrameSamples() int {
	return 0
}

// Push analyses exactly FrameSamples samples. A wrong length is rejected and
// leaves every counter unchanged.
func (v *VoiceActivity) Push(samples []float64) (FrameInfo, error) {
	return FrameInfo{}, errNotImplemented
}

// PushFrames analyses a batch that must contain a whole number of frames. The
// batch is validated as a whole before any frame is consumed.
func (v *VoiceActivity) PushFrames(samples []float64) ([]FrameInfo, error) {
	return nil, errNotImplemented
}

// IsSpeech reports the current gated decision.
func (v *VoiceActivity) IsSpeech() bool {
	return false
}

// NoiseFloorDB reports the tracked noise floor in dBFS. It is initialised to
// the configured threshold and follows quiet frames with a 0.9/0.1 EMA.
func (v *VoiceActivity) NoiseFloorDB() float64 {
	return 0
}

// TotalFrames returns the number of analysed frames.
func (v *VoiceActivity) TotalFrames() int {
	return 0
}

// SpeechFrames returns the number of frames reported as speech.
func (v *VoiceActivity) SpeechFrames() int {
	return 0
}

// SpeechRatio returns SpeechFrames/TotalFrames, or 0 before the first frame.
func (v *VoiceActivity) SpeechRatio() float64 {
	return 0
}

// SetEnergyThreshold changes the frame threshold; it must be within [-90, 0].
func (v *VoiceActivity) SetEnergyThreshold(dbfs float64) error {
	return errNotImplemented
}

// Reset clears all counters and restores the initial noise floor.
func (v *VoiceActivity) Reset() {
}

// Snapshot serialises the detector state so a restarted worker keeps its
// hangover decisions.
func (v *VoiceActivity) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// snapshotState is the on-the-wire form of VoiceActivity.
type snapshotState struct {
	FrameIndex   int     `json:"frame_index"`
	RunLength    int     `json:"run_length"`
	HangoverLeft int     `json:"hangover_left"`
	Speech       bool    `json:"speech"`
	TotalFrames  int     `json:"total_frames"`
	SpeechFrames int     `json:"speech_frames"`
	NoiseFloorDB float64 `json:"noise_floor_db"`
}
