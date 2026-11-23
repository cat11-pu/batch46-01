package task

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-b-06"
)

var bankNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

func bankHash(prev, opID, member string, tabIndex int, itemID string, count int) string {
	sum := sha256.Sum256([]byte(prev + "|" + opID + "|" + member + "|" + strconv.Itoa(tabIndex) + "|" + itemID + "|" + strconv.Itoa(count)))
	return hex.EncodeToString(sum[:])
}

func mustBank(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("setup failed: %v", err)
	}
}

func mustDeposit(t *testing.T, g *taskpkg.GuildBankGuard, opID, member string, tab int, item taskpkg.Item, at time.Time) {
	t.Helper()
	if _, err := g.Deposit(opID, member, tab, item, at); err != nil {
		t.Fatalf("setup deposit failed: %v", err)
	}
}

func newSeededBank(t *testing.T) *taskpkg.GuildBankGuard {
	t.Helper()
	g := taskpkg.NewGuildBankGuard(taskpkg.Config{DailyQuota: 10, LedgerCapacity: 100})
	mustBank(t, g.AddMember(taskpkg.Member{Name: "alice", Rank: taskpkg.RankOfficer}))
	mustBank(t, g.AddMember(taskpkg.Member{Name: "bob", Rank: taskpkg.RankMember}))
	mustBank(t, g.CreateTab(1, taskpkg.RankMember))
	mustBank(t, g.CreateTab(2, taskpkg.RankOfficer))
	mustDeposit(t, g, "seed-ore", "alice", 1, taskpkg.Item{ID: "ore", Count: 100}, bankNow)
	mustDeposit(t, g, "seed-gold", "alice", 2, taskpkg.Item{ID: "gold", Count: 20}, bankNow)
	return g
}

func bankWithdraw(t *testing.T, g *taskpkg.GuildBankGuard, opID, member string, tab int, item string, count int, at time.Time) error {
	t.Helper()
	_, err := g.Withdraw(taskpkg.WithdrawalRequest{OpID: opID, Member: member, TabIndex: tab, ItemID: item, Count: count, At: at})
	return err
}

func TestWithdrawRequiresRank(t *testing.T) {
	g := newSeededBank(t)
	err := bankWithdraw(t, g, "w-rank", "bob", 2, "gold", 1, bankNow)
	if !errors.Is(err, taskpkg.ErrRankDenied) {
		t.Fatalf("member rank on an officer tab must yield ErrRankDenied, got %v", err)
	}
	items, cerr := g.TabContents(2)
	if cerr != nil {
		t.Fatalf("TabContents: %v", cerr)
	}
	if len(items) != 1 || items[0].ID != "gold" || items[0].Count != 20 {
		t.Fatalf("denied withdrawal changed the tab: %+v", items)
	}
}

func TestWithdrawUnknownMember(t *testing.T) {
	g := newSeededBank(t)
	err := bankWithdraw(t, g, "w-ghost", "carol", 1, "ore", 1, bankNow)
	if !errors.Is(err, taskpkg.ErrUnknownMember) {
		t.Fatalf("unknown member must yield ErrUnknownMember, got %v", err)
	}
}

func TestLockedTabRejectsWithdraw(t *testing.T) {
	g := newSeededBank(t)
	if err := g.LockTab(1); err != nil {
		t.Fatalf("LockTab: %v", err)
	}
	err := bankWithdraw(t, g, "w-locked", "alice", 1, "ore", 5, bankNow)
	if !errors.Is(err, taskpkg.ErrTabLocked) {
		t.Fatalf("locked tab must yield ErrTabLocked, got %v", err)
	}
}

