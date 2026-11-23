package task

import (
	"errors"
	"time"
)

// Sentinels returned by RaidRoster. Callers must be able to tell a lock window
// problem from a capacity problem and from a boss lockout.
var (
	ErrRosterLocked     = errors.New("raid: roster locked")
	ErrRaidFull         = errors.New("raid: raid full")
	ErrRoleQuota        = errors.New("raid: role quota exceeded")
	ErrDuplicateMember  = errors.New("raid: member already on roster")
	ErrNotInRaid        = errors.New("raid: member not on roster")
	ErrIllegalBench     = errors.New("raid: bench full")
	ErrSwapWindowClosed = errors.New("raid: swap window closed")
	ErrRosterUnlocked   = errors.New("raid: roster is not locked")
	ErrLockoutActive    = errors.New("raid: boss lockout active")
	ErrUnknownBoss      = errors.New("raid: unknown boss")
)

var errNotImplemented = errors.New("raid: not implemented")

// Role is a raid role.
type Role string

const (
	RoleTank   Role = "tank"
	RoleHealer Role = "healer"
	RoleDPS    Role = "dps"
)

// Config is the roster policy.
type Config struct {
	Size       int
	MaxTanks   int
	MaxHealers int
	MaxDPS     int
	BenchSize  int
	SwapWindow time.Duration
}

// Member is one raider.
type Member struct {
	Name      string
	Role      Role
	GearScore int
}

// Slot is one roster entry.
type Slot struct {
	Name      string
	Role      Role
	GearScore int
	Bench     bool
}

// PullRecord is the result of one boss pull. Members is the sorted active
// roster at pull time.
type PullRecord struct {
	BossID  string
	At      time.Time
	Members []string
}

// RaidRoster enforces raid size, role quotas, bench rules and pull locks.
type RaidRoster struct {
	cfg Config
}

// NewRaidRoster returns an empty roster for raidID.
func NewRaidRoster(cfg Config, raidID string) *RaidRoster {
	return &RaidRoster{cfg: cfg}
}

// NewRaidRosterFromSnapshot restores active members, bench, lock time and boss
// lockouts from a snapshot.
func NewRaidRosterFromSnapshot(cfg Config, data []byte) (*RaidRoster, error) {
	return nil, errNotImplemented
}

// Add places a member on the active roster.
func (r *RaidRoster) Add(m Member, now time.Time) error {
	return errNotImplemented
}

// Bench moves an active member to the bench.
func (r *RaidRoster) Bench(name string, now time.Time) error {
	return errNotImplemented
}

// Swap exchanges a benched member with an active member atomically.
func (r *RaidRoster) Swap(benchedName, activeName string, now time.Time) error {
	return errNotImplemented
}

// Lock freezes the roster and opens the swap window.
func (r *RaidRoster) Lock(now time.Time) error {
	return errNotImplemented
}

// Unlock reopens the roster for changes.
func (r *RaidRoster) Unlock() error {
	return errNotImplemented
}

// Locked reports whether the roster is locked.
func (r *RaidRoster) Locked() bool {
	return false
}

// Pull records a boss pull and returns the attendance snapshot.
func (r *RaidRoster) Pull(bossID string, now time.Time) (PullRecord, error) {
	return PullRecord{}, errNotImplemented
}

// UnlockBoss clears the lockout of a boss so it may be pulled again.
func (r *RaidRoster) UnlockBoss(bossID string) error {
	return errNotImplemented
}

// Roster returns a copy of the roster sorted by role (tank, healer, dps) then
// name, with benched members last.
func (r *RaidRoster) Roster() []Slot {
	return nil
}

// BenchList returns the sorted bench names.
func (r *RaidRoster) BenchList() []string {
	return nil
}

// Attendance returns the sorted members recorded by the most recent pull of a
// boss.
func (r *RaidRoster) Attendance(bossID string) ([]string, error) {
	return nil, errNotImplemented
}

// Count returns the number of active members with the given role.
func (r *RaidRoster) Count(role Role) int {
	return 0
}

// Snapshot serialises active members, bench, lock time and boss lockouts.
func (r *RaidRoster) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
