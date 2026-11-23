package task

import (
	"errors"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Hard limits shared by every account.
const (
	// MaxGold is the maximum gold balance one account may hold.
	MaxGold int64 = 1_000_000_000
	// MaxStack is the maximum quantity of one item stack.
	MaxStack = 999
)

// Sentinel errors returned by AccountMerge. A retired source account must be
// distinguishable from an id that never existed.
var (
	ErrInvalidAccount  = errors.New("merge: invalid account")
	ErrAccountNotFound = errors.New("merge: account not found")
	ErrAccountMerged   = errors.New("merge: account already merged")
	ErrSameAccount     = errors.New("merge: source and target are the same")
	ErrDuplicatePlan   = errors.New("merge: duplicate merge id or account already planned")
	ErrPlanNotFound    = errors.New("merge: merge plan not found")
	ErrPlanCancelled   = errors.New("merge: merge plan cancelled")
	ErrMergeCommitted  = errors.New("merge: merge already committed")
	ErrNicknameTaken   = errors.New("merge: nickname already taken")
	ErrGoldOverflow    = errors.New("merge: gold limit exceeded")
	ErrStackOverflow   = errors.New("merge: item stack limit exceeded")
	ErrInvalidPolicy   = errors.New("merge: unknown nickname policy")
)

// Name policies accepted by MergePlan.NicknamePolicy.
const (
	PolicyKeepTarget = "keep_target"
	PolicyKeepSource = "keep_source"
	PolicySuffix     = "suffix"
)

// Account is one game account.
type Account struct {
	ID        string
	Nickname  string
	Gold      int64
	Items     map[string]int
	Level     int
	CreatedAt time.Time
}

// MergePlan describes one pending guest-to-main merge.
type MergePlan struct {
	MergeID        string
	SourceID       string
	TargetID       string
	NicknamePolicy string
}

// MergePreview is the dry-run result of a plan.
type MergePreview struct {
	MergeID     string
	Target      Account
	GoldMoved   int64
	ItemsMerged map[string]int
	Nickname    string
}

// MergeRecord is one committed merge.
type MergeRecord struct {
	MergeID   string
	SourceID  string
	TargetID  string
	At        time.Time
	GoldMoved int64
}

// AccountMerge plans and executes account merges.
type AccountMerge struct{}

// NewAccountMerge returns an empty merge service.
func NewAccountMerge() *AccountMerge {
	return &AccountMerge{}
}

// NewAccountMergeFromSnapshot restores accounts, plans and history from data.
func NewAccountMergeFromSnapshot(data []byte) (*AccountMerge, error) {
	return nil, errNotImplemented
}

// Put inserts or replaces an account. Accounts whose id was already consumed
// as a merge source cannot be put back.
func (m *AccountMerge) Put(a Account) error {
	return errNotImplemented
}

// Get returns one account. A retired source returns ErrAccountMerged.
func (m *AccountMerge) Get(id string) (Account, error) {
	return Account{}, errNotImplemented
}

// Plan validates a merge and stores it as pending. It does not modify any
// account.
func (m *AccountMerge) Plan(p MergePlan, at time.Time) (MergePreview, error) {
	return MergePreview{}, errNotImplemented
}

// Apply executes a pending plan. Repeating an applied plan returns the same
// merged account without merging twice.
func (m *AccountMerge) Apply(mergeID string, at time.Time) (Account, error) {
	return Account{}, errNotImplemented
}

// Cancel drops a pending plan.
func (m *AccountMerge) Cancel(mergeID string) error {
	return errNotImplemented
}

// Pending returns the pending plans sorted by MergeID.
func (m *AccountMerge) Pending() []MergePlan {
	return nil
}

// History returns the committed merges in execution order.
func (m *AccountMerge) History() []MergeRecord {
	return nil
}

// Snapshot serialises accounts, plans, retired ids and history.
func (m *AccountMerge) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
