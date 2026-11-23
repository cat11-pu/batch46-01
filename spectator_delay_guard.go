package task

import (
	"errors"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by SpectatorDelay. A viewer that fell behind the
// ring buffer must be told to resync instead of silently losing events.
var (
	ErrInvalidSeq        = errors.New("spectator: invalid sequence")
	ErrConflictingEvent  = errors.New("spectator: conflicting duplicate event")
	ErrSpectatorExists   = errors.New("spectator: spectator already registered")
	ErrSpectatorNotFound = errors.New("spectator: spectator not found")
	ErrTooManySpectators = errors.New("spectator: too many spectators")
	ErrResyncRequired    = errors.New("spectator: resync required")
	ErrDelayRange        = errors.New("spectator: delay out of range")
)

// Config holds the per-match delivery policy.
type Config struct {
	// Delay is the default broadcast delay added to every event.
	Delay time.Duration
	// MaxDelay caps SetDelay and the default delay.
	MaxDelay time.Duration
	// MaxPending is how many undelivered events one match may buffer.
	MaxPending int
	// MaxSpectators caps the viewers of one match.
	MaxSpectators int
}

// Event is one play-by-play item produced by the game server. Seq is strictly
// positive and grows with ServerTime.
type Event struct {
	Seq        int64
	MatchID    string
	ServerTime time.Time
	Kind       string
	Payload    string
}

// Packet is an event as delivered to one spectator.
type Packet struct {
	Seq       int64
	MatchID   string
	Kind      string
	Payload   string
	ReleaseAt time.Time
}

// SpectatorDelay buffers match events and releases them after the viewer's
// delay.
type SpectatorDelay struct{}

// NewSpectatorDelay returns a delay service configured by cfg.
func NewSpectatorDelay(cfg Config) *SpectatorDelay {
	return &SpectatorDelay{}
}

// NewSpectatorDelayFromSnapshot restores buffers, cursors and delays from
// data.
func NewSpectatorDelayFromSnapshot(cfg Config, data []byte) (*SpectatorDelay, error) {
	return nil, errNotImplemented
}

// RegisterSpectator adds a viewer to a match. The viewer starts at the newest
// buffered sequence, so events published before the join are never delivered.
func (s *SpectatorDelay) RegisterSpectator(matchID, spectatorID string, now time.Time) error {
	return errNotImplemented
}

// Unregister removes a viewer.
func (s *SpectatorDelay) Unregister(spectatorID string) error {
	return errNotImplemented
}

// Publish buffers an event. Re-publishing the same sequence with the same
// content is a no-op.
func (s *SpectatorDelay) Publish(ev Event) error {
	return errNotImplemented
}

// Next returns at most max released packets for a spectator in ascending
// sequence order. max <= 0 means no limit. The cursor only moves over returned
// packets.
func (s *SpectatorDelay) Next(spectatorID string, now time.Time, max int) ([]Packet, error) {
	return nil, errNotImplemented
}

// Resync moves a spectator's cursor to the newest buffered sequence and
// clears the resync flag.
func (s *SpectatorDelay) Resync(spectatorID string, now time.Time) error {
	return errNotImplemented
}

// Pending reports how many buffered events are newer than the spectator's
// cursor.
func (s *SpectatorDelay) Pending(spectatorID string) (int, error) {
	return 0, errNotImplemented
}

// SetDelay changes one spectator's broadcast delay.
func (s *SpectatorDelay) SetDelay(spectatorID string, d time.Duration) error {
	return errNotImplemented
}

// Snapshot serialises buffers, cursors and delays.
func (s *SpectatorDelay) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
