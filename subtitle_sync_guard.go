package task

import (
	"errors"
	"sync"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by SubtitleSync.
var (
	ErrInvalidCue      = errors.New("subtitle: invalid cue")
	ErrCueOverlap      = errors.New("subtitle: cue overlaps its neighbour")
	ErrDuplicateCue    = errors.New("subtitle: duplicate cue")
	ErrTooManyCues     = errors.New("subtitle: cue count exceeds the limit")
	ErrCueTooShort     = errors.New("subtitle: cue shorter than the minimum duration")
	ErrNegativeTime    = errors.New("subtitle: shift would move a cue before zero")
	ErrInvalidFPS      = errors.New("subtitle: invalid frame rate")
	ErrSplitOutsideCue = errors.New("subtitle: split point outside the cue")
	ErrIndexOutOfRange = errors.New("subtitle: cue index out of range")
	ErrCorruptSnapshot = errors.New("subtitle: corrupt snapshot")
)

// Config bounds the subtitle track.
type Config struct {
	// MaxCues is the largest number of cues the track accepts; zero means no
	// limit.
	MaxCues int
	// MinDuration is the shortest cue, or split fragment, kept in the track.
	MinDuration time.Duration
}

// Cue is one subtitle line on the media timeline.
type Cue struct {
	Start time.Duration
	End   time.Duration
	Text  string
}

// Stats summarises the loaded track.
type Stats struct {
	Cues          int
	TotalDuration time.Duration
}

// SubtitleSync holds the cue track of one title.
type SubtitleSync struct {
	mu  sync.Mutex
	cfg Config
}

// NewSubtitleSync returns an empty track.
func NewSubtitleSync(cfg Config) *SubtitleSync {
	return &SubtitleSync{cfg: cfg}
}

// NewSubtitleSyncFromSnapshot restores a track and keeps its ordering.
func NewSubtitleSyncFromSnapshot(cfg Config, data []byte) (*SubtitleSync, error) {
	return nil, errNotImplemented
}

// LoadCues replaces the track after validating every cue. Cues are stored
// sorted by start time; the load is all or nothing.
func (s *SubtitleSync) LoadCues(cues []Cue) error {
	return errNotImplemented
}

// AppendCue inserts one cue at its sorted position.
func (s *SubtitleSync) AppendCue(c Cue) error {
	return errNotImplemented
}

// Cues returns a copy of the track, sorted by start time.
func (s *SubtitleSync) Cues() []Cue {
	return nil
}

// At returns the cue containing t, treating cues as [Start, End).
func (s *SubtitleSync) At(t time.Duration) (Cue, bool) {
	return Cue{}, false
}

// Shift moves every cue by offset; a negative result for any cue rejects the
// whole shift.
func (s *SubtitleSync) Shift(offset time.Duration) error {
	return errNotImplemented
}

// Rescale converts the track from one frame rate to another and rejects the
// whole conversion when a cue would become shorter than MinDuration.
func (s *SubtitleSync) Rescale(fromFPS, toFPS int) error {
	return errNotImplemented
}

// Merge joins neighbouring cues whose gap is at most maxGap and returns how
// many joins happened.
func (s *SubtitleSync) Merge(maxGap time.Duration) int {
	return 0
}

// Split cuts the cue at index at the absolute instant at.
func (s *SubtitleSync) Split(index int, at time.Duration) error {
	return errNotImplemented
}

// Remove deletes the cue at index.
func (s *SubtitleSync) Remove(index int) error {
	return errNotImplemented
}

// Stats summarises the track.
func (s *SubtitleSync) Stats() Stats {
	return Stats{}
}

// Snapshot serialises the cue track so that a restart keeps every timecode.
func (s *SubtitleSync) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
