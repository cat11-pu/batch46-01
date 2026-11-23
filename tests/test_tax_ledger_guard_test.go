package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-a-09"
)

var (
	taxDay1 = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)
	taxDay2 = time.Date(2026, 3, 2, 1, 0, 0, 0, time.UTC)
)

func newTax(t *testing.T) *taskpkg.TaxLedger {
	t.Helper()
	l, err := taskpkg.NewTaxLedger(taskpkg.TaxRule{RateBps: 500, MinTax: 5, CapTax: 1000})
	if err != nil {
		t.Fatalf("NewTaxLedger: %v", err)
	}
	return l
}

func trade(id, seller, buyer string, gross int64, at time.Time) taskpkg.Trade {
	return taskpkg.Trade{
		TradeID: id,
		Seller:  seller,
		Buyer:   buyer,
		ItemID:  "item-axe-01",
		Gross:   gross,
		At:      at,
	}
}

func recordTrade(t *testing.T, l *taskpkg.TaxLedger, tr taskpkg.Trade) taskpkg.Settlement {
	t.Helper()
	st, err := l.Record(tr)
	if err != nil {
		t.Fatalf("Record(%s): %v", tr.TradeID, err)
	}
	return st
}

func TestPreviewTaxRounding(t *testing.T) {
	l := newTax(t)

	cases := []struct {
		gross int64
		want  int64
	}{
		{12345, 617},
		{1, 5},
		{20000, 1000},
		{25000, 1000},
		{1000, 50},
	}
	for _, c := range cases {
		got, err := l.PreviewTax(c.gross)
		if err != nil {
			t.Fatalf("PreviewTax(%d): %v", c.gross, err)
		}
		if got != c.want {
			t.Fatalf("PreviewTax(%d): want %d, got %d", c.gross, c.want, got)
		}
	}
}

func TestPreviewTaxRejectsBadGross(t *testing.T) {
	l := newTax(t)
	for _, gross := range []int64{0, -1, -1000} {
		if _, err := l.PreviewTax(gross); !errors.Is(err, taskpkg.ErrBadTrade) {
			t.Fatalf("PreviewTax(%d) must be ErrBadTrade, got %v", gross, err)
		}
	}
}

func TestRecordCreditsPayout(t *testing.T) {
	l := newTax(t)
	st := recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))

	if st.Seller != "seller-1" || st.Gross != 10000 || st.Tax != 500 || st.Payout != 9500 || st.Trades != 1 {
		t.Fatalf("unexpected trade settlement: %+v", st)
	}
	if got := l.Payout("seller-1"); got != 9500 {
		t.Fatalf("payout: want 9500, got %d", got)
	}
	if got := l.TaxCollected(); got != 500 {
		t.Fatalf("tax collected: want 500, got %d", got)
	}
	tr, err := l.Trade("t-1")
	if err != nil {
		t.Fatalf("Trade: %v", err)
	}
	if tr.Gross != 10000 || tr.Seller != "seller-1" {
		t.Fatalf("unexpected stored trade: %+v", tr)
	}
}

func TestRecordIsIdempotent(t *testing.T) {
	l := newTax(t)
	first := recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))
	again := recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))

	if again != first {
		t.Fatalf("replay returned %+v, first %+v", again, first)
	}
	if got := l.Payout("seller-1"); got != 9500 {
		t.Fatalf("replayed trade double-credited payout: %d", got)
	}
	if got := l.TaxCollected(); got != 500 {
		t.Fatalf("replayed trade double-counted tax: %d", got)
	}
	if got := len(l.Trades("seller-1")); got != 1 {
		t.Fatalf("replayed trade stored again: %d", got)
	}
}

func TestRecordRejectsTradeIDConflict(t *testing.T) {
	l := newTax(t)
	recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))

	_, err := l.Record(trade("t-1", "seller-1", "buyer-1", 12000, taxDay1))
	if !errors.Is(err, taskpkg.ErrTradeIDConflict) {
		t.Fatalf("reused trade id must be ErrTradeIDConflict, got %v", err)
	}
	if got := l.Payout("seller-1"); got != 9500 {
		t.Fatalf("conflict changed payout: %d", got)
	}
	if got := l.TaxCollected(); got != 500 {
		t.Fatalf("conflict changed tax: %d", got)
	}
}

