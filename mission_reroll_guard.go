package task

import (
	"errors"
	"time"
)

// errNotImplemented is returned by every stub below.
var errNotImplemented = errors.New("mission: not implemented")

// Sentinel errors reported by MissionReroll.
var (
	// ErrNoReroll means the daily reroll budget is used up for the day.
	ErrNoReroll = errors.New("mission: daily reroll budget exhausted")
	// ErrLocked means the slot has been locked and may not be rerolled.
	ErrLocked = errors.New("mission: slot is locked")
	// ErrBadMission means a mission lacks an id or kind, or its tier is
	// outside [Policy.MinTier, Policy.MaxTier].
	ErrBadMission = errors.New("mission: invalid mission")
	// ErrPoolFull means AddToPool reached Policy.PoolCap.
	ErrPoolFull = errors.New("mission: reroll pool at capacity")
	// ErrPoolExhausted means no pool mission is free for the slot.
	ErrPoolExhausted = errors.New("mission: no unused reroll candidate")
	// ErrBadSlot means the index is outside [0, Policy.Slots).
	ErrBadSlot = errors.New("mission: slot out of range")
	// ErrRequestConflict means the request id was already applied to a
	// different slot.
	ErrRequestConflict = errors.New("mission: request id bound to another slot")
	// ErrSnapshotInvalid means the payload cannot be restored.
	ErrSnapshotInvalid = errors.New("mission: invalid snapshot")
)

// Mission is one daily mission offer.
type Mission struct {
	ID     string
	Kind   string
	Tier   int
	Reward int
}

// Policy bounds the reroll board of one player.
type Policy struct {
	// Slots is the number of mission slots on the board.
	Slots int
	// DailyBudget is how many rerolls one player gets per UTC day.
	DailyBudget int
	// MinTier and MaxTier bound acceptable mission tiers, inclusive.
	MinTier int
	MaxTier int
	// PoolCap is the number of distinct candidates the reroll pool keeps.
	PoolCap int
}

// RerollRecord is one applied reroll of one slot.
type RerollRecord struct {
	Slot      int
	FromID    string
	ToID      string
	RequestID string
	Day       string
	At        time.Time
}

// MissionReroll is the daily mission board of one player.
type MissionReroll struct {
	playerID string
	policy   Policy
}

// NewMissionReroll creates the board of playerID under policy.
func NewMissionReroll(playerID string, policy Policy) *MissionReroll {
	return &MissionReroll{playerID: playerID, policy: policy}
}

// PlayerID returns the board owner.
func (m *MissionReroll) PlayerID() string {
	return ""
}

// SeedSlot places the initial offer of a slot.
func (m *MissionReroll) SeedSlot(slot int, mission Mission) error {
	return errNotImplemented
}

// Slot returns the current offer of a slot.
func (m *MissionReroll) Slot(slot int) (Mission, error) {
	return Mission{}, errNotImplemented
}

// AddToPool adds reroll candidates. Duplicate ids are skipped; when PoolCap is
// reached the call stops and reports ErrPoolFull with the accepted count.
func (m *MissionReroll) AddToPool(missions ...Mission) (int, error) {
	return 0, errNotImplemented
}

// PoolSize returns the number of distinct candidates held.
func (m *MissionReroll) PoolSize() int {
	return 0
}

// Lock makes a slot ineligible for reroll.
func (m *MissionReroll) Lock(slot int) error {
	return errNotImplemented
}

// RerollsLeft reports the remaining budget for the UTC day containing now.
func (m *MissionReroll) RerollsLeft(now time.Time) int {
	return 0
}

// Reroll replaces the offer of a slot with a candidate that is not used by any
// slot. requestID makes the call idempotent.
func (m *MissionReroll) Reroll(slot int, requestID string, now time.Time) (Mission, error) {
	return Mission{}, errNotImplemented
}

// History returns the applied rerolls of a slot in application order.
func (m *MissionReroll) History(slot int) ([]RerollRecord, error) {
	return nil, errNotImplemented
}

// RequestResult returns the mission produced by a request id.
func (m *MissionReroll) RequestResult(requestID string) (Mission, bool) {
	return Mission{}, false
}

// Snapshot serialises the board so a restarted process keeps its state.
func (m *MissionReroll) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// NewMissionRerollFromSnapshot restores a board produced by Snapshot.
func NewMissionRerollFromSnapshot(data []byte) (*MissionReroll, error) {
	return nil, errNotImplemented
}
