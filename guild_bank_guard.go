package task

import (
	"errors"
	"time"
)

// Sentinels returned by the guild bank guard. Callers must be able to tell a
// permission problem from a stock problem and from a duplicated operation.
var (
	ErrUnknownMember     = errors.New("guildbank: unknown member")
	ErrUnknownTab        = errors.New("guildbank: unknown tab")
	ErrTabLocked         = errors.New("guildbank: tab locked")
	ErrRankDenied        = errors.New("guildbank: rank denied")
	ErrInvalidCount      = errors.New("guildbank: invalid count")
	ErrInsufficientCount = errors.New("guildbank: insufficient item count")
	ErrQuotaExceeded     = errors.New("guildbank: daily quota exceeded")
	ErrDuplicateOp       = errors.New("guildbank: duplicate operation id")
	ErrLedgerFull        = errors.New("guildbank: ledger capacity reached")
	ErrChainBroken       = errors.New("guildbank: ledger chain broken")
)

var errNotImplemented = errors.New("guildbank: not implemented")

// Rank is the guild permission level of a member.
type Rank int

const (
	RankMember      Rank = 1
	RankOfficer     Rank = 2
	RankGuildMaster Rank = 3
)

// Config is the bank policy.
type Config struct {
	// DailyQuota is how many item units one member may withdraw per UTC day.
	DailyQuota int
	// LedgerCapacity is the maximum number of audit entries kept.
	LedgerCapacity int
}

// Member is a guild member with a rank.
type Member struct {
	Name string
	Rank Rank
}

// Item is one item stack.
type Item struct {
	ID    string
	Count int
}

// Tab is one bank tab with its rank requirement and stock.
type Tab struct {
	Index   int
	MinRank Rank
	Locked  bool
	Items   []Item
}

// WithdrawalRequest describes one withdrawal to be audited.
type WithdrawalRequest struct {
	OpID     string
	Member   string
	TabIndex int
	ItemID   string
	Count    int
	At       time.Time
}

// AuditEntry is one immutable ledger record. Hash is the hex SHA-256 of
// PrevHash, OpID, Member, TabIndex, ItemID and Count joined by "|".
type AuditEntry struct {
	Seq      int
	OpID     string
	Kind     string
	Member   string
	TabIndex int
	ItemID   string
	Count    int
	At       time.Time
	PrevHash string
	Hash     string
}

// GuildBankGuard enforces rank, quota and audit rules for a guild bank.
type GuildBankGuard struct {
	cfg Config
}

// NewGuildBankGuard returns an empty guard configured by cfg.
func NewGuildBankGuard(cfg Config) *GuildBankGuard {
	return &GuildBankGuard{cfg: cfg}
}

// NewGuildBankGuardFromSnapshot restores members, tabs, stock, ledger and the
// used operation ids from a snapshot produced by Snapshot.
func NewGuildBankGuardFromSnapshot(cfg Config, data []byte) (*GuildBankGuard, error) {
	return nil, errNotImplemented
}

// AddMember registers a member with an initial rank.
func (g *GuildBankGuard) AddMember(m Member) error {
	return errNotImplemented
}

// SetRank changes the rank of an existing member.
func (g *GuildBankGuard) SetRank(name string, r Rank) error {
	return errNotImplemented
}

// CreateTab creates a tab index with a minimum rank.
func (g *GuildBankGuard) CreateTab(index int, minRank Rank) error {
	return errNotImplemented
}

// LockTab freezes a tab so that no further deposit or withdrawal succeeds.
func (g *GuildBankGuard) LockTab(index int) error {
	return errNotImplemented
}

// Deposit stores item units in a tab and appends an audit entry.
func (g *GuildBankGuard) Deposit(opID, member string, tabIndex int, item Item, at time.Time) (AuditEntry, error) {
	return AuditEntry{}, errNotImplemented
}

// Withdraw removes item units from a tab after checking rank, daily quota and
// stock, and appends an audit entry.
func (g *GuildBankGuard) Withdraw(req WithdrawalRequest) (AuditEntry, error) {
	return AuditEntry{}, errNotImplemented
}

// TabContents returns a copy of the stacks of one tab in stack order.
func (g *GuildBankGuard) TabContents(index int) ([]Item, error) {
	return nil, errNotImplemented
}

// Ledger returns a copy of the audit entries in sequence order.
func (g *GuildBankGuard) Ledger() []AuditEntry {
	return nil
}

// VerifyChain recomputes every audit hash and reports ErrChainBroken on the
// first mismatch.
func (g *GuildBankGuard) VerifyChain() error {
	return errNotImplemented
}

// Snapshot serialises members, tabs, stock, ledger and used operation ids.
func (g *GuildBankGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// DailyTotal returns how many item units a member withdrew on the UTC day
// containing day.
func (g *GuildBankGuard) DailyTotal(member string, day time.Time) int {
	return 0
}
