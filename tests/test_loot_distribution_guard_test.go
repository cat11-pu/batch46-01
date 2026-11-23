package task

import (
	"errors"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-a-04"
)

func lootCfg() taskpkg.Config {
	return taskpkg.Config{MaxItemsPerMember: 2, GuildBankID: "guild-bank"}
}

func raidMembers() []taskpkg.Member {
	return []taskpkg.Member{
		{ID: "alice", Seniority: 5},
		{ID: "bob", Seniority: 3},
		{ID: "cara", Seniority: 1},
		{ID: "dave", Seniority: 2},
		{ID: "erin", Seniority: 2},
	}
}

func swordItem() taskpkg.Item {
	return taskpkg.Item{ID: "itm-sword", Name: "Epic Sword", Qty: 1, MinSeniority: 3}
}

func ringItem() taskpkg.Item {
	return taskpkg.Item{ID: "itm-ring", Name: "Signet Ring", Qty: 1, MinSeniority: 2}
}

func gemItem() taskpkg.Item {
	return taskpkg.Item{ID: "itm-gem", Name: "Gem Cluster", Qty: 2, MinSeniority: 0}
}

func need(member, item string, roll int) taskpkg.Claim {
	return taskpkg.Claim{MemberID: member, ItemID: item, Choice: taskpkg.ChoiceNeed, Roll: roll}
}

func greed(member, item string, roll int) taskpkg.Claim {
	return taskpkg.Claim{MemberID: member, ItemID: item, Choice: taskpkg.ChoiceGreed, Roll: roll}
}

func pass(member, item string) taskpkg.Claim {
	return taskpkg.Claim{MemberID: member, ItemID: item, Choice: taskpkg.ChoicePass}
}

func newLoot(t *testing.T) (*taskpkg.LootDistributor, string) {
	t.Helper()
	d := taskpkg.NewLootDistributor(lootCfg())
	if err := d.OpenRun(taskpkg.Run{RunID: "run-42", Members: raidMembers()}); err != nil {
		t.Fatalf("OpenRun: %v", err)
	}
	return d, "run-42"
}

func addLoot(t *testing.T, d *taskpkg.LootDistributor, runID string, items ...taskpkg.Item) {
	t.Helper()
	if err := d.AddLoot(runID, items); err != nil {
		t.Fatalf("AddLoot: %v", err)
	}
}

func claim(t *testing.T, d *taskpkg.LootDistributor, runID string, c taskpkg.Claim) {
	t.Helper()
	if err := d.SubmitClaim(runID, c); err != nil {
		t.Fatalf("SubmitClaim %+v: %v", c, err)
	}
}

func resolve(t *testing.T, d *taskpkg.LootDistributor, runID string) []taskpkg.Award {
	t.Helper()
	awards, err := d.Resolve(runID)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	return awards
}

// Opening a raid validates members and run ids, and unknown runs are reported.
func TestOpenRunValidation(t *testing.T) {
	d := taskpkg.NewLootDistributor(lootCfg())
	if err := d.OpenRun(taskpkg.Run{RunID: "run-empty"}); !errors.Is(err, taskpkg.ErrEmptyRaid) {
		t.Fatalf("empty raid = %v, want ErrEmptyRaid", err)
	}
	dup := taskpkg.Run{RunID: "run-dup", Members: []taskpkg.Member{{ID: "alice", Seniority: 5}, {ID: "alice", Seniority: 3}}}
	if err := d.OpenRun(dup); !errors.Is(err, taskpkg.ErrDuplicateMember) {
		t.Fatalf("duplicate member = %v, want ErrDuplicateMember", err)
	}
	d, runID := newLoot(t)
	if err := d.OpenRun(taskpkg.Run{RunID: runID, Members: raidMembers()}); !errors.Is(err, taskpkg.ErrRunExists) {
		t.Fatalf("duplicate run = %v, want ErrRunExists", err)
	}
	if err := d.AddLoot("run-missing", []taskpkg.Item{swordItem()}); !errors.Is(err, taskpkg.ErrRunNotFound) {
		t.Fatalf("AddLoot on an unknown run = %v, want ErrRunNotFound", err)
	}
	if _, err := d.Resolve("run-missing"); !errors.Is(err, taskpkg.ErrRunNotFound) {
		t.Fatalf("Resolve on an unknown run = %v, want ErrRunNotFound", err)
	}
	if err := d.SubmitClaim("run-missing", need("alice", "itm-sword", 50)); !errors.Is(err, taskpkg.ErrRunNotFound) {
		t.Fatalf("SubmitClaim on an unknown run = %v, want ErrRunNotFound", err)
	}
	if err := d.CloseRun("run-missing"); !errors.Is(err, taskpkg.ErrRunNotFound) {
		t.Fatalf("CloseRun on an unknown run = %v, want ErrRunNotFound", err)
	}
}

