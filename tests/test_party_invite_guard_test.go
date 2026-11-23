package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-b-09"
)

var partyNow = time.Date(2026, 6, 1, 19, 0, 0, 0, time.UTC)

func partyCfg() taskpkg.Config {
	return taskpkg.Config{
		MaxSize:              4,
		InviteTTL:            2 * time.Minute,
		Cooldown:             30 * time.Second,
		MaxPendingPerInvitee: 2,
	}
}

func newParties(t *testing.T) *taskpkg.PartyInviteGuard {
	t.Helper()
	g := taskpkg.NewPartyInviteGuard(partyCfg())
	if err := g.CreateParty("p-1", "alice", "east", partyNow); err != nil {
		t.Fatalf("CreateParty p-1: %v", err)
	}
	if err := g.CreateParty("p-2", "dave", "east", partyNow); err != nil {
		t.Fatalf("CreateParty p-2: %v", err)
	}
	if err := g.CreateParty("p-3", "erin", "east", partyNow); err != nil {
		t.Fatalf("CreateParty p-3: %v", err)
	}
	return g
}

func mustInvite(t *testing.T, g *taskpkg.PartyInviteGuard, partyID, inviter, invitee, realm string, at time.Time) taskpkg.Invite {
	t.Helper()
	inv, err := g.Invite(partyID, inviter, invitee, realm, at)
	if err != nil {
		t.Fatalf("Invite(%s -> %s): %v", inviter, invitee, err)
	}
	return inv
}

func mustAccept(t *testing.T, g *taskpkg.PartyInviteGuard, inviteID string, at time.Time) taskpkg.Party {
	t.Helper()
	p, err := g.Accept(inviteID, at)
	if err != nil {
		t.Fatalf("Accept(%s): %v", inviteID, err)
	}
	return p
}

func TestInviteRequiresLeader(t *testing.T) {
	g := newParties(t)
	_, err := g.Invite("p-1", "bob", "carol", "east", partyNow.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrNotLeader) {
		t.Fatalf("non-leader invite must yield ErrNotLeader, got %v", err)
	}
	if got := len(g.PendingFor("carol")); got != 0 {
		t.Fatalf("rejected invite produced %d pending invites", got)
	}
}

func TestSelfInviteRejected(t *testing.T) {
	g := newParties(t)
	_, err := g.Invite("p-1", "alice", "alice", "east", partyNow.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrSamePlayer) {
		t.Fatalf("self invite must yield ErrSamePlayer, got %v", err)
	}
}

func TestPartyFullRejected(t *testing.T) {
	g := newParties(t)
	for i, name := range []string{"bob", "carol", "frank"} {
		at := partyNow.Add(time.Duration(i*2+1) * time.Second)
		inv := mustInvite(t, g, "p-1", "alice", name, "east", at)
		mustAccept(t, g, inv.ID, at.Add(time.Second))
	}
	p, ok := g.PartyOf("alice")
	if !ok || len(p.Members) != 4 {
		t.Fatalf("party size = %d, want 4", len(p.Members))
	}
	_, err := g.Invite("p-1", "alice", "zoe", "east", partyNow.Add(20*time.Second))
	if !errors.Is(err, taskpkg.ErrPartyFull) {
		t.Fatalf("invite to a full party must yield ErrPartyFull, got %v", err)
	}
}

func TestDuplicatePendingInviteRejected(t *testing.T) {
	g := newParties(t)
	mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	_, err := g.Invite("p-1", "alice", "carol", "east", partyNow.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrInvitePending) {
		t.Fatalf("duplicate pending invite must yield ErrInvitePending, got %v", err)
	}
	if got := len(g.PendingFor("carol")); got != 1 {
		t.Fatalf("pending invites = %d, want 1", got)
	}
}

