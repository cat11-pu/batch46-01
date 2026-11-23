package task

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	taskpkg "gsb/batch46-safety-e-09"
)

func ebpfConfig() taskpkg.Config {
	return taskpkg.Config{
		MaxInsns:         32,
		MaxMapsPerTenant: 4,
		MaxKeySize:       32,
		MaxValueSize:     256,
		MaxEntries:       1024,
		AllowedHelpers:   []int{1, 2, 5},
		GPLOnlyHelpers:   []int{5},
	}
}

func gplProgram(name string) taskpkg.Program {
	return taskpkg.Program{
		Name:    name,
		Type:    "xdp",
		License: "GPL",
		Insns: []taskpkg.Insn{
			{Op: taskpkg.OpMov, Dst: 0, Imm: 0},
			{Op: taskpkg.OpCall, Imm: 1},
			{Op: taskpkg.OpExit},
		},
	}
}

func smallMap(name string) taskpkg.MapSpec {
	return taskpkg.MapSpec{Name: name, KeySize: 4, ValueSize: 8, MaxEntries: 64}
}

// A program longer than Config.MaxInsns must not reach the jump walker.
func TestInstructionLimitEnforced(t *testing.T) {
	cfg := ebpfConfig()
	cfg.MaxInsns = 4
	g := taskpkg.NewEbpfGuard(cfg)
	p := gplProgram("too-long")
	p.Insns = []taskpkg.Insn{
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpAdd},
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpExit},
	}
	if err := g.Verify(p); !errors.Is(err, taskpkg.ErrInstrLimit) {
		t.Fatalf("Verify on a 6 instruction program with MaxInsns=4 = %v, want ErrInstrLimit", err)
	}
}

// A forward jump landing past the last instruction must be rejected.
func TestJumpOutOfBoundsRejected(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	p := gplProgram("bad-jump")
	p.Insns = []taskpkg.Insn{
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpJump, Off: 50},
		{Op: taskpkg.OpExit},
	}
	if err := g.Verify(p); !errors.Is(err, taskpkg.ErrBadJump) {
		t.Fatalf("Verify with a jump past the program end = %v, want ErrBadJump", err)
	}
}

// A backward jump is an unbounded loop and must be refused.
func TestBackwardJumpRejected(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	p := gplProgram("looping")
	p.Insns = []taskpkg.Insn{
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpJump, Off: -2},
		{Op: taskpkg.OpExit},
	}
	if err := g.Verify(p); !errors.Is(err, taskpkg.ErrLoop) {
		t.Fatalf("Verify with a backward jump = %v, want ErrLoop", err)
	}
}

// The last instruction must be exit.
func TestProgramWithoutExitRejected(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	p := gplProgram("falls-through")
	p.Insns = []taskpkg.Insn{
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpMov},
	}
	if err := g.Verify(p); !errors.Is(err, taskpkg.ErrNoExit) {
		t.Fatalf("Verify on a program ending in mov = %v, want ErrNoExit", err)
	}
}

// A GPL-only helper must require the GPL license string.
func TestGPLOnlyHelperRequiresGPL(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	p := gplProgram("proprietary")
	p.License = "Proprietary"
	p.Insns = []taskpkg.Insn{
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpCall, Imm: 5},
		{Op: taskpkg.OpExit},
	}
	if err := g.Verify(p); !errors.Is(err, taskpkg.ErrLicense) {
		t.Fatalf("Verify calling a GPL-only helper from a proprietary program = %v, want ErrLicense", err)
	}
	if err := g.Verify(gplProgram("gpl-ok")); err != nil {
		t.Fatalf("Verify of an allowlisted helper under GPL = %v, want nil", err)
	}
}

// A helper outside the allowlist must be refused until the allowlist grows.
func TestUnknownHelperRejected(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	p := gplProgram("helper-99")
	p.License = "Proprietary"
	p.Insns = []taskpkg.Insn{
		{Op: taskpkg.OpMov},
		{Op: taskpkg.OpCall, Imm: 99},
		{Op: taskpkg.OpExit},
	}
	if err := g.Verify(p); !errors.Is(err, taskpkg.ErrHelperDenied) {
		t.Fatalf("Verify calling helper 99 = %v, want ErrHelperDenied", err)
	}
	if err := g.SetAllowedHelpers([]int{1, 2, 5, 99}); err != nil {
		t.Fatalf("SetAllowedHelpers = %v", err)
	}
	if err := g.Verify(p); err != nil {
		t.Fatalf("Verify after allowlisting helper 99 = %v, want nil", err)
	}
}