// Loot ids are unique per run and quantities must be positive.
func TestAddLootValidation(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem(), ringItem())
	if err := d.AddLoot(runID, []taskpkg.Item{swordItem()}); !errors.Is(err, taskpkg.ErrDuplicateItem) {
		t.Fatalf("duplicate item id = %v, want ErrDuplicateItem", err)
	}
	zero := taskpkg.Item{ID: "itm-zero", Name: "Empty", Qty: 0}
	if err := d.AddLoot(runID, []taskpkg.Item{zero}); !errors.Is(err, taskpkg.ErrInvalidQuantity) {
		t.Fatalf("zero quantity = %v, want ErrInvalidQuantity", err)
	}
	negative := taskpkg.Item{ID: "itm-neg", Name: "Debt", Qty: -2}
	if err := d.AddLoot(runID, []taskpkg.Item{negative}); !errors.Is(err, taskpkg.ErrInvalidQuantity) {
		t.Fatalf("negative quantity = %v, want ErrInvalidQuantity", err)
	}
	awards, err := d.Awards(runID)
	if err != nil {
		t.Fatalf("Awards before resolve: %v", err)
	}
	if len(awards) != 0 {
		t.Fatalf("Awards before resolve = %+v, want none", awards)
	}
}

// Claims must reference raid members and dropped items.
func TestClaimRequiresKnownMemberAndItem(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem())
	if err := d.SubmitClaim(runID, need("zoe", "itm-sword", 50)); !errors.Is(err, taskpkg.ErrUnknownMember) {
		t.Fatalf("claim from an outsider = %v, want ErrUnknownMember", err)
	}
	if err := d.SubmitClaim(runID, need("alice", "itm-ghost", 50)); !errors.Is(err, taskpkg.ErrUnknownItem) {
		t.Fatalf("claim for an unknown item = %v, want ErrUnknownItem", err)
	}
	claim(t, d, runID, need("alice", "itm-sword", 50))
	if err := d.SubmitClaim(runID, greed("alice", "itm-sword", 60)); !errors.Is(err, taskpkg.ErrDuplicateClaim) {
		t.Fatalf("second claim for the same item = %v, want ErrDuplicateClaim", err)
	}
}

// Choice and roll values are validated, and a pass needs no roll.
func TestClaimValidation(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem(), ringItem())
	if err := d.SubmitClaim(runID, taskpkg.Claim{MemberID: "alice", ItemID: "itm-sword", Choice: taskpkg.Choice(9), Roll: 50}); !errors.Is(err, taskpkg.ErrInvalidChoice) {
		t.Fatalf("invalid choice = %v, want ErrInvalidChoice", err)
	}
	if err := d.SubmitClaim(runID, need("alice", "itm-sword", 0)); !errors.Is(err, taskpkg.ErrInvalidRoll) {
		t.Fatalf("need roll 0 = %v, want ErrInvalidRoll", err)
	}
	if err := d.SubmitClaim(runID, greed("alice", "itm-sword", 101)); !errors.Is(err, taskpkg.ErrInvalidRoll) {
		t.Fatalf("greed roll 101 = %v, want ErrInvalidRoll", err)
	}
	if err := d.SubmitClaim(runID, pass("alice", "itm-sword")); err != nil {
		t.Fatalf("pass claim: %v", err)
	}
	claims, err := d.MemberClaims(runID, "alice")
	if err != nil {
		t.Fatalf("MemberClaims: %v", err)
	}
	if len(claims) != 1 || claims[0].Choice != taskpkg.ChoicePass {
		t.Fatalf("MemberClaims = %+v, want the recorded pass", claims)
	}
}

