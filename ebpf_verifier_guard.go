package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the verifier and the loader. A verifier failure
// must be distinguishable from a resource-quota failure.
var (
	ErrEmptyProgram   = errors.New("ebpf: program has no instructions")
	ErrInstrLimit     = errors.New("ebpf: instruction limit exceeded")
	ErrBadJump        = errors.New("ebpf: jump out of bounds")
	ErrLoop           = errors.New("ebpf: unbounded loop")
	ErrNoExit         = errors.New("ebpf: program does not end with exit")
	ErrHelperDenied   = errors.New("ebpf: helper not allowed")
	ErrLicense        = errors.New("ebpf: helper requires the GPL license")
	ErrMapQuota       = errors.New("ebpf: map quota exceeded")
	ErrMapTooLarge    = errors.New("ebpf: map exceeds the verifier limit")
	ErrProgramExists  = errors.New("ebpf: program already loaded")
	ErrStaleGen       = errors.New("ebpf: stale load generation")
	ErrUnknownProgram = errors.New("ebpf: unknown program")
)

// Opcodes understood by the verifier.
const (
	// OpCall is a helper call; Imm carries the helper id.
	OpCall uint8 = 0x85
	// OpJump is a PC-relative jump; Off is the displacement.
	OpJump uint8 = 0x05
	// OpExit terminates the program.
	OpExit uint8 = 0x95
	// OpMov and OpAdd are arithmetic instructions used between jumps.
	OpMov uint8 = 0xb7
	OpAdd uint8 = 0x07
)

// Insn is one verifier instruction.
type Insn struct {
	Op  uint8
	Dst uint8
	Src uint8
	Off int16
	Imm int32
}

// Program is a program image submitted for verification.
type Program struct {
	Name string
	Type string
	// License must be exactly "GPL" for helpers listed in GPLOnlyHelpers.
	License string
	Insns   []Insn
}

// MapSpec is one map created together with a program.
type MapSpec struct {
	Name       string
	KeySize    uint32
	ValueSize  uint32
	MaxEntries uint32
}

// Config holds the verifier and quota limits of a guard.
type Config struct {
	MaxInsns         int
	MaxMapsPerTenant int
	MaxKeySize       uint32
	MaxValueSize     uint32
	MaxEntries       uint32
	AllowedHelpers   []int
	GPLOnlyHelpers   []int
}

// ProgramHandle identifies one loaded program.
type ProgramHandle struct {
	ID     uint64
	Name   string
	Type   string
	Tenant string
	// Insns is the verified instruction count.
	Insns int
	// Gen is the load generation the program was admitted under.
	Gen uint64
}

// Stats is a point-in-time view of the guard.
type Stats struct {
	Programs int
	Maps     int
	Insns    int
}

// EbpfGuard verifies and admits programs for a set of tenants.
type EbpfGuard struct {
	cfg Config
}

// NewEbpfGuard returns a guard with the limits described by cfg.
func NewEbpfGuard(cfg Config) *EbpfGuard {
	return &EbpfGuard{cfg: cfg}
}

// NewEbpfGuardFromSnapshot restores loaded programs and their generations from
// a snapshot produced by Snapshot.
func NewEbpfGuardFromSnapshot(cfg Config, data []byte) (*EbpfGuard, error) {
	return nil, errNotImplemented
}

// Verify checks a program image without registering anything: instruction
// count, jump targets, final exit, helper allowlist and license.
func (g *EbpfGuard) Verify(p Program) error {
	return errNotImplemented
}

// Load verifies p, checks the attached maps, and admits the pair atomically.
// If any check fails nothing is registered and the generation does not move.
func (g *EbpfGuard) Load(tenant string, p Program, maps []MapSpec) (ProgramHandle, error) {
	return ProgramHandle{}, errNotImplemented
}

// Unload removes a program. gen must match the current load generation.
func (g *EbpfGuard) Unload(id uint64, gen uint64) error {
	return errNotImplemented
}

// Lookup returns the handle of a loaded program by tenant and name.
func (g *EbpfGuard) Lookup(tenant, name string) (ProgramHandle, error) {
	return ProgramHandle{}, errNotImplemented
}

// ProgramCount returns how many programs a tenant has loaded.
func (g *EbpfGuard) ProgramCount(tenant string) int {
	return 0
}

// MapCount returns how many maps a tenant owns.
func (g *EbpfGuard) MapCount(tenant string) int {
	return 0
}

// Generation returns the current load generation.
func (g *EbpfGuard) Generation() uint64 {
	return 0
}

// SetAllowedHelpers replaces the helper allowlist used by Verify and Load.
func (g *EbpfGuard) SetAllowedHelpers(ids []int) error {
	return errNotImplemented
}

// Stats returns program, map and instruction totals.
func (g *EbpfGuard) Stats() Stats {
	return Stats{}
}

// Snapshot serialises loaded programs, map counts and the generation for a
// restart.
func (g *EbpfGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// Reset drops every program and map owned by one tenant.
func (g *EbpfGuard) Reset(tenant string) error {
	return errNotImplemented
}
