package task

import (
	"errors"
	"sync"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by AbrLadder.
var (
	ErrInvalidRung     = errors.New("abr: invalid rung")
	ErrDuplicateRung   = errors.New("abr: duplicate rung id")
	ErrLadderOrder     = errors.New("abr: ladder is not ordered by bandwidth")
	ErrTooFewRungs     = errors.New("abr: ladder has fewer rungs than required")
	ErrNoRungFits      = errors.New("abr: no rung fits the budget")
	ErrInvalidSample   = errors.New("abr: invalid bandwidth sample")
	ErrUnknownRung     = errors.New("abr: unknown rung id")
	ErrLadderNotSet    = errors.New("abr: ladder not configured")
	ErrCorruptSnapshot = errors.New("abr: corrupt snapshot")
)

// Config is the switching policy of one player.
type Config struct {
	// MinRungs is the smallest ladder the player accepts.
	MinRungs int
	// SettleWindow is the minimum time between two automatic switches.
	SettleWindow time.Duration
	// EWMAlpha weights the newest bandwidth sample, in (0, 1].
	EWMAlpha float64
	// UpSwitchBuffer is the buffer that must be filled before switching up.
	UpSwitchBuffer time.Duration
	// MaxUpStep is the largest number of rungs climbed by one switch.
	MaxUpStep int
}

// Rung is one rendition of the ladder.
type Rung struct {
	ID        int
	Height    int
	Bandwidth int // bits per second
}

// Stats describes the player state.
type Stats struct {
	Rungs    int
	Current  int
	Estimate int
	Samples  uint64
	Switches uint64
}

// AbrLadder selects a rendition from measured bandwidth and buffer level.
type AbrLadder struct {
	mu  sync.Mutex
	cfg Config
}

// NewAbrLadder returns a player without a ladder.
func NewAbrLadder(cfg Config) *AbrLadder {
	return &AbrLadder{cfg: cfg}
}

// NewAbrLadderFromSnapshot restores the ladder, the estimate, the current rung
// and the counters.
func NewAbrLadderFromSnapshot(cfg Config, data []byte) (*AbrLadder, error) {
	return nil, errNotImplemented
}

// SetLadder replaces the ladder. Rung ids are unique, bandwidth and height are
// positive and bandwidth must strictly increase.
func (a *AbrLadder) SetLadder(rungs []Rung) error {
	return errNotImplemented
}

// Rungs returns a copy of the ladder, lowest bandwidth first.
func (a *AbrLadder) Rungs() []Rung {
	return nil
}

// RungForBandwidth returns the highest rung at or below bandwidth.
func (a *AbrLadder) RungForBandwidth(bandwidth int) (Rung, error) {
	return Rung{}, errNotImplemented
}

// RungForHeight returns the highest rung at or below height.
func (a *AbrLadder) RungForHeight(height int) (Rung, error) {
	return Rung{}, errNotImplemented
}

// ObserveSample feeds one bandwidth measurement into the moving average.
func (a *AbrLadder) ObserveSample(bps int, now time.Time) error {
	return errNotImplemented
}

// Estimate returns the current bandwidth estimate.
func (a *AbrLadder) Estimate() int {
	return 0
}

// Decide returns the rung the player should use. It never switches during the
// settle window, never climbs more than MaxUpStep rungs, and only climbs when
// the buffer has reached UpSwitchBuffer.
func (a *AbrLadder) Decide(buffer time.Duration, now time.Time) (Rung, bool, error) {
	return Rung{}, false, errNotImplemented
}

// ForceRung pins the player to one rung regardless of the estimate.
func (a *AbrLadder) ForceRung(id int, now time.Time) error {
	return errNotImplemented
}

// Current returns the active rung.
func (a *AbrLadder) Current() (Rung, bool) {
	return Rung{}, false
}

// Cooldown returns how much of the settle window is still left at now.
func (a *AbrLadder) Cooldown(now time.Time) time.Duration {
	return 0
}

// Stats reports the player state.
func (a *AbrLadder) Stats() Stats {
	return Stats{}
}

// Snapshot serialises the ladder and the player state so that a restart keeps
// the same estimate, rung and cooldown.
func (a *AbrLadder) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
