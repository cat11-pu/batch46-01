package task

import (
	"errors"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-c-02"
)

func arenaConfig() taskpkg.Config {
	return taskpkg.Config{MaxDamagePerEvent: 1000, MaxTickRewind: 10}
}

func setupArena(t *testing.T, cfg taskpkg.Config) *taskpkg.HitRegistration {
	t.Helper()
	h := taskpkg.NewHitRegistration(cfg)
	if err := h.RegisterWeapon(taskpkg.WeaponProfile{
		ID:             "rifle",
		BaseDamage:     100,
		HeadMultiplier: 2.5,
		LimbMultiplier: 0.6,
		MaxRange:       100,
		CooldownTicks:  3,
	}); err != nil {
		t.Fatalf("RegisterWeapon: %v", err)
	}
	if err := h.SpawnPlayer("attacker", "alpha", 100); err != nil {
		t.Fatalf("SpawnPlayer(attacker): %v", err)
	}
	if err := h.SpawnPlayer("victim", "bravo", 200); err != nil {
		t.Fatalf("SpawnPlayer(victim): %v", err)
	}
	return h
}

func shot(shotID string, tick uint32, zone string, distance float64) taskpkg.DamageEvent {
	return taskpkg.DamageEvent{
		ShotID:     shotID,
		AttackerID: "attacker",
		VictimID:   "victim",
		WeaponID:   "rifle",
		Tick:       tick,
		Zone:       zone,
		Distance:   distance,
	}
}

// A head hit must use the weapon head multiplier and can kill.
func TestHeadZoneMultiplier(t *testing.T) {
	h := setupArena(t, arenaConfig())
	res, err := h.Submit(shot("shot-head", 10, "head", 40))
	if err != nil {
		t.Fatalf("headshot rejected: %v", err)
	}
	if res.AppliedDamage != 250 {
		t.Fatalf("headshot damage = %d, want 250", res.AppliedDamage)
	}
	if res.RemainingHP != 0 {
		t.Fatalf("remaining hp = %d, want 0", res.RemainingHP)
	}
	if !res.Killed {
		t.Fatalf("headshot did not report a kill")
	}
}

// A shot beyond the weapon range must be rejected without damage.
func TestOutOfRangeShotRejected(t *testing.T) {
	h := setupArena(t, arenaConfig())
	if _, err := h.Submit(shot("shot-far", 10, "torso", 120)); !errors.Is(err, taskpkg.ErrOutOfRange) {
		t.Fatalf("long shot error = %v, want ErrOutOfRange", err)
	}
	hp, err := h.Health("victim")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hp != 200 {
		t.Fatalf("health changed on rejected shot: %d", hp)
	}
}

// Damage above the per-event cap must be rejected.
func TestDamageCapRejected(t *testing.T) {
	cfg := arenaConfig()
	cfg.MaxDamagePerEvent = 150
	h := setupArena(t, cfg)
	if _, err := h.Submit(shot("shot-cap", 10, "head", 40)); !errors.Is(err, taskpkg.ErrDamageCap) {
		t.Fatalf("over-cap shot error = %v, want ErrDamageCap", err)
	}
}

// An unregistered weapon must be rejected.
func TestUnknownWeaponRejected(t *testing.T) {
	h := setupArena(t, arenaConfig())
	ev := shot("shot-unknown", 10, "torso", 40)
	ev.WeaponID = "railgun"
	if _, err := h.Submit(ev); !errors.Is(err, taskpkg.ErrUnknownWeapon) {
		t.Fatalf("unknown weapon error = %v, want ErrUnknownWeapon", err)
	}
}

// Friendly fire must be rejected when the match disables it.
func TestFriendlyFireRejectedWhenDisabled(t *testing.T) {
	h := setupArena(t, arenaConfig())
	if err := h.SpawnPlayer("ally", "alpha", 200); err != nil {
		t.Fatalf("SpawnPlayer(ally): %v", err)
	}
	ev := shot("shot-ff", 10, "torso", 40)
	ev.VictimID = "ally"
	if _, err := h.Submit(ev); !errors.Is(err, taskpkg.ErrFriendlyFire) {
		t.Fatalf("friendly fire error = %v, want ErrFriendlyFire", err)
	}
	hp, err := h.Health("ally")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hp != 200 {
		t.Fatalf("ally took friendly damage: %d", hp)
	}
}

