package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-b-04"
)

func newEngine(t *testing.T) *taskpkg.CrossServerTransfer {
	t.Helper()
	tr := taskpkg.NewCrossServerTransfer(taskpkg.Config{TTL: time.Minute})
	if err := tr.RegisterServer("s1", 1_000_000); err != nil {
		t.Fatalf("RegisterServer s1: %v", err)
	}
	if err := tr.RegisterServer("s2", 1_000_000); err != nil {
		t.Fatalf("RegisterServer s2: %v", err)
	}
	return tr
}

func deposit(t *testing.T, tr *taskpkg.CrossServerTransfer, server, player string, gold int64, items map[string]int) {
	t.Helper()
	if err := tr.Deposit(server, player, gold, items); err != nil {
		t.Fatalf("Deposit(%s, %s): %v", server, player, err)
	}
}

func holdings(t *testing.T, tr *taskpkg.CrossServerTransfer, server, player string) taskpkg.Balance {
	t.Helper()
	bal, err := tr.Balance(server, player)
	if err != nil {
		t.Fatalf("Balance(%s, %s): %v", server, player, err)
	}
	return bal
}

func mustState(t *testing.T, tr *taskpkg.CrossServerTransfer, id string) taskpkg.TransferState {
	t.Helper()
	st, err := tr.State(id)
	if err != nil {
		t.Fatalf("State(%q): %v", id, err)
	}
	return st
}

// Unknown destinations and self transfers are rejected before any freeze.
func TestBeginRejectsUnknownAndSameServer(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s3", Gold: 100,
	}, base); !errors.Is(err, taskpkg.ErrUnknownServer) {
		t.Fatalf("unknown destination must yield ErrUnknownServer, got %v", err)
	}
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t2", PlayerID: "p1", FromServer: "s1", ToServer: "s1", Gold: 100,
	}, base); !errors.Is(err, taskpkg.ErrSameServer) {
		t.Fatalf("same server transfer must yield ErrSameServer, got %v", err)
	}
}

// A request above the player's balance is refused and freezes nothing.
func TestBeginRejectsInsufficientFunds(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 100, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 150,
	}, base); !errors.Is(err, taskpkg.ErrInsufficientFunds) {
		t.Fatalf("overdraft must yield ErrInsufficientFunds, got %v", err)
	}
	if got := holdings(t, tr, "s1", "p1").Gold; got != 100 {
		t.Fatalf("source gold after rejected begin = %d, want 100", got)
	}
	if _, err := tr.State("t1"); !errors.Is(err, taskpkg.ErrTransferNotFound) {
		t.Fatalf("rejected begin must not create a transfer, got %v", err)
	}
}

// Begin moves the goods into escrow on the source server.
func TestBeginFreezesSource(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, map[string]int{"sword": 2, "gem": 5})
	id, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2",
		Gold: 200, Items: map[string]int{"gem": 2},
	}, base)
	if err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if id != "t1" {
		t.Fatalf("Begin returned %q, want t1", id)
	}
	bal := holdings(t, tr, "s1", "p1")
	if bal.Gold != 300 {
		t.Fatalf("source gold after freeze = %d, want 300", bal.Gold)
	}
	if bal.Items["sword"] != 2 || bal.Items["gem"] != 3 {
		t.Fatalf("source items after freeze = %+v, want sword 2 gem 3", bal.Items)
	}
	if st := mustState(t, tr, "t1"); st != taskpkg.StateFrozen {
		t.Fatalf("state after begin = %q, want frozen", st)
	}
}

// Replaying the same begin is idempotent: same id, single deduction.
func TestBeginIdempotentSameRequest(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	req := taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 200,
	}
	first, err := tr.Begin(req, base)
	if err != nil {
		t.Fatalf("first Begin: %v", err)
	}
	second, err := tr.Begin(req, base)
	if err != nil {
		t.Fatalf("replayed Begin: %v", err)
	}
	if first != second {
		t.Fatalf("replayed Begin returned %q, want %q", second, first)
	}
	if got := holdings(t, tr, "s1", "p1").Gold; got != 300 {
		t.Fatalf("source gold after replay = %d, want 300", got)
	}
}

// Reusing an id with another payload is a conflict, not a replay.
func TestBeginConflictingDuplicate(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 100,
	}, base); err != nil {
		t.Fatalf("first Begin: %v", err)
	}
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 200,
	}, base); !errors.Is(err, taskpkg.ErrTransferConflict) {
		t.Fatalf("reused id with new payload must yield ErrTransferConflict, got %v", err)
	}
	if got := holdings(t, tr, "s1", "p1").Gold; got != 400 {
		t.Fatalf("source gold after conflict = %d, want 400", got)
	}
}