// A map whose key is wider than Config.MaxKeySize must be refused.
func TestMapKeySizeLimit(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	_, err := g.Load("tenant-a", gplProgram("map-key"), []taskpkg.MapSpec{{
		Name:       "wide-key",
		KeySize:    64,
		ValueSize:  8,
		MaxEntries: 16,
	}})
	if !errors.Is(err, taskpkg.ErrMapTooLarge) {
		t.Fatalf("Load with a 64 byte key = %v, want ErrMapTooLarge", err)
	}
}

// A load that would push a tenant over MaxMapsPerTenant must be refused as a
// whole.
func TestMapQuotaPerTenant(t *testing.T) {
	cfg := ebpfConfig()
	cfg.MaxMapsPerTenant = 2
	g := taskpkg.NewEbpfGuard(cfg)
	_, err := g.Load("tenant-a", gplProgram("three-maps"), []taskpkg.MapSpec{
		smallMap("m1"),
		smallMap("m2"),
		smallMap("m3"),
	})
	if !errors.Is(err, taskpkg.ErrMapQuota) {
		t.Fatalf("Load with three maps and MaxMapsPerTenant=2 = %v, want ErrMapQuota", err)
	}
	if got := g.ProgramCount("tenant-a"); got != 0 {
		t.Fatalf("ProgramCount = %d after a refused load, want 0", got)
	}
}

// Two programs may not share a name inside one tenant.
func TestDuplicateProgramNameRejected(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	if _, err := g.Load("tenant-a", gplProgram("firewall"), []taskpkg.MapSpec{smallMap("m1")}); err != nil {
		t.Fatalf("first Load = %v", err)
	}
	_, err := g.Load("tenant-a", gplProgram("firewall"), nil)
	if !errors.Is(err, taskpkg.ErrProgramExists) {
		t.Fatalf("second Load of the same name = %v, want ErrProgramExists", err)
	}
	if _, err := g.Load("tenant-b", gplProgram("firewall"), nil); err != nil {
		t.Fatalf("Load under another tenant = %v, want nil", err)
	}
}

// A failed map check must leave programs, maps and the generation untouched.
func TestLoadIsAtomicOnMapFailure(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	_, err := g.Load("tenant-a", gplProgram("atomic"), []taskpkg.MapSpec{
		smallMap("good"),
		{Name: "huge-value", KeySize: 4, ValueSize: 4096, MaxEntries: 16},
	})
	if !errors.Is(err, taskpkg.ErrMapTooLarge) {
		t.Fatalf("Load with one oversized map = %v, want ErrMapTooLarge", err)
	}
	if got := g.ProgramCount("tenant-a"); got != 0 {
		t.Fatalf("ProgramCount = %d after a failed load, want 0", got)
	}
	if got := g.MapCount("tenant-a"); got != 0 {
		t.Fatalf("MapCount = %d after a failed load, want 0", got)
	}
	if got := g.Generation(); got != 0 {
		t.Fatalf("Generation() = %d after a failed load, want 0", got)
	}
}

// A successful load must return a handle bound to the new generation.
func TestLoadReturnsGenerationAndCounts(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	h, err := g.Load("tenant-a", gplProgram("ok"), []taskpkg.MapSpec{smallMap("m1"), smallMap("m2")})
	if err != nil {
		t.Fatalf("Load = %v", err)
	}
	if h.ID == 0 || h.Name != "ok" || h.Tenant != "tenant-a" {
		t.Fatalf("handle = %+v, want a named handle for tenant-a", h)
	}
	if h.Gen != 1 || h.Insns != 3 {
		t.Fatalf("handle = %+v, want Gen 1 with 3 instructions", h)
	}
	if got := g.Generation(); got != 1 {
		t.Fatalf("Generation() = %d, want 1", got)
	}
	if got := g.ProgramCount("tenant-a"); got != 1 {
		t.Fatalf("ProgramCount = %d, want 1", got)
	}
	if got := g.MapCount("tenant-a"); got != 2 {
		t.Fatalf("MapCount = %d, want 2", got)
	}
	if s := g.Stats(); s.Programs != 1 || s.Maps != 2 || s.Insns != 3 {
		t.Fatalf("Stats() = %+v, want {1 2 3}", s)
	}
	again, err := g.Lookup("tenant-a", "ok")
	if err != nil {
		t.Fatalf("Lookup = %v", err)
	}
	if again.ID != h.ID {
		t.Fatalf("Lookup returned id %d, want %d", again.ID, h.ID)
	}
}

