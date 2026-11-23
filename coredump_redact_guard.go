package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the guard.
var (
	ErrInvalidRegion  = errors.New("coredump: invalid region")
	ErrRegionOverlap  = errors.New("coredump: input segments overlap")
	ErrDuplicateRule  = errors.New("coredump: duplicate rule id")
	ErrRuleLimit      = errors.New("coredump: rule limit exceeded")
	ErrUnknownRule    = errors.New("coredump: unknown rule")
	ErrInvalidMode    = errors.New("coredump: invalid redaction mode")
	ErrStaleEpoch     = errors.New("coredump: stale redaction epoch")
	ErrBudgetExceeded = errors.New("coredump: output budget exceeded")
)

// RedactMode says what happens to the bytes a rule covers.
type RedactMode uint8

const (
	// ModeKeep leaves the covered bytes untouched.
	ModeKeep RedactMode = 1
	// ModeZero overwrites the covered bytes with 0x00.
	ModeZero RedactMode = 2
	// ModeFill overwrites the covered bytes with Rule.Fill.
	ModeFill RedactMode = 3
)

// Region is a half-open byte range [Start, End) of the dumped address space.
type Region struct {
	Start uint64
	End   uint64
}

// Size returns End-Start, or 0 for an inverted range.
func (r Region) Size() uint64 {
	if r.End <= r.Start {
		return 0
	}
	return r.End - r.Start
}

// Rule describes how one region is treated before the dump is written.
type Rule struct {
	ID string
	// Region is the address range the rule covers.
	Region Region
	Mode   RedactMode
	// Fill is written when Mode is ModeFill.
	Fill byte
	// Tenant scopes the rule; an empty tenant matches every dump.
	Tenant string
	// Priority resolves overlaps: the higher value wins, then the smaller ID.
	Priority int
}

// Segment is a chunk of the dumped address space plus its payload.
type Segment struct {
	Region Region
	// Data must be exactly Region.Size() bytes long.
	Data []byte
}

// DumpRequest is one dump being redacted for one tenant.
type DumpRequest struct {
	Tenant   string
	Segments []Segment
}

// Config holds the guard's resource limits.
type Config struct {
	// PageSize is the accounting granularity used for Protected lookups.
	PageSize uint64
	// MaxRules caps the number of registered rules.
	MaxRules int
	// MaxOutputBytes caps the size of one redacted dump.
	MaxOutputBytes uint64
}

// Stats is a point-in-time view of the guard.
type Stats struct {
	Rules    int
	Runs     int
	Redacted uint64
}

// CoreDumpGuard redacts protected regions of process core dumps.
type CoreDumpGuard struct {
	cfg Config
}

// NewCoreDumpGuard returns a guard with no rules registered.
func NewCoreDumpGuard(cfg Config) *CoreDumpGuard {
	return &CoreDumpGuard{cfg: cfg}
}

// NewCoreDumpGuardFromSnapshot restores rules and counters from a snapshot
// produced by Snapshot.
func NewCoreDumpGuardFromSnapshot(cfg Config, data []byte) (*CoreDumpGuard, error) {
	return nil, errNotImplemented
}

// RegisterRule installs a rule and returns the new redaction epoch. Rules with
// an invalid region or an invalid mode, duplicate ids and quota overruns must
// be rejected without changing anything.
func (g *CoreDumpGuard) RegisterRule(r Rule) (uint64, error) {
	return 0, errNotImplemented
}

// RemoveRule deletes the rule with the given id. epoch must equal the current
// redaction epoch, otherwise the call is stale and changes nothing.
func (g *CoreDumpGuard) RemoveRule(id string, epoch uint64) error {
	return errNotImplemented
}

// Rule returns a copy of one registered rule.
func (g *CoreDumpGuard) Rule(id string) (Rule, error) {
	return Rule{}, errNotImplemented
}

// RuleCount returns the number of registered rules.
func (g *CoreDumpGuard) RuleCount() int {
	return 0
}

// Epoch returns the current redaction epoch.
func (g *CoreDumpGuard) Epoch() uint64 {
	return 0
}

// Plan copies the request segments, sorts them by address and merges adjacent
// ones. Invalid or overlapping input segments must be rejected.
func (g *CoreDumpGuard) Plan(req DumpRequest) ([]Segment, error) {
	return nil, errNotImplemented
}

// Redact returns the dump payload with every applicable rule applied, in plan
// order. The request must fit Config.MaxOutputBytes.
func (g *CoreDumpGuard) Redact(req DumpRequest) ([]byte, error) {
	return nil, errNotImplemented
}

// RedactedBytes returns the cumulative number of bytes overwritten by
// redaction since the guard was created.
func (g *CoreDumpGuard) RedactedBytes() uint64 {
	return 0
}

// Stats returns rule, run and redacted-byte counters.
func (g *CoreDumpGuard) Stats() Stats {
	return Stats{}
}

// Protected reports whether addr is covered by a rule that applies to tenant.
func (g *CoreDumpGuard) Protected(addr uint64, tenant string) bool {
	return false
}

// Snapshot serialises rules and counters for a restart.
func (g *CoreDumpGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// Reset drops every rule and counter and starts a fresh epoch.
func (g *CoreDumpGuard) Reset() {
}