func TestDailyQuotaRejectsOverLimitAtomically(t *testing.T) {
	g := newSeededBank(t)
	if err := bankWithdraw(t, g, "q-1", "alice", 1, "ore", 4, bankNow); err != nil {
		t.Fatalf("first withdrawal: %v", err)
	}
	if err := bankWithdraw(t, g, "q-2", "alice", 1, "ore", 4, bankNow); err != nil {
		t.Fatalf("second withdrawal: %v", err)
	}
	err := bankWithdraw(t, g, "q-3", "alice", 1, "ore", 3, bankNow)
	if !errors.Is(err, taskpkg.ErrQuotaExceeded) {
		t.Fatalf("over quota must yield ErrQuotaExceeded, got %v", err)
	}
	items, cerr := g.TabContents(1)
	if cerr != nil {
		t.Fatalf("TabContents: %v", cerr)
	}
	if len(items) != 1 || items[0].Count != 92 {
		t.Fatalf("rejected withdrawal must not touch stock, got %+v", items)
	}
	if got := g.DailyTotal("alice", bankNow); got != 8 {
		t.Fatalf("DailyTotal = %d, want 8", got)
	}
}

func TestQuotaResetsOnNextUTCDay(t *testing.T) {
	g := newSeededBank(t)
	late := time.Date(2026, 3, 1, 23, 58, 0, 0, time.UTC)
	if err := bankWithdraw(t, g, "d-1", "alice", 1, "ore", 10, late); err != nil {
		t.Fatalf("full quota withdrawal: %v", err)
	}
	if err := bankWithdraw(t, g, "d-2", "alice", 1, "ore", 1, late.Add(time.Minute)); !errors.Is(err, taskpkg.ErrQuotaExceeded) {
		t.Fatalf("same UTC day must stay blocked, got %v", err)
	}
	next := time.Date(2026, 3, 2, 0, 1, 0, 0, time.UTC)
	if err := bankWithdraw(t, g, "d-3", "alice", 1, "ore", 1, next); err != nil {
		t.Fatalf("new UTC day must reset quota, got %v", err)
	}
	if got := g.DailyTotal("alice", late); got != 10 {
		t.Fatalf("first day total = %d, want 10", got)
	}
	if got := g.DailyTotal("alice", next); got != 1 {
		t.Fatalf("second day total = %d, want 1", got)
	}
}

func TestDuplicateOpIDRejected(t *testing.T) {
	g := newSeededBank(t)
	if err := bankWithdraw(t, g, "w-dup", "alice", 1, "ore", 1, bankNow); err != nil {
		t.Fatalf("first withdrawal: %v", err)
	}
	err := bankWithdraw(t, g, "w-dup", "alice", 1, "ore", 1, bankNow.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrDuplicateOp) {
		t.Fatalf("duplicate operation id must yield ErrDuplicateOp, got %v", err)
	}
	items, cerr := g.TabContents(1)
	if cerr != nil {
		t.Fatalf("TabContents: %v", cerr)
	}
	if len(items) != 1 || items[0].Count != 99 {
		t.Fatalf("duplicate must apply once, got %+v", items)
	}
}

func TestInsufficientCountRejected(t *testing.T) {
	g := newSeededBank(t)
	err := bankWithdraw(t, g, "w-over", "alice", 1, "ore", 101, bankNow)
	if !errors.Is(err, taskpkg.ErrInsufficientCount) {
		t.Fatalf("count above stock must yield ErrInsufficientCount, got %v", err)
	}
	items, cerr := g.TabContents(1)
	if cerr != nil {
		t.Fatalf("TabContents: %v", cerr)
	}
	if len(items) != 1 || items[0].Count != 100 {
		t.Fatalf("rejected withdrawal changed stock: %+v", items)
	}
}

func TestInvalidCountRejected(t *testing.T) {
	g := newSeededBank(t)
	for _, count := range []int{0, -3} {
		if err := bankWithdraw(t, g, "w-bad"+strconv.Itoa(count), "alice", 1, "ore", count, bankNow); !errors.Is(err, taskpkg.ErrInvalidCount) {
			t.Fatalf("count %d must yield ErrInvalidCount, got %v", count, err)
		}
	}
}

