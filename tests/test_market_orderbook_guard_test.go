package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-a-07"
)

const bookItem = "item-sword-01"

func newBook(t *testing.T) *taskpkg.OrderBook {
	t.Helper()
	return taskpkg.NewOrderBook(bookItem, 5)
}

func order(id, player string, side taskpkg.Side, price, qty int64) taskpkg.Order {
	return taskpkg.Order{ID: id, Player: player, Side: side, ItemID: bookItem, Price: price, Qty: qty}
}

func submitOrder(t *testing.T, b *taskpkg.OrderBook, o taskpkg.Order) []taskpkg.Fill {
	t.Helper()
	fills, err := b.Submit(o)
	if err != nil {
		t.Fatalf("Submit(%s): %v", o.ID, err)
	}
	return fills
}

func TestPricePriorityBeatsArrival(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-105", "seller-1", taskpkg.Sell, 105, 5))
	submitOrder(t, b, order("a-100", "seller-1", taskpkg.Sell, 100, 5))

	fills := submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 105, 3))
	if len(fills) != 1 {
		t.Fatalf("want 1 fill, got %d", len(fills))
	}
	if fills[0].SellOrderID != "a-100" || fills[0].Price != 100 || fills[0].Qty != 3 {
		t.Fatalf("buyer must hit the cheaper ask first: %+v", fills[0])
	}
	price, qty, ok := b.BestAsk()
	if !ok || price != 100 || qty != 2 {
		t.Fatalf("best ask after fill: price=%d qty=%d ok=%v", price, qty, ok)
	}
}

func TestTimePriorityWithinPrice(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 5))
	submitOrder(t, b, order("a-2", "seller-2", taskpkg.Sell, 100, 5))

	fills := submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 100, 7))
	if len(fills) != 2 {
		t.Fatalf("want 2 fills, got %d", len(fills))
	}
	if fills[0].SellOrderID != "a-1" || fills[0].Qty != 5 {
		t.Fatalf("older ask must fill first: %+v", fills[0])
	}
	if fills[1].SellOrderID != "a-2" || fills[1].Qty != 2 {
		t.Fatalf("second fill must rest on the next ask: %+v", fills[1])
	}
	price, qty, ok := b.BestAsk()
	if !ok || price != 100 || qty != 3 {
		t.Fatalf("remaining ask: price=%d qty=%d ok=%v", price, qty, ok)
	}
}

func TestPartialFillRestsRemainder(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 4))

	fills := submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 100, 10))
	if len(fills) != 1 || fills[0].Qty != 4 || fills[0].Price != 100 {
		t.Fatalf("unexpected fills: %+v", fills)
	}
	price, qty, ok := b.BestBid()
	if !ok || price != 100 || qty != 6 {
		t.Fatalf("unfilled remainder must rest as a bid: price=%d qty=%d ok=%v", price, qty, ok)
	}
	rem, err := b.Remaining("b-1")
	if err != nil {
		t.Fatalf("Remaining: %v", err)
	}
	if rem != 6 {
		t.Fatalf("remaining quantity: want 6, got %d", rem)
	}
	if _, _, ok := b.BestAsk(); ok {
		t.Fatalf("ask side must be empty after being fully consumed")
	}
}

func TestNonCrossingOrderRests(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 5))

	fills := submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 95, 5))
	if len(fills) != 0 {
		t.Fatalf("a bid below the ask must not fill, got %+v", fills)
	}
	price, qty, ok := b.BestBid()
	if !ok || price != 95 || qty != 5 {
		t.Fatalf("bid must rest: price=%d qty=%d ok=%v", price, qty, ok)
	}
	if price, qty, ok := b.BestAsk(); !ok || price != 100 || qty != 5 {
		t.Fatalf("ask must stay untouched: price=%d qty=%d ok=%v", price, qty, ok)
	}
	if got := len(b.Trades()); got != 0 {
		t.Fatalf("no trade expected, got %d", got)
	}
}

func TestSweepMultipleLevels(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-100", "seller-1", taskpkg.Sell, 100, 5))
	submitOrder(t, b, order("a-105", "seller-2", taskpkg.Sell, 105, 5))
	submitOrder(t, b, order("a-110", "seller-3", taskpkg.Sell, 110, 5))

	fills := submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 110, 12))
	if len(fills) != 3 {
		t.Fatalf("want 3 fills, got %d", len(fills))
	}
	wantPrice := []int64{100, 105, 110}
	wantQty := []int64{5, 5, 2}
	for i, f := range fills {
		if f.Price != wantPrice[i] || f.Qty != wantQty[i] {
			t.Fatalf("fill %d: got price=%d qty=%d", i, f.Price, f.Qty)
		}
		if f.Seq != int64(i+1) {
			t.Fatalf("fill %d seq: want %d, got %d", i, i+1, f.Seq)
		}
	}
	price, qty, ok := b.BestAsk()
	if !ok || price != 110 || qty != 3 {
		t.Fatalf("top ask after sweep: price=%d qty=%d ok=%v", price, qty, ok)
	}
}

