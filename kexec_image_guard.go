package task

import (
	"errors"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the guard. A denial caused by a revoked key must
// never be reported as an unknown signer.
var (
	ErrUnsignedImage    = errors.New("kexec: image is not signed")
	ErrSignatureInvalid = errors.New("kexec: image signature invalid")
	ErrKeyUnknown       = errors.New("kexec: signing key is not trusted")
	ErrKeyRevoked       = errors.New("kexec: signing key revoked")
	ErrKeyExpired       = errors.New("kexec: signing key expired")
	ErrDuplicateKey     = errors.New("kexec: trusted key already present")
	ErrDigestMismatch   = errors.New("kexec: image digest mismatch")
	ErrImageTooLarge    = errors.New("kexec: image exceeds the size limit")
	ErrLockdownDenied   = errors.New("kexec: denied by lockdown level")
	ErrInvalidLockdown  = errors.New("kexec: invalid lockdown level")
	ErrSyscallDenied    = errors.New("kexec: load syscall not allowed")
	ErrKeyQuota         = errors.New("kexec: trusted key quota exceeded")
	ErrStaleEpoch       = errors.New("kexec: stale revocation epoch")
)

// LockdownLevel mirrors the kernel lockdown levels that affect kexec.
type LockdownLevel uint8

const (
	// LockdownNone keeps kexec available for signed and unsigned images.
	LockdownNone LockdownLevel = 0
	// LockdownIntegrity only allows the in-kernel verifying file load path.
	LockdownIntegrity LockdownLevel = 1
	// LockdownConfidentiality disables kexec entirely.
	LockdownConfidentiality LockdownLevel = 2
)

// TrustedKey is one key allowed to sign boot images for one tenant.
type TrustedKey struct {
	ID       string
	Alg      string
	Tenant   string
	NotAfter time.Time
}

// Image is one kernel or initrd image submitted for loading.
type Image struct {
	Name   string
	Tenant string
	// Content is the image payload; its SHA-256 must match Digest.
	Content []byte
	// Digest is the lower-case hex SHA-256 claimed by the loader.
	Digest string
	SignerID string
	// Signed reports whether a detached signature was attached at all.
	Signed bool
	// SignatureValid is the outcome of checking that signature.
	SignatureValid bool
	// Syscall is "kexec_load" or "kexec_file_load".
	Syscall string
}

// Config holds the policy and resource limits of a guard.
type Config struct {
	Lockdown LockdownLevel
	// MaxImageBytes caps len(Image.Content).
	MaxImageBytes uint64
	// MaxKeys caps the number of trusted keys.
	MaxKeys int
	// AllowedSyscalls restricts the load syscalls; empty means both.
	AllowedSyscalls []string
}

// LoadRecord is the journal entry written when an image is admitted.
type LoadRecord struct {
	Name     string
	Tenant   string
	SignerID string
	Bytes    uint64
	Epoch    uint64
	At       time.Time
}

// Stats is a point-in-time view of the guard.
type Stats struct {
	Keys     int
	Loaded   int
	Rejected int
}

// KexecGuard verifies kexec images against a tenant-scoped key set.
type KexecGuard struct {
	cfg Config
}

// NewKexecGuard returns a guard with the policy described by cfg.
func NewKexecGuard(cfg Config) *KexecGuard {
	return &KexecGuard{cfg: cfg}
}

// NewKexecGuardFromSnapshot restores trusted keys, revocations and the load
// journal from a snapshot produced by Snapshot.
func NewKexecGuardFromSnapshot(cfg Config, data []byte) (*KexecGuard, error) {
	return nil, errNotImplemented
}

// AddTrustedKey installs a trusted key and returns the new revocation epoch.
// Duplicate ids and a full key ring must be rejected without changing anything.
func (g *KexecGuard) AddTrustedKey(k TrustedKey) (uint64, error) {
	return 0, errNotImplemented
}

// RevokeKey revokes a trusted key. epoch must equal the current revocation
// epoch, otherwise the call is stale and changes nothing.
func (g *KexecGuard) RevokeKey(id string, epoch uint64) (uint64, error) {
	return 0, errNotImplemented
}

// IsRevoked reports whether a key id has been revoked.
func (g *KexecGuard) IsRevoked(id string) bool {
	return false
}

// KeyCount returns the number of installed trusted keys.
func (g *KexecGuard) KeyCount() int {
	return 0
}

// Epoch returns the current revocation epoch.
func (g *KexecGuard) Epoch() uint64 {
	return 0
}

// SetLockdown switches the lockdown level; levels above 2 are invalid.
func (g *KexecGuard) SetLockdown(l LockdownLevel) error {
	return errNotImplemented
}

// Lockdown returns the current lockdown level.
func (g *KexecGuard) Lockdown() LockdownLevel {
	return 0
}

// Verify decides whether an image may be loaded without touching the journal.
func (g *KexecGuard) Verify(img Image, now time.Time) error {
	return errNotImplemented
}

// Load verifies an image and, on success, appends a journal entry and returns
// it.
func (g *KexecGuard) Load(img Image, now time.Time) (LoadRecord, error) {
	return LoadRecord{}, errNotImplemented
}

// Records returns the load journal in append order.
func (g *KexecGuard) Records() []LoadRecord {
	return nil
}

// Stats returns key, load and rejection counters.
func (g *KexecGuard) Stats() Stats {
	return Stats{}
}

// Snapshot serialises keys, revocations and the journal for a restart.
func (g *KexecGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// ResetJournal clears the load journal without touching keys or revocations.
func (g *KexecGuard) ResetJournal() error {
	return errNotImplemented
}
