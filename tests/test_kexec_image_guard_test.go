package task

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	taskpkg "gsb/batch46-safety-e-10"
)

func kexecNow() time.Time {
	return time.Unix(1700000000, 0)
}

func kexecConfig() taskpkg.Config {
	return taskpkg.Config{
		Lockdown:      taskpkg.LockdownIntegrity,
		MaxImageBytes: 1 << 20,
		MaxKeys:       4,
	}
}

func digestOf(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func trustedKey(id, tenant string, now time.Time) taskpkg.TrustedKey {
	return taskpkg.TrustedKey{
		ID:       id,
		Alg:      "rsa2048",
		Tenant:   tenant,
		NotAfter: now.Add(24 * time.Hour),
	}
}

func validImage(name, tenant string) taskpkg.Image {
	content := []byte("bzImage-6.8.0-" + name)
	return taskpkg.Image{
		Name:           name,
		Tenant:         tenant,
		Content:        content,
		Digest:         digestOf(content),
		SignerID:       "k1",
		Signed:         true,
		SignatureValid: true,
		Syscall:        "kexec_file_load",
	}
}

func mustKey(t *testing.T, g *taskpkg.KexecGuard, k taskpkg.TrustedKey) uint64 {
	t.Helper()
	epoch, err := g.AddTrustedKey(k)
	if err != nil {
		t.Fatalf("AddTrustedKey(%s): %v", k.ID, err)
	}
	return epoch
}

// A signed image from a trusted tenant key must verify, load and be journalled.
func TestTrustedSignedImageLoads(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	epoch := mustKey(t, g, trustedKey("k1", "tenant-a", now))
	if epoch != 1 {
		t.Fatalf("first AddTrustedKey returned epoch %d, want 1", epoch)
	}
	img := validImage("vmlinuz-6.8", "tenant-a")
	if err := g.Verify(img, now); err != nil {
		t.Fatalf("Verify of a trusted signed image = %v, want nil", err)
	}
	rec, err := g.Load(img, now)
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if rec.Name != "vmlinuz-6.8" || rec.Tenant != "tenant-a" || rec.SignerID != "k1" {
		t.Fatalf("record = %+v, want the image identity", rec)
	}
	if rec.Bytes != uint64(len(img.Content)) {
		t.Fatalf("record bytes = %d, want %d", rec.Bytes, len(img.Content))
	}
	if rec.Epoch != epoch {
		t.Fatalf("record epoch = %d, want %d", rec.Epoch, epoch)
	}
	if got := len(g.Records()); got != 1 {
		t.Fatalf("len(Records()) = %d, want 1", got)
	}
	if s := g.Stats(); s.Keys != 1 || s.Loaded != 1 || s.Rejected != 0 {
		t.Fatalf("Stats() = %+v, want {1 1 0}", s)
	}
	if err := g.ResetJournal(); err != nil {
		t.Fatalf("ResetJournal = %v", err)
	}
	if got := len(g.Records()); got != 0 {
		t.Fatalf("len(Records()) = %d after ResetJournal, want 0", got)
	}
}

// An unsigned image must be refused and counted as a rejection.
func TestUnsignedImageRejected(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	img := validImage("unsigned", "tenant-a")
	img.Signed = false
	_, err := g.Load(img, now)
	if !errors.Is(err, taskpkg.ErrUnsignedImage) {
		t.Fatalf("Load of an unsigned image = %v, want ErrUnsignedImage", err)
	}
	if got := len(g.Records()); got != 0 {
		t.Fatalf("len(Records()) = %d after a rejected image, want 0", got)
	}
	if s := g.Stats(); s.Rejected != 1 || s.Loaded != 0 {
		t.Fatalf("Stats() = %+v, want Rejected 1 Loaded 0", s)
	}
}

// A broken detached signature must be reported as a signature failure.
func TestInvalidSignatureRejected(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	img := validImage("bad-sig", "tenant-a")
	img.SignatureValid = false
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrSignatureInvalid) {
		t.Fatalf("Verify with a broken signature = %v, want ErrSignatureInvalid", err)
	}
}

// A signer that is unknown, or only trusted for another tenant, must be
// refused.
func TestUnknownSignerRejected(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	img := validImage("foreign-signer", "tenant-a")
	img.SignerID = "k-missing"
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrKeyUnknown) {
		t.Fatalf("Verify with an unknown signer = %v, want ErrKeyUnknown", err)
	}
	img = validImage("cross-tenant", "tenant-b")
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrKeyUnknown) {
		t.Fatalf("Verify of a tenant-b image signed by a tenant-a key = %v, want ErrKeyUnknown", err)
	}
}

