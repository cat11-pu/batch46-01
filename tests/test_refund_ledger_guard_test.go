package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-a-08"
)

var refundNow = time.Date(2026, 3, 1, 10, 0, 0, 0, time.UTC)

func newLedger(t *testing.T, floor int64) *taskpkg.RefundLedger {
	t.Helper()
	l, err := taskpkg.NewRefundLedger(floor)
	if err != nil {
		t.Fatalf("NewRefundLedger(%d): %v", floor, err)
	}
	return l
}

func grant(t *testing.T, l *taskpkg.RefundLedger, orderID, player string, amount int64) {
	t.Helper()
	if err := l.Grant(taskpkg.Order{OrderID: orderID, Player: player, Amount: amount, At: refundNow}); err != nil {
		t.Fatalf("Grant(%s): %v", orderID, err)
	}
}

func refundReq(refundID, orderID, player string, amount int64) taskpkg.RefundRequest {
	return taskpkg.RefundRequest{
		RefundID: refundID,
		OrderID:  orderID,
		Player:   player,
		Amount:   amount,
		Reason:   "chargeback",
		At:       refundNow.Add(time.Hour),
	}
}

func TestGrantCreditsBalance(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)

	if got := l.Balance("p1"); got != 1000 {
		t.Fatalf("balance: want 1000, got %d", got)
	}
	o, err := l.Order("o-1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if o.Amount != 1000 || o.Refunded != 0 || o.Player != "p1" {
		t.Fatalf("unexpected order: %+v", o)
	}
	if got := l.Refunds("o-1"); len(got) != 0 {
		t.Fatalf("fresh order has refunds: %+v", got)
	}
}

func TestDuplicateOrderRejected(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)

	err := l.Grant(taskpkg.Order{OrderID: "o-1", Player: "p1", Amount: 900, At: refundNow})
	if !errors.Is(err, taskpkg.ErrDuplicateOrder) {
		t.Fatalf("duplicate order must be ErrDuplicateOrder, got %v", err)
	}
	if got := l.Balance("p1"); got != 1000 {
		t.Fatalf("duplicate order changed the balance: %d", got)
	}
}

func TestRefundClawsBackIntoDebt(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)
	if err := l.Spend("p1", 1000); err != nil {
		t.Fatalf("Spend: %v", err)
	}

	bal, err := l.Refund(refundReq("rf-1", "o-1", "p1", 400))
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if bal != -400 || l.Balance("p1") != -400 {
		t.Fatalf("balance after clawback: return=%d stored=%d", bal, l.Balance("p1"))
	}
	if !l.DebtLocked("p1") {
		t.Fatalf("negative balance must lock spending")
	}
	o, err := l.Order("o-1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if o.Refunded != 400 {
		t.Fatalf("order refunded total: want 400, got %d", o.Refunded)
	}
}

func TestRefundExceedsOrderAmount(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)
	if _, err := l.Refund(refundReq("rf-1", "o-1", "p1", 600)); err != nil {
		t.Fatalf("first refund: %v", err)
	}

	_, err := l.Refund(refundReq("rf-2", "o-1", "p1", 500))
	if !errors.Is(err, taskpkg.ErrRefundExceeds) {
		t.Fatalf("over-refund must be ErrRefundExceeds, got %v", err)
	}
	if got := l.Balance("p1"); got != 1600 {
		t.Fatalf("rejected refund changed balance: %d", got)
	}
	o, err := l.Order("o-1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if o.Refunded != 600 {
		t.Fatalf("refunded total after rejection: want 600, got %d", o.Refunded)
	}
	if got := len(l.Refunds("o-1")); got != 1 {
		t.Fatalf("rejected refund was recorded: %d", got)
	}
}

func TestPartialRefundsAccumulate(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)

	amounts := []int64{250, 350, 400}
	for i, amount := range amounts {
		if _, err := l.Refund(refundReq(fmt.Sprintf("rf-%d", i+1), "o-1", "p1", amount)); err != nil {
			t.Fatalf("refund %d: %v", i+1, err)
		}
	}
	o, err := l.Order("o-1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if o.Refunded != 1000 {
		t.Fatalf("refunded total: want 1000, got %d", o.Refunded)
	}
	if got := l.Balance("p1"); got != 2000 {
		t.Fatalf("balance: want 2000, got %d", got)
	}
	if _, err := l.Refund(refundReq("rf-4", "o-1", "p1", 1)); !errors.Is(err, taskpkg.ErrRefundExceeds) {
		t.Fatalf("refund on a fully refunded order must fail, got %v", err)
	}
	if got := len(l.Refunds("o-1")); got != 3 {
		t.Fatalf("refund records: want 3, got %d", got)
	}
}

