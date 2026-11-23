package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-a-10"
)

var (
	gemNow  = time.Date(2026, 3, 1, 9, 0, 0, 0, time.UTC)
	gemDay2 = time.Date(2026, 3, 2, 0, 30, 0, 0, time.UTC)
)

func newSink(t *testing.T, window time.Duration) *taskpkg.GemSink {
	t.Helper()
	l, err := taskpkg.NewGemSink(window)
	if err != nil {
		t.Fatalf("NewGemSink: %v", err)
	}
	return l
}

func topUp(t *testing.T, l *taskpkg.GemSink, player string, gems int64, ref string) {
	t.Helper()
	if err := l.TopUp(player, gems, ref); err != nil {
		t.Fatalf("TopUp(%s, %d, %s): %v", player, gems, ref, err)
	}
}

func spendReq(entryID, player string, amount int64, refundable bool, at time.Time) taskpkg.SpendRequest {
	return taskpkg.SpendRequest{
		EntryID:    entryID,
		Player:     player,
		Amount:     amount,
		Feature:    "gacha",
		Refundable: refundable,
		At:         at,
	}
}

func TestTopUpCreditsBalance(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	topUp(t, l, "p1", 700, "inv-2")

	if got := l.Balance("p1"); got != 1200 {
		t.Fatalf("balance: want 1200, got %d", got)
	}
	if got := l.Balance("p2"); got != 0 {
		t.Fatalf("unknown player balance: want 0, got %d", got)
	}
	if got := len(l.Entries("p1")); got != 0 {
		t.Fatalf("top-ups must not create spend entries: %d", got)
	}
}

func TestTopUpIsIdempotentByRef(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	topUp(t, l, "p1", 500, "inv-1")

	if got := l.Balance("p1"); got != 500 {
		t.Fatalf("replayed top-up double-credited: %d", got)
	}
}

func TestTopUpRefConflict(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")

	err := l.TopUp("p1", 600, "inv-1")
	if !errors.Is(err, taskpkg.ErrTopUpRefConflict) {
		t.Fatalf("reused reference must be ErrTopUpRefConflict, got %v", err)
	}
	if got := l.Balance("p1"); got != 500 {
		t.Fatalf("conflicting top-up changed balance: %d", got)
	}
}

func TestSpendDeductsAndRecords(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")

	bal, err := l.Spend(spendReq("sp-1", "p1", 200, true, gemNow))
	if err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if bal != 300 || l.Balance("p1") != 300 {
		t.Fatalf("balance after spend: return=%d stored=%d", bal, l.Balance("p1"))
	}
	e, err := l.Entry("sp-1")
	if err != nil {
		t.Fatalf("Entry: %v", err)
	}
	if e.Player != "p1" || e.Amount != 200 || e.Feature != "gacha" || !e.Refundable || e.Refunded {
		t.Fatalf("unexpected entry: %+v", e)
	}
	if ids := l.Entries("p1"); len(ids) != 1 || ids[0] != "sp-1" {
		t.Fatalf("entries: %v", ids)
	}
	if got := l.DailySpent("p1", "2026-03-01"); got != 200 {
		t.Fatalf("daily spent: want 200, got %d", got)
	}
}

func TestSpendInsufficientGems(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 100, "inv-1")

	_, err := l.Spend(spendReq("sp-1", "p1", 101, true, gemNow))
	if !errors.Is(err, taskpkg.ErrInsufficientGems) {
		t.Fatalf("overspend must be ErrInsufficientGems, got %v", err)
	}
	if got := l.Balance("p1"); got != 100 {
		t.Fatalf("rejected spend changed balance: %d", got)
	}
	if got := len(l.Entries("p1")); got != 0 {
		t.Fatalf("rejected spend was recorded: %d", got)
	}
	if got := l.DailySpent("p1", "2026-03-01"); got != 0 {
		t.Fatalf("rejected spend hit the daily counter: %d", got)
	}
}

func TestSpendIsIdempotentByEntryID(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	req := spendReq("sp-1", "p1", 200, true, gemNow)

	first, err := l.Spend(req)
	if err != nil {
		t.Fatalf("first Spend: %v", err)
	}
	again, err := l.Spend(req)
	if err != nil {
		t.Fatalf("replayed Spend must succeed: %v", err)
	}
	if again != first || again != 300 {
		t.Fatalf("replay returned %d, first %d", again, first)
	}
	if got := l.Balance("p1"); got != 300 {
		t.Fatalf("replay double-charged: %d", got)
	}
	if got := len(l.Entries("p1")); got != 1 {
		t.Fatalf("replay stored another entry: %d", got)
	}
	if got := l.DailySpent("p1", "2026-03-01"); got != 200 {
		t.Fatalf("replay double-counted daily spend: %d", got)
	}
}

