package task

import (
	"errors"
	"time"
)

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by CrossServerTransfer. A duplicate idempotent
// call must stay distinguishable from a reused id with a new payload.
var (
	ErrUnknownServer     = errors.New("transfer: unknown server")
	ErrSameServer        = errors.New("transfer: source and destination are the same")
	ErrInsufficientFunds = errors.New("transfer: insufficient funds")
	ErrInvalidTransfer   = errors.New("transfer: invalid request")
	ErrTransferConflict  = errors.New("transfer: transfer id reused with a different payload")
	ErrTransferInFlight  = errors.New("transfer: player already has a transfer in flight")
	ErrTransferNotFound  = errors.New("transfer: transfer not found")
	ErrInvalidTransition = errors.New("transfer: invalid state transition")
	ErrDestinationFull   = errors.New("transfer: destination capacity exceeded")
	ErrCapacityInvalid   = errors.New("transfer: invalid capacity")
)

// Config holds the transfer policy.
type Config struct {
	// TTL is how long a frozen or reserved transfer may stay open.
	TTL time.Duration
}

// TransferState is the lifecycle state of one transfer.
type TransferState string

// Transfer states.
const (
	StateFrozen     TransferState = "frozen"
	StateReserved   TransferState = "reserved"
	StateCommitted  TransferState = "committed"
	StateRolledBack TransferState = "rolled_back"
)

// Balance is one player's holdings on one server.
type Balance struct {
	Gold  int64
	Items map[string]int
}

// TransferRequest is the payload used to start a transfer. TransferID is the
// caller supplied idempotency key.
type TransferRequest struct {
	TransferID string
	PlayerID   string
	FromServer string
	ToServer   string
	Gold       int64
	Items      map[string]int
}

// CrossServerTransfer moves players and goods between game servers with an
// explicit two phase protocol.
type CrossServerTransfer struct{}

// NewCrossServerTransfer returns an empty transfer engine configured by cfg.
func NewCrossServerTransfer(cfg Config) *CrossServerTransfer {
	return &CrossServerTransfer{}
}

// NewCrossServerTransferFromSnapshot restores balances, capacities and open
// transfers from data.
func NewCrossServerTransferFromSnapshot(cfg Config, data []byte) (*CrossServerTransfer, error) {
	return nil, errNotImplemented
}

// RegisterServer registers a server and its total gold capacity.
func (t *CrossServerTransfer) RegisterServer(serverID string, goldCapacity int64) error {
	return errNotImplemented
}

// Deposit credits gold and items to one player on one server.
func (t *CrossServerTransfer) Deposit(serverID, playerID string, gold int64, items map[string]int) error {
	return errNotImplemented
}

// Balance returns the holdings of one player. Unknown players hold nothing.
func (t *CrossServerTransfer) Balance(serverID, playerID string) (Balance, error) {
	return Balance{}, errNotImplemented
}

// Begin validates the request, freezes the goods on the source server and
// returns the transfer id. Repeating the same request returns the same id
// without freezing twice.
func (t *CrossServerTransfer) Begin(req TransferRequest, now time.Time) (string, error) {
	return "", errNotImplemented
}

// Approve reserves destination capacity for a frozen transfer.
func (t *CrossServerTransfer) Approve(transferID string, now time.Time) error {
	return errNotImplemented
}

// Commit credits the destination player. Repeating a commit is a no-op.
func (t *CrossServerTransfer) Commit(transferID string, now time.Time) error {
	return errNotImplemented
}

// Rollback returns frozen or reserved goods to the source player.
func (t *CrossServerTransfer) Rollback(transferID string, now time.Time) error {
	return errNotImplemented
}

// State returns the lifecycle state of one transfer.
func (t *CrossServerTransfer) State(transferID string) (TransferState, error) {
	return "", errNotImplemented
}

// Expire rolls back every frozen or reserved transfer older than Config.TTL
// and returns their ids sorted ascending.
func (t *CrossServerTransfer) Expire(now time.Time) ([]string, error) {
	return nil, errNotImplemented
}

// Snapshot serialises the whole engine so a restart keeps open transfers.
func (t *CrossServerTransfer) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
