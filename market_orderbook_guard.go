package task

import "errors"

// errNotImplemented is returned by every stub below.
var errNotImplemented = errors.New("orderbook: not implemented")

// Sentinel errors reported by OrderBook.
var (
	// ErrBadOrder means an order has an empty id/player, a quantity that is
	// not positive, a price that is not a positive multiple of the tick, or
	// an item that does not belong to this book.
	ErrBadOrder = errors.New("orderbook: invalid order")
	// ErrDuplicateOrder means the order id is already known to the book.
	ErrDuplicateOrder = errors.New("orderbook: duplicate order id")
	// ErrUnknownOrder means the order is not open: it was filled or
	// cancelled, or never existed.
	ErrUnknownOrder = errors.New("orderbook: unknown order")
	// ErrNotOwner means the cancel came from a player other than the owner.
	ErrNotOwner = errors.New("orderbook: order belongs to another player")
	// ErrSnapshotInvalid means the payload cannot be restored.
	ErrSnapshotInvalid = errors.New("orderbook: invalid snapshot")
)

// Side is the side of an order.
type Side int

// Buy and Sell are the two book sides.
const (
	Buy Side = iota
	Sell
)

// Order is one limit order submitted to the book.
type Order struct {
	ID     string
	Player string
	Side   Side
	ItemID string
	Price  int64
	Qty    int64
}

// Fill is one executed match. Price is always the resting order price.
type Fill struct {
	TradeID     string
	BuyOrderID  string
	SellOrderID string
	ItemID      string
	Price       int64
	Qty         int64
	Seq         int64
}

// Level is one aggregated price level of the book.
type Level struct {
	Price  int64
	Qty    int64
	Orders int
}

// OrderBook is a single-item limit order book with price-time priority.
type OrderBook struct {
	itemID string
	tick   int64
}

// NewOrderBook creates a book for itemID. Prices must be positive multiples of
// tick.
func NewOrderBook(itemID string, tick int64) *OrderBook {
	return &OrderBook{itemID: itemID, tick: tick}
}

// Submit places an order, matches it against the opposite side in price-time
// priority and returns the fills it produced. The unmatched remainder rests.
func (b *OrderBook) Submit(o Order) ([]Fill, error) {
	return nil, errNotImplemented
}

// Cancel removes an open order of player and returns its remaining quantity.
func (b *OrderBook) Cancel(orderID, player string) (int64, error) {
	return 0, errNotImplemented
}

// BestBid returns the highest bid price, its total quantity, and whether any
// bid exists.
func (b *OrderBook) BestBid() (int64, int64, bool) {
	return 0, 0, false
}

// BestAsk returns the lowest ask price, its total quantity, and whether any
// ask exists.
func (b *OrderBook) BestAsk() (int64, int64, bool) {
	return 0, 0, false
}

// Depth aggregates open quantity per price level: bids descending, asks
// ascending, at most levels entries (levels <= 0 means all).
func (b *OrderBook) Depth(side Side, levels int) []Level {
	return nil
}

// Remaining returns the unfilled quantity of an open order.
func (b *OrderBook) Remaining(orderID string) (int64, error) {
	return 0, errNotImplemented
}

// Trades returns every fill the book produced, in execution order.
func (b *OrderBook) Trades() []Fill {
	return nil
}

// OpenOrders returns the sorted ids of the player's open orders.
func (b *OrderBook) OpenOrders(player string) []string {
	return nil
}

// Snapshot serialises the book so a restarted process keeps matching state.
func (b *OrderBook) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}

// NewOrderBookFromSnapshot restores a book produced by Snapshot.
func NewOrderBookFromSnapshot(data []byte) (*OrderBook, error) {
	return nil, errNotImplemented
}