func TestSpendEntryIDConflict(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	if _, err := l.Spend(spendReq("sp-1", "p1", 200, true, gemNow)); err != nil {
		t.Fatalf("first Spend: %v", err)
	}

	_, err := l.Spend(spendReq("sp-1", "p1", 250, true, gemNow))
	if !errors.Is(err, taskpkg.ErrEntryIDConflict) {
		t.Fatalf("reused entry id must be ErrEntryIDConflict, got %v", err)
	}
	if got := l.Balance("p1"); got != 300 {
		t.Fatalf("conflicting spend changed balance: %d", got)
	}
	if got := l.DailySpent("p1", "2026-03-01"); got != 200 {
		t.Fatalf("conflicting spend changed daily counter: %d", got)
	}
}

func TestDailyCapBlocksSpend(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	if err := l.SetDailyCap("p1", 300); err != nil {
		t.Fatalf("SetDailyCap: %v", err)
	}
	topUp(t, l, "p1", 1000, "inv-1")
	if _, err := l.Spend(spendReq("sp-1", "p1", 200, true, gemNow)); err != nil {
		t.Fatalf("first Spend: %v", err)
	}

	_, err := l.Spend(spendReq("sp-2", "p1", 150, true, gemNow))
	if !errors.Is(err, taskpkg.ErrDailyCap) {
		t.Fatalf("spend above the cap must be ErrDailyCap, got %v", err)
	}
	if got := l.Balance("p1"); got != 800 {
		t.Fatalf("capped spend changed balance: %d", got)
	}
	if got := l.DailySpent("p1", "2026-03-01"); got != 200 {
		t.Fatalf("capped spend hit the daily counter: %d", got)
	}
}

func TestDailyCapResetsNextDay(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	if err := l.SetDailyCap("p1", 300); err != nil {
		t.Fatalf("SetDailyCap: %v", err)
	}
	topUp(t, l, "p1", 1000, "inv-1")

	if _, err := l.Spend(spendReq("sp-1", "p1", 300, true, gemNow)); err != nil {
		t.Fatalf("day one Spend: %v", err)
	}
	if _, err := l.Spend(spendReq("sp-2", "p1", 300, true, gemDay2)); err != nil {
		t.Fatalf("day two Spend must use the new cap: %v", err)
	}
	if got := l.Balance("p1"); got != 400 {
		t.Fatalf("balance: want 400, got %d", got)
	}
	if got := l.DailySpent("p1", "2026-03-01"); got != 300 {
		t.Fatalf("day one spend: want 300, got %d", got)
	}
	if got := l.DailySpent("p1", "2026-03-02"); got != 300 {
		t.Fatalf("day two spend: want 300, got %d", got)
	}
}

func TestRefundWithinWindow(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	if _, err := l.Spend(spendReq("sp-1", "p1", 200, true, gemNow)); err != nil {
		t.Fatalf("Spend: %v", err)
	}

	bal, err := l.Refund("sp-1", "p1", gemNow.Add(time.Hour))
	if err != nil {
		t.Fatalf("Refund: %v", err)
	}
	if bal != 500 || l.Balance("p1") != 500 {
		t.Fatalf("balance after refund: return=%d stored=%d", bal, l.Balance("p1"))
	}
	e, err := l.Entry("sp-1")
	if err != nil {
		t.Fatalf("Entry: %v", err)
	}
	if !e.Refunded {
		t.Fatalf("entry not marked refunded")
	}
	if got := l.DailySpent("p1", "2026-03-01"); got != 0 {
		t.Fatalf("refund must release the daily counter: got %d", got)
	}
}

