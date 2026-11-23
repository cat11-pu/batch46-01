package task

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	taskpkg "gsb/batch46-safety-e-06"
)

func msrConfig() taskpkg.Config {
	return taskpkg.Config{
		MaxRules: 4,
		MaxBatch: 4,
		Reserved: []taskpkg.MsrRange{{Lo: 0x40000000, Hi: 0x400000FF}},
		ReadOnly: []taskpkg.MsrRange{
			{Lo: 0x10, Hi: 0x10},
			{Lo: 0xC0000080, Hi: 0xC0000084},
		},
		VCPUs: 2,
	}
}

func mustRule(t *testing.T, g *taskpkg.KvmMsrGuard, r taskpkg.MsrRule) uint64 {
	t.Helper()
	rev, err := g.AddRule(r)
	if err != nil {
		t.Fatalf("AddRule(%+v) returned %v", r, err)
	}
	return rev
}

// An MSR inside an allowlisted range must be accepted for the allowlisted mode.
func TestAllowlistedReadAccepted(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	mustRule(t, g, taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0xC0000081, Hi: 0xC0000084},
		Access: taskpkg.AccessRead,
		VCPU:   -1,
	})
	if err := g.Check(0, 0xC0000083, taskpkg.AccessRead); err != nil {
		t.Fatalf("Check(0, 0xC0000083, read) = %v, want nil", err)
	}
	if got := g.RuleCount(); got != 1 {
		t.Fatalf("RuleCount() = %d, want 1", got)
	}
	if got := g.Revision(); got != 1 {
		t.Fatalf("Revision() = %d, want 1 after one AddRule", got)
	}
}

// With no rule covering an MSR the default decision must be denial.
func TestUnlistedMsrDeniedByDefault(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	err := g.Check(0, 0x1A0, taskpkg.AccessRead)
	if !errors.Is(err, taskpkg.ErrMsrDenied) {
		t.Fatalf("Check on an unlisted MSR = %v, want ErrMsrDenied", err)
	}
	c := g.Counters(0)
	if c.Accepted != 0 || c.Denied != 1 {
		t.Fatalf("counters = %+v, want Accepted 0 Denied 1", c)
	}
}

// A rule overlapping a reserved range must not be installed.
func TestReservedRangeCannotBeAllowlisted(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	_, err := g.AddRule(taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0x400000F0, Hi: 0x40000110},
		Access: taskpkg.AccessWrite,
		VCPU:   -1,
	})
	if !errors.Is(err, taskpkg.ErrRangeOverlap) {
		t.Fatalf("AddRule over a reserved range = %v, want ErrRangeOverlap", err)
	}
	if got := g.RuleCount(); got != 0 {
		t.Fatalf("RuleCount() = %d after a rejected rule, want 0", got)
	}
}

// Lo greater than Hi must be rejected as a malformed range.
func TestInvertedRangeRejected(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	_, err := g.AddRule(taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0x20, Hi: 0x10},
		Access: taskpkg.AccessRead,
		VCPU:   -1,
	})
	if !errors.Is(err, taskpkg.ErrInvalidRange) {
		t.Fatalf("AddRule with Lo>Hi = %v, want ErrInvalidRange", err)
	}
}

// A write to an architecturally read-only MSR must fail with the dedicated
// readonly error even when a rule allowlists writes for that range.
func TestReadOnlyMsrRejectsWrite(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	mustRule(t, g, taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0xC0000080, Hi: 0xC0000084},
		Access: taskpkg.AccessWrite,
		VCPU:   -1,
	})
	err := g.Check(0, 0xC0000082, taskpkg.AccessWrite)
	if !errors.Is(err, taskpkg.ErrMsrReadOnly) {
		t.Fatalf("write to a read-only MSR = %v, want ErrMsrReadOnly", err)
	}
}

// A rule that allowlists reads must not allow writes. IA32_APIC_BASE is not in
// the read-only set, so the only reason to reject the write is the mode
// mismatch.
func TestAccessModeMismatchDenied(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	mustRule(t, g, taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0x1B, Hi: 0x1B},
		Access: taskpkg.AccessRead,
		VCPU:   -1,
	})
	err := g.Check(0, 0x1B, taskpkg.AccessWrite)
	if !errors.Is(err, taskpkg.ErrMsrDenied) {
		t.Fatalf("write on a read-only mode rule = %v, want ErrMsrDenied", err)
	}
}