// A revoked key must be reported as revoked, not as unknown.
func TestRevokedKeyRejected(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	epoch := mustKey(t, g, trustedKey("k1", "tenant-a", now))
	if _, err := g.RevokeKey("k1", epoch); err != nil {
		t.Fatalf("RevokeKey = %v", err)
	}
	img := validImage("after-revoke", "tenant-a")
	err := g.Verify(img, now)
	if !errors.Is(err, taskpkg.ErrKeyRevoked) {
		t.Fatalf("Verify after revoking the signer = %v, want ErrKeyRevoked", err)
	}
	if errors.Is(err, taskpkg.ErrKeyUnknown) {
		t.Fatalf("revocation was reported as an unknown key")
	}
}

// A key whose NotAfter is in the past must be refused.
func TestExpiredKeyRejected(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	expired := trustedKey("k1", "tenant-a", now)
	expired.NotAfter = now.Add(-time.Minute)
	mustKey(t, g, expired)
	img := validImage("expired-key", "tenant-a")
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrKeyExpired) {
		t.Fatalf("Verify with an expired key = %v, want ErrKeyExpired", err)
	}
}

// Payload tampering must be caught by comparing the SHA-256 digest.
func TestDigestMismatchRejected(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	img := validImage("tampered", "tenant-a")
	img.Content = append([]byte("evil-"), img.Content...)
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrDigestMismatch) {
		t.Fatalf("Verify of a tampered payload = %v, want ErrDigestMismatch", err)
	}
}

// An image larger than Config.MaxImageBytes must be refused.
func TestOversizedImageRejected(t *testing.T) {
	now := kexecNow()
	cfg := kexecConfig()
	cfg.MaxImageBytes = 16
	g := taskpkg.NewKexecGuard(cfg)
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	img := validImage("too-big", "tenant-a")
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrImageTooLarge) {
		t.Fatalf("Verify of a %d byte image with a 16 byte limit = %v, want ErrImageTooLarge", len(img.Content), err)
	}
}

// Confidentiality lockdown must disable kexec entirely, and invalid levels must
// be rejected.
func TestConfidentialityLockdownDeniesKexec(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	if got := g.Lockdown(); got != taskpkg.LockdownIntegrity {
		t.Fatalf("Lockdown() = %d, want %d", got, taskpkg.LockdownIntegrity)
	}
	if err := g.SetLockdown(taskpkg.LockdownConfidentiality); err != nil {
		t.Fatalf("SetLockdown(Confidentiality) = %v", err)
	}
	if got := g.Lockdown(); got != taskpkg.LockdownConfidentiality {
		t.Fatalf("Lockdown() = %d, want %d", got, taskpkg.LockdownConfidentiality)
	}
	if err := g.SetLockdown(taskpkg.LockdownLevel(9)); !errors.Is(err, taskpkg.ErrInvalidLockdown) {
		t.Fatalf("SetLockdown(9) = %v, want ErrInvalidLockdown", err)
	}
	img := validImage("under-lockdown", "tenant-a")
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrLockdownDenied) {
		t.Fatalf("Verify under confidentiality lockdown = %v, want ErrLockdownDenied", err)
	}
}

// Integrity lockdown must force the verifying file load path.
func TestIntegrityLockdownRejectsKexecLoad(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	img := validImage("old-syscall", "tenant-a")
	img.Syscall = "kexec_load"
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrLockdownDenied) {
		t.Fatalf("Verify through kexec_load under integrity lockdown = %v, want ErrLockdownDenied", err)
	}
	img.Syscall = "kexec_file_load"
	if err := g.Verify(img, now); err != nil {
		t.Fatalf("Verify through kexec_file_load = %v, want nil", err)
	}
}

// An explicit syscall allowlist must be honoured outside lockdown.
func TestDisallowedSyscallRejected(t *testing.T) {
	now := kexecNow()
	cfg := kexecConfig()
	cfg.Lockdown = taskpkg.LockdownNone
	cfg.AllowedSyscalls = []string{"kexec_file_load"}
	g := taskpkg.NewKexecGuard(cfg)
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	img := validImage("syscall-gate", "tenant-a")
	img.Syscall = "kexec_load"
	if err := g.Verify(img, now); !errors.Is(err, taskpkg.ErrSyscallDenied) {
		t.Fatalf("Verify through a disallowed syscall = %v, want ErrSyscallDenied", err)
	}
	img.Syscall = "kexec_file_load"
	if err := g.Verify(img, now); err != nil {
		t.Fatalf("Verify through the allowed syscall = %v, want nil", err)
	}
}

