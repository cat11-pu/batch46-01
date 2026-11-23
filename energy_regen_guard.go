package task

import (
	"errors"
	"time"
)

// Sentinel errors reported by EnergyRegen.
var (
	ErrUnknownPlayer    = errors.New("energy: unknown player")
	ErrPlayerExists     = errors.New("energy: player already registered")
	ErrInvalidPlayer    = errors.New("energy: invalid player record")
	ErrInvalidAmount    = errors.New("energy: invalid amount")
	ErrInsufficientEnergy = errors.New("energy: insufficient energy")
	ErrClockSkew        = errors.New("energy: clock moved backwards")
	ErrMissingRequestID = errors.New("energy: missing request id")
	ErrIDConflict       = errors.New("energy: request id reused with another payload")
	ErrInvalidCap       = errors.New("energy: invalid energy cap")
)

var errNotImplemented = errors.New("not implemented")

// Config is the regeneration policy shared by all players.
type Config struct {
	// RegenInterval is the time needed for one energy point.
	RegenInterval time.Duration
	// ReserveCap is how many overflow points the reserve bucket holds.
	ReserveCap int
	// MaxOfflineSpan is how much idle time is compensated at once.
	MaxOfflineSpan time.Duration
}

// Player is the stamina record supplied at registration.
type Player struct {
	PlayerID   string
	Energy     int
	Reserve    int
	MaxEnergy  int
	LastRegenAt time.Time
}

// Grant reports how a regeneration settlement was distributed.
type Grant struct {
	ToEnergy  int
	ToReserve int
	Dropped   int
}

// SpendRequest spends energy, first from the bar and then from the reserve.
type SpendRequest struct {
	RequestID string
	PlayerID  string
	Amount    int
}

// SpendResult is the settled spend.
type SpendResult struct {
	RequestID     string
	PlayerID      string
	Amount        int
	FromEnergy    int
	FromReserve   int
	EnergyLeft    int
	ReserveLeft   int
}

// RefillRequest adds purchased energy.
type RefillRequest struct {
	RequestID string
	PlayerID  string
	Amount    int
}

// RefillResult reports how a refill was distributed.
type RefillResult struct {
	RequestID string
	PlayerID  string
	Amount    int
	ToEnergy  int
	ToReserve int
	Dropped   int
}

// State is the read-only view of one player at a point in time.
type State struct {
	Energy        int
	Reserve       int
	MaxEnergy     int
	ReserveCap    int
	PendingPoints int
	NextPointIn   time.Duration
}

// EnergyRegen settles stamina regeneration, spending and refills.
type EnergyRegen struct {
	cfg Config
}

// NewEnergyRegen builds an empty stamina service.
func NewEnergyRegen(cfg Config) *EnergyRegen {
	return &EnergyRegen{cfg: cfg}
}

// NewEnergyRegenFromSnapshot restores players, their regeneration anchors and
// the settled request ids from a Snapshot payload.
func NewEnergyRegenFromSnapshot(cfg Config, data []byte) (*EnergyRegen, error) {
	return nil, errNotImplemented
}

// RegisterPlayer adds a player. Duplicate ids, negative values and an energy
// above the player cap are rejected.
func (e *EnergyRegen) RegisterPlayer(p Player) error {
	return errNotImplemented
}

// Regen settles every full interval that elapsed since the last anchor. Points
// fill the bar first, then the reserve bucket, then they are dropped. More than
// Config.MaxOfflineSpan of idle time is not compensated. The anchor always
// advances past the settled intervals, so settling twice at the same instant
// grants nothing.
func (e *EnergyRegen) Regen(playerID string, now time.Time) (Grant, error) {
	return Grant{}, errNotImplemented
}

// PendingPoints reports how many points Regen would grant at now without
// mutating anything.
func (e *EnergyRegen) PendingPoints(playerID string, now time.Time) int {
	return 0
}

// NextPointIn reports how long until the next point accrues.
func (e *EnergyRegen) NextPointIn(playerID string, now time.Time) time.Duration {
	return 0
}

// State returns the read-only view of a player at now. It never mutates.
func (e *EnergyRegen) State(playerID string, now time.Time) (State, error) {
	return State{}, errNotImplemented
}

// Spend settles regeneration first and then deducts the amount from the bar
// and the reserve bucket. A rejected amount (zero or negative) or a moved-back
// clock leaves the player untouched; a genuine shortfall is reported after the
// regeneration was already settled. Replaying a request id with the same
// payload returns the stored result and settles nothing.
func (e *EnergyRegen) Spend(req SpendRequest, now time.Time) (SpendResult, error) {
	return SpendResult{}, errNotImplemented
}

// Refill settles regeneration first and then adds purchased energy to the bar,
// the reserve bucket and finally drops the overflow.
func (e *EnergyRegen) Refill(req RefillRequest, now time.Time) (RefillResult, error) {
	return RefillResult{}, errNotImplemented
}

// RaiseCap raises the player maximum. The new cap must be strictly greater
// than the current one and never fills the bar.
func (e *EnergyRegen) RaiseCap(playerID string, newMax int) error {
	return errNotImplemented
}

// Snapshot serialises players, anchors and settled request ids so a restart
// keeps the same stamina state.
func (e *EnergyRegen) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