// A member below the item seniority may greed but never need.
func TestNeedEligibilityBySeniority(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem())
	if err := d.SubmitClaim(runID, need("cara", "itm-sword", 60)); !errors.Is(err, taskpkg.ErrNotEligible) {
		t.Fatalf("need claim from cara = %v, want ErrNotEligible", err)
	}
	claim(t, d, runID, greed("cara", "itm-sword", 60))
	claim(t, d, runID, need("alice", "itm-sword", 55))
	awards := resolve(t, d, runID)
	if len(awards) != 1 {
		t.Fatalf("awards = %+v, want one copy", awards)
	}
	if awards[0].MemberID != "alice" || awards[0].Reason != "need" {
		t.Fatalf("award = %+v, want alice by need", awards[0])
	}
}

// A need claim beats a much higher greed roll.
func TestNeedBeatsGreed(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem())
	claim(t, d, runID, greed("bob", "itm-sword", 95))
	claim(t, d, runID, need("alice", "itm-sword", 40))
	awards := resolve(t, d, runID)
	if len(awards) != 1 {
		t.Fatalf("awards = %+v, want one copy", awards)
	}
	if awards[0].MemberID != "alice" || awards[0].Roll != 40 || awards[0].Reason != "need" {
		t.Fatalf("award = %+v, want alice with roll 40 by need", awards[0])
	}
}

// Among need claims the highest roll wins.
func TestHighestRollWinsNeed(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, ringItem())
	claim(t, d, runID, need("alice", "itm-ring", 55))
	claim(t, d, runID, need("bob", "itm-ring", 90))
	claim(t, d, runID, need("dave", "itm-ring", 70))
	awards := resolve(t, d, runID)
	if len(awards) != 1 {
		t.Fatalf("awards = %+v, want one copy", awards)
	}
	if awards[0].MemberID != "bob" || awards[0].Roll != 90 {
		t.Fatalf("award = %+v, want bob with roll 90", awards[0])
	}
}

// Equal rolls fall back to seniority and then to the member id.
func TestTieBrokenBySeniorityThenMemberID(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem(), ringItem())
	claim(t, d, runID, need("alice", "itm-sword", 70))
	claim(t, d, runID, need("bob", "itm-sword", 70))
	claim(t, d, runID, need("dave", "itm-ring", 60))
	claim(t, d, runID, need("erin", "itm-ring", 60))
	awards := resolve(t, d, runID)
	if len(awards) != 2 {
		t.Fatalf("awards = %+v, want two copies", awards)
	}
	if awards[0].MemberID != "alice" {
		t.Fatalf("sword tie winner = %s, want alice by seniority", awards[0].MemberID)
	}
	if awards[1].MemberID != "dave" {
		t.Fatalf("ring tie winner = %s, want dave by member id", awards[1].MemberID)
	}
}

// Greed claims decide the item only when nobody needs it.
func TestGreedWinsWhenNoNeed(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, ringItem())
	claim(t, d, runID, greed("cara", "itm-ring", 30))
	claim(t, d, runID, greed("bob", "itm-ring", 80))
	awards := resolve(t, d, runID)
	if len(awards) != 1 {
		t.Fatalf("awards = %+v, want one copy", awards)
	}
	if awards[0].MemberID != "bob" || awards[0].Reason != "greed" {
		t.Fatalf("award = %+v, want bob by greed", awards[0])
	}
}

