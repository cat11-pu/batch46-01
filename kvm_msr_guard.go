package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the guard. Denials caused by policy must be
// distinguishable from malformed requests and from stale control-plane state.
var (
	ErrMsrDenied     = errors.New("kvm: msr access denied")
	ErrMsrReadOnly   = errors.New("kvm: msr is read-only")
	ErrRangeOverlap  = errors.New("kvm: range overlaps a reserved region")
	ErrInvalidRange  = errors.New("kvm: invalid msr range")
	ErrRuleQuota     = errors.New("kvm: allowlist rule quota exceeded")
	ErrBatchTooLarge = errors.New("kvm: batch exceeds the size limit")
	ErrStaleRevision = errors.New("kvm: stale policy revision")
	ErrUnknownVCPU   = errors.New("kvm: unknown vcpu")
)

// MsrAccess is the access mode a guest requests for an MSR.
type MsrAccess uint8

const (
	// AccessRead is a guest read through KVM_GET_MSRS.
	AccessRead MsrAccess = 1
	// AccessWrite is a guest write through KVM_SET_MSRS.
	AccessWrite MsrAccess = 2
)

// MsrRange is an inclusive range of MSR indices.
type MsrRange struct {
	Lo uint32
	Hi uint32
}

// Contains reports whether msr falls inside the range.
func (r MsrRange) Contains(msr uint32) bool {
	return msr >= r.Lo && msr <= r.Hi
}

// MsrRule allowlists a range for one access mode on one or all vCPUs.
type MsrRule struct {
	Range MsrRange
	// Access is the only mode this range is allowlisted for.
	Access MsrAccess
	// VCPU pins the rule to one vCPU id; -1 means every vCPU.
	VCPU int
}

// Config holds VM-wide MSR policy limits.
type Config struct {
	// MaxRules caps the number of live allowlist rules.
	MaxRules int
	// MaxBatch caps how many MSR indices CheckBatch accepts at once.
	MaxBatch int
	// Reserved ranges can never be allowlisted, whoever asks.
	Reserved []MsrRange
	// ReadOnly ranges reject every write even when allowlisted for write.
	ReadOnly []MsrRange
	// VCPUs is the number of vCPUs attached to this VM.
	VCPUs int
}

// VCPUCounters reports how one vCPU's MSR traffic was decided.
type VCPUCounters struct {
	VCPU     int
	Accepted int
	Denied   int
}

// KvmMsrGuard enforces the MSR allowlist of one VM.
type KvmMsrGuard struct {
	cfg Config
}

// NewKvmMsrGuard returns a guard with an empty allowlist and a default-deny
// policy configured by cfg.
func NewKvmMsrGuard(cfg Config) *KvmMsrGuard {
	return &KvmMsrGuard{cfg: cfg}
}

// NewKvmMsrGuardFromSnapshot restores rules, runtime revocations, policy
// revision and per-vCPU counters from a snapshot produced by Snapshot.
func NewKvmMsrGuardFromSnapshot(cfg Config, data []byte) (*KvmMsrGuard, error) {
	return nil, errNotImplemented
}

// AddRule appends an allowlist rule and returns the new policy revision.
// Inverted ranges, ranges overlapping a reserved range, duplicate ranges and
// quota violations must be rejected without changing anything.
func (g *KvmMsrGuard) AddRule(rule MsrRule) (uint64, error) {
	return 0, errNotImplemented
}

// RemoveRule deletes the rule described by rule. revision must equal the
// current policy revision, otherwise the call is stale and changes nothing.
func (g *KvmMsrGuard) RemoveRule(rule MsrRule, revision uint64) error {
	return errNotImplemented
}

// Check decides a single MSR access for one vCPU and updates that vCPU's
// counters exactly once.
func (g *KvmMsrGuard) Check(vcpu int, msr uint32, access MsrAccess) error {
	return errNotImplemented
}

// CheckBatch decides a batch of MSR indices atomically: if any index is
// denied, none of the counters advance.
func (g *KvmMsrGuard) CheckBatch(vcpu int, msrs []uint32, access MsrAccess) error {
	return errNotImplemented
}

// Revoke denies msr for access mode until the VM is destroyed. Revocation
// outranks every allowlist rule and returns the new policy revision.
func (g *KvmMsrGuard) Revoke(msr uint32, access MsrAccess) (uint64, error) {
	return 0, errNotImplemented
}

// IsRevoked reports whether a runtime revocation covers msr for access.
func (g *KvmMsrGuard) IsRevoked(msr uint32, access MsrAccess) bool {
	return false
}

// RuleCount returns the number of live allowlist rules.
func (g *KvmMsrGuard) RuleCount() int {
	return 0
}

// Revision returns the current policy revision.
func (g *KvmMsrGuard) Revision() uint64 {
	return 0
}

// Counters returns the decision counters accumulated for one vCPU.
func (g *KvmMsrGuard) Counters(vcpu int) VCPUCounters {
	return VCPUCounters{}
}

// ResetCounters clears the counters of one vCPU.
func (g *KvmMsrGuard) ResetCounters(vcpu int) error {
	return errNotImplemented
}

// Snapshot serialises rules, revocations, the revision and the counters so a
// restarted guard keeps rejecting what it already rejected.
func (g *KvmMsrGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