func TestDepositMergesStacksInOrder(t *testing.T) {
	g := taskpkg.NewGuildBankGuard(taskpkg.Config{DailyQuota: 10, LedgerCapacity: 50})
	mustBank(t, g.AddMember(taskpkg.Member{Name: "alice", Rank: taskpkg.RankOfficer}))
	mustBank(t, g.CreateTab(1, taskpkg.RankMember))
	mustDeposit(t, g, "m-1", "alice", 1, taskpkg.Item{ID: "ore", Count: 5}, bankNow)
	mustDeposit(t, g, "m-2", "alice", 1, taskpkg.Item{ID: "gem", Count: 2}, bankNow)
	mustDeposit(t, g, "m-3", "alice", 1, taskpkg.Item{ID: "ore", Count: 3}, bankNow)
	items, err := g.TabContents(1)
	if err != nil {
		t.Fatalf("TabContents: %v", err)
	}
	if len(items) != 2 || items[0].ID != "ore" || items[0].Count != 8 || items[1].ID != "gem" || items[1].Count != 2 {
		t.Fatalf("deposit must merge by id and keep order, got %+v", items)
	}
	if err := bankWithdraw(t, g, "m-w", "alice", 1, "ore", 8, bankNow); err != nil {
		t.Fatalf("withdraw merged stack: %v", err)
	}
	items, err = g.TabContents(1)
	if err != nil {
		t.Fatalf("TabContents: %v", err)
	}
	if len(items) != 1 || items[0].ID != "gem" || items[0].Count != 2 {
		t.Fatalf("empty stack must be removed, got %+v", items)
	}
}

func TestDepositUnknownTabRejected(t *testing.T) {
	g := newSeededBank(t)
	_, err := g.Deposit("u-1", "alice", 9, taskpkg.Item{ID: "ore", Count: 1}, bankNow)
	if !errors.Is(err, taskpkg.ErrUnknownTab) {
		t.Fatalf("unknown tab must yield ErrUnknownTab, got %v", err)
	}
}

func TestLedgerHashChainIsVerifiable(t *testing.T) {
	g := newSeededBank(t)
	if err := bankWithdraw(t, g, "h-1", "alice", 1, "ore", 2, bankNow); err != nil {
		t.Fatalf("withdrawal: %v", err)
	}
	entries := g.Ledger()
	if len(entries) != 3 {
		t.Fatalf("ledger length = %d, want 3", len(entries))
	}
	if entries[0].PrevHash != "0000000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("first PrevHash = %q, want 64 zeros", entries[0].PrevHash)
	}
	for i, e := range entries {
		if e.Seq != i+1 {
			t.Fatalf("entry %d seq = %d, want %d", i, e.Seq, i+1)
		}
		want := bankHash(e.PrevHash, e.OpID, e.Member, e.TabIndex, e.ItemID, e.Count)
		if e.Hash != want {
			t.Fatalf("entry %d hash = %q, want %q", i, e.Hash, want)
		}
		if i > 0 && e.PrevHash != entries[i-1].Hash {
			t.Fatalf("entry %d PrevHash does not link to previous hash", i)
		}
	}
	if err := g.VerifyChain(); err != nil {
		t.Fatalf("VerifyChain on a clean ledger: %v", err)
	}
}

func TestLedgerCapacityRejectsAtomically(t *testing.T) {
	g := taskpkg.NewGuildBankGuard(taskpkg.Config{DailyQuota: 100, LedgerCapacity: 2})
	mustBank(t, g.AddMember(taskpkg.Member{Name: "alice", Rank: taskpkg.RankOfficer}))
	mustBank(t, g.CreateTab(1, taskpkg.RankMember))
	mustDeposit(t, g, "c-1", "alice", 1, taskpkg.Item{ID: "ore", Count: 5}, bankNow)
	mustDeposit(t, g, "c-2", "alice", 1, taskpkg.Item{ID: "ore", Count: 5}, bankNow)
	_, err := g.Deposit("c-3", "alice", 1, taskpkg.Item{ID: "ore", Count: 5}, bankNow)
	if !errors.Is(err, taskpkg.ErrLedgerFull) {
		t.Fatalf("full ledger must yield ErrLedgerFull, got %v", err)
	}
	items, cerr := g.TabContents(1)
	if cerr != nil {
		t.Fatalf("TabContents: %v", cerr)
	}
	if len(items) != 1 || items[0].Count != 10 {
		t.Fatalf("rejected deposit must not change stock, got %+v", items)
	}
	if got := len(g.Ledger()); got != 2 {
		t.Fatalf("ledger length = %d, want 2", got)
	}
}