// Copies nobody claims go to the guild bank.
func TestUnclaimedItemGoesToGuildBank(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem())
	claim(t, d, runID, pass("alice", "itm-sword"))
	awards := resolve(t, d, runID)
	if len(awards) != 1 {
		t.Fatalf("awards = %+v, want one copy", awards)
	}
	if awards[0].MemberID != "guild-bank" || awards[0].Reason != "bank" {
		t.Fatalf("award = %+v, want the guild bank", awards[0])
	}
}

// A stack with two copies is split between the two best claimants.
func TestQuantityAwardsDistinctMembers(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, gemItem())
	claim(t, d, runID, need("alice", "itm-gem", 90))
	claim(t, d, runID, need("bob", "itm-gem", 70))
	claim(t, d, runID, need("cara", "itm-gem", 60))
	awards := resolve(t, d, runID)
	if len(awards) != 2 {
		t.Fatalf("awards = %+v, want two copies", awards)
	}
	if awards[0].MemberID != "alice" || awards[1].MemberID != "bob" {
		t.Fatalf("awards = %+v, want alice then bob", awards)
	}
	if awards[0].MemberID == awards[1].MemberID {
		t.Fatalf("both copies went to %s", awards[0].MemberID)
	}
}

// The per-member cap is applied in loot order and hands the copy down.
func TestMaxItemsPerMemberRespected(t *testing.T) {
	d := taskpkg.NewLootDistributor(taskpkg.Config{MaxItemsPerMember: 1, GuildBankID: "guild-bank"})
	if err := d.OpenRun(taskpkg.Run{RunID: "run-cap", Members: raidMembers()}); err != nil {
		t.Fatalf("OpenRun: %v", err)
	}
	addLoot(t, d, "run-cap", swordItem(), ringItem())
	claim(t, d, "run-cap", need("alice", "itm-sword", 99))
	claim(t, d, "run-cap", need("bob", "itm-sword", 50))
	claim(t, d, "run-cap", need("alice", "itm-ring", 99))
	claim(t, d, "run-cap", need("bob", "itm-ring", 80))
	awards := resolve(t, d, "run-cap")
	if len(awards) != 2 {
		t.Fatalf("awards = %+v, want two copies", awards)
	}
	if awards[0].MemberID != "alice" {
		t.Fatalf("sword went to %s, want alice", awards[0].MemberID)
	}
	if awards[1].MemberID != "bob" {
		t.Fatalf("ring went to %s, want bob because alice is capped", awards[1].MemberID)
	}
	count, err := d.AwardCount("run-cap", "alice")
	if err != nil {
		t.Fatalf("AwardCount alice: %v", err)
	}
	if count != 1 {
		t.Fatalf("alice award count = %d, want 1", count)
	}
}

// Resolution is deterministic under concurrency and cannot be repeated with
// different claims.
func TestResolveIsIdempotentUnderConcurrency(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem(), gemItem())
	claim(t, d, runID, need("alice", "itm-sword", 70))
	claim(t, d, runID, greed("bob", "itm-sword", 90))
	claim(t, d, runID, need("cara", "itm-gem", 40))
	claim(t, d, runID, greed("dave", "itm-gem", 50))
	results := make([][]taskpkg.Award, 8)
	errs := make([]error, 8)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = d.Resolve(runID)
		}(i)
	}
	wg.Wait()
	for i := 0; i < 8; i++ {
		if errs[i] != nil {
			t.Fatalf("concurrent Resolve %d: %v", i, errs[i])
		}
		if len(results[i]) != len(results[0]) {
			t.Fatalf("concurrent Resolve returned %d and %d awards", len(results[i]), len(results[0]))
		}
		for j := range results[0] {
			if results[i][j] != results[0][j] {
				t.Fatalf("concurrent Resolve differs at %d: %+v vs %+v", j, results[i][j], results[0][j])
			}
		}
	}
	if err := d.SubmitClaim(runID, need("erin", "itm-sword", 50)); !errors.Is(err, taskpkg.ErrAlreadyResolved) {
		t.Fatalf("claim after resolve = %v, want ErrAlreadyResolved", err)
	}
	if err := d.AddLoot(runID, []taskpkg.Item{{ID: "itm-late", Name: "Late", Qty: 1}}); !errors.Is(err, taskpkg.ErrAlreadyResolved) {
		t.Fatalf("loot after resolve = %v, want ErrAlreadyResolved", err)
	}
}

