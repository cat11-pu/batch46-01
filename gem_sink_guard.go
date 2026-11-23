package task

import (
	"errors"
	"time"
)

// errNotImplemented is returned by every stub below.
var errNotImplemented = errors.New("gems: not implemented")

// Sentinel errors reported by GemSink.
var (
	// ErrBadAmount means an amount or cap is not positive, or the refund
	// window passed to NewGemSink is not positive.
	ErrBadAmount = errors.New("gems: invalid amount")
	// ErrTopUpRefConflict means a top-up reference was reused with a
	// different amount or player.
	ErrTopUpRefConflict = errors.New("gems: top-up reference reused with different payload")
	// ErrBadSpend means a spend lacks an entry id, player or feature, or its
	// amount is not positive.
	ErrBadSpend = errors.New("gems: invalid spend")
	// ErrEntryIDConflict means a spend entry id was reused with a different
	// payload.
	ErrEntryIDConflict = errors.New("gems: entry id reused with different payload")
	// ErrInsufficientGems means the wallet cannot cover the spend.
	ErrInsufficientGems = errors.New("gems: insufficient balance")
	// ErrDailyCap means the spend would exceed the player's daily cap.
	ErrDailyCap = errors.New("gems: daily spend cap reached")
	// ErrUnknownEntry means the spend entry id is unknown.
	ErrUnknownEntry = errors.New("gems: unknown entry")
	// ErrEntryOwner means the refund names a player who does not own the
	// entry.
	ErrEntryOwner = errors.New("gems: entry belongs to another player")
	// ErrAlreadyRefunded means the entry has already been refunded.
	ErrAlreadyRefunded = errors.New("gems: entry already refunded")
	// ErrNotRefundable means the entry was funded by bonus gems and cannot
	// be refunded.
	ErrNotRefundable = errors.New("gems: entry is not refundable")
	// ErrRefundWindowClosed means the refund arrived after the window.
	ErrRefundWindowClosed = errors.New("gems: refund window closed")
	// ErrSnapshotInvalid means the payload cannot be restored.
	ErrSnapshotInvalid = errors.New("gems: invalid snapshot")
)

// Entry is one recorded gem spend.
type Entry struct {
	EntryID    string
	Player     string
	Amount     int64
	Feature    string
	Refundable bool
	At         time.Time
	Refunded   bool
}

// SpendRequest asks the sink to charge gems for a feature.
type SpendRequest struct {
	EntryID    string
	Player     string
	Amount     int64
	Feature    string
	Refundable bool
	At         time.Time
}

// GemSink is the premium currency wallet and spend ledger of all players.
type GemSink struct {
	refundWindow time.Duration
}

// NewGemSink creates a sink where a spend may be refunded within
// refundWindow.
func NewGemSink(refundWindow time.Duration) (*GemSink, error) {
	if refundWindow <= 0 {
		return nil, ErrBadAmount
	}
	return &GemSink{refundWindow: refundWindow}, nil
}

// TopUp credits gems bought or granted under ref. The call is idempotent by
// reference.
func (l *GemSink) TopUp(player string, gems int64, ref string) error {
	return errNotImplemented
}

// Balance returns the gem balance of the player.
func (l *GemSink) Balance(player string) int64 {
	return 0
}

// Spend charges gems and returns the resulting balance. The call is idempotent
// by EntryID.
func (l *GemSink) Spend(req SpendRequest) (int64, error) {
	return 0, errNotImplemented
}

// Refund returns the gems of one spend entry and reports the new balance.
func (l *GemSink) Refund(entryID, player string, now time.Time) (int64, error) {
	return 0, errNotImplemented
}

// Entry returns one recorded spend.
func (l *GemSink) Entry(entryID string) (Entry, error) {
	return Entry{}, errNotImplemented
}

// Entries returns the spend ids of a player in spend order.
func (l *GemSink) Entries(player string) []string {
	return nil
}

// DailySpent returns the gems the player spent on the UTC day formatted as
// YYYY-MM-DD.
func (l *GemSink) DailySpent(player, day string) int64 {
	return 0
}

// SetDailyCap sets the daily spend cap of a player. A cap of zero disables the
// check.
func (l *GemSink) SetDailyCap(player string, cap int64) error {
	return errNotImplemented
}

// Snapshot serialises the sink so a restarted process keeps balances, entries
// and caps.
func (l *GemSink) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// NewGemSinkFromSnapshot restores a sink produced by Snapshot.
func NewGemSinkFromSnapshot(data []byte) (*GemSink, error) {
	return nil, errNotImplemented
}