func TestDeclineStartsCooldown(t *testing.T) {
	g := newParties(t)
	inv := mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	if err := g.Decline(inv.ID, partyNow.Add(2*time.Second)); err != nil {
		t.Fatalf("Decline: %v", err)
	}
	_, err := g.Invite("p-1", "alice", "carol", "east", partyNow.Add(3*time.Second))
	if !errors.Is(err, taskpkg.ErrCooldown) {
		t.Fatalf("immediate re-invite after decline must yield ErrCooldown, got %v", err)
	}
	if _, err := g.Invite("p-1", "alice", "carol", "east", partyNow.Add(33*time.Second)); err != nil {
		t.Fatalf("re-invite after the cooldown: %v", err)
	}
}

func TestInviteeBusyRejected(t *testing.T) {
	g := newParties(t)
	inv := mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	mustAccept(t, g, inv.ID, partyNow.Add(time.Second))
	_, err := g.Invite("p-2", "dave", "carol", "east", partyNow.Add(2*time.Second))
	if !errors.Is(err, taskpkg.ErrPlayerBusy) {
		t.Fatalf("inviting a grouped player must yield ErrPlayerBusy, got %v", err)
	}
}

func TestInboxCapEnforced(t *testing.T) {
	g := newParties(t)
	mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	mustInvite(t, g, "p-2", "dave", "carol", "east", partyNow.Add(time.Second))
	_, err := g.Invite("p-3", "erin", "carol", "east", partyNow.Add(2*time.Second))
	if !errors.Is(err, taskpkg.ErrInboxFull) {
		t.Fatalf("third pending invite must yield ErrInboxFull, got %v", err)
	}
	if got := len(g.PendingFor("carol")); got != 2 {
		t.Fatalf("pending invites = %d, want 2", got)
	}
}

func TestCrossRealmInviteRejected(t *testing.T) {
	g := newParties(t)
	_, err := g.Invite("p-1", "alice", "frank", "west", partyNow)
	if !errors.Is(err, taskpkg.ErrCrossRealm) {
		t.Fatalf("cross realm invite must yield ErrCrossRealm, got %v", err)
	}
}

func TestAcceptExpiredInviteRejected(t *testing.T) {
	g := newParties(t)
	inv := mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	_, err := g.Accept(inv.ID, partyNow.Add(121*time.Second))
	if !errors.Is(err, taskpkg.ErrInviteExpired) {
		t.Fatalf("expired invite must yield ErrInviteExpired, got %v", err)
	}
	if got := len(g.PendingFor("carol")); got != 0 {
		t.Fatalf("expired invite must be removed, %d pending left", got)
	}
}

func TestAcceptJoinsPartyAndClearsPending(t *testing.T) {
	g := newParties(t)
	first := mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	second := mustInvite(t, g, "p-2", "dave", "carol", "east", partyNow.Add(time.Second))
	p := mustAccept(t, g, first.ID, partyNow.Add(2*time.Second))
	if len(p.Members) != 2 || p.Members[0] != "alice" || p.Members[1] != "carol" {
		t.Fatalf("members after accept = %v, want [alice carol]", p.Members)
	}
	if got := len(g.PendingFor("carol")); got != 0 {
		t.Fatalf("accept must clear other pending invites, %d left", got)
	}
	if _, err := g.Accept(second.ID, partyNow.Add(3*time.Second)); !errors.Is(err, taskpkg.ErrInviteNotFound) {
		t.Fatalf("invite cleared by accept must be gone, got %v", err)
	}
	if got, ok := g.PartyOf("carol"); !ok || len(got.Members) != 2 {
		t.Fatalf("PartyOf(carol) = %+v ok=%v", got, ok)
	}
}

func TestPendingForSorted(t *testing.T) {
	g := newParties(t)
	oldest := mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	newer := mustInvite(t, g, "p-2", "dave", "carol", "east", partyNow.Add(5*time.Second))
	mustInvite(t, g, "p-1", "alice", "zoe", "east", partyNow.Add(time.Second))
	pending := g.PendingFor("carol")
	if len(pending) != 2 {
		t.Fatalf("carol pending = %d, want 2", len(pending))
	}
	if pending[0].ID != oldest.ID || pending[1].ID != newer.ID {
		t.Fatalf("pending order = [%s %s], want oldest first", pending[0].ID, pending[1].ID)
	}
	if got := len(g.PendingFor("zoe")); got != 1 {
		t.Fatalf("zoe pending = %d, want 1", got)
	}
}