func TestCancelReturnsRemaining(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 5))
	if fills := submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 100, 2)); len(fills) != 1 {
		t.Fatalf("setup fill: %+v", fills)
	}

	rem, err := b.Cancel("a-1", "seller-1")
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if rem != 3 {
		t.Fatalf("cancel must return the untouched quantity: want 3, got %d", rem)
	}
	if _, _, ok := b.BestAsk(); ok {
		t.Fatalf("cancelled ask must leave the book")
	}
	if ids := b.OpenOrders("seller-1"); len(ids) != 0 {
		t.Fatalf("cancelled order still open: %v", ids)
	}
	if _, err := b.Remaining("a-1"); !errors.Is(err, taskpkg.ErrUnknownOrder) {
		t.Fatalf("cancelled order must be ErrUnknownOrder, got %v", err)
	}
}

func TestCancelRejectsForeignPlayer(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 5))

	if _, err := b.Cancel("a-1", "seller-2"); !errors.Is(err, taskpkg.ErrNotOwner) {
		t.Fatalf("foreign cancel must be ErrNotOwner, got %v", err)
	}
	price, qty, ok := b.BestAsk()
	if !ok || price != 100 || qty != 5 {
		t.Fatalf("order changed after rejected cancel: price=%d qty=%d ok=%v", price, qty, ok)
	}
	if ids := b.OpenOrders("seller-1"); len(ids) != 1 || ids[0] != "a-1" {
		t.Fatalf("unexpected open orders: %v", ids)
	}
}

func TestDuplicateOrderIDRejected(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 5))

	_, err := b.Submit(order("a-1", "buyer-1", taskpkg.Buy, 95, 5))
	if !errors.Is(err, taskpkg.ErrDuplicateOrder) {
		t.Fatalf("duplicate id must be ErrDuplicateOrder, got %v", err)
	}
	price, qty, ok := b.BestAsk()
	if !ok || price != 100 || qty != 5 {
		t.Fatalf("book changed after duplicate: price=%d qty=%d ok=%v", price, qty, ok)
	}
	if _, _, ok := b.BestBid(); ok {
		t.Fatalf("duplicate order must not rest")
	}
}

func TestInvalidOrdersRejected(t *testing.T) {
	b := newBook(t)

	cases := []taskpkg.Order{
		order("bad-tick", "buyer-1", taskpkg.Buy, 97, 5),
		order("bad-qty", "buyer-1", taskpkg.Buy, 95, 0),
		{ID: "bad-item", Player: "buyer-1", Side: taskpkg.Buy, ItemID: "item-shield-02", Price: 95, Qty: 5},
		order("", "buyer-1", taskpkg.Buy, 95, 5),
		order("bad-player", "", taskpkg.Buy, 95, 5),
	}
	for _, o := range cases {
		if _, err := b.Submit(o); !errors.Is(err, taskpkg.ErrBadOrder) {
			t.Fatalf("order %+v must be ErrBadOrder, got %v", o, err)
		}
	}
	if _, _, ok := b.BestBid(); ok {
		t.Fatalf("rejected orders must not rest")
	}
	if _, _, ok := b.BestAsk(); ok {
		t.Fatalf("rejected orders must not rest")
	}
	if got := len(b.Trades()); got != 0 {
		t.Fatalf("rejected orders produced %d trades", got)
	}
}

func TestDepthAggregatesPriceLevels(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 2))
	submitOrder(t, b, order("a-2", "seller-1", taskpkg.Sell, 100, 3))
	submitOrder(t, b, order("a-3", "seller-2", taskpkg.Sell, 105, 4))
	submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 95, 6))
	submitOrder(t, b, order("b-2", "buyer-2", taskpkg.Buy, 90, 7))

	asks := b.Depth(taskpkg.Sell, 2)
	if len(asks) != 2 {
		t.Fatalf("want 2 ask levels, got %d", len(asks))
	}
	if asks[0] != (taskpkg.Level{Price: 100, Qty: 5, Orders: 2}) {
		t.Fatalf("first ask level: %+v", asks[0])
	}
	if asks[1] != (taskpkg.Level{Price: 105, Qty: 4, Orders: 1}) {
		t.Fatalf("second ask level: %+v", asks[1])
	}
	bids := b.Depth(taskpkg.Buy, 1)
	if len(bids) != 1 {
		t.Fatalf("want 1 bid level, got %d", len(bids))
	}
	if bids[0] != (taskpkg.Level{Price: 95, Qty: 6, Orders: 1}) {
		t.Fatalf("first bid level: %+v", bids[0])
	}
}

