package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by TeamBuilder. Callers branch on these values, so
// a rejected hero swap must not be reported as a generic validation failure.
var (
	ErrRosterInvalid   = errors.New("team: invalid roster")
	ErrDuplicateMember = errors.New("team: duplicate member")
	ErrDuplicateHero   = errors.New("team: duplicate hero")
	ErrHeroBanned      = errors.New("team: banned hero")
	ErrRoleLimit       = errors.New("team: role limit exceeded")
	ErrRosterFull      = errors.New("team: roster full")
	ErrMemberNotFound  = errors.New("team: member not found")
	ErrStaleRevision   = errors.New("team: stale revision")
	ErrNotReady        = errors.New("team: member not ready")
)

// MaxSnapshotBytes caps the serialised roster accepted by
// NewTeamBuilderFromSnapshot.
const MaxSnapshotBytes = 256 << 10

// Rules describes the static limits of one queue.
type Rules struct {
	// Size is the maximum number of members.
	Size int
	// MaxPerRole caps how many members may take a given role. Roles that are
	// absent from the map are unlimited.
	MaxPerRole map[string]int
	// BannedHeroes cannot be picked by any member.
	BannedHeroes []string
	// RequireCaptain makes a captain mandatory for Validate.
	RequireCaptain bool
}

// Member is one roster slot as submitted by the client.
type Member struct {
	PlayerID string
	HeroID   string
	Role     string
	Ready    bool
	Captain  bool
}

// TeamBuilder validates and mutates the roster of one team.
type TeamBuilder struct {
	rules Rules
}

// NewTeamBuilder returns an empty roster enforcing rules.
func NewTeamBuilder(rules Rules) *TeamBuilder {
	return &TeamBuilder{rules: rules}
}

// NewTeamBuilderFromSnapshot restores a roster and its revision from data.
func NewTeamBuilderFromSnapshot(rules Rules, data []byte) (*TeamBuilder, error) {
	return nil, errNotImplemented
}

// AddMember appends a member. Captaincy is never taken from m; it can only be
// assigned through SetCaptain.
func (b *TeamBuilder) AddMember(m Member) error {
	return errNotImplemented
}

// RemoveMember removes playerID and returns the removed member. Removing the
// captain leaves the roster without one.
func (b *TeamBuilder) RemoveMember(playerID string) (Member, error) {
	return Member{}, errNotImplemented
}

// SetHero changes the hero of playerID. On any rejection the previous hero is
// kept.
func (b *TeamBuilder) SetHero(playerID, heroID string) error {
	return errNotImplemented
}

// SetReady sets the ready flag of playerID.
func (b *TeamBuilder) SetReady(playerID string, ready bool) error {
	return errNotImplemented
}

// SetCaptain makes playerID the only captain and returns the previous
// captain's player id ("" when there was none).
func (b *TeamBuilder) SetCaptain(playerID string) (string, error) {
	return "", errNotImplemented
}

// Roster returns a copy of the members sorted by PlayerID ascending.
func (b *TeamBuilder) Roster() []Member {
	return nil
}

// Validate checks the whole roster: non-empty, no duplicate heroes, role caps,
// a captain when required, and every member ready.
func (b *TeamBuilder) Validate() error {
	return errNotImplemented
}

// Revision returns the optimistic concurrency token of the roster.
func (b *TeamBuilder) Revision() int64 {
	return 0
}

// ReplaceAll replaces the roster atomically when expectedRevision matches.
func (b *TeamBuilder) ReplaceAll(members []Member, expectedRevision int64) error {
	return errNotImplemented
}

// Snapshot serialises the roster and its revision.
func (b *TeamBuilder) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
