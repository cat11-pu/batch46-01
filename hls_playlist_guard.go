package task

import (
	"errors"
	"sync"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by HlsPlaylist.
var (
	ErrNoVariantFit     = errors.New("hls: no variant fits the requested bandwidth")
	ErrUnknownVariant   = errors.New("hls: unknown variant uri")
	ErrDuplicateVariant = errors.New("hls: duplicate variant uri")
	ErrInvalidVariant   = errors.New("hls: invalid variant")
	ErrInvalidSegment   = errors.New("hls: invalid segment")
	ErrSegmentOrder     = errors.New("hls: media sequence out of order")
	ErrSegmentDuration  = errors.New("hls: segment duration exceeds target duration")
	ErrDuplicateSegment = errors.New("hls: duplicate segment in the live window")
	ErrPDTRegression    = errors.New("hls: program date time regressed")
	ErrWindowTooLarge   = errors.New("hls: window size exceeds configured maximum")
	ErrNoSegments       = errors.New("hls: playlist has no segments")
	ErrCorruptSnapshot  = errors.New("hls: corrupt snapshot")
)

// Config is the playback policy of one live playlist.
type Config struct {
	// TargetDuration is the largest EXTINF value accepted for a segment.
	TargetDuration time.Duration
	// MaxWindow is the number of newest segments kept in the live window.
	MaxWindow int
	// PDTTolerance is how much program date time may drift backwards without
	// being treated as a regression.
	PDTTolerance time.Duration
}

// Variant is one entry of the master playlist.
type Variant struct {
	URI       string
	Bandwidth int // bits per second
	Height    int // video height in pixels, 0 when unknown
}

// Segment is one media segment of the live playlist.
type Segment struct {
	MediaSequence uint64
	// DiscontinuitySeq is assigned by the playlist when the segment is
	// appended; callers do not supply it.
	DiscontinuitySeq uint64
	URI              string
	Duration         time.Duration
	PDT              time.Time
	Gap              bool
}

// WindowStats summarises the current live window.
type WindowStats struct {
	Segments        int
	Gaps            int
	FirstSequence   uint64
	LastSequence    uint64
	Discontinuities uint64
	Duration        time.Duration
}

// HlsPlaylist tracks the live window and the variant ladder of one stream.
type HlsPlaylist struct {
	mu  sync.Mutex
	cfg Config
}

// NewHlsPlaylist returns an empty playlist configured by cfg.
func NewHlsPlaylist(cfg Config) *HlsPlaylist {
	return &HlsPlaylist{cfg: cfg}
}

// NewHlsPlaylistFromSnapshot restores a playlist, including the variant
// ladder, the selected variant and the live window.
func NewHlsPlaylistFromSnapshot(cfg Config, data []byte) (*HlsPlaylist, error) {
	return nil, errNotImplemented
}

// AddVariant registers a rendition. URIs are unique; the ladder stays sorted
// by bandwidth, then height, then URI.
func (p *HlsPlaylist) AddVariant(v Variant) error {
	return errNotImplemented
}

// VariantCount returns the number of registered renditions.
func (p *HlsPlaylist) VariantCount() int {
	return 0
}

// SelectVariant picks the highest bandwidth at or below bandwidth whose height
// is at most heightCap; heightCap <= 0 means any height.
func (p *HlsPlaylist) SelectVariant(bandwidth, heightCap int) (Variant, error) {
	return Variant{}, errNotImplemented
}

// SetVariant switches rendering to uri. A real switch advances the
// discontinuity sequence; re-selecting the current variant is a no-op.
func (p *HlsPlaylist) SetVariant(uri string, now time.Time) error {
	return errNotImplemented
}

// Current returns the selected variant URI, empty before the first switch.
func (p *HlsPlaylist) Current() string {
	return ""
}

// AppendSegment appends the next segment. The media sequence must be exactly
// one after the newest segment, the duration must not exceed TargetDuration,
// program date time must not regress beyond PDTTolerance, and the segment gets
// the current discontinuity sequence.
func (p *HlsPlaylist) AppendSegment(seg Segment) error {
	return errNotImplemented
}

// Window returns a copy of the live window, oldest segment first.
func (p *HlsPlaylist) Window() []Segment {
	return nil
}

// MediaSequence returns the media sequence the next segment must carry.
func (p *HlsPlaylist) MediaSequence() (uint64, error) {
	return 0, errNotImplemented
}

// DiscontinuitySequence returns the discontinuity sequence currently assigned.
func (p *HlsPlaylist) DiscontinuitySequence() uint64 {
	return 0
}

// Stats summarises the live window.
func (p *HlsPlaylist) Stats() WindowStats {
	return WindowStats{}
}

// TrimWindow drops the oldest segments until at most max remain and returns
// how many were dropped. max must be positive and must not exceed the
// configured MaxWindow.
func (p *HlsPlaylist) TrimWindow(max int) (int, error) {
	return 0, errNotImplemented
}

// LiveEdgeDelay returns how far now trails the end of the newest segment; a
// clock ahead of the live edge yields zero.
func (p *HlsPlaylist) LiveEdgeDelay(now time.Time) time.Duration {
	return 0
}

// Snapshot serialises the ladder and the live window so that a restarted
// loader keeps rejecting duplicates and keeps its discontinuity numbering.
func (p *HlsPlaylist) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