func TestInvalidTradesRejected(t *testing.T) {
	l := newTax(t)
	cases := []taskpkg.Trade{
		trade("t-1", "seller-1", "seller-1", 10000, taxDay1),
		trade("t-2", "seller-1", "buyer-1", 0, taxDay1),
		trade("t-3", "", "buyer-1", 10000, taxDay1),
		trade("", "seller-1", "buyer-1", 10000, taxDay1),
		trade("t-4", "seller-1", "", 10000, taxDay1),
	}
	for _, tr := range cases {
		if _, err := l.Record(tr); !errors.Is(err, taskpkg.ErrBadTrade) {
			t.Fatalf("trade %+v must be ErrBadTrade, got %v", tr, err)
		}
	}
	if got := l.Payout("seller-1"); got != 0 {
		t.Fatalf("invalid trades credited payout: %d", got)
	}
	if got := l.TaxCollected(); got != 0 {
		t.Fatalf("invalid trades collected tax: %d", got)
	}
	if got := len(l.Trades("seller-1")); got != 0 {
		t.Fatalf("invalid trades were stored: %d", got)
	}
}

func TestSellerSettlementAggregates(t *testing.T) {
	l := newTax(t)
	recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))
	recordTrade(t, l, trade("t-2", "seller-2", "buyer-2", 5000, taxDay1))
	recordTrade(t, l, trade("t-3", "seller-1", "buyer-3", 20000, taxDay1))

	got := l.SellerSettlement("seller-1")
	if got.Seller != "seller-1" || got.Gross != 30000 || got.Tax != 1500 || got.Payout != 28500 || got.Trades != 2 {
		t.Fatalf("unexpected aggregate: %+v", got)
	}
	if p := l.Payout("seller-1"); p != 28500 {
		t.Fatalf("payout: want 28500, got %d", p)
	}
	if ids := l.Trades("seller-1"); len(ids) != 2 || ids[0] != "t-1" || ids[1] != "t-3" {
		t.Fatalf("seller trade ids: %v", ids)
	}
}

func TestReverseRestoresTotals(t *testing.T) {
	l := newTax(t)
	recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))

	rev, err := l.Reverse("t-1", "buyer chargeback")
	if err != nil {
		t.Fatalf("Reverse: %v", err)
	}
	if rev.TradeID != "t-1" || rev.Gross != 10000 {
		t.Fatalf("unexpected reversed trade: %+v", rev)
	}
	if got := l.Payout("seller-1"); got != 0 {
		t.Fatalf("payout after reversal: want 0, got %d", got)
	}
	if got := l.TaxCollected(); got != 0 {
		t.Fatalf("tax after reversal: want 0, got %d", got)
	}
	if got := l.SellerSettlement("seller-1"); got.Trades != 0 || got.Gross != 0 || got.Payout != 0 {
		t.Fatalf("settlement after reversal: %+v", got)
	}
	if got := len(l.Trades("seller-1")); got != 0 {
		t.Fatalf("reversed trade still listed: %d", got)
	}
	if gross, tax := l.DailyTotals("2026-03-01"); gross != 0 || tax != 0 {
		t.Fatalf("daily totals after reversal: gross=%d tax=%d", gross, tax)
	}
}

func TestReverseTwiceRejected(t *testing.T) {
	l := newTax(t)
	recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))
	if _, err := l.Reverse("t-1", "first"); err != nil {
		t.Fatalf("first Reverse: %v", err)
	}

	if _, err := l.Reverse("t-1", "second"); !errors.Is(err, taskpkg.ErrAlreadyReversed) {
		t.Fatalf("second Reverse must be ErrAlreadyReversed, got %v", err)
	}
	if got := l.Payout("seller-1"); got != 0 {
		t.Fatalf("double reversal changed payout: %d", got)
	}
	if got := l.TaxCollected(); got != 0 {
		t.Fatalf("double reversal changed tax: %d", got)
	}
}

func TestReverseUnknownTrade(t *testing.T) {
	l := newTax(t)
	if _, err := l.Reverse("t-404", "ghost"); !errors.Is(err, taskpkg.ErrUnknownTrade) {
		t.Fatalf("unknown reversal must be ErrUnknownTrade, got %v", err)
	}
}

func TestDailyTotalsByUTCDay(t *testing.T) {
	l := newTax(t)
	recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))
	recordTrade(t, l, trade("t-2", "seller-2", "buyer-2", 20000, taxDay1))
	recordTrade(t, l, trade("t-3", "seller-1", "buyer-3", 5000, taxDay2))

	if gross, tax := l.DailyTotals("2026-03-01"); gross != 30000 || tax != 1500 {
		t.Fatalf("day one totals: gross=%d tax=%d", gross, tax)
	}
	if gross, tax := l.DailyTotals("2026-03-02"); gross != 5000 || tax != 250 {
		t.Fatalf("day two totals: gross=%d tax=%d", gross, tax)
	}
	if gross, tax := l.DailyTotals("2026-03-05"); gross != 0 || tax != 0 {
		t.Fatalf("empty day totals: gross=%d tax=%d", gross, tax)
	}
}

