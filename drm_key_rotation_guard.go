package task

import (
	"errors"
	"sync"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors reported by DrmKeyRotation.
var (
	ErrInvalidKey       = errors.New("drm: invalid key")
	ErrDuplicateKey     = errors.New("drm: duplicate key id")
	ErrUnknownKey       = errors.New("drm: unknown key id")
	ErrKeyOverlap       = errors.New("drm: key windows overlap")
	ErrTooManyKeys      = errors.New("drm: key count exceeds the limit")
	ErrNoActiveKey      = errors.New("drm: no active key")
	ErrRotationBoundary = errors.New("drm: rotation does not fall on a key boundary")
	ErrKeyRevoked       = errors.New("drm: key revoked")
	ErrKeyExpired       = errors.New("drm: key outside its usable window")
	ErrCrossTenant      = errors.New("drm: key belongs to another tenant")
	ErrInvalidDevice    = errors.New("drm: invalid device id")
	ErrCorruptSnapshot  = errors.New("drm: corrupt snapshot")
)

// Config is the rotation policy of one deployment.
type Config struct {
	// RotationInterval is the window of a key added without an explicit end.
	RotationInterval time.Duration
	// GracePeriod keeps an old key usable for decryption after its window.
	GracePeriod time.Duration
	// LicenseTTL is the base validity of an issued license.
	LicenseTTL time.Duration
	// MaxKeys is the hard cap on retained keys; zero means no limit.
	MaxKeys int
}

// Key is a content key valid for one tenant during one window.
type Key struct {
	ID        string
	TenantID  string
	NotBefore time.Time
	NotAfter  time.Time
	Revoked   bool
}

// License authorises one device to use one key.
type License struct {
	KeyID     string
	TenantID  string
	DeviceID  string
	IssuedAt  time.Time
	ExpiresAt time.Time
}

// Stats describes the key store.
type Stats struct {
	Keys      int
	Tenants   int
	Revoked   int
	Licenses  uint64
	Rotations uint64
}

// DrmKeyRotation manages tenant keys, rotation boundaries and licenses.
type DrmKeyRotation struct {
	mu  sync.Mutex
	cfg Config
}

// NewDrmKeyRotation returns an empty key store.
func NewDrmKeyRotation(cfg Config) *DrmKeyRotation {
	return &DrmKeyRotation{cfg: cfg}
}

// NewDrmKeyRotationFromSnapshot restores keys, revocation flags and counters.
func NewDrmKeyRotationFromSnapshot(cfg Config, data []byte) (*DrmKeyRotation, error) {
	return nil, errNotImplemented
}

// AddKey registers a key window. Windows of one tenant must not overlap; a
// zero NotAfter means NotBefore plus Config.RotationInterval.
func (d *DrmKeyRotation) AddKey(k Key) error {
	return errNotImplemented
}

// KeyByID returns one key.
func (d *DrmKeyRotation) KeyByID(keyID string) (Key, error) {
	return Key{}, errNotImplemented
}

// ActiveKey returns the key whose window contains at.
func (d *DrmKeyRotation) ActiveKey(tenantID string, at time.Time) (Key, error) {
	return Key{}, errNotImplemented
}

// Rotate activates keyID at at. The key window must contain at and the
// previous key of the tenant must end exactly at keyID's NotBefore; the first
// key of a tenant may be activated at its own start.
func (d *DrmKeyRotation) Rotate(tenantID, keyID string, at time.Time) (Key, error) {
	return Key{}, errNotImplemented
}

// NextKey returns the next scheduled key of the tenant strictly after at.
func (d *DrmKeyRotation) NextKey(tenantID string, at time.Time) (Key, bool) {
	return Key{}, false
}

// IssueLicense authorises deviceID to use keyID for tenantID at now. A
// license never outlives the key's grace period.
func (d *DrmKeyRotation) IssueLicense(tenantID, keyID, deviceID string, now time.Time) (License, error) {
	return License{}, errNotImplemented
}

// DecryptAllowed reports whether keyID may decrypt content at at, including
// the grace period.
func (d *DrmKeyRotation) DecryptAllowed(keyID string, at time.Time) bool {
	return false
}

// Revoke permanently disables a key. Revoking twice is not an error.
func (d *DrmKeyRotation) Revoke(keyID string) error {
	return errNotImplemented
}

// Keys returns the keys of one tenant ordered by NotBefore.
func (d *DrmKeyRotation) Keys(tenantID string) []Key {
	return nil
}

// Stats reports the key store state.
func (d *DrmKeyRotation) Stats() Stats {
	return Stats{}
}

// Snapshot serialises keys and counters so that a restarted license server
// keeps rejecting revoked keys and keeps its rotation history length.
func (d *DrmKeyRotation) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
