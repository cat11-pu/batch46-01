package task

import (
	"errors"
	"time"
)

// Sentinels returned by PartyInviteGuard. Spam control and party capacity must
// be distinguishable from a missing invite.
var (
	ErrUnknownParty   = errors.New("party: unknown party")
	ErrDuplicateParty = errors.New("party: duplicate party id")
	ErrNotLeader      = errors.New("party: caller is not the leader")
	ErrSamePlayer     = errors.New("party: cannot invite self")
	ErrPartyFull      = errors.New("party: party is full")
	ErrInvitePending  = errors.New("party: invite already pending")
	ErrCooldown       = errors.New("party: invite cooldown active")
	ErrPlayerBusy     = errors.New("party: player already in a party")
	ErrInboxFull      = errors.New("party: invitee inbox full")
	ErrCrossRealm     = errors.New("party: cross realm invite")
	ErrInviteExpired  = errors.New("party: invite expired")
	ErrInviteNotFound = errors.New("party: invite not found")
	ErrNotInParty     = errors.New("party: player not in a party")
)

var errNotImplemented = errors.New("party: not implemented")

// Config is the invite policy.
type Config struct {
	MaxSize              int
	InviteTTL            time.Duration
	Cooldown             time.Duration
	MaxPendingPerInvitee int
}

// Invite is one pending party invitation.
type Invite struct {
	ID        string
	PartyID   string
	Inviter   string
	Invitee   string
	Realm     string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// Party is one party; Members lists join order and Members[0] is the leader.
type Party struct {
	ID      string
	Leader  string
	Realm   string
	Members []string
}

// PartyInviteGuard enforces invite dedupe, cooldowns, capacity and expiry.
type PartyInviteGuard struct {
	cfg Config
}

// NewPartyInviteGuard returns an empty guard configured by cfg.
func NewPartyInviteGuard(cfg Config) *PartyInviteGuard {
	return &PartyInviteGuard{cfg: cfg}
}

// NewPartyInviteGuardFromSnapshot restores parties, pending invites and the
// cooldown state from a snapshot.
func NewPartyInviteGuardFromSnapshot(cfg Config, data []byte) (*PartyInviteGuard, error) {
	return nil, errNotImplemented
}

// CreateParty creates a party with leader as its first member.
func (g *PartyInviteGuard) CreateParty(partyID, leader, realm string, now time.Time) error {
	return errNotImplemented
}

// Invite issues an invitation from the party leader to invitee.
func (g *PartyInviteGuard) Invite(partyID, inviter, invitee, realm string, now time.Time) (Invite, error) {
	return Invite{}, errNotImplemented
}

// Accept consumes an invitation and joins the invitee to the party.
func (g *PartyInviteGuard) Accept(inviteID string, now time.Time) (Party, error) {
	return Party{}, errNotImplemented
}

// Decline rejects an invitation and starts the inviter to invitee cooldown.
func (g *PartyInviteGuard) Decline(inviteID string, now time.Time) error {
	return errNotImplemented
}

// Cancel withdraws the pending invitation a party holds for invitee.
func (g *PartyInviteGuard) Cancel(partyID, invitee string, now time.Time) error {
	return errNotImplemented
}

// PendingFor returns a copy of the pending invites of an invitee, oldest first.
func (g *PartyInviteGuard) PendingFor(invitee string) []Invite {
	return nil
}

// PartyOf looks up the party of a player.
func (g *PartyInviteGuard) PartyOf(player string) (Party, bool) {
	return Party{}, false
}

// Leader returns the current leader of a party.
func (g *PartyInviteGuard) Leader(partyID string) (string, error) {
	return "", errNotImplemented
}

// Promote moves leadership to an existing member of the party.
func (g *PartyInviteGuard) Promote(partyID, newLeader string) error {
	return errNotImplemented
}

// Leave removes a player from a party; a departing leader hands leadership to
// the earliest joined remaining member.
func (g *PartyInviteGuard) Leave(player string, now time.Time) error {
	return errNotImplemented
}

// Expire removes every invite whose ExpiresAt is not after now and returns how
// many were removed.
func (g *PartyInviteGuard) Expire(now time.Time) int {
	return 0
}

// Snapshot serialises parties, pending invites and cooldowns.
func (g *PartyInviteGuard) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
