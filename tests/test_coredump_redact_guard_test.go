package task

import (
	"bytes"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	taskpkg "gsb/batch46-safety-e-07"
)

func dumpConfig() taskpkg.Config {
	return taskpkg.Config{
		PageSize:       4096,
		MaxRules:       6,
		MaxOutputBytes: 1 << 16,
	}
}

func mustRule(t *testing.T, g *taskpkg.CoreDumpGuard, r taskpkg.Rule) uint64 {
	t.Helper()
	epoch, err := g.RegisterRule(r)
	if err != nil {
		t.Fatalf("RegisterRule(%s) returned %v", r.ID, err)
	}
	return epoch
}

func repeat(b byte, n int) []byte {
	return bytes.Repeat([]byte{b}, n)
}

// ModeZero must zero exactly the covered bytes and leave the rest intact.
func TestZeroModeWipesCoveredBytesOnly(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	mustRule(t, g, taskpkg.Rule{
		ID:     "tls-key",
		Region: taskpkg.Region{Start: 0x1000, End: 0x1010},
		Mode:   taskpkg.ModeZero,
	})
	out, err := g.Redact(taskpkg.DumpRequest{
		Tenant: "svc-a",
		Segments: []taskpkg.Segment{{
			Region: taskpkg.Region{Start: 0x1000, End: 0x1020},
			Data:   repeat(0xAB, 32),
		}},
	})
	if err != nil {
		t.Fatalf("Redact = %v", err)
	}
	if len(out) != 32 {
		t.Fatalf("Redact returned %d bytes, want 32", len(out))
	}
	for i := 0; i < 16; i++ {
		if out[i] != 0 {
			t.Fatalf("byte %d inside the protected region = 0x%02x, want 0x00", i, out[i])
		}
	}
	for i := 16; i < 32; i++ {
		if out[i] != 0xAB {
			t.Fatalf("byte %d outside the protected region = 0x%02x, want 0xAB", i, out[i])
		}
	}
	if got := g.RedactedBytes(); got != 16 {
		t.Fatalf("RedactedBytes() = %d, want 16", got)
	}
	if s := g.Stats(); s.Rules != 1 || s.Runs != 1 || s.Redacted != 16 {
		t.Fatalf("Stats() = %+v, want {1 1 16}", s)
	}
}

// ModeFill must write Rule.Fill into the covered bytes.
func TestFillModeWritesConfiguredByte(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	mustRule(t, g, taskpkg.Rule{
		ID:     "jwt-secret",
		Region: taskpkg.Region{Start: 0x2000, End: 0x2008},
		Mode:   taskpkg.ModeFill,
		Fill:   0xCC,
	})
	out, err := g.Redact(taskpkg.DumpRequest{
		Tenant: "svc-b",
		Segments: []taskpkg.Segment{{
			Region: taskpkg.Region{Start: 0x2000, End: 0x2010},
			Data:   repeat(0x12, 16),
		}},
	})
	if err != nil {
		t.Fatalf("Redact = %v", err)
	}
	for i := 0; i < 8; i++ {
		if out[i] != 0xCC {
			t.Fatalf("byte %d inside the filled region = 0x%02x, want 0xCC", i, out[i])
		}
	}
	for i := 8; i < 16; i++ {
		if out[i] != 0x12 {
			t.Fatalf("byte %d outside the filled region = 0x%02x, want 0x12", i, out[i])
		}
	}
}