func TestRefundAfterWindowRejected(t *testing.T) {
	l := newSink(t, time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	if _, err := l.Spend(spendReq("sp-1", "p1", 200, true, gemNow)); err != nil {
		t.Fatalf("Spend: %v", err)
	}

	_, err := l.Refund("sp-1", "p1", gemNow.Add(2*time.Hour))
	if !errors.Is(err, taskpkg.ErrRefundWindowClosed) {
		t.Fatalf("late refund must be ErrRefundWindowClosed, got %v", err)
	}
	if got := l.Balance("p1"); got != 300 {
		t.Fatalf("late refund changed balance: %d", got)
	}
	e, err := l.Entry("sp-1")
	if err != nil {
		t.Fatalf("Entry: %v", err)
	}
	if e.Refunded {
		t.Fatalf("late refund marked the entry refunded")
	}
}

func TestRefundTwiceRejected(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	if _, err := l.Spend(spendReq("sp-1", "p1", 200, true, gemNow)); err != nil {
		t.Fatalf("Spend: %v", err)
	}
	if _, err := l.Refund("sp-1", "p1", gemNow.Add(time.Hour)); err != nil {
		t.Fatalf("first Refund: %v", err)
	}

	_, err := l.Refund("sp-1", "p1", gemNow.Add(2*time.Hour))
	if !errors.Is(err, taskpkg.ErrAlreadyRefunded) {
		t.Fatalf("second refund must be ErrAlreadyRefunded, got %v", err)
	}
	if got := l.Balance("p1"); got != 500 {
		t.Fatalf("double refund changed balance: %d", got)
	}
}

func TestRefundNonRefundableEntry(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	if _, err := l.Spend(spendReq("sp-1", "p1", 200, false, gemNow)); err != nil {
		t.Fatalf("Spend: %v", err)
	}

	_, err := l.Refund("sp-1", "p1", gemNow.Add(time.Hour))
	if !errors.Is(err, taskpkg.ErrNotRefundable) {
		t.Fatalf("bonus spend must be ErrNotRefundable, got %v", err)
	}
	if got := l.Balance("p1"); got != 300 {
		t.Fatalf("rejected refund changed balance: %d", got)
	}
}

func TestSnapshotRestoresSink(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 500, "inv-1")
	if err := l.SetDailyCap("p1", 500); err != nil {
		t.Fatalf("SetDailyCap: %v", err)
	}
	if _, err := l.Spend(spendReq("sp-1", "p1", 200, true, gemNow)); err != nil {
		t.Fatalf("spend sp-1: %v", err)
	}
	if _, err := l.Refund("sp-1", "p1", gemNow.Add(time.Hour)); err != nil {
		t.Fatalf("refund sp-1: %v", err)
	}
	if _, err := l.Spend(spendReq("sp-2", "p1", 100, true, gemNow.Add(2*time.Hour))); err != nil {
		t.Fatalf("spend sp-2: %v", err)
	}

	snap, err := l.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewGemSinkFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewGemSinkFromSnapshot: %v", err)
	}
	if got := restored.Balance("p1"); got != 400 {
		t.Fatalf("restored balance: want 400, got %d", got)
	}
	if ids := restored.Entries("p1"); len(ids) != 2 || ids[0] != "sp-1" || ids[1] != "sp-2" {
		t.Fatalf("restored entries: %v", ids)
	}
	e, err := restored.Entry("sp-1")
	if err != nil {
		t.Fatalf("restored Entry: %v", err)
	}
	if !e.Refunded || e.Amount != 200 {
		t.Fatalf("restored entry: %+v", e)
	}
	if got := restored.DailySpent("p1", "2026-03-01"); got != 100 {
		t.Fatalf("restored daily spent: want 100, got %d", got)
	}
	bal, err := restored.Spend(spendReq("sp-2", "p1", 100, true, gemNow.Add(2*time.Hour)))
	if err != nil {
		t.Fatalf("replayed spend after restore: %v", err)
	}
	if bal != 400 {
		t.Fatalf("replayed spend after restore changed balance: %d", bal)
	}
}

func TestConcurrentSpendNoOverdraw(t *testing.T) {
	l := newSink(t, 24*time.Hour)
	topUp(t, l, "p1", 100, "inv-1")

	var wg sync.WaitGroup
	errs := make([]error, 30)
	for i := 0; i < 30; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = l.Spend(spendReq(fmt.Sprintf("sp-%02d", i), "p1", 10, true, gemNow))
		}()
	}
	wg.Wait()

	success := 0
	for _, err := range errs {
		if err == nil {
			success++
			continue
		}
		if !errors.Is(err, taskpkg.ErrInsufficientGems) {
			t.Fatalf("unexpected spend error: %v", err)
		}
	}
	if success != 10 {
		t.Fatalf("balance 100 in 10s must allow 10 spends, got %d", success)
	}
	if got := l.Balance("p1"); got != 0 {
		t.Fatalf("balance after race: want 0, got %d", got)
	}
	if got := len(l.Entries("p1")); got != 10 {
		t.Fatalf("stored spends: want 10, got %d", got)
	}
	if got := l.DailySpent("p1", "2026-03-01"); got != 100 {
		t.Fatalf("daily spent after race: want 100, got %d", got)
	}
}