// One player may have only one open transfer.
func TestBeginRejectsSecondInFlight(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 100,
	}, base); err != nil {
		t.Fatalf("first Begin: %v", err)
	}
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t2", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 100,
	}, base); !errors.Is(err, taskpkg.ErrTransferInFlight) {
		t.Fatalf("second in flight transfer must yield ErrTransferInFlight, got %v", err)
	}
	if got := holdings(t, tr, "s1", "p1").Gold; got != 400 {
		t.Fatalf("source gold after rejected second begin = %d, want 400", got)
	}
}

// Approve only accepts a frozen transfer, once.
func TestApproveInvalidTransition(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if err := tr.Approve("missing", base); !errors.Is(err, taskpkg.ErrTransferNotFound) {
		t.Fatalf("unknown transfer must yield ErrTransferNotFound, got %v", err)
	}
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 100,
	}, base); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tr.Approve("t1", base); err != nil {
		t.Fatalf("first Approve: %v", err)
	}
	if st := mustState(t, tr, "t1"); st != taskpkg.StateReserved {
		t.Fatalf("state after approve = %q, want reserved", st)
	}
	if err := tr.Approve("t1", base); !errors.Is(err, taskpkg.ErrInvalidTransition) {
		t.Fatalf("double approve must yield ErrInvalidTransition, got %v", err)
	}
}

// A full destination blocks approve and leaves the transfer frozen.
func TestApproveRejectsDestinationFull(t *testing.T) {
	tr := taskpkg.NewCrossServerTransfer(taskpkg.Config{TTL: time.Minute})
	if err := tr.RegisterServer("s1", 1_000_000); err != nil {
		t.Fatalf("RegisterServer s1: %v", err)
	}
	if err := tr.RegisterServer("s2", 100); err != nil {
		t.Fatalf("RegisterServer s2: %v", err)
	}
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s2", "p9", 100, nil)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 50,
	}, base); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tr.Approve("t1", base); !errors.Is(err, taskpkg.ErrDestinationFull) {
		t.Fatalf("full destination must yield ErrDestinationFull, got %v", err)
	}
	if st := mustState(t, tr, "t1"); st != taskpkg.StateFrozen {
		t.Fatalf("state after rejected approve = %q, want frozen", st)
	}
	if err := tr.Rollback("t1", base); err != nil {
		t.Fatalf("Rollback after rejected approve: %v", err)
	}
	if got := holdings(t, tr, "s1", "p1").Gold; got != 500 {
		t.Fatalf("source gold after rollback = %d, want 500", got)
	}
}

// Commit credits the destination and keeps the source deduction.
func TestCommitCreditsDestinationOnce(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 200,
	}, base); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tr.Approve("t1", base); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := tr.Commit("t1", base); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if st := mustState(t, tr, "t1"); st != taskpkg.StateCommitted {
		t.Fatalf("state after commit = %q, want committed", st)
	}
	if got := holdings(t, tr, "s2", "p1").Gold; got != 200 {
		t.Fatalf("destination gold = %d, want 200", got)
	}
	if got := holdings(t, tr, "s1", "p1").Gold; got != 300 {
		t.Fatalf("source gold = %d, want 300", got)
	}
}

// Committing twice must not credit twice.
func TestCommitIdempotent(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 200,
	}, base); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tr.Approve("t1", base); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := tr.Commit("t1", base); err != nil {
		t.Fatalf("first Commit: %v", err)
	}
	if err := tr.Commit("t1", base); err != nil {
		t.Fatalf("replayed Commit: %v", err)
	}
	if got := holdings(t, tr, "s2", "p1").Gold; got != 200 {
		t.Fatalf("destination gold after replay = %d, want 200", got)
	}
}

// Rollback returns gold and items to the source player.
func TestRollbackRestoresSource(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, map[string]int{"gem": 5})
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2",
		Gold: 200, Items: map[string]int{"gem": 2},
	}, base); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tr.Approve("t1", base); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := tr.Rollback("t1", base); err != nil {
		t.Fatalf("Rollback: %v", err)
	}
	bal := holdings(t, tr, "s1", "p1")
	if bal.Gold != 500 || bal.Items["gem"] != 5 {
		t.Fatalf("source after rollback = %+v, want gold 500 gem 5", bal)
	}
	if st := mustState(t, tr, "t1"); st != taskpkg.StateRolledBack {
		t.Fatalf("state after rollback = %q, want rolled_back", st)
	}
}

// A committed transfer cannot be rolled back.
func TestRollbackAfterCommitRejected(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 200,
	}, base); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tr.Approve("t1", base); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if err := tr.Commit("t1", base); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if err := tr.Rollback("t1", base); !errors.Is(err, taskpkg.ErrInvalidTransition) {
		t.Fatalf("rollback after commit must yield ErrInvalidTransition, got %v", err)
	}
	if st := mustState(t, tr, "t1"); st != taskpkg.StateCommitted {
		t.Fatalf("state after rejected rollback = %q, want committed", st)
	}
}

