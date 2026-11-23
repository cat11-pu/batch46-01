package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by BracketGuard. A rejected winner must not be
// reported as a missing match, and a replay of the same result must not look
// like a conflict.
var (
	ErrBracketConfig     = errors.New("bracket: invalid configuration")
	ErrDuplicateEntrant  = errors.New("bracket: duplicate entrant")
	ErrSeedConflict      = errors.New("bracket: seed conflict")
	ErrMatchNotFound     = errors.New("bracket: match not found")
	ErrNotParticipant    = errors.New("bracket: winner is not a participant")
	ErrMatchClosed       = errors.New("bracket: match already decided")
	ErrMatchNotReady     = errors.New("bracket: match not ready")
	ErrConflictResult    = errors.New("bracket: conflicting result")
	ErrTournamentRunning = errors.New("bracket: tournament still running")
)

// Match status values used by Match.Status.
const (
	StatusPending = "pending"
	StatusReady   = "ready"
	StatusBye     = "bye"
	StatusDone    = "done"
)

// Entrant is one seeded competitor. Seed is one-based and defines the slot.
type Entrant struct {
	ID     string
	Seed   int
	Region string
}

// Match is one node of the single elimination tree. ID has the form
// "R<round>-M<slot>" with both numbers starting at 1.
type Match struct {
	ID       string
	Round    int
	Slot     int
	// HomeID and AwayID are entrant ids; "" means not decided yet.
	HomeID   string
	AwayID   string
	WinnerID string
	Status   string
}

// BracketGuard builds and advances one single elimination bracket.
type BracketGuard struct {
	size int
}

// NewBracketGuard creates a bracket for size entrants. size must be a power of
// two in [4, 256].
func NewBracketGuard(size int) (*BracketGuard, error) {
	return nil, errNotImplemented
}

// NewBracketGuardFromSnapshot restores a bracket, including all reported
// results, from data.
func NewBracketGuardFromSnapshot(data []byte) (*BracketGuard, error) {
	return nil, errNotImplemented
}

// Seed places the entrants on the standard bracket slots and resolves byes.
// It may be called once.
func (g *BracketGuard) Seed(entrants []Entrant) error {
	return errNotImplemented
}

// Matches returns every match sorted by round and slot.
func (g *BracketGuard) Matches() []Match {
	return nil
}

// Match returns one match by id.
func (g *BracketGuard) Match(id string) (Match, error) {
	return Match{}, errNotImplemented
}

// RoundMatches returns the matches of one one-based round.
func (g *BracketGuard) RoundMatches(round int) ([]Match, error) {
	return nil, errNotImplemented
}

// ReportResult records the winner of a ready match. Repeating the same winner
// is a no-op; a different winner is a conflict.
func (g *BracketGuard) ReportResult(matchID, winnerID string) error {
	return errNotImplemented
}

// Champion returns the winner of the final, or ErrTournamentRunning while the
// final is undecided.
func (g *BracketGuard) Champion() (string, error) {
	return "", errNotImplemented
}

// Revision returns how many accepted mutations (seed or result) happened.
func (g *BracketGuard) Revision() int64 {
	return 0
}

// Snapshot serialises the whole bracket so a restart keeps every result.
func (g *BracketGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
