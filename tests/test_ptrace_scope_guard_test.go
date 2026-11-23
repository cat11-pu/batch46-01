package task

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	taskpkg "gsb/batch46-safety-e-08"
)

var attachNow = time.Unix(1700000000, 0)

func ptraceConfig(mode taskpkg.ScopeMode) taskpkg.Config {
	return taskpkg.Config{
		Mode:                 mode,
		MaxAttachesPerWindow: 16,
		Window:               time.Minute,
		MaxTasks:             8,
	}
}

func mustTask(t *testing.T, g *taskpkg.PtraceGuard, tk taskpkg.Task) {
	t.Helper()
	if err := g.RegisterTask(tk); err != nil {
		t.Fatalf("RegisterTask(%d): %v", tk.PID, err)
	}
}

func attachReq(tracer, tracee int, uid uint32, cap bool) taskpkg.AttachRequest {
	return taskpkg.AttachRequest{
		Tracer: tracer,
		Tracee: tracee,
		Creds:  taskpkg.Credentials{UID: uid, HasCapSysPtrace: cap},
		Now:    attachNow,
	}
}

// Scope 0 must accept an attach from a same-uid tracer and hand back a usable
// grant.
func TestClassicSameUidAllowed(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeClassic))
	mustTask(t, g, taskpkg.Task{PID: 600, PPID: 1, UID: 1000})
	grant, err := g.RequestAttach(attachReq(500, 600, 1000, false))
	if err != nil {
		t.Fatalf("RequestAttach = %v, want a grant", err)
	}
	if grant.Nonce == "" {
		t.Fatalf("grant has an empty nonce")
	}
	if grant.Tracer != 500 || grant.Tracee != 600 {
		t.Fatalf("grant = %+v, want tracer 500 tracee 600", grant)
	}
	if got := g.ActiveGrants(600); got != 1 {
		t.Fatalf("ActiveGrants(600) = %d, want 1", got)
	}
	if s := g.Stats(); s.Granted != 1 || s.Tasks != 1 {
		t.Fatalf("Stats() = %+v, want Granted 1 Tasks 1", s)
	}
}

// A different uid must be refused in the classic scope.
func TestClassicUidMismatchDenied(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeClassic))
	mustTask(t, g, taskpkg.Task{PID: 601, PPID: 1, UID: 1000})
	_, err := g.RequestAttach(attachReq(501, 601, 4242, false))
	if !errors.Is(err, taskpkg.ErrUidMismatch) {
		t.Fatalf("attach across uids = %v, want ErrUidMismatch", err)
	}
	if got := g.ActiveGrants(601); got != 0 {
		t.Fatalf("ActiveGrants(601) = %d after a denied attach, want 0", got)
	}
}

// Scope 1 must refuse a tracer that is not in the tracee's parent chain.
func TestRestrictedRequiresAncestor(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeRestricted))
	mustTask(t, g, taskpkg.Task{PID: 400, PPID: 1, UID: 1000})
	_, err := g.RequestAttach(attachReq(500, 400, 1000, false))
	if !errors.Is(err, taskpkg.ErrNotAncestor) {
		t.Fatalf("attach from an unrelated tracer = %v, want ErrNotAncestor", err)
	}
}

// Scope 1 must accept a grandparent tracer.
func TestRestrictedAncestorChainAllowed(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeRestricted))
	mustTask(t, g, taskpkg.Task{PID: 100, PPID: 1, UID: 1000})
	mustTask(t, g, taskpkg.Task{PID: 200, PPID: 100, UID: 1000})
	mustTask(t, g, taskpkg.Task{PID: 300, PPID: 200, UID: 1000})
	grant, err := g.RequestAttach(attachReq(100, 300, 1000, false))
	if err != nil {
		t.Fatalf("grandparent attach = %v, want a grant", err)
	}
	if grant.Nonce == "" {
		t.Fatalf("grandparent attach returned an empty nonce")
	}
	if got := g.ActiveGrants(300); got != 1 {
		t.Fatalf("ActiveGrants(300) = %d, want 1", got)
	}
}

// IsAncestor must walk the whole parent chain and must reject unknown pids.
func TestIsAncestorWalksChain(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeRestricted))
	mustTask(t, g, taskpkg.Task{PID: 100, PPID: 1, UID: 1000})
	mustTask(t, g, taskpkg.Task{PID: 200, PPID: 100, UID: 1000})
	mustTask(t, g, taskpkg.Task{PID: 300, PPID: 200, UID: 1000})
	ok, err := g.IsAncestor(100, 300)
	if err != nil {
		t.Fatalf("IsAncestor(100,300) = %v", err)
	}
	if !ok {
		t.Fatalf("IsAncestor(100,300) = false, want true")
	}
	ok, err = g.IsAncestor(300, 100)
	if err != nil {
		t.Fatalf("IsAncestor(300,100) = %v", err)
	}
	if ok {
		t.Fatalf("IsAncestor(300,100) = true, want false")
	}
	if _, err := g.IsAncestor(999, 300); !errors.Is(err, taskpkg.ErrUnknownTask) {
		t.Fatalf("IsAncestor with an unknown tracer = %v, want ErrUnknownTask", err)
	}
}

