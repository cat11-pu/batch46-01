package task

import (
	"errors"
	"time"
)

// Sentinels returned by EvidenceGuard. Replay of evidence and a broken chain
// must be distinguishable from quota and lookup failures.
var (
	ErrInvalidSeverity   = errors.New("evidence: invalid severity")
	ErrDuplicateReport   = errors.New("evidence: duplicate report id")
	ErrUnknownReport     = errors.New("evidence: unknown report")
	ErrReportClosed      = errors.New("evidence: report closed")
	ErrEvidenceInvalid   = errors.New("evidence: invalid evidence")
	ErrDuplicateEvidence = errors.New("evidence: duplicate evidence")
	ErrQuotaExceeded     = errors.New("evidence: quota exceeded")
	ErrEvidenceMissing   = errors.New("evidence: evidence not found")
	ErrChainBroken       = errors.New("evidence: chain broken")
)

var errNotImplemented = errors.New("evidence: not implemented")

// Severity is the triage level of a report.
type Severity int

const (
	SeverityLow    Severity = 1
	SeverityMedium Severity = 2
	SeverityHigh   Severity = 3
)

// ReportStatus is the lifecycle state of a report.
type ReportStatus string

const (
	StatusOpen   ReportStatus = "open"
	StatusClosed ReportStatus = "closed"
)

// Config is the evidence retention policy.
type Config struct {
	MaxEvidencePerReport int
	MaxTotalBytes        int64
	ReporterDailyQuota   int
	Retention            time.Duration
}

// Evidence is one uploaded artefact.
type Evidence struct {
	ID         string
	Kind       string
	SHA256     string
	Size       int64
	CapturedAt time.Time
}

// Report is one player report.
type Report struct {
	ID       string
	Reporter string
	Accused  string
	Reason   string
	Severity Severity
	Status   ReportStatus
	OpenedAt time.Time
	ClosedAt time.Time
}

// ChainEntry is one link of the evidence chain. Hash is the hex SHA-256 of
// PrevHash, ReportID, EvidenceID, SHA256 and Size joined by "|".
type ChainEntry struct {
	Seq        int
	ReportID   string
	EvidenceID string
	SHA256     string
	Size       int64
	PrevHash   string
	Hash       string
	At         time.Time
}

// EvidenceGuard stores reports, evidence and tamper-evident chains.
type EvidenceGuard struct {
	cfg Config
}

// NewEvidenceGuard returns an empty guard configured by cfg.
func NewEvidenceGuard(cfg Config) *EvidenceGuard {
	return &EvidenceGuard{cfg: cfg}
}

// NewEvidenceGuardFromSnapshot restores reports, evidence, chains and the
// reporter daily counters from a snapshot.
func NewEvidenceGuardFromSnapshot(cfg Config, data []byte) (*EvidenceGuard, error) {
	return nil, errNotImplemented
}

// OpenReport registers a new report. Severity must be 1, 2 or 3.
func (g *EvidenceGuard) OpenReport(id, reporter, accused, reason string, sev Severity, now time.Time) error {
	return errNotImplemented
}

// CloseReport closes a report so that no more evidence is accepted; closing a
// closed report is a no-op.
func (g *EvidenceGuard) CloseReport(id string, now time.Time) error {
	return errNotImplemented
}

// AddEvidence appends evidence to an open report and to its chain.
func (g *EvidenceGuard) AddEvidence(reportID string, e Evidence, now time.Time) (ChainEntry, error) {
	return ChainEntry{}, errNotImplemented
}

// GetEvidence returns one stored piece of evidence.
func (g *EvidenceGuard) GetEvidence(reportID, evidenceID string) (Evidence, error) {
	return Evidence{}, errNotImplemented
}

// Chain returns a copy of the evidence chain of a report in sequence order.
func (g *EvidenceGuard) Chain(reportID string) []ChainEntry {
	return nil
}

// VerifyChain recomputes every link and reports ErrChainBroken on mismatch.
func (g *EvidenceGuard) VerifyChain(reportID string) error {
	return errNotImplemented
}

// Report looks up a report.
func (g *EvidenceGuard) Report(id string) (Report, bool) {
	return Report{}, false
}

// ReporterDailyCount returns how many evidence items a reporter uploaded on
// the UTC day containing day.
func (g *EvidenceGuard) ReporterDailyCount(reporter string, day time.Time) int {
	return 0
}

// TotalBytes returns the stored evidence bytes of a report.
func (g *EvidenceGuard) TotalBytes(reportID string) int64 {
	return 0
}

// PurgeExpired drops reports closed longer than Retention ago and returns how
// many reports were purged. Open reports are never purged.
func (g *EvidenceGuard) PurgeExpired(now time.Time) int {
	return 0
}

// Snapshot serialises reports, evidence, chains and daily counters.
func (g *EvidenceGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