func TestDebtFloorBlocksRefund(t *testing.T) {
	l := newLedger(t, 300)
	grant(t, l, "o-1", "p1", 1000)
	if err := l.Spend("p1", 1000); err != nil {
		t.Fatalf("Spend: %v", err)
	}

	_, err := l.Refund(refundReq("rf-big", "o-1", "p1", 500))
	if !errors.Is(err, taskpkg.ErrFloorBreach) {
		t.Fatalf("refund below the floor must be ErrFloorBreach, got %v", err)
	}
	if got := l.Balance("p1"); got != 0 {
		t.Fatalf("rejected refund changed balance: %d", got)
	}
	if _, ok := l.RefundByID("rf-big"); ok {
		t.Fatalf("rejected refund was stored")
	}
	o, err := l.Order("o-1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if o.Refunded != 0 {
		t.Fatalf("rejected refund updated the order: %d", o.Refunded)
	}

	bal, err := l.Refund(refundReq("rf-edge", "o-1", "p1", 300))
	if err != nil {
		t.Fatalf("refund to the exact floor must succeed: %v", err)
	}
	if bal != -300 {
		t.Fatalf("balance at the floor: want -300, got %d", bal)
	}
}

func TestRefundRejectsForeignPlayer(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)

	_, err := l.Refund(refundReq("rf-1", "o-1", "p2", 400))
	if !errors.Is(err, taskpkg.ErrOrderOwner) {
		t.Fatalf("refund by another player must be ErrOrderOwner, got %v", err)
	}
	if got := l.Balance("p1"); got != 1000 {
		t.Fatalf("balance changed: %d", got)
	}
	if got := len(l.Refunds("o-1")); got != 0 {
		t.Fatalf("refund recorded for wrong player: %d", got)
	}
}

func TestRefundUnknownOrder(t *testing.T) {
	l := newLedger(t, 5000)
	_, err := l.Refund(refundReq("rf-1", "o-404", "p1", 100))
	if !errors.Is(err, taskpkg.ErrUnknownOrder) {
		t.Fatalf("refund of an unknown order must be ErrUnknownOrder, got %v", err)
	}
	if got := l.Balance("p1"); got != 0 {
		t.Fatalf("unknown order credited balance: %d", got)
	}
}

func TestRefundIDIsIdempotent(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)
	req := refundReq("rf-1", "o-1", "p1", 400)

	first, err := l.Refund(req)
	if err != nil {
		t.Fatalf("first refund: %v", err)
	}
	again, err := l.Refund(req)
	if err != nil {
		t.Fatalf("replayed refund must succeed: %v", err)
	}
	if again != first || again != 1400 {
		t.Fatalf("replay returned %d, first %d", again, first)
	}
	o, err := l.Order("o-1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if o.Refunded != 400 {
		t.Fatalf("replay double-counted the refund: %d", o.Refunded)
	}
	if got := len(l.Refunds("o-1")); got != 1 {
		t.Fatalf("replay stored another record: %d", got)
	}
}

func TestRefundIDConflict(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)
	if _, err := l.Refund(refundReq("rf-1", "o-1", "p1", 400)); err != nil {
		t.Fatalf("first refund: %v", err)
	}

	_, err := l.Refund(refundReq("rf-1", "o-1", "p1", 300))
	if !errors.Is(err, taskpkg.ErrRefundIDConflict) {
		t.Fatalf("reused refund id must be ErrRefundIDConflict, got %v", err)
	}
	if got := l.Balance("p1"); got != 1400 {
		t.Fatalf("conflicting refund changed balance: %d", got)
	}
	o, err := l.Order("o-1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if o.Refunded != 400 {
		t.Fatalf("conflicting refund updated the order: %d", o.Refunded)
	}
}

func TestSpendInsufficientCredits(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 100)

	err := l.Spend("p1", 150)
	if !errors.Is(err, taskpkg.ErrInsufficientCredits) {
		t.Fatalf("overspend must be ErrInsufficientCredits, got %v", err)
	}
	if got := l.Balance("p1"); got != 100 {
		t.Fatalf("rejected spend changed balance: %d", got)
	}
	if l.DebtLocked("p1") {
		t.Fatalf("positive balance must not be locked")
	}
}