// Scope 2 must require the capability, and SetMode must reject out-of-range
// levels.
func TestAdminOnlyRequiresCapability(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeClassic))
	if err := g.SetMode(taskpkg.ScopeAdminOnly); err != nil {
		t.Fatalf("SetMode(AdminOnly) = %v", err)
	}
	if got := g.Mode(); got != taskpkg.ScopeAdminOnly {
		t.Fatalf("Mode() = %d, want %d", got, taskpkg.ScopeAdminOnly)
	}
	if err := g.SetMode(taskpkg.ScopeMode(9)); !errors.Is(err, taskpkg.ErrInvalidMode) {
		t.Fatalf("SetMode(9) = %v, want ErrInvalidMode", err)
	}
	mustTask(t, g, taskpkg.Task{PID: 1100, PPID: 1, UID: 1000})
	_, err := g.RequestAttach(attachReq(1200, 1100, 1000, false))
	if !errors.Is(err, taskpkg.ErrScopeDenied) {
		t.Fatalf("unprivileged attach in scope 2 = %v, want ErrScopeDenied", err)
	}
}

// The capability must override the uid check.
func TestCapabilityBypassesUidCheck(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeAdminOnly))
	mustTask(t, g, taskpkg.Task{PID: 1300, PPID: 1, UID: 1000})
	grant, err := g.RequestAttach(attachReq(1400, 1300, 4242, true))
	if err != nil {
		t.Fatalf("capable attach across uids = %v, want a grant", err)
	}
	if grant.Tracee != 1300 {
		t.Fatalf("grant tracee = %d, want 1300", grant.Tracee)
	}
}

// Scope 3 must refuse every attach even for a capable tracer.
func TestNoAttachModeDeniedEvenWithCapability(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeNoAttach))
	mustTask(t, g, taskpkg.Task{PID: 1500, PPID: 1, UID: 1000})
	_, err := g.RequestAttach(attachReq(1600, 1500, 1000, true))
	if !errors.Is(err, taskpkg.ErrScopeDenied) {
		t.Fatalf("capable attach in scope 3 = %v, want ErrScopeDenied", err)
	}
	if got := g.ActiveGrants(1500); got != 0 {
		t.Fatalf("ActiveGrants(1500) = %d, want 0", got)
	}
}

// A no_new_privs target must only be traceable by a capable tracer.
func TestNoNewPrivsDeniedWithoutCapability(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeClassic))
	mustTask(t, g, taskpkg.Task{PID: 1700, PPID: 1, UID: 1000, NoNewPrivs: true})
	_, err := g.RequestAttach(attachReq(1800, 1700, 1000, false))
	if !errors.Is(err, taskpkg.ErrNoNewPrivs) {
		t.Fatalf("unprivileged attach to a no_new_privs task = %v, want ErrNoNewPrivs", err)
	}
	if _, err := g.RequestAttach(attachReq(1800, 1700, 1000, true)); err != nil {
		t.Fatalf("capable attach to a no_new_privs task = %v, want a grant", err)
	}
}

// The attach rate limit must count only granted attaches and forget them once
// the window slides past.
func TestAttachRateLimitWindow(t *testing.T) {
	cfg := ptraceConfig(taskpkg.ScopeClassic)
	cfg.MaxAttachesPerWindow = 2
	g := taskpkg.NewPtraceGuard(cfg)
	mustTask(t, g, taskpkg.Task{PID: 1900, PPID: 1, UID: 1000})
	if _, err := g.RequestAttach(attachReq(2000, 1900, 1000, false)); err != nil {
		t.Fatalf("first attach = %v", err)
	}
	if _, err := g.RequestAttach(attachReq(2000, 1900, 1000, false)); err != nil {
		t.Fatalf("second attach = %v", err)
	}
	_, err := g.RequestAttach(attachReq(2000, 1900, 1000, false))
	if !errors.Is(err, taskpkg.ErrAttachQuota) {
		t.Fatalf("third attach in the window = %v, want ErrAttachQuota", err)
	}
	later := attachReq(2000, 1900, 1000, false)
	later.Now = attachNow.Add(61 * time.Second)
	if _, err := g.RequestAttach(later); err != nil {
		t.Fatalf("attach after the window slid = %v, want a grant", err)
	}
}

// Concurrent attach decisions must never exceed the window budget.
func TestConcurrentAttachRespectsQuota(t *testing.T) {
	cfg := ptraceConfig(taskpkg.ScopeClassic)
	cfg.MaxTasks = 20
	cfg.MaxAttachesPerWindow = 3
	g := taskpkg.NewPtraceGuard(cfg)
	for i := 0; i < 20; i++ {
		mustTask(t, g, taskpkg.Task{PID: 2000 + i, PPID: 1, UID: 1000})
	}
	var ok atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if _, err := g.RequestAttach(attachReq(3000, 2000+i, 1000, false)); err == nil {
				ok.Add(1)
			}
		}(i)
	}
	wg.Wait()
	if got := int(ok.Load()); got != 3 {
		t.Fatalf("%d concurrent attaches succeeded, want exactly 3", got)
	}
}

