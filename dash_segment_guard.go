package task

import (
	"errors"
	"sync"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by DashSegment.
var (
	ErrInvalidRepresentation   = errors.New("dash: invalid representation")
	ErrDuplicateRepresentation = errors.New("dash: duplicate representation id")
	ErrUnknownRepresentation   = errors.New("dash: unknown representation")
	ErrSegmentOutOfRange       = errors.New("dash: segment number outside the timeline")
	ErrBeforeStart             = errors.New("dash: instant before the presentation start")
	ErrSameRepresentation      = errors.New("dash: source and target representation are the same")
	ErrQueryTooLarge           = errors.New("dash: query exceeds the segment budget")
	ErrCorruptSnapshot         = errors.New("dash: corrupt snapshot")
)

// Config bounds the timeline work a single player may do.
type Config struct {
	// MaxSegmentsPerRep is the hard cap on segments declared by one
	// representation.
	MaxSegmentsPerRep uint64
	// MaxSegmentsPerQuery is the hard cap on segments returned by one call.
	MaxSegmentsPerQuery int
}

// Representation is one video or audio timeline of an MPD.
type Representation struct {
	ID              string
	Bandwidth       int
	Timescale       uint64 // ticks per second
	SegmentDuration uint64 // ticks per segment
	StartNumber     uint64
	// PresentationTimeOffset is the media time of the first segment, in ticks.
	PresentationTimeOffset uint64
	// MaxSegments is the number of segments in this timeline; it must be
	// positive and must not exceed Config.MaxSegmentsPerRep.
	MaxSegments uint64
}

// SegmentRef addresses one segment on the presentation timeline.
type SegmentRef struct {
	RepresentationID string
	Number           uint64
	Start            time.Duration
	End              time.Duration
	URI              string
}

// Stats describes the segment store.
type Stats struct {
	Representations int
	Current         string
	Served          uint64
}

// DashSegment resolves segment numbers on one or more MPD timelines.
type DashSegment struct {
	mu  sync.Mutex
	cfg Config
}

// NewDashSegment returns an empty segment store.
func NewDashSegment(cfg Config) *DashSegment {
	return &DashSegment{cfg: cfg}
}

// NewDashSegmentFromSnapshot restores the registered timelines, the selected
// representation and the served counter.
func NewDashSegmentFromSnapshot(cfg Config, data []byte) (*DashSegment, error) {
	return nil, errNotImplemented
}

// AddRepresentation registers a timeline. IDs are unique, the timescale,
// segment duration and segment count must be positive, and the count must fit
// the configured cap.
func (d *DashSegment) AddRepresentation(rep Representation) error {
	return errNotImplemented
}

// RepresentationCount returns how many timelines are registered.
func (d *DashSegment) RepresentationCount() int {
	return 0
}

// SegmentAt resolves an explicit segment number.
func (d *DashSegment) SegmentAt(repID string, number uint64) (SegmentRef, error) {
	return SegmentRef{}, errNotImplemented
}

// SegmentForTime returns the segment containing t on the timeline of repID.
func (d *DashSegment) SegmentForTime(repID string, t time.Duration) (SegmentRef, error) {
	return SegmentRef{}, errNotImplemented
}

// SegmentNumberAt returns the number of the segment containing t.
func (d *DashSegment) SegmentNumberAt(repID string, t time.Duration) (uint64, error) {
	return 0, errNotImplemented
}

// SegmentDuration returns the wall-clock length of one segment of repID.
func (d *DashSegment) SegmentDuration(repID string) (time.Duration, error) {
	return 0, errNotImplemented
}

// AlignFrom returns count consecutive segments starting with the segment that
// contains from. count must be positive and must not exceed
// Config.MaxSegmentsPerQuery.
func (d *DashSegment) AlignFrom(repID string, from time.Duration, count int) ([]SegmentRef, error) {
	return nil, errNotImplemented
}

// SwitchRepresentation aligns a switch from fromID to toID at instant at. The
// switch point must exist on the source timeline and the first target segment
// starts at the next boundary at or after at. On success the target becomes the
// current representation.
func (d *DashSegment) SwitchRepresentation(fromID, toID string, at time.Duration) (SegmentRef, error) {
	return SegmentRef{}, errNotImplemented
}

// Current returns the selected representation, empty before the first switch.
func (d *DashSegment) Current() string {
	return ""
}

// Stats reports the store state.
func (d *DashSegment) Stats() Stats {
	return Stats{}
}

// Snapshot serialises the timelines and the switch state so that a restart
// keeps the same numbering and the same selected representation.
func (d *DashSegment) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
