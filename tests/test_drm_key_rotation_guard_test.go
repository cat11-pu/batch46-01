package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-h-04"
)

var drmT0 = time.Unix(1700000000, 0)

func drmConfig() taskpkg.Config {
	return taskpkg.Config{
		RotationInterval: 60 * time.Second,
		GracePeriod:      10 * time.Second,
		LicenseTTL:       120 * time.Second,
		MaxKeys:          6,
	}
}

func drmKey(id, tenant string, startSec int) taskpkg.Key {
	return taskpkg.Key{ID: id, TenantID: tenant, NotBefore: drmT0.Add(time.Duration(startSec) * time.Second)}
}

func newDrm(t *testing.T) *taskpkg.DrmKeyRotation {
	t.Helper()
	return taskpkg.NewDrmKeyRotation(drmConfig())
}

func mustKey(t *testing.T, d *taskpkg.DrmKeyRotation, k taskpkg.Key) {
	t.Helper()
	if err := d.AddKey(k); err != nil {
		t.Fatalf("AddKey(%s): %v", k.ID, err)
	}
}

// A key added without an end time gets one rotation interval.
func TestAddKeyFillsRotationInterval(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	got, err := d.KeyByID("key-a1")
	if err != nil {
		t.Fatalf("KeyByID: %v", err)
	}
	if !got.NotAfter.Equal(drmT0.Add(60 * time.Second)) {
		t.Fatalf("NotAfter = %v, want %v", got.NotAfter, drmT0.Add(60*time.Second))
	}
	st := d.Stats()
	if st.Keys != 1 || st.Tenants != 1 || st.Revoked != 0 || st.Licenses != 0 || st.Rotations != 0 {
		t.Fatalf("Stats = %+v, want one clean key", st)
	}
	if _, err := d.KeyByID("missing"); !errors.Is(err, taskpkg.ErrUnknownKey) {
		t.Fatalf("KeyByID(missing) err = %v, want ErrUnknownKey", err)
	}
}

// Keys without an id, a tenant or a usable window are refused.
func TestAddKeyValidation(t *testing.T) {
	d := newDrm(t)
	bad := []taskpkg.Key{
		{ID: "", TenantID: "tenant-a", NotBefore: drmT0},
		{ID: "key-a1", TenantID: "", NotBefore: drmT0},
		{ID: "key-a2", TenantID: "tenant-a"},
		{ID: "key-a3", TenantID: "tenant-a", NotBefore: drmT0, NotAfter: drmT0},
		{ID: "key-a4", TenantID: "tenant-a", NotBefore: drmT0, NotAfter: drmT0.Add(-time.Second)},
		{ID: "key-a5", TenantID: "tenant-a", NotBefore: drmT0, NotAfter: drmT0.Add(time.Minute), Revoked: true},
	}
	for _, k := range bad {
		if err := d.AddKey(k); !errors.Is(err, taskpkg.ErrInvalidKey) {
			t.Fatalf("AddKey(%q) err = %v, want ErrInvalidKey", k.ID, err)
		}
	}
	if st := d.Stats(); st.Keys != 0 {
		t.Fatalf("Stats.Keys = %d after rejected keys, want 0", st.Keys)
	}
}

// Key ids are global, while window overlap is checked per tenant.
func TestAddKeyDuplicateAndOverlap(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	if err := d.AddKey(taskpkg.Key{ID: "key-a1", TenantID: "tenant-b", NotBefore: drmT0.Add(120 * time.Second)}); !errors.Is(err, taskpkg.ErrDuplicateKey) {
		t.Fatalf("duplicate id err = %v, want ErrDuplicateKey", err)
	}
	if err := d.AddKey(drmKey("key-a2", "tenant-a", 30)); !errors.Is(err, taskpkg.ErrKeyOverlap) {
		t.Fatalf("overlapping window err = %v, want ErrKeyOverlap", err)
	}
	if err := d.AddKey(drmKey("key-b1", "tenant-b", 0)); err != nil {
		t.Fatalf("same window on another tenant: %v", err)
	}
	if st := d.Stats(); st.Keys != 2 || st.Tenants != 2 {
		t.Fatalf("Stats = %+v, want 2 keys on 2 tenants", st)
	}
}

