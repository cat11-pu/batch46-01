package task

import (
	"errors"
	"time"
)

// Sentinel errors reported by RatingDecay. Callers must be able to tell a
// duplicated match report from an out-of-order rollback.
var (
	ErrUnknownPlayer   = errors.New("rating: unknown player")
	ErrPlayerExists    = errors.New("rating: player already registered")
	ErrInvalidPlayer   = errors.New("rating: invalid player record")
	ErrDuplicateMatch  = errors.New("rating: duplicate match id")
	ErrOutOfOrder      = errors.New("rating: match is not the latest applied match")
	ErrAlreadyReverted = errors.New("rating: match already reverted")
	ErrNothingToRevert = errors.New("rating: no applied match to revert")
	ErrSeasonClosed    = errors.New("rating: season closed")
)

var errNotImplemented = errors.New("not implemented")

// Config is the decay policy of one ladder season.
type Config struct {
	// DecayInterval is the inactivity span that costs one decay step.
	DecayInterval time.Duration
	// DecayPerDay is the number of points removed per full interval.
	DecayPerDay int
	// MinRating is the hard floor; neither losses nor decay go below it.
	MinRating int
	// HistoryCap is how many applied matches per player are remembered.
	HistoryCap int
}

// PlayerRating is the ladder record supplied when a player is registered.
type PlayerRating struct {
	PlayerID string
	SeasonID string
	Rating   int
	// DecayFrom anchors the last settled decay step.
	DecayFrom time.Time
}

// MatchResult is one rated match reported by the match service.
type MatchResult struct {
	MatchID     string
	RatingDelta int
	PlayedAt    time.Time
}

// RatingDecay applies match deltas, inactivity decay and rollbacks for a
// season. Every mutation is guarded so that a restarted process keeps the
// same answer.
type RatingDecay struct {
	cfg Config
}

// NewRatingDecay builds an empty ladder for one season.
func NewRatingDecay(cfg Config) *RatingDecay {
	return &RatingDecay{cfg: cfg}
}

// NewRatingDecayFromSnapshot restores players, their applied match history,
// the revert state and the decay anchors from a Snapshot payload.
func NewRatingDecayFromSnapshot(cfg Config, data []byte) (*RatingDecay, error) {
	return nil, errNotImplemented
}

// RegisterPlayer adds one player. A duplicated player id is rejected with
// ErrPlayerExists, an empty player id or empty season id with ErrInvalidPlayer
// and a season that is already closed with ErrSeasonClosed.
func (r *RatingDecay) RegisterPlayer(p PlayerRating) error {
	return errNotImplemented
}

// ApplyMatch applies one rated match and returns the resulting rating.
// A match id that was already applied (even if reverted) is rejected with
// ErrDuplicateMatch. A negative delta never pushes the rating below
// Config.MinRating, and the recorded applied delta is the clamped one.
func (r *RatingDecay) ApplyMatch(playerID string, m MatchResult, now time.Time) (int, error) {
	return 0, errNotImplemented
}

// ApplyDecay settles every full DecayInterval that elapsed since the decay
// anchor and returns the number of points removed. Calling it twice with the
// same now removes nothing the second time. Decay stops at Config.MinRating.
func (r *RatingDecay) ApplyDecay(playerID string, now time.Time) (int, error) {
	return 0, errNotImplemented
}

// PendingDecay reports how many points ApplyDecay would remove at now without
// mutating any state.
func (r *RatingDecay) PendingDecay(playerID string, now time.Time) int {
	return 0
}

// RevertMatch rolls back the latest applied match and returns the points that
// were given back. Rolling back anything but the latest applied match fails
// with ErrOutOfOrder, a match that was already rolled back with
// ErrAlreadyReverted, and an unknown or evicted match with ErrNothingToRevert.
// Rollback only restores the delta that actually took effect and never
// cancels decay that was settled meanwhile.
func (r *RatingDecay) RevertMatch(playerID, matchID string, now time.Time) (int, error) {
	return 0, errNotImplemented
}

// Rating returns the current rating of a player.
func (r *RatingDecay) Rating(playerID string) (int, error) {
	return 0, errNotImplemented
}

// History returns the applied matches in application order, capped by
// Config.HistoryCap; the oldest entries are evicted first and stop being
// revertible.
func (r *RatingDecay) History(playerID string) []MatchResult {
	return nil
}

// CloseSeason freezes a season: every mutating call for a player of that
// season returns ErrSeasonClosed afterwards. Ratings stay readable.
func (r *RatingDecay) CloseSeason(seasonID string) error {
	return errNotImplemented
}

// SeasonClosed reports whether a season was closed.
func (r *RatingDecay) SeasonClosed(seasonID string) bool {
	return false
}

// Snapshot serialises players, applied match ids, history, revert state and
// decay anchors so a restart continues with identical ratings.
func (r *RatingDecay) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