// An exec on the tracee must invalidate every grant that referenced it.
func TestExecRevokesGrants(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeClassic))
	mustTask(t, g, taskpkg.Task{PID: 3100, PPID: 1, UID: 1000})
	grant, err := g.RequestAttach(attachReq(3200, 3100, 1000, false))
	if err != nil {
		t.Fatalf("attach = %v", err)
	}
	if err := g.OnExec(3100); err != nil {
		t.Fatalf("OnExec = %v", err)
	}
	if got := g.ActiveGrants(3100); got != 0 {
		t.Fatalf("ActiveGrants(3100) = %d after exec, want 0", got)
	}
	if err := g.ReleaseAttach(grant.Nonce); !errors.Is(err, taskpkg.ErrStaleGeneration) {
		t.Fatalf("ReleaseAttach after exec = %v, want ErrStaleGeneration", err)
	}
	after := attachReq(3200, 3100, 1000, false)
	after.TraceeGen = 1
	if _, err := g.RequestAttach(after); err != nil {
		t.Fatalf("attach against the post-exec generation = %v, want a grant", err)
	}
}

// An attach request that observed an old generation must be refused.
func TestStaleGenerationRejected(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeClassic))
	mustTask(t, g, taskpkg.Task{PID: 3300, PPID: 1, UID: 1000, Generation: 1})
	req := attachReq(3400, 3300, 1000, false)
	req.TraceeGen = 2
	if _, err := g.RequestAttach(req); !errors.Is(err, taskpkg.ErrStaleGeneration) {
		t.Fatalf("attach with a stale generation = %v, want ErrStaleGeneration", err)
	}
	req.TraceeGen = 1
	if _, err := g.RequestAttach(req); err != nil {
		t.Fatalf("attach with the current generation = %v, want a grant", err)
	}
}

// The task table must enforce Config.MaxTasks and reject duplicate pids.
func TestTaskQuotaEnforced(t *testing.T) {
	cfg := ptraceConfig(taskpkg.ScopeClassic)
	cfg.MaxTasks = 2
	g := taskpkg.NewPtraceGuard(cfg)
	mustTask(t, g, taskpkg.Task{PID: 3500, PPID: 1, UID: 1000})
	mustTask(t, g, taskpkg.Task{PID: 3501, PPID: 1, UID: 1000})
	if err := g.RegisterTask(taskpkg.Task{PID: 3500, PPID: 1, UID: 1000}); !errors.Is(err, taskpkg.ErrDuplicateTask) {
		t.Fatalf("RegisterTask with a duplicate pid = %v, want ErrDuplicateTask", err)
	}
	if err := g.RegisterTask(taskpkg.Task{PID: 3502, PPID: 1, UID: 1000}); !errors.Is(err, taskpkg.ErrTaskQuota) {
		t.Fatalf("RegisterTask with a full table = %v, want ErrTaskQuota", err)
	}
	if _, err := g.Task(3502); !errors.Is(err, taskpkg.ErrUnknownTask) {
		t.Fatalf("Task(3502) = %v, want ErrUnknownTask", err)
	}
}

// Grants issued before a restart must still be releasable afterwards.
func TestSnapshotRestoresGrants(t *testing.T) {
	g := taskpkg.NewPtraceGuard(ptraceConfig(taskpkg.ScopeClassic))
	mustTask(t, g, taskpkg.Task{PID: 700, PPID: 1, UID: 1000})
	mustTask(t, g, taskpkg.Task{PID: 800, PPID: 700, UID: 1000})
	first, err := g.RequestAttach(attachReq(800, 700, 1000, false))
	if err != nil {
		t.Fatalf("first attach = %v", err)
	}
	if _, err := g.RequestAttach(attachReq(800, 700, 1000, false)); err != nil {
		t.Fatalf("second attach = %v", err)
	}
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot = %v", err)
	}
	restored, err := taskpkg.NewPtraceGuardFromSnapshot(ptraceConfig(taskpkg.ScopeClassic), snap)
	if err != nil {
		t.Fatalf("NewPtraceGuardFromSnapshot = %v", err)
	}
	if got := restored.ActiveGrants(700); got != 2 {
		t.Fatalf("restored ActiveGrants(700) = %d, want 2", got)
	}
	if err := restored.ReleaseAttach(first.Nonce); err != nil {
		t.Fatalf("restored ReleaseAttach = %v, want nil", err)
	}
	if got := restored.ActiveGrants(700); got != 1 {
		t.Fatalf("ActiveGrants(700) = %d after release, want 1", got)
	}
}
