package task

import "errors"

// Sentinel errors reported by LootDistributor.
var (
	ErrRunNotFound     = errors.New("loot: unknown run")
	ErrRunExists       = errors.New("loot: run already opened")
	ErrEmptyRaid       = errors.New("loot: raid has no members")
	ErrDuplicateMember = errors.New("loot: duplicate member")
	ErrUnknownMember   = errors.New("loot: unknown member")
	ErrUnknownItem     = errors.New("loot: unknown item")
	ErrDuplicateItem   = errors.New("loot: duplicate item id")
	ErrInvalidQuantity = errors.New("loot: invalid item quantity")
	ErrDuplicateClaim  = errors.New("loot: member already claimed this item")
	ErrInvalidChoice   = errors.New("loot: invalid claim choice")
	ErrInvalidRoll     = errors.New("loot: roll out of range")
	ErrNotEligible     = errors.New("loot: member is not eligible for a need claim")
	ErrAlreadyResolved = errors.New("loot: run already resolved")
	ErrRunClosed       = errors.New("loot: run closed")
)

var errNotImplemented = errors.New("not implemented")

// Reason values reported on an Award.
const (
	ReasonNeed  = "need"
	ReasonGreed = "greed"
	ReasonBank  = "bank"
)

// Config is the distribution policy.
type Config struct {
	// MaxItemsPerMember caps how many copies one member may win in a run.
	MaxItemsPerMember int
	// GuildBankID receives copies nobody claimed or that hit the cap.
	GuildBankID string
}

// Member is one raider.
type Member struct {
	ID        string
	Seniority int
}

// Item is one loot entry dropped by the boss.
type Item struct {
	ID   string
	Name string
	// Qty is how many copies dropped; each copy is awarded separately.
	Qty int
	// MinSeniority is the seniority a member needs for a need claim.
	MinSeniority int
}

// Choice is how a member bids for an item.
type Choice int

const (
	// ChoicePass declines the item.
	ChoicePass Choice = iota
	// ChoiceGreed asks for the item after every need claim.
	ChoiceGreed
	// ChoiceNeed asks for the item first.
	ChoiceNeed
)

// Claim is one member bid for one item.
type Claim struct {
	MemberID string
	ItemID   string
	Choice   Choice
	// Roll is the /roll result, 1..100 for need and greed claims.
	Roll int
}

// Award is one copy handed out; bank awards carry Config.GuildBankID as
// MemberID and Reason "bank".
type Award struct {
	ItemID   string
	MemberID string
	Roll     int
	Choice   Choice
	Reason   string
}

// Run is one raid night opened for distribution.
type Run struct {
	RunID   string
	Members []Member
}

// LootDistributor resolves need/greed conflicts deterministically.
type LootDistributor struct {
	cfg Config
}

// NewLootDistributor builds an empty distributor.
func NewLootDistributor(cfg Config) *LootDistributor {
	return &LootDistributor{cfg: cfg}
}

// NewLootDistributorFromSnapshot restores runs, loot, claims, awards and the
// closed state from a Snapshot payload.
func NewLootDistributorFromSnapshot(cfg Config, data []byte) (*LootDistributor, error) {
	return nil, errNotImplemented
}

// OpenRun registers a raid. Empty raids, duplicate member ids and ids that
// already exist are rejected.
func (d *LootDistributor) OpenRun(run Run) error {
	return errNotImplemented
}

// AddLoot appends items to a run in loot order. Item ids are unique per run
// and quantity must be positive. Adding loot after the run was resolved fails
// with ErrAlreadyResolved, after it was closed with ErrRunClosed.
func (d *LootDistributor) AddLoot(runID string, items []Item) error {
	return errNotImplemented
}

// SubmitClaim records one bid. A need claim below the item MinSeniority is
// rejected with ErrNotEligible; a roll outside 1..100 with ErrInvalidRoll;
// one member may claim an item only once.
func (d *LootDistributor) SubmitClaim(runID string, c Claim) error {
	return errNotImplemented
}

// Resolve hands out every copy in loot order. For one copy the best need claim
// wins; greed claims only compete when no need claim exists. Ranking is roll
// descending, then seniority descending, then member id ascending. Members
// already at Config.MaxItemsPerMember are skipped, and copies without a
// candidate go to the guild bank. Resolving again returns the same awards.
func (d *LootDistributor) Resolve(runID string) ([]Award, error) {
	return nil, errNotImplemented
}

// Awards returns the awards of a resolved run in distribution order.
func (d *LootDistributor) Awards(runID string) ([]Award, error) {
	return nil, errNotImplemented
}

// AwardCount returns how many copies a member received in a run; bank awards
// are not counted for members.
func (d *LootDistributor) AwardCount(runID, memberID string) (int, error) {
	return 0, errNotImplemented
}

// MemberClaims returns the claims a member submitted in a run.
func (d *LootDistributor) MemberClaims(runID, memberID string) ([]Claim, error) {
	return nil, errNotImplemented
}

// CloseRun resolves a run if needed and freezes it. Closing twice is a no-op.
func (d *LootDistributor) CloseRun(runID string) error {
	return errNotImplemented
}

// RunClosed reports whether a run was closed.
func (d *LootDistributor) RunClosed(runID string) bool {
	return false
}

// Snapshot serialises runs, awards and the closed state so a restart keeps the
// same distribution.
func (d *LootDistributor) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