// The number of live rules must never exceed Config.MaxRules.
func TestRuleQuotaEnforced(t *testing.T) {
	cfg := msrConfig()
	cfg.MaxRules = 2
	g := taskpkg.NewKvmMsrGuard(cfg)
	mustRule(t, g, taskpkg.MsrRule{Range: taskpkg.MsrRange{Lo: 0xC0000081, Hi: 0xC0000081}, Access: taskpkg.AccessRead, VCPU: -1})
	mustRule(t, g, taskpkg.MsrRule{Range: taskpkg.MsrRange{Lo: 0xC0000082, Hi: 0xC0000082}, Access: taskpkg.AccessRead, VCPU: -1})
	_, err := g.AddRule(taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0xC0000084, Hi: 0xC0000084},
		Access: taskpkg.AccessRead,
		VCPU:   -1,
	})
	if !errors.Is(err, taskpkg.ErrRuleQuota) {
		t.Fatalf("third AddRule with MaxRules=2 = %v, want ErrRuleQuota", err)
	}
	if got := g.RuleCount(); got != 2 {
		t.Fatalf("RuleCount() = %d, want 2", got)
	}
}

// Concurrent AddRule calls must not overshoot the quota.
func TestConcurrentRuleAddRespectsQuota(t *testing.T) {
	cfg := msrConfig()
	cfg.MaxRules = 8
	g := taskpkg.NewKvmMsrGuard(cfg)
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 40; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			lo := uint32(0xC0000100 + uint32(i))
			if _, err := g.AddRule(taskpkg.MsrRule{
				Range:  taskpkg.MsrRange{Lo: lo, Hi: lo},
				Access: taskpkg.AccessRead,
				VCPU:   -1,
			}); err == nil {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := int(ok.Load()); got != 8 {
		t.Fatalf("%d concurrent AddRule calls succeeded, want exactly 8", got)
	}
	if got := g.RuleCount(); got != 8 {
		t.Fatalf("RuleCount() = %d, want 8", got)
	}
}

// A batch containing one denied index must reject the whole batch and leave
// the accepted counter untouched.
func TestBatchAllOrNothing(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	mustRule(t, g, taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0xC0000082, Hi: 0xC0000082},
		Access: taskpkg.AccessRead,
		VCPU:   -1,
	})
	err := g.CheckBatch(0, []uint32{0xC0000082, 0x1A0}, taskpkg.AccessRead)
	if !errors.Is(err, taskpkg.ErrMsrDenied) {
		t.Fatalf("mixed batch = %v, want ErrMsrDenied", err)
	}
	if c := g.Counters(0); c.Accepted != 0 {
		t.Fatalf("accepted counter = %d after a rejected batch, want 0", c.Accepted)
	}
}

// A batch longer than Config.MaxBatch must be refused before any decision.
func TestOversizedBatchRejected(t *testing.T) {
	cfg := msrConfig()
	cfg.MaxBatch = 2
	g := taskpkg.NewKvmMsrGuard(cfg)
	err := g.CheckBatch(0, []uint32{0xC0000081, 0xC0000082, 0xC0000084}, taskpkg.AccessRead)
	if !errors.Is(err, taskpkg.ErrBatchTooLarge) {
		t.Fatalf("oversized batch = %v, want ErrBatchTooLarge", err)
	}
	if c := g.Counters(0); c.Accepted != 0 || c.Denied != 0 {
		t.Fatalf("counters moved on an oversized batch: %+v", c)
	}
}

// RemoveRule must refuse a revision that is not the current one.
func TestRemoveRuleRequiresCurrentRevision(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	rule := taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0xC0000081, Hi: 0xC0000081},
		Access: taskpkg.AccessRead,
		VCPU:   -1,
	}
	rev := mustRule(t, g, rule)
	if err := g.RemoveRule(rule, rev+1); !errors.Is(err, taskpkg.ErrStaleRevision) {
		t.Fatalf("RemoveRule with revision %d (current %d) = %v, want ErrStaleRevision", rev+1, rev, err)
	}
	if got := g.RuleCount(); got != 1 {
		t.Fatalf("RuleCount() = %d after a stale removal, want 1", got)
	}
	if err := g.RemoveRule(rule, rev); err != nil {
		t.Fatalf("RemoveRule with the current revision = %v, want nil", err)
	}
	if got := g.RuleCount(); got != 0 {
		t.Fatalf("RuleCount() = %d after removal, want 0", got)
	}
}