// Without a matching rule the payload must be emitted verbatim.
func TestUncoveredPayloadPreserved(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	in := repeat(0x5A, 24)
	out, err := g.Redact(taskpkg.DumpRequest{
		Tenant: "svc-a",
		Segments: []taskpkg.Segment{{
			Region: taskpkg.Region{Start: 0x3000, End: 0x3018},
			Data:   in,
		}},
	})
	if err != nil {
		t.Fatalf("Redact = %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("Redact changed an unprotected payload:\n got %x\nwant %x", out, in)
	}
	if got := g.RedactedBytes(); got != 0 {
		t.Fatalf("RedactedBytes() = %d with no rule applied, want 0", got)
	}
}

// Plan must emit segments ordered by ascending address.
func TestPlanSortsSegmentsByAddress(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	plan, err := g.Plan(taskpkg.DumpRequest{
		Tenant: "svc-a",
		Segments: []taskpkg.Segment{
			{Region: taskpkg.Region{Start: 0x4000, End: 0x4004}, Data: repeat(0x44, 4)},
			{Region: taskpkg.Region{Start: 0x3000, End: 0x3004}, Data: repeat(0x33, 4)},
		},
	})
	if err != nil {
		t.Fatalf("Plan = %v", err)
	}
	if len(plan) != 2 {
		t.Fatalf("Plan returned %d segments, want 2", len(plan))
	}
	if plan[0].Region.Start != 0x3000 || plan[1].Region.Start != 0x4000 {
		t.Fatalf("plan starts = [0x%x 0x%x], want [0x3000 0x4000]", plan[0].Region.Start, plan[1].Region.Start)
	}
	if plan[0].Data[0] != 0x33 || plan[1].Data[0] != 0x44 {
		t.Fatalf("plan payloads were not sorted with their regions")
	}
}

// Adjacent segments must be merged into one contiguous output segment.
func TestPlanMergesAdjacentSegments(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	plan, err := g.Plan(taskpkg.DumpRequest{
		Tenant: "svc-a",
		Segments: []taskpkg.Segment{
			{Region: taskpkg.Region{Start: 0x5004, End: 0x5008}, Data: repeat(0x52, 4)},
			{Region: taskpkg.Region{Start: 0x5000, End: 0x5004}, Data: repeat(0x51, 4)},
		},
	})
	if err != nil {
		t.Fatalf("Plan = %v", err)
	}
	if len(plan) != 1 {
		t.Fatalf("Plan returned %d segments for adjacent input, want 1", len(plan))
	}
	if plan[0].Region.Start != 0x5000 || plan[0].Region.End != 0x5008 {
		t.Fatalf("merged region = [0x%x,0x%x), want [0x5000,0x5008)", plan[0].Region.Start, plan[0].Region.End)
	}
	if len(plan[0].Data) != 8 {
		t.Fatalf("merged payload has %d bytes, want 8", len(plan[0].Data))
	}
	if plan[0].Data[0] != 0x51 || plan[0].Data[7] != 0x52 {
		t.Fatalf("merged payload order is wrong: %x", plan[0].Data)
	}
}

// Overlapping input segments describe the same memory twice and must be
// refused.
func TestPlanRejectsOverlappingSegments(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	_, err := g.Plan(taskpkg.DumpRequest{
		Tenant: "svc-a",
		Segments: []taskpkg.Segment{
			{Region: taskpkg.Region{Start: 0x6000, End: 0x6008}, Data: repeat(0x60, 8)},
			{Region: taskpkg.Region{Start: 0x6004, End: 0x600C}, Data: repeat(0x61, 8)},
		},
	})
	if !errors.Is(err, taskpkg.ErrRegionOverlap) {
		t.Fatalf("Plan with overlapping segments = %v, want ErrRegionOverlap", err)
	}
}

// Empty, inverted and length-mismatched regions must all be refused.
func TestInvalidRegionRejected(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	_, err := g.RegisterRule(taskpkg.Rule{
		ID:     "empty",
		Region: taskpkg.Region{Start: 0x7000, End: 0x7000},
		Mode:   taskpkg.ModeZero,
	})
	if !errors.Is(err, taskpkg.ErrInvalidRegion) {
		t.Fatalf("RegisterRule with an empty region = %v, want ErrInvalidRegion", err)
	}
	if _, err := g.Plan(taskpkg.DumpRequest{
		Tenant:   "svc-a",
		Segments: []taskpkg.Segment{{Region: taskpkg.Region{Start: 0x10, End: 0x08}}},
	}); !errors.Is(err, taskpkg.ErrInvalidRegion) {
		t.Fatalf("Plan with an inverted region = %v, want ErrInvalidRegion", err)
	}
	if _, err := g.Plan(taskpkg.DumpRequest{
		Tenant:   "svc-a",
		Segments: []taskpkg.Segment{{Region: taskpkg.Region{Start: 0x20, End: 0x30}, Data: repeat(0x20, 4)}},
	}); !errors.Is(err, taskpkg.ErrInvalidRegion) {
		t.Fatalf("Plan with a payload shorter than its region = %v, want ErrInvalidRegion", err)
	}
}

// Two rules may not share an id.
func TestDuplicateRuleIDRejected(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	mustRule(t, g, taskpkg.Rule{
		ID:     "dup",
		Region: taskpkg.Region{Start: 0x100, End: 0x110},
		Mode:   taskpkg.ModeZero,
	})
	_, err := g.RegisterRule(taskpkg.Rule{
		ID:     "dup",
		Region: taskpkg.Region{Start: 0x200, End: 0x210},
		Mode:   taskpkg.ModeZero,
	})
	if !errors.Is(err, taskpkg.ErrDuplicateRule) {
		t.Fatalf("second RegisterRule with the same id = %v, want ErrDuplicateRule", err)
	}
	if got := g.RuleCount(); got != 1 {
		t.Fatalf("RuleCount() = %d, want 1", got)
	}
}

// Config.MaxRules must bound the rule table.
func TestRuleLimitEnforced(t *testing.T) {
	cfg := dumpConfig()
	cfg.MaxRules = 2
	g := taskpkg.NewCoreDumpGuard(cfg)
	mustRule(t, g, taskpkg.Rule{ID: "r1", Region: taskpkg.Region{Start: 0x100, End: 0x110}, Mode: taskpkg.ModeZero})
	mustRule(t, g, taskpkg.Rule{ID: "r2", Region: taskpkg.Region{Start: 0x200, End: 0x210}, Mode: taskpkg.ModeZero})
	_, err := g.RegisterRule(taskpkg.Rule{
		ID:     "r3",
		Region: taskpkg.Region{Start: 0x300, End: 0x310},
		Mode:   taskpkg.ModeZero,
	})
	if !errors.Is(err, taskpkg.ErrRuleLimit) {
		t.Fatalf("third RegisterRule with MaxRules=2 = %v, want ErrRuleLimit", err)
	}
	if got := g.RuleCount(); got != 2 {
		t.Fatalf("RuleCount() = %d, want 2", got)
	}
}

// Removing a rule requires the epoch that is currently live.
func TestRemoveRuleRequiresCurrentEpoch(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	epoch := mustRule(t, g, taskpkg.Rule{
		ID:     "page-tables",
		Region: taskpkg.Region{Start: 0x1000, End: 0x2000},
		Mode:   taskpkg.ModeZero,
	})
	if err := g.RemoveRule("page-tables", epoch+5); !errors.Is(err, taskpkg.ErrStaleEpoch) {
		t.Fatalf("RemoveRule with epoch %d (current %d) = %v, want ErrStaleEpoch", epoch+5, epoch, err)
	}
	if got := g.RuleCount(); got != 1 {
		t.Fatalf("RuleCount() = %d after a stale removal, want 1", got)
	}
	if err := g.RemoveRule("page-tables", epoch); err != nil {
		t.Fatalf("RemoveRule with the current epoch = %v, want nil", err)
	}
	if got := g.RuleCount(); got != 0 {
		t.Fatalf("RuleCount() = %d after removal, want 0", got)
	}
	r, err := g.Rule("page-tables")
	if !errors.Is(err, taskpkg.ErrUnknownRule) {
		t.Fatalf("Rule on a removed rule = %v (rule %+v), want ErrUnknownRule", err, r)
	}
}

// A dump larger than Config.MaxOutputBytes must be refused before any payload
// is produced.
func TestOutputBudgetExceeded(t *testing.T) {
	cfg := dumpConfig()
	cfg.MaxOutputBytes = 8
	g := taskpkg.NewCoreDumpGuard(cfg)
	out, err := g.Redact(taskpkg.DumpRequest{
		Tenant: "svc-a",
		Segments: []taskpkg.Segment{{
			Region: taskpkg.Region{Start: 0x8000, End: 0x8010},
			Data:   repeat(0x33, 16),
		}},
	})
	if !errors.Is(err, taskpkg.ErrBudgetExceeded) {
		t.Fatalf("Redact over the output budget = %v, want ErrBudgetExceeded", err)
	}
	if out != nil {
		t.Fatalf("Redact returned %d bytes along with the budget error", len(out))
	}
}

// A tenant-scoped rule must only touch dumps of that tenant, and Protected must
// answer per tenant as well.
func TestTenantScopedRuleIgnoredForOtherTenant(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	mustRule(t, g, taskpkg.Rule{
		ID:     "tenant-b-secret",
		Region: taskpkg.Region{Start: 0x8000, End: 0x8010},
		Mode:   taskpkg.ModeZero,
		Tenant: "tenant-b",
	})
	in := repeat(0x77, 16)
	out, err := g.Redact(taskpkg.DumpRequest{
		Tenant:   "svc-a",
		Segments: []taskpkg.Segment{{Region: taskpkg.Region{Start: 0x8000, End: 0x8010}, Data: in}},
	})
	if err != nil {
		t.Fatalf("Redact = %v", err)
	}
	if !bytes.Equal(out, in) {
		t.Fatalf("a tenant-b rule redacted a svc-a dump: got %x", out)
	}
	if !g.Protected(0x8005, "tenant-b") {
		t.Fatalf("Protected(0x8005, tenant-b) = false, want true")
	}
	if g.Protected(0x8005, "svc-a") {
		t.Fatalf("Protected(0x8005, svc-a) = true, want false")
	}
}

// On overlapping rules the higher priority wins, and the loser's bytes outside
// the winner's range are still redacted by the loser.
func TestHigherPriorityRuleWinsOnOverlap(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	mustRule(t, g, taskpkg.Rule{
		ID:       "zero-wide",
		Region:   taskpkg.Region{Start: 0x9000, End: 0x9010},
		Mode:     taskpkg.ModeZero,
		Priority: 1,
	})
	mustRule(t, g, taskpkg.Rule{
		ID:       "fill-narrow",
		Region:   taskpkg.Region{Start: 0x9008, End: 0x9018},
		Mode:     taskpkg.ModeFill,
		Fill:     0xFF,
		Priority: 5,
	})
	out, err := g.Redact(taskpkg.DumpRequest{
		Tenant: "svc-a",
		Segments: []taskpkg.Segment{{
			Region: taskpkg.Region{Start: 0x9000, End: 0x9020},
			Data:   repeat(0x11, 32),
		}},
	})
	if err != nil {
		t.Fatalf("Redact = %v", err)
	}
	for i := 0; i < 8; i++ {
		if out[i] != 0x00 {
			t.Fatalf("byte %d = 0x%02x, want 0x00 from the zero rule", i, out[i])
		}
	}
	for i := 8; i < 24; i++ {
		if out[i] != 0xFF {
			t.Fatalf("byte %d = 0x%02x, want 0xFF from the higher priority fill rule", i, out[i])
		}
	}
	for i := 24; i < 32; i++ {
		if out[i] != 0x11 {
			t.Fatalf("byte %d = 0x%02x, want the original 0x11", i, out[i])
		}
	}
}

// Rules registered before a restart must still redact afterwards.
func TestSnapshotRestoresRules(t *testing.T) {
	g := taskpkg.NewCoreDumpGuard(dumpConfig())
	mustRule(t, g, taskpkg.Rule{
		ID:     "creds",
		Region: taskpkg.Region{Start: 0xA000, End: 0xA008},
		Mode:   taskpkg.ModeZero,
	})
	mustRule(t, g, taskpkg.Rule{
		ID:     "stack-canary",
		Region: taskpkg.Region{Start: 0xB000, End: 0xB004},
		Mode:   taskpkg.ModeFill,
		Fill:   0xEE,
	})
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot = %v", err)
	}
	restored, err := taskpkg.NewCoreDumpGuardFromSnapshot(dumpConfig(), snap)
	if err != nil {
		t.Fatalf("NewCoreDumpGuardFromSnapshot = %v", err)
	}
	if got := restored.RuleCount(); got != 2 {
		t.Fatalf("restored RuleCount() = %d, want 2", got)
	}
	if _, err := restored.Rule("stack-canary"); err != nil {
		t.Fatalf("restored Rule(stack-canary) = %v", err)
	}
	out, err := restored.Redact(taskpkg.DumpRequest{
		Tenant: "svc-a",
		Segments: []taskpkg.Segment{
			{Region: taskpkg.Region{Start: 0xA000, End: 0xA008}, Data: repeat(0x01, 8)},
			{Region: taskpkg.Region{Start: 0xB000, End: 0xB004}, Data: repeat(0x02, 4)},
		},
	})
	if err != nil {
		t.Fatalf("restored Redact = %v", err)
	}
	if !bytes.Equal(out, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0xEE, 0xEE, 0xEE, 0xEE}) {
		t.Fatalf("restored Redact = %x, want 8 zeroes followed by four 0xEE", out)
	}
}

// Concurrent registrations must not overshoot Config.MaxRules.
func TestConcurrentRegisterRespectsLimit(t *testing.T) {
	cfg := dumpConfig()
	cfg.MaxRules = 5
	g := taskpkg.NewCoreDumpGuard(cfg)
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 24; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := uint64(0x10000 + i*0x10)
			id := []byte{'r', byte('a' + i)}
			if _, err := g.RegisterRule(taskpkg.Rule{
				ID:     string(id),
				Region: taskpkg.Region{Start: start, End: start + 0x10},
				Mode:   taskpkg.ModeZero,
			}); err == nil {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := int(ok.Load()); got != 5 {
		t.Fatalf("%d concurrent registrations succeeded, want exactly 5", got)
	}
	if got := g.RuleCount(); got != 5 {
		t.Fatalf("RuleCount() = %d, want 5", got)
	}
}