func TestTradesSequenceIsStable(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 2))
	submitOrder(t, b, order("a-2", "seller-2", taskpkg.Sell, 105, 3))
	submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 105, 3))

	trades := b.Trades()
	if len(trades) != 2 {
		t.Fatalf("want 2 trades, got %d", len(trades))
	}
	for i, tr := range trades {
		if tr.Seq != int64(i+1) {
			t.Fatalf("trade %d seq: want %d, got %d", i, i+1, tr.Seq)
		}
		if tr.TradeID == "" {
			t.Fatalf("trade %d has no id", i)
		}
		if tr.BuyOrderID != "b-1" || tr.ItemID != bookItem {
			t.Fatalf("unexpected trade: %+v", tr)
		}
	}
	if trades[0].TradeID == trades[1].TradeID {
		t.Fatalf("trade ids must be unique")
	}
	if trades[0].SellOrderID != "a-1" || trades[1].SellOrderID != "a-2" {
		t.Fatalf("trades out of price-time order: %+v", trades)
	}
}

func TestSnapshotRestoresBook(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-1", "seller-1", taskpkg.Sell, 100, 5))
	submitOrder(t, b, order("a-2", "seller-1", taskpkg.Sell, 105, 5))
	submitOrder(t, b, order("b-1", "buyer-1", taskpkg.Buy, 100, 3))

	snap, err := b.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewOrderBookFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewOrderBookFromSnapshot: %v", err)
	}
	price, qty, ok := restored.BestAsk()
	if !ok || price != 100 || qty != 2 {
		t.Fatalf("restored ask: price=%d qty=%d ok=%v", price, qty, ok)
	}
	ids := restored.OpenOrders("seller-1")
	if len(ids) != 2 || ids[0] != "a-1" || ids[1] != "a-2" {
		t.Fatalf("restored open orders: %v", ids)
	}

	fills, err := restored.Submit(order("b-2", "buyer-2", taskpkg.Buy, 105, 2))
	if err != nil {
		t.Fatalf("Submit after restore: %v", err)
	}
	if len(fills) != 1 || fills[0].Price != 100 || fills[0].Seq != 2 {
		t.Fatalf("fill sequence must continue after restore: %+v", fills)
	}
	if got := len(restored.Trades()); got != 2 {
		t.Fatalf("restored book must keep old trades: got %d", got)
	}
}

func TestSnapshotRejectsGarbage(t *testing.T) {
	if _, err := taskpkg.NewOrderBookFromSnapshot(nil); !errors.Is(err, taskpkg.ErrSnapshotInvalid) {
		t.Fatalf("nil snapshot must be ErrSnapshotInvalid, got %v", err)
	}
	if _, err := taskpkg.NewOrderBookFromSnapshot([]byte("not a book")); !errors.Is(err, taskpkg.ErrSnapshotInvalid) {
		t.Fatalf("garbage snapshot must be ErrSnapshotInvalid, got %v", err)
	}
}

func TestConcurrentBuyersConsumeSellerExactly(t *testing.T) {
	b := newBook(t)
	submitOrder(t, b, order("a-sell", "seller-1", taskpkg.Sell, 100, 20))

	var wg sync.WaitGroup
	errs := make([]error, 20)
	fillsPerBuy := make([]int, 20)
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			fills, err := b.Submit(order(fmt.Sprintf("b-%02d", i), fmt.Sprintf("buyer-%d", i), taskpkg.Buy, 100, 1))
			errs[i] = err
			fillsPerBuy[i] = len(fills)
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("buyer %d failed: %v", i, err)
		}
		if fillsPerBuy[i] != 1 {
			t.Fatalf("buyer %d got %d fills, want 1", i, fillsPerBuy[i])
		}
	}
	if _, _, ok := b.BestAsk(); ok {
		t.Fatalf("seller must be fully consumed")
	}
	if _, err := b.Remaining("a-sell"); !errors.Is(err, taskpkg.ErrUnknownOrder) {
		t.Fatalf("filled sell order must be gone, got %v", err)
	}
	trades := b.Trades()
	if len(trades) != 20 {
		t.Fatalf("want 20 trades, got %d", len(trades))
	}
	var total int64
	for _, tr := range trades {
		total += tr.Qty
	}
	if total != 20 {
		t.Fatalf("total matched quantity: want 20, got %d", total)
	}
}

func TestConcurrentDuplicateSubmitOneWins(t *testing.T) {
	b := newBook(t)
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := b.Submit(order("dup-1", "buyer-1", taskpkg.Buy, 95, 5))
			errs[i] = err
		}()
	}
	wg.Wait()

	success := 0
	duplicates := 0
	for _, err := range errs {
		switch {
		case err == nil:
			success++
		case errors.Is(err, taskpkg.ErrDuplicateOrder):
			duplicates++
		default:
			t.Fatalf("unexpected submit error: %v", err)
		}
	}
	if success != 1 || duplicates != 9 {
		t.Fatalf("want 1 success and 9 duplicates, got %d/%d", success, duplicates)
	}
	price, qty, ok := b.BestBid()
	if !ok || price != 95 || qty != 5 {
		t.Fatalf("only one order may rest: price=%d qty=%d ok=%v", price, qty, ok)
	}
	if ids := b.OpenOrders("buyer-1"); len(ids) != 1 || ids[0] != "dup-1" {
		t.Fatalf("unexpected open orders: %v", ids)
	}
}
