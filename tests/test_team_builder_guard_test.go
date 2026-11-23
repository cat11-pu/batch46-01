package task

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-b-01"
)

func baseRules() taskpkg.Rules {
	return taskpkg.Rules{
		Size:           5,
		MaxPerRole:     map[string]int{"tank": 1, "support": 2, "carry": 2},
		BannedHeroes:   []string{"pudge"},
		RequireCaptain: true,
	}
}

func newMember(playerID, heroID, role string, ready bool) taskpkg.Member {
	return taskpkg.Member{PlayerID: playerID, HeroID: heroID, Role: role, Ready: ready}
}

func find(t *testing.T, b *taskpkg.TeamBuilder, playerID string) taskpkg.Member {
	t.Helper()
	for _, m := range b.Roster() {
		if m.PlayerID == playerID {
			return m
		}
	}
	t.Fatalf("roster has no member %q", playerID)
	return taskpkg.Member{}
}

// A player id can appear only once in a roster.
func TestAddMemberRejectsDuplicatePlayer(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("first AddMember: %v", err)
	}
	if err := b.AddMember(newMember("p1", "cm", "support", true)); !errors.Is(err, taskpkg.ErrDuplicateMember) {
		t.Fatalf("duplicate player id must yield ErrDuplicateMember, got %v", err)
	}
	if got := b.Roster(); len(got) != 1 {
		t.Fatalf("roster after rejected duplicate holds %d members, want 1", len(got))
	}
}

// Two members of one team cannot lock the same hero.
func TestAddMemberRejectsDuplicateHero(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("first AddMember: %v", err)
	}
	if err := b.AddMember(newMember("p2", "axe", "carry", true)); !errors.Is(err, taskpkg.ErrDuplicateHero) {
		t.Fatalf("duplicate hero must yield ErrDuplicateHero, got %v", err)
	}
	if got := b.Roster(); len(got) != 1 {
		t.Fatalf("roster after rejected hero holds %d members, want 1", len(got))
	}
}

// Heroes from the queue ban list are refused.
func TestAddMemberRejectsBannedHero(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "pudge", "tank", true)); !errors.Is(err, taskpkg.ErrHeroBanned) {
		t.Fatalf("banned hero must yield ErrHeroBanned, got %v", err)
	}
}

// The roster cannot grow past Rules.Size.
func TestAddMemberEnforcesRosterSize(t *testing.T) {
	rules := baseRules()
	rules.Size = 2
	rules.RequireCaptain = false
	b := taskpkg.NewTeamBuilder(rules)
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if err := b.AddMember(newMember("p2", "cm", "support", true)); err != nil {
		t.Fatalf("AddMember p2: %v", err)
	}
	if err := b.AddMember(newMember("p3", "luna", "carry", true)); !errors.Is(err, taskpkg.ErrRosterFull) {
		t.Fatalf("third member must yield ErrRosterFull, got %v", err)
	}
	if got := b.Roster(); len(got) != 2 {
		t.Fatalf("roster size %d, want 2", len(got))
	}
}

// A role cap from Rules.MaxPerRole is enforced at add time.
func TestAddMemberEnforcesRoleCap(t *testing.T) {
	rules := baseRules()
	rules.MaxPerRole = map[string]int{"support": 1}
	rules.RequireCaptain = false
	b := taskpkg.NewTeamBuilder(rules)
	if err := b.AddMember(newMember("p1", "cm", "support", true)); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if err := b.AddMember(newMember("p2", "wd", "support", true)); !errors.Is(err, taskpkg.ErrRoleLimit) {
		t.Fatalf("second support must yield ErrRoleLimit, got %v", err)
	}
	if got := b.Roster(); len(got) != 1 {
		t.Fatalf("roster size %d, want 1", len(got))
	}
}

// A rejected hero swap must leave the old pick untouched.
func TestSetHeroKeepsOldHeroOnConflict(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if err := b.AddMember(newMember("p2", "cm", "carry", true)); err != nil {
		t.Fatalf("AddMember p2: %v", err)
	}
	if err := b.SetHero("p1", "cm"); !errors.Is(err, taskpkg.ErrDuplicateHero) {
		t.Fatalf("swap onto a taken hero must yield ErrDuplicateHero, got %v", err)
	}
	if got := find(t, b, "p1"); got.HeroID != "axe" {
		t.Fatalf("hero after rejected swap = %q, want %q", got.HeroID, "axe")
	}
}

