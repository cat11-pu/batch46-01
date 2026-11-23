package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by HitRegistration.
var (
	ErrInvalidWeapon = errors.New("hit: invalid weapon profile")
	ErrWeaponExists  = errors.New("hit: weapon already registered")
	ErrUnknownWeapon = errors.New("hit: unknown weapon")
	ErrInvalidPlayer = errors.New("hit: invalid player")
	ErrPlayerExists  = errors.New("hit: player already spawned")
	ErrUnknownPlayer = errors.New("hit: unknown player")
	ErrInvalidEvent  = errors.New("hit: invalid damage event")
	ErrOutOfRange    = errors.New("hit: shot outside weapon range")
	ErrDamageCap     = errors.New("hit: damage above per-event cap")
	ErrShotReplayed  = errors.New("hit: shot already registered")
	ErrStaleTick     = errors.New("hit: event tick outside rewind window")
	ErrVictimDead    = errors.New("hit: victim already dead")
	ErrFriendlyFire  = errors.New("hit: friendly fire disabled")
	ErrCooldown      = errors.New("hit: weapon still cooling down")
	ErrBatchRejected = errors.New("hit: batch rejected")
	ErrShotNotFound  = errors.New("hit: shot not found")
)

// Config controls damage validation for one match.
type Config struct {
	// MaxDamagePerEvent caps the damage of a single event after zone
	// multipliers are applied. Zero means 10000.
	MaxDamagePerEvent int
	// MaxTickRewind is how many ticks behind an attacker's latest accepted
	// shot an event may arrive. Zero means no rewind allowed.
	MaxTickRewind uint32
	// FriendlyFire allows damage between players on the same team.
	FriendlyFire bool
}

// WeaponProfile describes one server-side weapon.
type WeaponProfile struct {
	ID             string
	BaseDamage     int
	HeadMultiplier float64
	LimbMultiplier float64
	MaxRange       float64
	// CooldownTicks is the minimum distance in ticks between two accepted
	// shots with this weapon by the same attacker.
	CooldownTicks uint32
}

// DamageEvent is one client-reported hit, validated by the server.
type DamageEvent struct {
	ShotID     string
	AttackerID string
	VictimID   string
	WeaponID   string
	Tick       uint32
	// Zone is "head", "torso" or "limb".
	Zone     string
	Distance float64
}

// HitResult is the outcome of one accepted event.
type HitResult struct {
	ShotID        string
	VictimID      string
	AppliedDamage int
	RemainingHP   int
	Killed        bool
	// Absorbed marks a hit that was accepted but negated by invulnerability.
	Absorbed bool
}

// PlayerState is the authoritative state of one player.
type PlayerState struct {
	ID                string
	Team              string
	HP                int
	MaxHP             int
	InvulnerableUntil uint32
}

// RollbackResult describes the effect of undoing one shot.
type RollbackResult struct {
	ShotID     string
	VictimID   string
	RestoredHP int
	Revived    bool
}

// HitRegistration validates damage events, applies them atomically and keeps
// an undo journal so a rejected or disputed shot can be rolled back. All
// exported methods are safe for concurrent use.
type HitRegistration struct {
	cfg Config
}

// NewHitRegistration returns an empty registration configured by cfg.
func NewHitRegistration(cfg Config) *HitRegistration {
	return &HitRegistration{cfg: cfg}
}

// NewHitRegistrationFromSnapshot restores players, weapons, accepted shot ids
// and cooldown state from bytes produced by Snapshot.
func NewHitRegistrationFromSnapshot(data []byte) (*HitRegistration, error) {
	return nil, errNotImplemented
}

// RegisterWeapon registers a weapon profile.
func (h *HitRegistration) RegisterWeapon(p WeaponProfile) error {
	return errNotImplemented
}

// SpawnPlayer adds a player at full health.
func (h *HitRegistration) SpawnPlayer(id, team string, hp int) error {
	return errNotImplemented
}

// SetInvulnerable makes a player immune to damage for every tick strictly
// below untilTick.
func (h *HitRegistration) SetInvulnerable(id string, untilTick uint32) error {
	return errNotImplemented
}

// Submit validates and applies one damage event.
//
// Validation order: weapon profile, both players, self damage and zone,
// distance, range, damage cap, shot id replay, tick rewind, dead victim,
// friendly fire, weapon cooldown. Hits on an invulnerable victim are accepted
// and reported with Absorbed set.
func (h *HitRegistration) Submit(ev DamageEvent) (HitResult, error) {
	return HitResult{}, errNotImplemented
}

// SubmitBatch applies a list of events atomically. If any event is rejected
// the whole batch fails with ErrBatchRejected and no health, cooldown or shot
// id changes.
func (h *HitRegistration) SubmitBatch(events []DamageEvent) ([]HitResult, error) {
	return nil, errNotImplemented
}

// RollbackShot undoes the damage of one accepted shot, restores the victim's
// health (reviving it if this shot killed it) and frees the shot id.
func (h *HitRegistration) RollbackShot(shotID string) (RollbackResult, error) {
	return RollbackResult{}, errNotImplemented
}

// Health returns the current health of a player.
func (h *HitRegistration) Health(id string) (int, error) {
	return 0, errNotImplemented
}

// Player returns the authoritative state of one player.
func (h *HitRegistration) Player(id string) (PlayerState, error) {
	return PlayerState{}, errNotImplemented
}

// Snapshot serialises the whole registration state.
func (h *HitRegistration) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
