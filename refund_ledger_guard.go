package task

import (
	"errors"
	"time"
)

// errNotImplemented is returned by every stub below.
var errNotImplemented = errors.New("refund: not implemented")

// Sentinel errors reported by RefundLedger.
var (
	// ErrBadAmount means an amount is not positive, or the debt floor passed
	// to NewRefundLedger is not positive.
	ErrBadAmount = errors.New("refund: invalid amount")
	// ErrDuplicateOrder means the order id has already been granted.
	ErrDuplicateOrder = errors.New("refund: duplicate order id")
	// ErrUnknownOrder means the order id is unknown.
	ErrUnknownOrder = errors.New("refund: unknown order")
	// ErrOrderOwner means the refund names a player who does not own the
	// order.
	ErrOrderOwner = errors.New("refund: order belongs to another player")
	// ErrRefundExceeds means the requested amount exceeds the order amount
	// not yet refunded.
	ErrRefundExceeds = errors.New("refund: amount exceeds refundable remainder")
	// ErrFloorBreach means the refund would push the balance below the
	// negative floor.
	ErrFloorBreach = errors.New("refund: debt floor breached")
	// ErrRefundIDConflict means a refund id was reused with a different
	// payload.
	ErrRefundIDConflict = errors.New("refund: refund id reused with different payload")
	// ErrInsufficientCredits means a spend or repay is larger than the
	// balance and the balance is not negative.
	ErrInsufficientCredits = errors.New("refund: insufficient credits")
	// ErrDebtLocked means the balance is negative, so spending is blocked.
	ErrDebtLocked = errors.New("refund: spending blocked by negative balance")
	// ErrSnapshotInvalid means the payload cannot be restored.
	ErrSnapshotInvalid = errors.New("refund: invalid snapshot")
)

// Order is a purchase whose credits may later be clawed back.
type Order struct {
	OrderID string
	Player  string
	Amount  int64
	At      time.Time
	Note    string
	// Refunded is maintained by the ledger and reported by Order.
	Refunded int64
}

// RefundRequest asks the ledger to claw back part of an order.
type RefundRequest struct {
	RefundID string
	OrderID  string
	Player   string
	Amount   int64
	Reason   string
	At       time.Time
}

// Refund is one applied clawback.
type Refund struct {
	RefundID string
	OrderID  string
	Player   string
	Amount   int64
	Reason   string
	At       time.Time
}

// RefundLedger tracks player credit balances, orders and clawbacks.
type RefundLedger struct {
	debtFloor int64
}

// NewRefundLedger creates a ledger whose balance may fall to -debtFloor but
// never below it.
func NewRefundLedger(debtFloor int64) (*RefundLedger, error) {
	if debtFloor <= 0 {
		return nil, ErrBadAmount
	}
	return &RefundLedger{debtFloor: debtFloor}, nil
}

// Grant records an order and credits its amount to the player.
func (l *RefundLedger) Grant(o Order) error {
	return errNotImplemented
}

// Spend debits credits from the player. Spending is refused while the balance
// is negative.
func (l *RefundLedger) Spend(player string, amount int64) error {
	return errNotImplemented
}

// Refund claws back amount credits for an order and returns the resulting
// balance. The call is idempotent by RefundID.
func (l *RefundLedger) Refund(req RefundRequest) (int64, error) {
	return 0, errNotImplemented
}

// Repay adds credits, for example through a new purchase, to clear debt.
func (l *RefundLedger) Repay(player string, amount int64) error {
	return errNotImplemented
}

// Balance returns the current credit balance of the player.
func (l *RefundLedger) Balance(player string) int64 {
	return 0
}

// Order returns one recorded order including its Refunded total.
func (l *RefundLedger) Order(orderID string) (Order, error) {
	return Order{}, errNotImplemented
}

// Refunds returns the applied clawbacks of an order in application order.
func (l *RefundLedger) Refunds(orderID string) []Refund {
	return nil
}

// RefundByID returns one applied clawback.
func (l *RefundLedger) RefundByID(refundID string) (Refund, bool) {
	return Refund{}, false
}

// DebtLocked reports whether the player's balance is negative.
func (l *RefundLedger) DebtLocked(player string) bool {
	return false
}

// Snapshot serialises the ledger so a restarted process keeps balances and
// idempotency keys.
func (l *RefundLedger) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// NewRefundLedgerFromSnapshot restores a ledger produced by Snapshot.
func NewRefundLedgerFromSnapshot(data []byte) (*RefundLedger, error) {
	return nil, errNotImplemented
}