// The configured key ceiling is a hard limit.
func TestAddKeyEnforcesMaxKeys(t *testing.T) {
	cfg := drmConfig()
	cfg.MaxKeys = 3
	d := taskpkg.NewDrmKeyRotation(cfg)
	for i := 0; i < 3; i++ {
		mustKey(t, d, drmKey(fmt.Sprintf("key-a%d", i+1), "tenant-a", i*60))
	}
	if err := d.AddKey(drmKey("key-a4", "tenant-a", 180)); !errors.Is(err, taskpkg.ErrTooManyKeys) {
		t.Fatalf("over-cap key err = %v, want ErrTooManyKeys", err)
	}
	if st := d.Stats(); st.Keys != 3 {
		t.Fatalf("Stats.Keys = %d, want 3", st.Keys)
	}
}

// The active key is selected by the half-open window [NotBefore, NotAfter).
func TestActiveKeyWindowBoundaries(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	mustKey(t, d, drmKey("key-a2", "tenant-a", 60))
	first, err := d.ActiveKey("tenant-a", drmT0)
	if err != nil || first.ID != "key-a1" {
		t.Fatalf("ActiveKey(start) = (%q, %v), want key-a1", first.ID, err)
	}
	edge, err := d.ActiveKey("tenant-a", drmT0.Add(60*time.Second-time.Nanosecond))
	if err != nil || edge.ID != "key-a1" {
		t.Fatalf("ActiveKey(boundary-1ns) = (%q, %v), want key-a1", edge.ID, err)
	}
	second, err := d.ActiveKey("tenant-a", drmT0.Add(60*time.Second))
	if err != nil || second.ID != "key-a2" {
		t.Fatalf("ActiveKey(boundary) = (%q, %v), want key-a2", second.ID, err)
	}
	if _, err := d.ActiveKey("tenant-a", drmT0.Add(120*time.Second)); !errors.Is(err, taskpkg.ErrNoActiveKey) {
		t.Fatalf("ActiveKey(past end) err = %v, want ErrNoActiveKey", err)
	}
	if _, err := d.ActiveKey("tenant-a", drmT0.Add(-time.Second)); !errors.Is(err, taskpkg.ErrNoActiveKey) {
		t.Fatalf("ActiveKey(before start) err = %v, want ErrNoActiveKey", err)
	}
	if _, err := d.ActiveKey("tenant-zzz", drmT0); !errors.Is(err, taskpkg.ErrNoActiveKey) {
		t.Fatalf("ActiveKey(unknown tenant) err = %v, want ErrNoActiveKey", err)
	}
}

// Rotation must land on the previous key's end; the first key is exempt.
func TestRotateRequiresAdjacentBoundary(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	mustKey(t, d, drmKey("key-a3", "tenant-a", 70))
	if _, err := d.Rotate("tenant-a", "key-a3", drmT0.Add(70*time.Second)); !errors.Is(err, taskpkg.ErrRotationBoundary) {
		t.Fatalf("rotation across a gap err = %v, want ErrRotationBoundary", err)
	}
	mustKey(t, d, taskpkg.Key{ID: "key-a2", TenantID: "tenant-a", NotBefore: drmT0.Add(60 * time.Second), NotAfter: drmT0.Add(70 * time.Second)})
	rotated, err := d.Rotate("tenant-a", "key-a2", drmT0.Add(60*time.Second))
	if err != nil {
		t.Fatalf("Rotate(key-a2): %v", err)
	}
	if rotated.ID != "key-a2" {
		t.Fatalf("Rotate returned %q, want key-a2", rotated.ID)
	}
	if st := d.Stats(); st.Rotations != 1 {
		t.Fatalf("Stats.Rotations = %d, want 1", st.Rotations)
	}
	if _, err := d.Rotate("tenant-a", "key-a2", drmT0.Add(60*time.Second)); err != nil {
		t.Fatalf("repeat Rotate: %v", err)
	}
	if st := d.Stats(); st.Rotations != 1 {
		t.Fatalf("Stats.Rotations = %d after repeat, want 1", st.Rotations)
	}
	if _, err := d.Rotate("tenant-a", "key-a1", drmT0); err != nil {
		t.Fatalf("first key activation: %v", err)
	}
	if st := d.Stats(); st.Rotations != 2 {
		t.Fatalf("Stats.Rotations = %d after first-key activation, want 2", st.Rotations)
	}
}

