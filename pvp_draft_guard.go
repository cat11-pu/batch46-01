package task

import (
	"errors"
	"time"
)

// Sentinels returned by PvpDraft. A caller must be able to tell who is allowed
// to act, which action kind is legal, and whether a champion is unavailable.
var (
	ErrInvalidPhase    = errors.New("draft: action kind not allowed in this phase")
	ErrAlreadyBanned   = errors.New("draft: champion already banned")
	ErrChampionTaken   = errors.New("draft: champion already banned or picked")
	ErrUnknownChampion = errors.New("draft: unknown champion")
	ErrNotYourTurn     = errors.New("draft: not your turn")
	ErrDuplicateAction = errors.New("draft: duplicate action id")
	ErrDraftClosed     = errors.New("draft: draft is closed")
	ErrTurnNotExpired  = errors.New("draft: turn deadline not reached")
)

var errNotImplemented = errors.New("draft: not implemented")

// Team identifies one side of the draft.
type Team string

const (
	TeamBlue Team = "blue"
	TeamRed  Team = "red"
)

// Phase is the current draft stage.
type Phase string

const (
	PhaseBan  Phase = "ban"
	PhasePick Phase = "pick"
	PhaseDone Phase = "done"
)

// ActionKind distinguishes a ban from a pick.
type ActionKind string

const (
	KindBan  ActionKind = "ban"
	KindPick ActionKind = "pick"
)

// Config is the draft policy.
type Config struct {
	BansPerTeam  int
	PicksPerTeam int
	TurnTimeout  time.Duration
}

// Champion is one selectable champion.
type Champion struct {
	ID   string
	Role string
}

// Action is one submitted ban or pick.
type Action struct {
	ID         string
	Team       Team
	Kind       ActionKind
	ChampionID string
	At         time.Time
}

// DraftState is an immutable view of the draft. Bans lists banned champion ids
// in submission order; BluePicks and RedPicks list picks in submission order.
type DraftState struct {
	Phase     Phase
	Turn      int
	Team      Team
	Bans      []string
	BluePicks []string
	RedPicks  []string
}

// PvpDraft enforces ban/pick order, champion availability and turn deadlines.
type PvpDraft struct {
	cfg       Config
	champions []Champion
}

// NewPvpDraft starts a draft at now with the given champion pool.
func NewPvpDraft(cfg Config, champions []Champion, now time.Time) (*PvpDraft, error) {
	return &PvpDraft{cfg: cfg, champions: append([]Champion(nil), champions...)}, nil
}

// NewPvpDraftFromSnapshot restores a draft, including history, accepted action
// ids and the current deadline, from a snapshot.
func NewPvpDraftFromSnapshot(cfg Config, champions []Champion, data []byte) (*PvpDraft, error) {
	return nil, errNotImplemented
}

// Submit applies one ban or pick and advances the turn.
func (d *PvpDraft) Submit(a Action, now time.Time) (DraftState, error) {
	return DraftState{}, errNotImplemented
}

// State returns a copy of the current draft state.
func (d *PvpDraft) State() DraftState {
	return DraftState{}
}

// Timeout resolves the current turn automatically when now has reached the
// deadline; it returns ErrTurnNotExpired before that.
func (d *PvpDraft) Timeout(now time.Time) (DraftState, error) {
	return DraftState{}, errNotImplemented
}

// Deadline returns the deadline of the current turn.
func (d *PvpDraft) Deadline() time.Time {
	return time.Time{}
}

// Available returns the sorted champion ids that are neither banned nor picked,
// filtered by role; an empty role means every role.
func (d *PvpDraft) Available(role string) []string {
	return nil
}

// Champions returns a copy of the champion pool.
func (d *PvpDraft) Champions() []Champion {
	return nil
}

// History returns the accepted actions in submission order.
func (d *PvpDraft) History() []Action {
	return nil
}

// Snapshot serialises the draft state, history and accepted action ids.
func (d *PvpDraft) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