// Removing a member returns the removed record; unknown ids are reported.
func TestRemoveMemberReportsMissingPlayer(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	removed, err := b.RemoveMember("p1")
	if err != nil {
		t.Fatalf("RemoveMember p1: %v", err)
	}
	if removed.PlayerID != "p1" || removed.HeroID != "axe" || removed.Role != "tank" {
		t.Fatalf("removed member = %+v", removed)
	}
	if _, err := b.RemoveMember("ghost"); !errors.Is(err, taskpkg.ErrMemberNotFound) {
		t.Fatalf("removing an unknown member must yield ErrMemberNotFound, got %v", err)
	}
}

// Captaincy moves to exactly one member and reports the previous holder.
func TestSetCaptainTransfersOwnership(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if err := b.AddMember(newMember("p2", "cm", "support", true)); err != nil {
		t.Fatalf("AddMember p2: %v", err)
	}
	prev, err := b.SetCaptain("p1")
	if err != nil {
		t.Fatalf("SetCaptain p1: %v", err)
	}
	if prev != "" {
		t.Fatalf("first captain reported previous %q, want empty", prev)
	}
	prev, err = b.SetCaptain("p2")
	if err != nil {
		t.Fatalf("SetCaptain p2: %v", err)
	}
	if prev != "p1" {
		t.Fatalf("SetCaptain p2 reported previous %q, want p1", prev)
	}
	captains := 0
	for _, m := range b.Roster() {
		if m.Captain {
			captains++
			if m.PlayerID != "p2" {
				t.Fatalf("captain flag sits on %q, want p2", m.PlayerID)
			}
		}
	}
	if captains != 1 {
		t.Fatalf("roster has %d captains, want 1", captains)
	}
}

// Rejected or idempotent calls must not advance the revision.
func TestRevisionAdvancesOnlyOnSuccess(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if got := b.Revision(); got != 0 {
		t.Fatalf("fresh roster revision = %d, want 0", got)
	}
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if got := b.Revision(); got != 1 {
		t.Fatalf("revision after add = %d, want 1", got)
	}
	if err := b.AddMember(newMember("p1", "cm", "support", true)); err == nil {
		t.Fatal("duplicate add unexpectedly succeeded")
	}
	if got := b.Revision(); got != 1 {
		t.Fatalf("revision after rejected add = %d, want 1", got)
	}
	if err := b.SetHero("p1", "axe"); err != nil {
		t.Fatalf("SetHero to the same hero must be a no-op, got %v", err)
	}
	if err := b.SetReady("p1", true); err != nil {
		t.Fatalf("SetReady to the same value must be a no-op, got %v", err)
	}
	if got := b.Revision(); got != 1 {
		t.Fatalf("revision after no-op calls = %d, want 1", got)
	}
	if _, err := b.SetCaptain("p1"); err != nil {
		t.Fatalf("SetCaptain: %v", err)
	}
	if got := b.Revision(); got != 2 {
		t.Fatalf("revision after captain change = %d, want 2", got)
	}
	if _, err := b.RemoveMember("p1"); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if got := b.Revision(); got != 3 {
		t.Fatalf("revision after removal = %d, want 3", got)
	}
}

// A replace built on an old revision is refused without touching the roster.
func TestReplaceAllRejectsStaleRevision(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	stale := []taskpkg.Member{
		newMember("p2", "axe", "tank", true),
		newMember("p3", "cm", "support", true),
	}
	if err := b.ReplaceAll(stale, 0); !errors.Is(err, taskpkg.ErrStaleRevision) {
		t.Fatalf("stale revision must yield ErrStaleRevision, got %v", err)
	}
	roster := b.Roster()
	if len(roster) != 1 || roster[0].PlayerID != "p1" {
		t.Fatalf("roster changed after stale replace: %+v", roster)
	}
}

// ReplaceAll applies every member or none of them.
func TestReplaceAllIsAllOrNothing(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if _, err := b.SetCaptain("p1"); err != nil {
		t.Fatalf("SetCaptain: %v", err)
	}
	rev := b.Revision()
	bad := []taskpkg.Member{
		newMember("p2", "axe", "tank", true),
		newMember("p3", "axe", "support", true),
	}
	if err := b.ReplaceAll(bad, rev); !errors.Is(err, taskpkg.ErrDuplicateHero) {
		t.Fatalf("replace with duplicate heroes must yield ErrDuplicateHero, got %v", err)
	}
	roster := b.Roster()
	if len(roster) != 1 || roster[0].PlayerID != "p1" {
		t.Fatalf("roster changed after rejected replace: %+v", roster)
	}
	if got := b.Revision(); got != rev {
		t.Fatalf("revision after rejected replace = %d, want %d", got, rev)
	}
}

