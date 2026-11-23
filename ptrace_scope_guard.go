package task

import (
	"errors"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the guard. A policy denial must never be
// reported as a stale-generation problem and vice versa.
var (
	ErrScopeDenied      = errors.New("ptrace: denied by yama scope")
	ErrNotAncestor      = errors.New("ptrace: tracer is not an ancestor")
	ErrUidMismatch      = errors.New("ptrace: uid mismatch")
	ErrNoNewPrivs       = errors.New("ptrace: target has no_new_privs")
	ErrAttachQuota      = errors.New("ptrace: attach rate limit exceeded")
	ErrTaskQuota        = errors.New("ptrace: task table is full")
	ErrUnknownTask      = errors.New("ptrace: unknown task")
	ErrDuplicateTask    = errors.New("ptrace: task already registered")
	ErrStaleGeneration  = errors.New("ptrace: stale process generation")
	ErrUnknownGrant     = errors.New("ptrace: unknown attach grant")
	ErrInvalidMode      = errors.New("ptrace: invalid scope mode")
)

// ScopeMode mirrors the kernel's yama ptrace_scope levels.
type ScopeMode uint8

const (
	// ScopeClassic (0) allows any same-uid tracer.
	ScopeClassic ScopeMode = 0
	// ScopeRestricted (1) only allows an ancestor tracer.
	ScopeRestricted ScopeMode = 1
	// ScopeAdminOnly (2) only allows a privileged tracer.
	ScopeAdminOnly ScopeMode = 2
	// ScopeNoAttach (3) refuses every attach, privileged or not.
	ScopeNoAttach ScopeMode = 3
)

// Task is one process as seen by the guard.
type Task struct {
	PID  int
	PPID int
	UID  uint32
	// Generation increments on every exec; attaches tied to an older
	// generation are stale.
	Generation uint64
	NoNewPrivs bool
}

// Credentials describe the tracer's privileges.
type Credentials struct {
	UID             uint32
	HasCapSysPtrace bool
}

// AttachRequest is one PTRACE_ATTACH decision request.
type AttachRequest struct {
	Tracer int
	Tracee int
	Creds  Credentials
	// TraceeGen is the generation the caller observed; 0 means "current".
	TraceeGen uint64
	Now       time.Time
}

// AttachGrant is the successful result of RequestAttach.
type AttachGrant struct {
	Nonce     string
	Tracer    int
	Tracee    int
	GrantedAt time.Time
	ExpiresAt time.Time
}

// Config holds the policy and resource limits of a guard.
type Config struct {
	Mode ScopeMode
	// MaxAttachesPerWindow caps granted attaches inside Window.
	MaxAttachesPerWindow int
	Window               time.Duration
	// MaxTasks caps how many tasks may be registered at once.
	MaxTasks int
}

// Stats is a point-in-time view of the guard.
type Stats struct {
	Tasks   int
	Granted int
	Denied  int
}

// PtraceGuard enforces a yama-style attach policy for a process tree.
type PtraceGuard struct {
	cfg Config
}

// NewPtraceGuard returns a guard with the policy described by cfg.
func NewPtraceGuard(cfg Config) *PtraceGuard {
	return &PtraceGuard{cfg: cfg}
}

// NewPtraceGuardFromSnapshot restores tasks, grants and revoked nonces from a
// snapshot produced by Snapshot.
func NewPtraceGuardFromSnapshot(cfg Config, data []byte) (*PtraceGuard, error) {
	return nil, errNotImplemented
}

// RegisterTask adds a process to the tree. Duplicate pids and a full task
// table must be rejected.
func (g *PtraceGuard) RegisterTask(t Task) error {
	return errNotImplemented
}

// UnregisterTask removes a process and every grant that involves it.
func (g *PtraceGuard) UnregisterTask(pid int) error {
	return errNotImplemented
}

// Task returns a copy of one registered task.
func (g *PtraceGuard) Task(pid int) (Task, error) {
	return Task{}, errNotImplemented
}

// SetMode switches the scope level. Values outside 0..3 are invalid.
func (g *PtraceGuard) SetMode(m ScopeMode) error {
	return errNotImplemented
}

// Mode returns the current scope level.
func (g *PtraceGuard) Mode() ScopeMode {
	return 0
}

// IsAncestor reports whether tracer is a transitive parent of tracee.
func (g *PtraceGuard) IsAncestor(tracer, tracee int) (bool, error) {
	return false, errNotImplemented
}

// RequestAttach decides one attach and returns a single-use grant on success.
func (g *PtraceGuard) RequestAttach(req AttachRequest) (AttachGrant, error) {
	return AttachGrant{}, errNotImplemented
}

// ReleaseAttach retires a grant. Grants destroyed by an exec are reported as
// stale rather than unknown.
func (g *PtraceGuard) ReleaseAttach(nonce string) error {
	return errNotImplemented
}

// ActiveGrants returns the number of live grants attached to a tracee.
func (g *PtraceGuard) ActiveGrants(pid int) int {
	return 0
}

// OnExec bumps the generation of pid and drops every grant that involves it.
func (g *PtraceGuard) OnExec(pid int) error {
	return errNotImplemented
}

// Snapshot serialises tasks, grants and revoked nonces for a restart.
func (g *PtraceGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// Stats returns task, grant and denial counters.
func (g *PtraceGuard) Stats() Stats {
	return Stats{}
}