// Unloading with the generation observed before a later load must be refused.
func TestUnloadStaleGenerationRejected(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	first, err := g.Load("tenant-a", gplProgram("p1"), nil)
	if err != nil {
		t.Fatalf("first Load = %v", err)
	}
	second, err := g.Load("tenant-a", gplProgram("p2"), nil)
	if err != nil {
		t.Fatalf("second Load = %v", err)
	}
	if err := g.Unload(first.ID, first.Gen); !errors.Is(err, taskpkg.ErrStaleGen) {
		t.Fatalf("Unload(first.ID, %d) with generation %d = %v, want ErrStaleGen", first.Gen, g.Generation(), err)
	}
	if err := g.Unload(second.ID, second.Gen); err != nil {
		t.Fatalf("Unload of the current program = %v, want nil", err)
	}
	if got := g.ProgramCount("tenant-a"); got != 1 {
		t.Fatalf("ProgramCount = %d after unload, want 1", got)
	}
}

// Unloading an id that was never loaded must be reported distinctly.
func TestUnloadUnknownProgram(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	if err := g.Unload(9999, 1); !errors.Is(err, taskpkg.ErrUnknownProgram) {
		t.Fatalf("Unload of an unknown id = %v, want ErrUnknownProgram", err)
	}
}

// Programs loaded before a restart must still be resolvable afterwards.
func TestSnapshotRestoresPrograms(t *testing.T) {
	g := taskpkg.NewEbpfGuard(ebpfConfig())
	if _, err := g.Load("tenant-a", gplProgram("allow-list"), []taskpkg.MapSpec{smallMap("m1")}); err != nil {
		t.Fatalf("first Load = %v", err)
	}
	if _, err := g.Load("tenant-a", gplProgram("rate-limit"), []taskpkg.MapSpec{smallMap("m2"), smallMap("m3")}); err != nil {
		t.Fatalf("second Load = %v", err)
	}
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot = %v", err)
	}
	restored, err := taskpkg.NewEbpfGuardFromSnapshot(ebpfConfig(), snap)
	if err != nil {
		t.Fatalf("NewEbpfGuardFromSnapshot = %v", err)
	}
	if got := restored.ProgramCount("tenant-a"); got != 2 {
		t.Fatalf("restored ProgramCount = %d, want 2", got)
	}
	if got := restored.MapCount("tenant-a"); got != 3 {
		t.Fatalf("restored MapCount = %d, want 3", got)
	}
	h, err := restored.Lookup("tenant-a", "rate-limit")
	if err != nil {
		t.Fatalf("restored Lookup = %v", err)
	}
	if h.Name != "rate-limit" {
		t.Fatalf("restored Lookup returned %q, want rate-limit", h.Name)
	}
}

// Concurrent loads must not overshoot the per-tenant map quota.
func TestConcurrentLoadRespectsMapQuota(t *testing.T) {
	cfg := ebpfConfig()
	cfg.MaxMapsPerTenant = 4
	g := taskpkg.NewEbpfGuard(cfg)
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			name := fmt.Sprintf("prog-%02d", i)
			mname := fmt.Sprintf("map-%02d", i)
			if _, err := g.Load("tenant-a", gplProgram(name), []taskpkg.MapSpec{smallMap(mname)}); err == nil {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := int(ok.Load()); got != 4 {
		t.Fatalf("%d concurrent loads succeeded, want exactly 4", got)
	}
	if got := g.MapCount("tenant-a"); got != 4 {
		t.Fatalf("MapCount = %d, want 4", got)
	}
	if got := g.ProgramCount("tenant-a"); got != 4 {
		t.Fatalf("ProgramCount = %d, want 4", got)
	}
}
