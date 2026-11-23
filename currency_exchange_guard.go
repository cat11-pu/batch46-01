package task

import (
	"errors"
	"time"
)

// Sentinel errors reported by CurrencyExchange. A duplicated request id is not
// an error by itself: only a replayed id with a different payload is.
var (
	ErrUnknownAccount     = errors.New("exchange: unknown account")
	ErrInvalidAmount      = errors.New("exchange: invalid amount")
	ErrInvalidCurrency    = errors.New("exchange: invalid currency code")
	ErrInvalidRate        = errors.New("exchange: invalid rate")
	ErrInsufficientFunds  = errors.New("exchange: insufficient funds")
	ErrSameCurrency       = errors.New("exchange: source and target currency are equal")
	ErrRateNotFound       = errors.New("exchange: rate not found")
	ErrStaleRate          = errors.New("exchange: rate is stale")
	ErrDailyCap           = errors.New("exchange: daily cap exceeded")
	ErrMissingRequestID   = errors.New("exchange: missing request id")
	ErrIDConflict         = errors.New("exchange: request id reused with another payload")
	ErrHoldNotFound       = errors.New("exchange: hold not found")
	ErrHoldExpired        = errors.New("exchange: hold expired")
)

var errNotImplemented = errors.New("not implemented")

// Config is the exchange policy.
type Config struct {
	// FeeBasisPoints is the fee taken from the gross converted amount, in
	// hundredths of a percent (250 = 2.5%).
	FeeBasisPoints int64
	// MaxRateAge is how old a published rate may be.
	MaxRateAge time.Duration
	// HoldTTL is how long a frozen amount may stay unsettled.
	HoldTTL time.Duration
	// DailyCap maps a source currency to the units an account may spend per
	// UTC day; a missing entry or zero means unlimited.
	DailyCap map[string]int64
}

// Balance is the per-currency wallet view of one account.
type Balance struct {
	Available int64
	Frozen    int64
	Total     int64
}

// Rate is one directed conversion rate: 1 From is worth Num/Den To.
type Rate struct {
	From      string
	To        string
	Num       int64
	Den       int64
	UpdatedAt time.Time
}

// Quote is the outcome of a conversion without touching any balance.
type Quote struct {
	Gross int64
	Fee   int64
	Net   int64
}

// ExchangeRequest asks for a conversion.
type ExchangeRequest struct {
	RequestID string
	AccountID string
	From      string
	To        string
	Amount    int64
}

// ExchangeResult is the settled outcome of a conversion or a hold settlement.
type ExchangeResult struct {
	RequestID string
	AccountID string
	From      string
	To        string
	Spent     int64
	Gross     int64
	Fee       int64
	Net       int64
}

// HoldRequest freezes source funds for a later conversion.
type HoldRequest struct {
	RequestID string
	AccountID string
	From      string
	To        string
	Amount    int64
}

// Hold is a frozen conversion; the quote is captured at freeze time.
type Hold struct {
	RequestID string
	AccountID string
	From      string
	To        string
	Amount    int64
	Gross     int64
	Fee       int64
	Net       int64
	FrozenAt  time.Time
	ExpiresAt time.Time
}

// CurrencyExchange converts between game currencies while keeping frozen
// balances and per-day caps consistent.
type CurrencyExchange struct {
	cfg Config
}

// NewCurrencyExchange builds an empty exchange.
func NewCurrencyExchange(cfg Config) *CurrencyExchange {
	return &CurrencyExchange{cfg: cfg}
}

// NewCurrencyExchangeFromSnapshot restores balances, rates, holds and settled
// request ids from a Snapshot payload.
func NewCurrencyExchangeFromSnapshot(cfg Config, data []byte) (*CurrencyExchange, error) {
	return nil, errNotImplemented
}

// OpenAccount creates a wallet. Opening an existing account is a no-op.
func (x *CurrencyExchange) OpenAccount(accountID string) error {
	return errNotImplemented
}

// Credit adds currency to an account. Amounts must be positive and currency
// codes must be three upper-case letters.
func (x *CurrencyExchange) Credit(accountID, currency string, amount int64) error {
	return errNotImplemented
}

// Balance returns available, frozen and total units of one currency.
func (x *CurrencyExchange) Balance(accountID, currency string) (Balance, error) {
	return Balance{}, errNotImplemented
}

// SetRate publishes a directed rate. Reverse conversion is never implied.
func (x *CurrencyExchange) SetRate(from, to string, num, den int64, updatedAt time.Time) error {
	return errNotImplemented
}

// Quote computes the conversion for amount using the rate published no longer
// than Config.MaxRateAge ago. It never mutates a balance.
func (x *CurrencyExchange) Quote(from, to string, amount int64, now time.Time) (Quote, error) {
	return Quote{}, errNotImplemented
}

// Exchange moves funds atomically: the source is debited, the target is
// credited with the net amount, and the daily cap is consumed. Replaying a
// request id with the same payload returns the stored result; the same id with
// another payload yields ErrIDConflict.
func (x *CurrencyExchange) Exchange(req ExchangeRequest, now time.Time) (ExchangeResult, error) {
	return ExchangeResult{}, errNotImplemented
}

// Freeze reserves source funds and captures the quote and the daily-cap usage
// at freeze time. The source amount leaves Available and enters Frozen.
func (x *CurrencyExchange) Freeze(req HoldRequest, now time.Time) (Hold, error) {
	return Hold{}, errNotImplemented
}

// SettleHold converts a frozen hold using the quote captured at freeze time.
// Settling an expired hold returns ErrHoldExpired and leaves it frozen.
func (x *CurrencyExchange) SettleHold(requestID string, now time.Time) (ExchangeResult, error) {
	return ExchangeResult{}, errNotImplemented
}

// ReleaseHold returns a frozen amount to Available and gives its daily-cap
// reservation back.
func (x *CurrencyExchange) ReleaseHold(requestID string, now time.Time) error {
	return errNotImplemented
}

// ExpireHolds releases every hold whose TTL elapsed and returns how many were
// released.
func (x *CurrencyExchange) ExpireHolds(now time.Time) int {
	return 0
}

// DailyUsed reports the cap usage of an account and source currency on the
// UTC day that contains now, including frozen reservations.
func (x *CurrencyExchange) DailyUsed(accountID, currency string, now time.Time) int64 {
	return 0
}

// Snapshot serialises balances, frozen holds, published rates and settled
// request ids so a restart keeps the same answers.
func (x *CurrencyExchange) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