// Rotation rejects unknown, foreign, revoked and out-of-window keys.
func TestRotateRejectsBadTargets(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	mustKey(t, d, drmKey("key-a2", "tenant-a", 60))
	mustKey(t, d, drmKey("key-b1", "tenant-b", 0))
	if _, err := d.Rotate("tenant-a", "key-x", drmT0.Add(30*time.Second)); !errors.Is(err, taskpkg.ErrUnknownKey) {
		t.Fatalf("unknown key err = %v, want ErrUnknownKey", err)
	}
	if _, err := d.Rotate("tenant-a", "key-b1", drmT0.Add(30*time.Second)); !errors.Is(err, taskpkg.ErrCrossTenant) {
		t.Fatalf("foreign key err = %v, want ErrCrossTenant", err)
	}
	if _, err := d.Rotate("tenant-a", "key-a2", drmT0.Add(130*time.Second)); !errors.Is(err, taskpkg.ErrRotationBoundary) {
		t.Fatalf("out-of-window rotation err = %v, want ErrRotationBoundary", err)
	}
	if err := d.Revoke("key-a2"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if _, err := d.Rotate("tenant-a", "key-a2", drmT0.Add(60*time.Second)); !errors.Is(err, taskpkg.ErrKeyRevoked) {
		t.Fatalf("revoked rotation err = %v, want ErrKeyRevoked", err)
	}
	if st := d.Stats(); st.Rotations != 0 {
		t.Fatalf("Stats.Rotations = %d after failed rotations, want 0", st.Rotations)
	}
}

// Clients pre-fetch the key whose window starts after the current instant.
func TestNextKeyReturnsUpcomingKey(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	mustKey(t, d, drmKey("key-a2", "tenant-a", 60))
	mustKey(t, d, drmKey("key-a3", "tenant-a", 120))
	next, ok := d.NextKey("tenant-a", drmT0.Add(30*time.Second))
	if !ok || next.ID != "key-a2" {
		t.Fatalf("NextKey(30s) = (%q, %v), want key-a2", next.ID, ok)
	}
	next, ok = d.NextKey("tenant-a", drmT0.Add(60*time.Second))
	if !ok || next.ID != "key-a3" {
		t.Fatalf("NextKey(60s) = (%q, %v), want key-a3", next.ID, ok)
	}
	if _, ok := d.NextKey("tenant-a", drmT0.Add(180*time.Second)); ok {
		t.Fatalf("NextKey(past the last key) reported an upcoming key")
	}
	if _, ok := d.NextKey("tenant-zzz", drmT0); ok {
		t.Fatalf("NextKey(unknown tenant) reported an upcoming key")
	}
}

// A license is valid from now until the key's grace period ends at the latest.
func TestIssueLicenseCapsAtGraceEnd(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	lic, err := d.IssueLicense("tenant-a", "key-a1", "tv-01", drmT0.Add(30*time.Second))
	if err != nil {
		t.Fatalf("IssueLicense: %v", err)
	}
	if lic.KeyID != "key-a1" || lic.TenantID != "tenant-a" || lic.DeviceID != "tv-01" {
		t.Fatalf("license identity = %+v", lic)
	}
	if !lic.IssuedAt.Equal(drmT0.Add(30 * time.Second)) {
		t.Fatalf("IssuedAt = %v, want 30s", lic.IssuedAt)
	}
	if !lic.ExpiresAt.Equal(drmT0.Add(70 * time.Second)) {
		t.Fatalf("ExpiresAt = %v, want grace end 70s", lic.ExpiresAt)
	}
	if st := d.Stats(); st.Licenses != 1 {
		t.Fatalf("Stats.Licenses = %d, want 1", st.Licenses)
	}
}

// Licenses are tenant scoped, need a device and never outlive the key.
func TestIssueLicenseRejections(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	mustKey(t, d, drmKey("key-a2", "tenant-a", 60))
	mustKey(t, d, drmKey("key-b1", "tenant-b", 0))
	if _, err := d.IssueLicense("tenant-a", "key-missing", "tv-01", drmT0); !errors.Is(err, taskpkg.ErrUnknownKey) {
		t.Fatalf("unknown key err = %v, want ErrUnknownKey", err)
	}
	if _, err := d.IssueLicense("tenant-a", "key-b1", "tv-01", drmT0); !errors.Is(err, taskpkg.ErrCrossTenant) {
		t.Fatalf("foreign key err = %v, want ErrCrossTenant", err)
	}
	if _, err := d.IssueLicense("tenant-a", "key-a1", "", drmT0); !errors.Is(err, taskpkg.ErrInvalidDevice) {
		t.Fatalf("empty device err = %v, want ErrInvalidDevice", err)
	}
	if _, err := d.IssueLicense("tenant-a", "key-a2", "tv-01", drmT0.Add(50*time.Second)); !errors.Is(err, taskpkg.ErrKeyExpired) {
		t.Fatalf("not-yet-valid key err = %v, want ErrKeyExpired", err)
	}
	if _, err := d.IssueLicense("tenant-a", "key-a1", "tv-01", drmT0.Add(70*time.Second)); !errors.Is(err, taskpkg.ErrKeyExpired) {
		t.Fatalf("after grace err = %v, want ErrKeyExpired", err)
	}
	if st := d.Stats(); st.Licenses != 0 {
		t.Fatalf("Stats.Licenses = %d after refused licenses, want 0", st.Licenses)
	}
}

// Decryption stays possible through the grace period and stops at its end.
func TestDecryptAllowedGraceWindow(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	mustKey(t, d, drmKey("key-a2", "tenant-a", 60))
	if !d.DecryptAllowed("key-a1", drmT0) {
		t.Fatalf("key not allowed at its start")
	}
	if !d.DecryptAllowed("key-a1", drmT0.Add(69*time.Second+999*time.Millisecond)) {
		t.Fatalf("key not allowed just before grace end")
	}
	if d.DecryptAllowed("key-a1", drmT0.Add(70*time.Second)) {
		t.Fatalf("key allowed at exactly the grace end")
	}
	if d.DecryptAllowed("key-a1", drmT0.Add(-time.Nanosecond)) {
		t.Fatalf("key allowed before its window")
	}
	if d.DecryptAllowed("key-missing", drmT0) {
		t.Fatalf("unknown key reported as allowed")
	}
	if err := d.Revoke("key-a2"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if d.DecryptAllowed("key-a2", drmT0.Add(61*time.Second)) {
		t.Fatalf("revoked key allowed to decrypt")
	}
}

// A license issued during grace ends at the grace boundary, not at its full
// TTL.
func TestLicenseDuringGraceEndsAtGrace(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	if !d.DecryptAllowed("key-a1", drmT0.Add(65*time.Second)) {
		t.Fatalf("key should still decrypt during grace")
	}
	lic, err := d.IssueLicense("tenant-a", "key-a1", "stb-09", drmT0.Add(65*time.Second))
	if err != nil {
		t.Fatalf("IssueLicense during grace: %v", err)
	}
	if !lic.ExpiresAt.Equal(drmT0.Add(70 * time.Second)) {
		t.Fatalf("ExpiresAt = %v, want 70s", lic.ExpiresAt)
	}
	if lic.ExpiresAt.Sub(lic.IssuedAt) >= 120*time.Second {
		t.Fatalf("license kept its full TTL past the grace end")
	}
}

// Revocation disables both licensing and decryption and is idempotent.
func TestRevokeDisablesKey(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	if err := d.Revoke("key-a1"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	if err := d.Revoke("key-a1"); err != nil {
		t.Fatalf("second Revoke: %v", err)
	}
	if err := d.Revoke("key-missing"); !errors.Is(err, taskpkg.ErrUnknownKey) {
		t.Fatalf("Revoke(unknown) err = %v, want ErrUnknownKey", err)
	}
	if d.DecryptAllowed("key-a1", drmT0.Add(10*time.Second)) {
		t.Fatalf("revoked key still decrypts")
	}
	if _, err := d.IssueLicense("tenant-a", "key-a1", "tv-01", drmT0.Add(10*time.Second)); !errors.Is(err, taskpkg.ErrKeyRevoked) {
		t.Fatalf("license for revoked key err = %v, want ErrKeyRevoked", err)
	}
	got, err := d.KeyByID("key-a1")
	if err != nil || !got.Revoked {
		t.Fatalf("KeyByID revoked flag = (%+v, %v)", got, err)
	}
	if st := d.Stats(); st.Revoked != 1 {
		t.Fatalf("Stats.Revoked = %d, want 1", st.Revoked)
	}
}

// Licenses issued concurrently while a rotation runs are all counted and the
// new key is active afterwards.
func TestConcurrentLicensesDuringRotation(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	mustKey(t, d, drmKey("key-a2", "tenant-a", 60))
	var wg sync.WaitGroup
	errs := make(chan string, 40)
	start := make(chan struct{})
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			lic, err := d.IssueLicense("tenant-a", "key-a1", fmt.Sprintf("dev-%02d", i), drmT0.Add(30*time.Second))
			if err != nil {
				errs <- "license failed: " + err.Error()
				return
			}
			if lic.ExpiresAt.After(drmT0.Add(70 * time.Second)) {
				errs <- "license outlived the grace period"
			}
		}(i)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		if _, err := d.Rotate("tenant-a", "key-a2", drmT0.Add(60*time.Second)); err != nil {
			errs <- "rotation failed: " + err.Error()
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Fatalf("concurrent license: %s", msg)
	}
	if st := d.Stats(); st.Licenses != 32 {
		t.Fatalf("Stats.Licenses = %d, want 32", st.Licenses)
	}
	active, err := d.ActiveKey("tenant-a", drmT0.Add(61*time.Second))
	if err != nil || active.ID != "key-a2" {
		t.Fatalf("active key after rotation = (%q, %v), want key-a2", active.ID, err)
	}
}

// A restart keeps keys, revocation, counters and the rotation history.
func TestSnapshotRestoreKeepsKeyStore(t *testing.T) {
	d := newDrm(t)
	mustKey(t, d, drmKey("key-a1", "tenant-a", 0))
	mustKey(t, d, drmKey("key-a2", "tenant-a", 60))
	if _, err := d.Rotate("tenant-a", "key-a2", drmT0.Add(60*time.Second)); err != nil {
		t.Fatalf("Rotate: %v", err)
	}
	if _, err := d.IssueLicense("tenant-a", "key-a1", "tv-01", drmT0.Add(30*time.Second)); err != nil {
		t.Fatalf("IssueLicense: %v", err)
	}
	if err := d.Revoke("key-a1"); err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	snap, err := d.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewDrmKeyRotationFromSnapshot(drmConfig(), snap)
	if err != nil {
		t.Fatalf("NewDrmKeyRotationFromSnapshot: %v", err)
	}
	st := restored.Stats()
	if st.Keys != 2 || st.Tenants != 1 || st.Revoked != 1 || st.Licenses != 1 || st.Rotations != 1 {
		t.Fatalf("restored Stats = %+v", st)
	}
	active, err := restored.ActiveKey("tenant-a", drmT0.Add(61*time.Second))
	if err != nil || active.ID != "key-a2" {
		t.Fatalf("restored active key = (%q, %v), want key-a2", active.ID, err)
	}
	if restored.DecryptAllowed("key-a1", drmT0.Add(10*time.Second)) {
		t.Fatalf("restored store forgot the revocation")
	}
	if _, err := restored.IssueLicense("tenant-a", "key-a1", "tv-02", drmT0.Add(10*time.Second)); !errors.Is(err, taskpkg.ErrKeyRevoked) {
		t.Fatalf("license after restore err = %v, want ErrKeyRevoked", err)
	}
	if _, err := taskpkg.NewDrmKeyRotationFromSnapshot(drmConfig(), []byte("{broken")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot err = %v, want ErrCorruptSnapshot", err)
	}
}