// Friendly fire must be accepted when the match enables it.
func TestFriendlyFireAllowedWhenEnabled(t *testing.T) {
	cfg := arenaConfig()
	cfg.FriendlyFire = true
	h := setupArena(t, cfg)
	if err := h.SpawnPlayer("ally2", "alpha", 200); err != nil {
		t.Fatalf("SpawnPlayer(ally2): %v", err)
	}
	ev := shot("shot-ff-ok", 10, "torso", 40)
	ev.VictimID = "ally2"
	res, err := h.Submit(ev)
	if err != nil {
		t.Fatalf("enabled friendly fire rejected: %v", err)
	}
	if res.AppliedDamage != 100 {
		t.Fatalf("friendly fire damage = %d, want 100", res.AppliedDamage)
	}
}

// A shot id must apply at most once.
func TestReplayedShotRejected(t *testing.T) {
	h := setupArena(t, arenaConfig())
	if _, err := h.Submit(shot("shot-replay", 10, "torso", 40)); err != nil {
		t.Fatalf("first shot: %v", err)
	}
	if _, err := h.Submit(shot("shot-replay", 30, "torso", 40)); !errors.Is(err, taskpkg.ErrShotReplayed) {
		t.Fatalf("replayed shot error = %v, want ErrShotReplayed", err)
	}
	hp, err := h.Health("victim")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hp != 100 {
		t.Fatalf("replayed shot applied damage twice: hp %d", hp)
	}
}

// The weapon cooldown must be enforced per attacker and weapon.
func TestCooldownEnforced(t *testing.T) {
	h := setupArena(t, arenaConfig())
	if _, err := h.Submit(shot("shot-cd-1", 10, "torso", 40)); err != nil {
		t.Fatalf("first shot: %v", err)
	}
	if _, err := h.Submit(shot("shot-cd-2", 12, "torso", 40)); !errors.Is(err, taskpkg.ErrCooldown) {
		t.Fatalf("early shot error = %v, want ErrCooldown", err)
	}
	if _, err := h.Submit(shot("shot-cd-3", 13, "torso", 40)); err != nil {
		t.Fatalf("shot after cooldown rejected: %v", err)
	}
}

// An event too far behind the attacker's latest tick must be rejected.
func TestStaleTickRejected(t *testing.T) {
	cfg := arenaConfig()
	cfg.MaxTickRewind = 20
	h := setupArena(t, cfg)
	if _, err := h.Submit(shot("shot-late", 100, "torso", 40)); err != nil {
		t.Fatalf("first shot: %v", err)
	}
	if _, err := h.Submit(shot("shot-old", 75, "torso", 40)); !errors.Is(err, taskpkg.ErrStaleTick) {
		t.Fatalf("stale shot error = %v, want ErrStaleTick", err)
	}
}

// A rejected batch must leave no health, shot id or cooldown behind.
func TestBatchAtomicRejected(t *testing.T) {
	h := setupArena(t, arenaConfig())
	batch := []taskpkg.DamageEvent{
		shot("shot-batch-ok", 10, "torso", 40),
		shot("shot-batch-bad", 20, "torso", 150),
	}
	if _, err := h.SubmitBatch(batch); !errors.Is(err, taskpkg.ErrBatchRejected) {
		t.Fatalf("batch error = %v, want ErrBatchRejected", err)
	}
	hp, err := h.Health("victim")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hp != 200 {
		t.Fatalf("part of a rejected batch was applied: hp %d", hp)
	}
	if _, err := h.Submit(batch[0]); err != nil {
		t.Fatalf("shot id from rejected batch was consumed: %v", err)
	}
}

// Rolling back a shot must restore the damage it applied.
func TestRollbackRestoresHealth(t *testing.T) {
	h := setupArena(t, arenaConfig())
	if _, err := h.Submit(shot("shot-limb-1", 10, "limb", 40)); err != nil {
		t.Fatalf("first limb shot: %v", err)
	}
	if _, err := h.Submit(shot("shot-limb-2", 20, "limb", 40)); err != nil {
		t.Fatalf("second limb shot: %v", err)
	}
	res, err := h.RollbackShot("shot-limb-1")
	if err != nil {
		t.Fatalf("RollbackShot: %v", err)
	}
	hp, err := h.Health("victim")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hp != 140 {
		t.Fatalf("health after rollback = %d, want 140", hp)
	}
	if res.RestoredHP != 140 {
		t.Fatalf("rollback reported hp %d, want 140", res.RestoredHP)
	}
}