func TestConcurrentWithdrawCannotOverdraw(t *testing.T) {
	g := taskpkg.NewGuildBankGuard(taskpkg.Config{DailyQuota: 100, LedgerCapacity: 100})
	mustBank(t, g.AddMember(taskpkg.Member{Name: "alice", Rank: taskpkg.RankOfficer}))
	mustBank(t, g.CreateTab(1, taskpkg.RankMember))
	mustDeposit(t, g, "race-seed", "alice", 1, taskpkg.Item{ID: "ore", Count: 4}, bankNow)
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = g.Withdraw(taskpkg.WithdrawalRequest{
				OpID:     "race-" + strconv.Itoa(i),
				Member:   "alice",
				TabIndex: 1,
				ItemID:   "ore",
				Count:    1,
				At:       bankNow,
			})
		}(i)
	}
	wg.Wait()
	successes, insufficient := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, taskpkg.ErrInsufficientCount) {
			insufficient++
		}
	}
	if successes != 4 || insufficient != 6 {
		t.Fatalf("successes=%d insufficient=%d, want 4 and 6", successes, insufficient)
	}
	items, cerr := g.TabContents(1)
	if cerr != nil {
		t.Fatalf("TabContents: %v", cerr)
	}
	if len(items) != 0 {
		t.Fatalf("stock must be exactly empty, got %+v", items)
	}
}

func TestSnapshotRestoresLedgerAndDedupe(t *testing.T) {
	g := newSeededBank(t)
	if err := bankWithdraw(t, g, "w-snap", "alice", 1, "ore", 3, bankNow); err != nil {
		t.Fatalf("withdrawal: %v", err)
	}
	before := g.Ledger()
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewGuildBankGuardFromSnapshot(taskpkg.Config{DailyQuota: 10, LedgerCapacity: 100}, snap)
	if err != nil {
		t.Fatalf("NewGuildBankGuardFromSnapshot: %v", err)
	}
	after := restored.Ledger()
	if len(after) != len(before) {
		t.Fatalf("ledger length after restore = %d, want %d", len(after), len(before))
	}
	for i := range before {
		if after[i].Hash != before[i].Hash || after[i].OpID != before[i].OpID {
			t.Fatalf("ledger entry %d changed across restart", i)
		}
	}
	if err := restored.VerifyChain(); err != nil {
		t.Fatalf("VerifyChain after restore: %v", err)
	}
	if err := bankWithdraw(t, restored, "w-snap", "alice", 1, "ore", 3, bankNow); !errors.Is(err, taskpkg.ErrDuplicateOp) {
		t.Fatalf("restart must remember used op ids, got %v", err)
	}
	items, cerr := restored.TabContents(1)
	if cerr != nil {
		t.Fatalf("TabContents: %v", cerr)
	}
	if len(items) != 1 || items[0].Count != 97 {
		t.Fatalf("stock after restore = %+v, want ore 97", items)
	}
}

func TestTabContentsAndLedgerAreCopies(t *testing.T) {
	g := newSeededBank(t)
	items, err := g.TabContents(1)
	if err != nil {
		t.Fatalf("TabContents: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("tab 1 stacks = %d, want 1", len(items))
	}
	items[0].Count = 999
	again, err := g.TabContents(1)
	if err != nil {
		t.Fatalf("TabContents: %v", err)
	}
	if len(again) != 1 || again[0].Count != 100 {
		t.Fatalf("caller mutation leaked into the tab: %+v", again)
	}
	entries := g.Ledger()
	if len(entries) == 0 {
		t.Fatalf("ledger must contain the seed deposits")
	}
	entries[0].Member = "mallory"
	if g.Ledger()[0].Member != "alice" {
		t.Fatalf("caller mutation leaked into the ledger")
	}
}