func TestDebtLocksSpendingUntilRepaid(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)
	if err := l.Spend("p1", 1000); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if _, err := l.Refund(refundReq("rf-1", "o-1", "p1", 400)); err != nil {
		t.Fatalf("Refund: %v", err)
	}

	if err := l.Spend("p1", 10); !errors.Is(err, taskpkg.ErrDebtLocked) {
		t.Fatalf("spending with debt must be ErrDebtLocked, got %v", err)
	}
	if err := l.Repay("p1", 400); err != nil {
		t.Fatalf("Repay: %v", err)
	}
	if got := l.Balance("p1"); got != 0 {
		t.Fatalf("balance after repay: want 0, got %d", got)
	}
	if l.DebtLocked("p1") {
		t.Fatalf("repaid account must be unlocked")
	}
	if err := l.Spend("p1", 10); !errors.Is(err, taskpkg.ErrInsufficientCredits) {
		t.Fatalf("unlocked overspend must be ErrInsufficientCredits, got %v", err)
	}
	if err := l.Repay("p1", 10); err != nil {
		t.Fatalf("Repay: %v", err)
	}
	if err := l.Spend("p1", 10); err != nil {
		t.Fatalf("spend after repay: %v", err)
	}
	if got := l.Balance("p1"); got != 0 {
		t.Fatalf("final balance: want 0, got %d", got)
	}
}

func TestSnapshotRestoresLedger(t *testing.T) {
	l := newLedger(t, 5000)
	grant(t, l, "o-1", "p1", 1000)
	grant(t, l, "o-2", "p1", 500)
	if _, err := l.Refund(refundReq("rf-1", "o-1", "p1", 400)); err != nil {
		t.Fatalf("refund o-1: %v", err)
	}
	if _, err := l.Refund(refundReq("rf-2", "o-2", "p1", 200)); err != nil {
		t.Fatalf("refund o-2: %v", err)
	}

	snap, err := l.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewRefundLedgerFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewRefundLedgerFromSnapshot: %v", err)
	}
	if got := restored.Balance("p1"); got != 2100 {
		t.Fatalf("restored balance: want 2100, got %d", got)
	}
	o, err := restored.Order("o-1")
	if err != nil {
		t.Fatalf("restored Order: %v", err)
	}
	if o.Refunded != 400 {
		t.Fatalf("restored refunded total: want 400, got %d", o.Refunded)
	}
	r, ok := restored.RefundByID("rf-1")
	if !ok || r.Amount != 400 || r.OrderID != "o-1" {
		t.Fatalf("restored refund: ok=%v refund=%+v", ok, r)
	}
	bal, err := restored.Refund(refundReq("rf-1", "o-1", "p1", 400))
	if err != nil {
		t.Fatalf("replayed refund after restore: %v", err)
	}
	if bal != 2100 {
		t.Fatalf("replayed refund after restore changed balance: %d", bal)
	}
}

func TestSnapshotRejectsGarbage(t *testing.T) {
	if _, err := taskpkg.NewRefundLedgerFromSnapshot(nil); !errors.Is(err, taskpkg.ErrSnapshotInvalid) {
		t.Fatalf("nil snapshot must be ErrSnapshotInvalid, got %v", err)
	}
	if _, err := taskpkg.NewRefundLedgerFromSnapshot([]byte{0xff, 0x00}); !errors.Is(err, taskpkg.ErrSnapshotInvalid) {
		t.Fatalf("garbage snapshot must be ErrSnapshotInvalid, got %v", err)
	}
}

func TestConcurrentRefundsRespectCap(t *testing.T) {
	l := newLedger(t, 1000)
	grant(t, l, "o-1", "p1", 500)

	var wg sync.WaitGroup
	errs := make([]error, 20)
	for i := 0; i < 20; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = l.Refund(refundReq(fmt.Sprintf("rf-%02d", i), "o-1", "p1", 100))
		}()
	}
	wg.Wait()

	success := 0
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		if !errors.Is(err, taskpkg.ErrRefundExceeds) {
			t.Fatalf("unexpected refund error: %v", err)
		}
	}
	if success != 5 {
		t.Fatalf("order 500 refunded in 100s must allow 5 refunds, got %d", success)
	}
	o, err := l.Order("o-1")
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	if o.Refunded != 500 {
		t.Fatalf("refunded total: want 500, got %d", o.Refunded)
	}
	if got := l.Balance("p1"); got != 1000 {
		t.Fatalf("balance: want 1000, got %d", got)
	}
	if got := len(l.Refunds("o-1")); got != 5 {
		t.Fatalf("stored refunds: want 5, got %d", got)
	}
}