// Rolling back a killing shot must revive the victim exactly once.
func TestRollbackRevivesKilledVictim(t *testing.T) {
	h := setupArena(t, arenaConfig())
	if _, err := h.Submit(shot("shot-kill", 10, "head", 40)); err != nil {
		t.Fatalf("killing shot: %v", err)
	}
	state, err := h.Player("victim")
	if err != nil {
		t.Fatalf("Player: %v", err)
	}
	if state.HP != 0 {
		t.Fatalf("victim survived a 250 damage headshot: %d", state.HP)
	}
	res, err := h.RollbackShot("shot-kill")
	if err != nil {
		t.Fatalf("RollbackShot: %v", err)
	}
	if !res.Revived {
		t.Fatalf("rollback did not report a revive")
	}
	state, err = h.Player("victim")
	if err != nil {
		t.Fatalf("Player after rollback: %v", err)
	}
	if state.HP != 200 {
		t.Fatalf("victim hp after revive = %d, want 200", state.HP)
	}
	if _, err := h.RollbackShot("shot-kill"); !errors.Is(err, taskpkg.ErrShotNotFound) {
		t.Fatalf("second rollback error = %v, want ErrShotNotFound", err)
	}
}

// Invulnerability must absorb damage but still consume the shot id.
func TestInvulnerableAbsorbsShot(t *testing.T) {
	h := setupArena(t, arenaConfig())
	if err := h.SetInvulnerable("victim", 50); err != nil {
		t.Fatalf("SetInvulnerable: %v", err)
	}
	res, err := h.Submit(shot("shot-iframe", 10, "head", 40))
	if err != nil {
		t.Fatalf("shot during invulnerability was rejected: %v", err)
	}
	if res.AppliedDamage != 0 || !res.Absorbed {
		t.Fatalf("invulnerable hit applied %d damage, absorbed=%v", res.AppliedDamage, res.Absorbed)
	}
	hp, err := h.Health("victim")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hp != 200 {
		t.Fatalf("invulnerable victim lost hp: %d", hp)
	}
	if _, err := h.Submit(shot("shot-iframe", 20, "head", 40)); !errors.Is(err, taskpkg.ErrShotReplayed) {
		t.Fatalf("absorbed shot id was reused: %v", err)
	}
}

// Concurrent submissions of one shot id must apply damage exactly once.
func TestConcurrentSameShotSingleApplication(t *testing.T) {
	h := setupArena(t, arenaConfig())
	ev := shot("shot-race", 10, "torso", 40)
	var wg sync.WaitGroup
	results := make(chan error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := h.Submit(ev)
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	accepted := 0
	replayed := 0
	for err := range results {
		switch {
		case err == nil:
			accepted++
		case errors.Is(err, taskpkg.ErrShotReplayed):
			replayed++
		default:
			t.Fatalf("unexpected concurrent submit error: %v", err)
		}
	}
	if accepted != 1 || replayed != 7 {
		t.Fatalf("accepted=%d replayed=%d, want 1 and 7", accepted, replayed)
	}
	hp, err := h.Health("victim")
	if err != nil {
		t.Fatalf("Health: %v", err)
	}
	if hp != 100 {
		t.Fatalf("concurrent shot applied %d damage, want 100", 200-hp)
	}
}

// A restarted server must still reject already accepted shot ids.
func TestSnapshotKeepsReplayGuard(t *testing.T) {
	h := setupArena(t, arenaConfig())
	if _, err := h.Submit(shot("shot-persist", 10, "torso", 40)); err != nil {
		t.Fatalf("first shot: %v", err)
	}
	snap, err := h.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewHitRegistrationFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewHitRegistrationFromSnapshot: %v", err)
	}
	if _, err := restored.Submit(shot("shot-persist", 30, "torso", 40)); !errors.Is(err, taskpkg.ErrShotReplayed) {
		t.Fatalf("replayed shot after restart error = %v, want ErrShotReplayed", err)
	}
	hp, err := restored.Health("victim")
	if err != nil {
		t.Fatalf("Health after restore: %v", err)
	}
	if hp != 100 {
		t.Fatalf("restored health = %d, want 100", hp)
	}
}