func TestSnapshotRestoresLedger(t *testing.T) {
	l := newTax(t)
	recordTrade(t, l, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))
	recordTrade(t, l, trade("t-2", "seller-1", "buyer-2", 20000, taxDay1))
	if _, err := l.Reverse("t-2", "chargeback"); err != nil {
		t.Fatalf("Reverse: %v", err)
	}

	snap, err := l.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewTaxLedgerFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewTaxLedgerFromSnapshot: %v", err)
	}
	if got := restored.Payout("seller-1"); got != 9500 {
		t.Fatalf("restored payout: want 9500, got %d", got)
	}
	if got := restored.TaxCollected(); got != 500 {
		t.Fatalf("restored tax: want 500, got %d", got)
	}
	if gross, tax := restored.DailyTotals("2026-03-01"); gross != 10000 || tax != 500 {
		t.Fatalf("restored daily totals: gross=%d tax=%d", gross, tax)
	}
	if ids := restored.Trades("seller-1"); len(ids) != 1 || ids[0] != "t-1" {
		t.Fatalf("restored trade ids: %v", ids)
	}

	again := recordTrade(t, restored, trade("t-1", "seller-1", "buyer-1", 10000, taxDay1))
	if again.Gross != 10000 || restored.Payout("seller-1") != 9500 || restored.TaxCollected() != 500 {
		t.Fatalf("replayed trade after restore changed totals: %+v", again)
	}
	if _, err := restored.Reverse("t-2", "again"); !errors.Is(err, taskpkg.ErrAlreadyReversed) {
		t.Fatalf("reversal state lost across restore, got %v", err)
	}
}

func TestSnapshotRejectsGarbage(t *testing.T) {
	if _, err := taskpkg.NewTaxLedgerFromSnapshot(nil); !errors.Is(err, taskpkg.ErrSnapshotInvalid) {
		t.Fatalf("nil snapshot must be ErrSnapshotInvalid, got %v", err)
	}
	if _, err := taskpkg.NewTaxLedgerFromSnapshot([]byte("tax? no")); !errors.Is(err, taskpkg.ErrSnapshotInvalid) {
		t.Fatalf("garbage snapshot must be ErrSnapshotInvalid, got %v", err)
	}
}

func TestZeroRateRuleStillChargesMinTax(t *testing.T) {
	l, err := taskpkg.NewTaxLedger(taskpkg.TaxRule{RateBps: 0, MinTax: 7, CapTax: 100})
	if err != nil {
		t.Fatalf("NewTaxLedger: %v", err)
	}
	tax, err := l.PreviewTax(5000)
	if err != nil {
		t.Fatalf("PreviewTax: %v", err)
	}
	if tax != 7 {
		t.Fatalf("a zero-rate rule must still charge MinTax: want 7, got %d", tax)
	}
	if _, err := taskpkg.NewTaxLedger(taskpkg.TaxRule{RateBps: -5, MinTax: 0, CapTax: 100}); !errors.Is(err, taskpkg.ErrBadRule) {
		t.Fatalf("negative rate must be ErrBadRule, got %v", err)
	}
	if _, err := taskpkg.NewTaxLedger(taskpkg.TaxRule{RateBps: 500, MinTax: 100, CapTax: 50}); !errors.Is(err, taskpkg.ErrBadRule) {
		t.Fatalf("CapTax below MinTax must be ErrBadRule, got %v", err)
	}
}

func TestConcurrentRecordsTotalsExact(t *testing.T) {
	l := newTax(t)
	var wg sync.WaitGroup
	errs := make([]error, 40)
	for i := 0; i < 40; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = l.Record(trade(fmt.Sprintf("ct-%02d", i), "seller-c", fmt.Sprintf("buyer-%02d", i), 1000, taxDay1))
		}()
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("record %d failed: %v", i, err)
		}
	}
	if got := l.Payout("seller-c"); got != 38000 {
		t.Fatalf("payout after concurrent records: want 38000, got %d", got)
	}
	if got := l.TaxCollected(); got != 2000 {
		t.Fatalf("tax after concurrent records: want 2000, got %d", got)
	}
	if got := len(l.Trades("seller-c")); got != 40 {
		t.Fatalf("stored trades: want 40, got %d", got)
	}
	if got := l.SellerSettlement("seller-c"); got.Gross != 40000 || got.Tax != 2000 || got.Payout != 38000 || got.Trades != 40 {
		t.Fatalf("aggregate after concurrent records: %+v", got)
	}
}