// Closing resolves the run, rejects late claims and keeps awards readable.
func TestCloseRunRejectsLateClaims(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem())
	claim(t, d, runID, need("alice", "itm-sword", 60))
	if err := d.CloseRun(runID); err != nil {
		t.Fatalf("CloseRun: %v", err)
	}
	if !d.RunClosed(runID) {
		t.Fatal("RunClosed = false after CloseRun")
	}
	if err := d.SubmitClaim(runID, greed("bob", "itm-sword", 99)); !errors.Is(err, taskpkg.ErrRunClosed) {
		t.Fatalf("claim after close = %v, want ErrRunClosed", err)
	}
	if err := d.AddLoot(runID, []taskpkg.Item{{ID: "itm-late", Name: "Late", Qty: 1}}); !errors.Is(err, taskpkg.ErrRunClosed) {
		t.Fatalf("loot after close = %v, want ErrRunClosed", err)
	}
	awards, err := d.Awards(runID)
	if err != nil {
		t.Fatalf("Awards after close: %v", err)
	}
	if len(awards) != 1 || awards[0].MemberID != "alice" {
		t.Fatalf("awards after close = %+v, want alice", awards)
	}
	if err := d.CloseRun(runID); err != nil {
		t.Fatalf("second CloseRun: %v", err)
	}
}

// A restart keeps awards, counts and the closed state.
func TestSnapshotRestoresAwardsAndCaps(t *testing.T) {
	d, runID := newLoot(t)
	addLoot(t, d, runID, swordItem(), gemItem())
	claim(t, d, runID, need("alice", "itm-sword", 80))
	claim(t, d, runID, need("bob", "itm-sword", 40))
	claim(t, d, runID, need("alice", "itm-gem", 70))
	claim(t, d, runID, need("cara", "itm-gem", 50))
	before := resolve(t, d, runID)
	if err := d.CloseRun(runID); err != nil {
		t.Fatalf("CloseRun: %v", err)
	}
	snap, err := d.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewLootDistributorFromSnapshot(lootCfg(), snap)
	if err != nil {
		t.Fatalf("NewLootDistributorFromSnapshot: %v", err)
	}
	after, err := restored.Awards(runID)
	if err != nil {
		t.Fatalf("restored Awards: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("restored awards = %d, want %d", len(after), len(before))
	}
	for i := range before {
		if after[i] != before[i] {
			t.Fatalf("restored award %d = %+v, want %+v", i, after[i], before[i])
		}
	}
	count, err := restored.AwardCount(runID, "alice")
	if err != nil {
		t.Fatalf("restored AwardCount: %v", err)
	}
	if count != 2 {
		t.Fatalf("restored alice count = %d, want 2", count)
	}
	if !restored.RunClosed(runID) {
		t.Fatal("restored run is not closed")
	}
	if err := restored.SubmitClaim(runID, need("bob", "itm-sword", 90)); !errors.Is(err, taskpkg.ErrRunClosed) {
		t.Fatalf("claim after restart = %v, want ErrRunClosed", err)
	}
	again, err := restored.Resolve(runID)
	if err != nil {
		t.Fatalf("Resolve after restart: %v", err)
	}
	for i := range before {
		if again[i] != before[i] {
			t.Fatalf("re-resolve after restart differs at %d: %+v vs %+v", i, again[i], before[i])
		}
	}
}