// Validate reports the not-ready member even when the rest is legal.
func TestValidateRejectsNotReadyMember(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if err := b.AddMember(newMember("p2", "cm", "support", false)); err != nil {
		t.Fatalf("AddMember p2: %v", err)
	}
	prev, err := b.SetCaptain("p1")
	if err != nil {
		t.Fatalf("SetCaptain: %v", err)
	}
	if prev != "" {
		t.Fatalf("unexpected previous captain %q", prev)
	}
	if err := b.Validate(); !errors.Is(err, taskpkg.ErrNotReady) {
		t.Fatalf("validate with an unready member must yield ErrNotReady, got %v", err)
	}
	if err := b.SetReady("p2", true); err != nil {
		t.Fatalf("SetReady p2: %v", err)
	}
	if err := b.Validate(); err != nil {
		t.Fatalf("validate on a legal roster failed: %v", err)
	}
}

// A snapshot keeps roster, captaincy and revision across a restart.
func TestSnapshotRestorePreservesRosterAndRevision(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if err := b.AddMember(newMember("p2", "cm", "support", true)); err != nil {
		t.Fatalf("AddMember p2: %v", err)
	}
	if _, err := b.SetCaptain("p2"); err != nil {
		t.Fatalf("SetCaptain: %v", err)
	}
	rev := b.Revision()
	want := b.Roster()
	snap, err := b.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap) == 0 {
		t.Fatal("Snapshot returned no bytes")
	}
	restored, err := taskpkg.NewTeamBuilderFromSnapshot(baseRules(), snap)
	if err != nil {
		t.Fatalf("NewTeamBuilderFromSnapshot: %v", err)
	}
	if got := restored.Roster(); !reflect.DeepEqual(got, want) {
		t.Fatalf("restored roster = %+v, want %+v", got, want)
	}
	if got := restored.Revision(); got != rev {
		t.Fatalf("restored revision = %d, want %d", got, rev)
	}
	if err := restored.ReplaceAll([]taskpkg.Member{newMember("p3", "luna", "carry", true)}, rev-1); !errors.Is(err, taskpkg.ErrStaleRevision) {
		t.Fatalf("restored roster must keep revision guards, got %v", err)
	}
	if _, err := taskpkg.NewTeamBuilderFromSnapshot(baseRules(), []byte("{not-json")); !errors.Is(err, taskpkg.ErrRosterInvalid) {
		t.Fatalf("corrupt snapshot must yield ErrRosterInvalid, got %v", err)
	}
	if _, err := taskpkg.NewTeamBuilderFromSnapshot(baseRules(), make([]byte, taskpkg.MaxSnapshotBytes+1)); !errors.Is(err, taskpkg.ErrRosterInvalid) {
		t.Fatalf("oversized snapshot must yield ErrRosterInvalid, got %v", err)
	}
}

// Concurrent adds must all land and count once each.
func TestConcurrentAddMemberCountsExactly(t *testing.T) {
	rules := taskpkg.Rules{Size: 100}
	b := taskpkg.NewTeamBuilder(rules)
	const workers = 64
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = b.AddMember(taskpkg.Member{
				PlayerID: fmt.Sprintf("p%02d", idx),
				HeroID:   fmt.Sprintf("hero-%02d", idx),
				Role:     "carry",
				Ready:    true,
			})
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent AddMember %d failed: %v", i, err)
		}
	}
	if got := b.Revision(); got != workers {
		t.Fatalf("revision after %d adds = %d, want %d", workers, got, workers)
	}
	if got := len(b.Roster()); got != workers {
		t.Fatalf("roster holds %d members, want %d", got, workers)
	}
}

// Roster is sorted and hands out copies, not the live slice.
func TestRosterReturnsSortedDefensiveCopy(t *testing.T) {
	b := taskpkg.NewTeamBuilder(baseRules())
	if err := b.AddMember(newMember("p3", "luna", "carry", true)); err != nil {
		t.Fatalf("AddMember p3: %v", err)
	}
	if err := b.AddMember(newMember("p1", "axe", "tank", true)); err != nil {
		t.Fatalf("AddMember p1: %v", err)
	}
	if err := b.AddMember(newMember("p2", "cm", "support", true)); err != nil {
		t.Fatalf("AddMember p2: %v", err)
	}
	got := b.Roster()
	if len(got) != 3 {
		t.Fatalf("roster size %d, want 3", len(got))
	}
	for i, want := range []string{"p1", "p2", "p3"} {
		if got[i].PlayerID != want {
			t.Fatalf("roster[%d] = %q, want %q", i, got[i].PlayerID, want)
		}
	}
	got[0].PlayerID = "hacked"
	got[0].HeroID = "hacked"
	again := b.Roster()
	if len(again) != 3 || again[0].PlayerID != "p1" || again[0].HeroID != "axe" {
		t.Fatalf("roster copy leaked into internal state: %+v", again)
	}
}