// Transfers older than the TTL are rolled back in id order.
func TestExpireRollsBackStaleTransfers(t *testing.T) {
	tr := taskpkg.NewCrossServerTransfer(taskpkg.Config{TTL: 30 * time.Second})
	if err := tr.RegisterServer("s1", 1_000_000); err != nil {
		t.Fatalf("RegisterServer s1: %v", err)
	}
	if err := tr.RegisterServer("s2", 1_000_000); err != nil {
		t.Fatalf("RegisterServer s2: %v", err)
	}
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	deposit(t, tr, "s1", "p2", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 100,
	}, base); err != nil {
		t.Fatalf("Begin t1: %v", err)
	}
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t2", PlayerID: "p2", FromServer: "s1", ToServer: "s2", Gold: 100,
	}, base.Add(10*time.Second)); err != nil {
		t.Fatalf("Begin t2: %v", err)
	}
	expired, err := tr.Expire(base.Add(31 * time.Second))
	if err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if len(expired) != 1 || expired[0] != "t1" {
		t.Fatalf("expired = %v, want [t1]", expired)
	}
	if st := mustState(t, tr, "t1"); st != taskpkg.StateRolledBack {
		t.Fatalf("t1 state = %q, want rolled_back", st)
	}
	if st := mustState(t, tr, "t2"); st != taskpkg.StateFrozen {
		t.Fatalf("t2 state = %q, want frozen", st)
	}
	if got := holdings(t, tr, "s1", "p1").Gold; got != 500 {
		t.Fatalf("p1 gold after expire = %d, want 500", got)
	}
	expired, err = tr.Expire(base.Add(41 * time.Second))
	if err != nil {
		t.Fatalf("second Expire: %v", err)
	}
	if len(expired) != 1 || expired[0] != "t2" {
		t.Fatalf("second expired = %v, want [t2]", expired)
	}
}

// Open transfers survive a restart and still commit exactly once.
func TestSnapshotRestoreKeepsInFlightState(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 200,
	}, base); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	snap, err := tr.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap) == 0 {
		t.Fatal("Snapshot returned no bytes")
	}
	restored, err := taskpkg.NewCrossServerTransferFromSnapshot(taskpkg.Config{TTL: time.Minute}, snap)
	if err != nil {
		t.Fatalf("NewCrossServerTransferFromSnapshot: %v", err)
	}
	if st := mustState(t, restored, "t1"); st != taskpkg.StateFrozen {
		t.Fatalf("restored state = %q, want frozen", st)
	}
	if got := holdings(t, restored, "s1", "p1").Gold; got != 300 {
		t.Fatalf("restored source gold = %d, want 300", got)
	}
	if err := restored.Approve("t1", base); err != nil {
		t.Fatalf("Approve after restore: %v", err)
	}
	if err := restored.Commit("t1", base); err != nil {
		t.Fatalf("Commit after restore: %v", err)
	}
	if err := restored.Commit("t1", base); err != nil {
		t.Fatalf("replayed Commit after restore: %v", err)
	}
	if got := holdings(t, restored, "s2", "p1").Gold; got != 200 {
		t.Fatalf("destination gold after restore = %d, want 200", got)
	}
	if _, err := taskpkg.NewCrossServerTransferFromSnapshot(taskpkg.Config{TTL: time.Minute}, []byte("{broken")); err == nil {
		t.Fatal("corrupt snapshot was accepted")
	}
}

// Concurrent commits of one transfer must credit the destination once.
func TestConcurrentCommitAppliesOnce(t *testing.T) {
	tr := newEngine(t)
	base := time.Unix(1700000000, 0)
	deposit(t, tr, "s1", "p1", 500, nil)
	if _, err := tr.Begin(taskpkg.TransferRequest{
		TransferID: "t1", PlayerID: "p1", FromServer: "s1", ToServer: "s2", Gold: 200,
	}, base); err != nil {
		t.Fatalf("Begin: %v", err)
	}
	if err := tr.Approve("t1", base); err != nil {
		t.Fatalf("Approve: %v", err)
	}
	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			errs[idx] = tr.Commit("t1", base)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Commit %d: %v", i, err)
		}
	}
	if got := holdings(t, tr, "s2", "p1").Gold; got != 200 {
		t.Fatalf("destination gold after concurrent commits = %d, want 200", got)
	}
	if st := mustState(t, tr, "t1"); st != taskpkg.StateCommitted {
		t.Fatalf("state after concurrent commits = %q, want committed", st)
	}
}