func TestExpirePurgesInvites(t *testing.T) {
	g := newParties(t)
	mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	if got := g.Expire(partyNow.Add(121 * time.Second)); got != 1 {
		t.Fatalf("expired invites = %d, want 1", got)
	}
	if got := len(g.PendingFor("carol")); got != 0 {
		t.Fatalf("pending after expire = %d, want 0", got)
	}
	if _, err := g.Invite("p-1", "alice", "carol", "east", partyNow.Add(122*time.Second)); err != nil {
		t.Fatalf("re-invite after expiry must be allowed, got %v", err)
	}
}

func TestLeaderLeaveTransfersLeadership(t *testing.T) {
	g := newParties(t)
	bobInv := mustInvite(t, g, "p-1", "alice", "bob", "east", partyNow.Add(time.Second))
	mustAccept(t, g, bobInv.ID, partyNow.Add(2*time.Second))
	carolInv := mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow.Add(3*time.Second))
	mustAccept(t, g, carolInv.ID, partyNow.Add(4*time.Second))
	if err := g.Leave("alice", partyNow.Add(5*time.Second)); err != nil {
		t.Fatalf("Leave: %v", err)
	}
	leader, err := g.Leader("p-1")
	if err != nil {
		t.Fatalf("Leader: %v", err)
	}
	if leader != "bob" {
		t.Fatalf("leader after the founder left = %q, want bob", leader)
	}
	if _, ok := g.PartyOf("alice"); ok {
		t.Fatalf("alice must not belong to a party after leaving")
	}
	p, ok := g.PartyOf("bob")
	if !ok || len(p.Members) != 2 || p.Members[0] != "bob" || p.Members[1] != "carol" {
		t.Fatalf("party after transfer = %+v ok=%v", p, ok)
	}
}

func TestSnapshotRestoresPartiesAndInvites(t *testing.T) {
	g := newParties(t)
	carolInv := mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow.Add(time.Second))
	mustAccept(t, g, carolInv.ID, partyNow.Add(2*time.Second))
	frankInv := mustInvite(t, g, "p-1", "alice", "frank", "east", partyNow.Add(3*time.Second))
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewPartyInviteGuardFromSnapshot(partyCfg(), snap)
	if err != nil {
		t.Fatalf("NewPartyInviteGuardFromSnapshot: %v", err)
	}
	p, ok := restored.PartyOf("carol")
	if !ok || len(p.Members) != 2 || p.Leader != "alice" {
		t.Fatalf("restored party = %+v ok=%v", p, ok)
	}
	if got := len(restored.PendingFor("frank")); got != 1 {
		t.Fatalf("restored pending invites = %d, want 1", got)
	}
	if _, err := restored.Invite("p-1", "alice", "frank", "east", partyNow.Add(4*time.Second)); !errors.Is(err, taskpkg.ErrInvitePending) {
		t.Fatalf("restart must keep pending invites, got %v", err)
	}
	p = mustAccept(t, restored, frankInv.ID, partyNow.Add(5*time.Second))
	if len(p.Members) != 3 || p.Members[2] != "frank" {
		t.Fatalf("party after restored accept = %v, want [alice carol frank]", p.Members)
	}
}

func TestConcurrentAcceptSingleWinner(t *testing.T) {
	g := newParties(t)
	first := mustInvite(t, g, "p-1", "alice", "carol", "east", partyNow)
	second := mustInvite(t, g, "p-2", "dave", "carol", "east", partyNow.Add(time.Second))
	ids := []string{first.ID, second.ID}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	at := partyNow.Add(2 * time.Second)
	for i, id := range ids {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, errs[i] = g.Accept(id, at)
		}(i, id)
	}
	wg.Wait()
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent accepts succeeded %d times, want exactly 1", successes)
	}
	if _, ok := g.PartyOf("carol"); !ok {
		t.Fatalf("carol must end up in exactly one party")
	}
	if got := len(g.PendingFor("carol")); got != 0 {
		t.Fatalf("pending invites after the race = %d, want 0", got)
	}
}