// The key ring must enforce Config.MaxKeys and reject duplicate ids.
func TestKeyQuotaEnforced(t *testing.T) {
	now := kexecNow()
	cfg := kexecConfig()
	cfg.MaxKeys = 2
	g := taskpkg.NewKexecGuard(cfg)
	mustKey(t, g, trustedKey("k1", "tenant-a", now))
	mustKey(t, g, trustedKey("k2", "tenant-a", now))
	if _, err := g.AddTrustedKey(trustedKey("k1", "tenant-a", now)); !errors.Is(err, taskpkg.ErrDuplicateKey) {
		t.Fatalf("AddTrustedKey with a duplicate id = %v, want ErrDuplicateKey", err)
	}
	if _, err := g.AddTrustedKey(trustedKey("k3", "tenant-a", now)); !errors.Is(err, taskpkg.ErrKeyQuota) {
		t.Fatalf("AddTrustedKey with a full ring = %v, want ErrKeyQuota", err)
	}
	if got := g.KeyCount(); got != 2 {
		t.Fatalf("KeyCount() = %d, want 2", got)
	}
}

// Revocation must require the live epoch, and IsRevoked must reflect it only
// after a successful revocation.
func TestRevokeRequiresCurrentEpoch(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	epoch := mustKey(t, g, trustedKey("k1", "tenant-a", now))
	if _, err := g.RevokeKey("k1", epoch+3); !errors.Is(err, taskpkg.ErrStaleEpoch) {
		t.Fatalf("RevokeKey with epoch %d (current %d) = %v, want ErrStaleEpoch", epoch+3, epoch, err)
	}
	if g.IsRevoked("k1") {
		t.Fatalf("IsRevoked is true after a stale revocation")
	}
	next, err := g.RevokeKey("k1", epoch)
	if err != nil {
		t.Fatalf("RevokeKey with the current epoch = %v, want nil", err)
	}
	if next <= epoch {
		t.Fatalf("RevokeKey returned epoch %d, want something above %d", next, epoch)
	}
	if !g.IsRevoked("k1") {
		t.Fatalf("IsRevoked is false right after RevokeKey")
	}
}

// Revocations must survive a snapshot and restore.
func TestRevocationSurvivesRestart(t *testing.T) {
	now := kexecNow()
	g := taskpkg.NewKexecGuard(kexecConfig())
	epoch := mustKey(t, g, trustedKey("k1", "tenant-a", now))
	mustKey(t, g, trustedKey("k2", "tenant-a", now))
	if _, err := g.RevokeKey("k2", epoch); err != nil {
		t.Fatalf("RevokeKey(k2) = %v", err)
	}
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot = %v", err)
	}
	restored, err := taskpkg.NewKexecGuardFromSnapshot(kexecConfig(), snap)
	if err != nil {
		t.Fatalf("NewKexecGuardFromSnapshot = %v", err)
	}
	if got := restored.KeyCount(); got != 2 {
		t.Fatalf("restored KeyCount() = %d, want 2", got)
	}
	if !restored.IsRevoked("k2") {
		t.Fatalf("revocation of k2 was forgotten across restart")
	}
	img := validImage("restored", "tenant-a")
	img.SignerID = "k2"
	if err := restored.Verify(img, now); !errors.Is(err, taskpkg.ErrKeyRevoked) {
		t.Fatalf("Verify after restore with the revoked key = %v, want ErrKeyRevoked", err)
	}
	img.SignerID = "k1"
	if err := restored.Verify(img, now); err != nil {
		t.Fatalf("Verify after restore with the live key = %v, want nil", err)
	}
}

// Concurrent key additions must not overshoot the key ring.
func TestConcurrentAddKeysRespectsQuota(t *testing.T) {
	now := kexecNow()
	cfg := kexecConfig()
	cfg.MaxKeys = 6
	g := taskpkg.NewKexecGuard(cfg)
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := g.AddTrustedKey(trustedKey(fmt.Sprintf("k-%02d", i), "tenant-a", now)); err == nil {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := int(ok.Load()); got != 6 {
		t.Fatalf("%d concurrent AddTrustedKey calls succeeded, want exactly 6", got)
	}
	if got := g.KeyCount(); got != 6 {
		t.Fatalf("KeyCount() = %d, want 6", got)
	}
}
