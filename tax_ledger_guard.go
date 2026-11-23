package task

import (
	"errors"
	"time"
)

// errNotImplemented is returned by every stub below.
var errNotImplemented = errors.New("tax: not implemented")

// Sentinel errors reported by TaxLedger.
var (
	// ErrBadRule means a tax rule cannot be used: RateBps outside [0,10000],
	// a negative MinTax, or a CapTax that is not positive or is below
	// MinTax.
	ErrBadRule = errors.New("tax: invalid rule")
	// ErrBadTrade means a trade has an empty id/seller/buyer/item, a seller
	// equal to the buyer, or a gross amount that is not positive.
	ErrBadTrade = errors.New("tax: invalid trade")
	// ErrTradeIDConflict means a trade id was reused with a different
	// payload.
	ErrTradeIDConflict = errors.New("tax: trade id reused with different payload")
	// ErrUnknownTrade means the trade id is unknown.
	ErrUnknownTrade = errors.New("tax: unknown trade")
	// ErrAlreadyReversed means the trade has already been reversed.
	ErrAlreadyReversed = errors.New("tax: trade already reversed")
	// ErrSnapshotInvalid means the payload cannot be restored.
	ErrSnapshotInvalid = errors.New("tax: invalid snapshot")
)

// Trade is one completed market sale.
type Trade struct {
	TradeID string
	Seller  string
	Buyer   string
	ItemID  string
	Gross   int64
	At      time.Time
}

// TaxRule configures auction house tax. Tax is RateBps basis points of the
// gross amount, rounded half up, clamped to [MinTax, CapTax].
type TaxRule struct {
	RateBps int64
	MinTax  int64
	CapTax  int64
}

// Settlement describes either one recorded trade (Trades == 1) or the
// aggregate of a seller.
type Settlement struct {
	Seller string
	Gross  int64
	Tax    int64
	Payout int64
	Trades int
}

// TaxLedger records trades, taxes and seller payouts.
type TaxLedger struct {
	rule TaxRule
}

// NewTaxLedger creates a ledger using rule.
func NewTaxLedger(rule TaxRule) (*TaxLedger, error) {
	if rule.RateBps < 0 || rule.RateBps > 10000 {
		return nil, ErrBadRule
	}
	if rule.MinTax < 0 || rule.CapTax <= 0 || rule.CapTax < rule.MinTax {
		return nil, ErrBadRule
	}
	return &TaxLedger{rule: rule}, nil
}

// PreviewTax returns the tax the rule charges on gross.
func (l *TaxLedger) PreviewTax(gross int64) (int64, error) {
	return 0, errNotImplemented
}

// Record books a trade, credits the seller payout and returns the settlement
// of that trade. The call is idempotent by TradeID.
func (l *TaxLedger) Record(t Trade) (Settlement, error) {
	return Settlement{}, errNotImplemented
}

// Payout returns the total net payout credited to the seller.
func (l *TaxLedger) Payout(seller string) int64 {
	return 0
}

// TaxCollected returns the total tax held by the house.
func (l *TaxLedger) TaxCollected() int64 {
	return 0
}

// SellerSettlement returns the aggregate settlement of one seller.
func (l *TaxLedger) SellerSettlement(seller string) Settlement {
	return Settlement{}
}

// Reverse undoes a booked trade, debiting the payout and the tax again.
func (l *TaxLedger) Reverse(tradeID, reason string) (Trade, error) {
	return Trade{}, errNotImplemented
}

// Trade returns one booked trade.
func (l *TaxLedger) Trade(tradeID string) (Trade, error) {
	return Trade{}, errNotImplemented
}

// Trades returns the ids of a seller's trades that are not reversed, in
// booking order.
func (l *TaxLedger) Trades(seller string) []string {
	return nil
}

// DailyTotals returns gross and tax for the UTC day formatted as YYYY-MM-DD.
func (l *TaxLedger) DailyTotals(day string) (int64, int64) {
	return 0, 0
}

// Snapshot serialises the ledger so a restarted process keeps totals.
func (l *TaxLedger) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// NewTaxLedgerFromSnapshot restores a ledger produced by Snapshot.
func NewTaxLedgerFromSnapshot(data []byte) (*TaxLedger, error) {
	return nil, errNotImplemented
}