// A runtime revocation must outrank an allowlist rule.
func TestRevokeBeatsAllowlist(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	mustRule(t, g, taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0xC0000082, Hi: 0xC0000082},
		Access: taskpkg.AccessRead,
		VCPU:   -1,
	})
	if err := g.Check(0, 0xC0000082, taskpkg.AccessRead); err != nil {
		t.Fatalf("baseline allowlisted read = %v, want nil", err)
	}
	if _, err := g.Revoke(0xC0000082, taskpkg.AccessRead); err != nil {
		t.Fatalf("Revoke = %v", err)
	}
	if !g.IsRevoked(0xC0000082, taskpkg.AccessRead) {
		t.Fatalf("IsRevoked = false right after Revoke")
	}
	if err := g.Check(0, 0xC0000082, taskpkg.AccessRead); !errors.Is(err, taskpkg.ErrMsrDenied) {
		t.Fatalf("revoked MSR check = %v, want ErrMsrDenied", err)
	}
}

// Revocations must survive a snapshot and restore.
func TestRevocationSurvivesRestart(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	mustRule(t, g, taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0x48, Hi: 0x48},
		Access: taskpkg.AccessWrite,
		VCPU:   -1,
	})
	if _, err := g.Revoke(0x48, taskpkg.AccessWrite); err != nil {
		t.Fatalf("Revoke = %v", err)
	}
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot = %v", err)
	}
	restored, err := taskpkg.NewKvmMsrGuardFromSnapshot(msrConfig(), snap)
	if err != nil {
		t.Fatalf("NewKvmMsrGuardFromSnapshot = %v", err)
	}
	if !restored.IsRevoked(0x48, taskpkg.AccessWrite) {
		t.Fatalf("revocation of MSR 0x48 was forgotten across restart")
	}
	if err := restored.Check(0, 0x48, taskpkg.AccessWrite); !errors.Is(err, taskpkg.ErrMsrDenied) {
		t.Fatalf("restored guard check = %v, want ErrMsrDenied", err)
	}
}

// A rule pinned to one vCPU must not leak to its siblings, and a vCPU outside
// Config.VCPUs must be reported distinctly.
func TestPerVCPURuleIsolation(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	mustRule(t, g, taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0xC0000081, Hi: 0xC0000081},
		Access: taskpkg.AccessRead,
		VCPU:   1,
	})
	if err := g.Check(1, 0xC0000081, taskpkg.AccessRead); err != nil {
		t.Fatalf("check on the pinned vCPU 1 = %v, want nil", err)
	}
	if err := g.Check(0, 0xC0000081, taskpkg.AccessRead); !errors.Is(err, taskpkg.ErrMsrDenied) {
		t.Fatalf("check on vCPU 0 = %v, want ErrMsrDenied", err)
	}
	if err := g.Check(7, 0xC0000081, taskpkg.AccessRead); !errors.Is(err, taskpkg.ErrUnknownVCPU) {
		t.Fatalf("check on vCPU 7 = %v, want ErrUnknownVCPU", err)
	}
}

// Accepted and denied decisions must be counted separately per vCPU and be
// resettable.
func TestCountersTrackDecisions(t *testing.T) {
	g := taskpkg.NewKvmMsrGuard(msrConfig())
	mustRule(t, g, taskpkg.MsrRule{
		Range:  taskpkg.MsrRange{Lo: 0xC0000081, Hi: 0xC0000081},
		Access: taskpkg.AccessRead,
		VCPU:   -1,
	})
	if err := g.Check(0, 0xC0000081, taskpkg.AccessRead); err != nil {
		t.Fatalf("first accepted check = %v", err)
	}
	if err := g.Check(0, 0xC0000081, taskpkg.AccessRead); err != nil {
		t.Fatalf("second accepted check = %v", err)
	}
	if err := g.Check(0, 0x1A0, taskpkg.AccessRead); !errors.Is(err, taskpkg.ErrMsrDenied) {
		t.Fatalf("denied check = %v, want ErrMsrDenied", err)
	}
	c := g.Counters(0)
	if c.VCPU != 0 || c.Accepted != 2 || c.Denied != 1 {
		t.Fatalf("counters = %+v, want {0 2 1}", c)
	}
	if err := g.ResetCounters(0); err != nil {
		t.Fatalf("ResetCounters = %v", err)
	}
	if c := g.Counters(0); c.Accepted != 0 || c.Denied != 0 {
		t.Fatalf("counters after reset = %+v, want zeroes", c)
	}
}
