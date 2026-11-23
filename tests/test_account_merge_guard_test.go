package task

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-b-05"
)

func account(id, nickname string, gold int64, items map[string]int, level int) taskpkg.Account {
	return taskpkg.Account{ID: id, Nickname: nickname, Gold: gold, Items: items, Level: level}
}

func put(t *testing.T, m *taskpkg.AccountMerge, a taskpkg.Account) {
	t.Helper()
	if err := m.Put(a); err != nil {
		t.Fatalf("Put(%s): %v", a.ID, err)
	}
}

func planMerge(t *testing.T, m *taskpkg.AccountMerge, p taskpkg.MergePlan) taskpkg.MergePreview {
	t.Helper()
	preview, err := m.Plan(p, time.Unix(1700000000, 0))
	if err != nil {
		t.Fatalf("Plan(%s): %v", p.MergeID, err)
	}
	return preview
}

func getAccount(t *testing.T, m *taskpkg.AccountMerge, id string) taskpkg.Account {
	t.Helper()
	a, err := m.Get(id)
	if err != nil {
		t.Fatalf("Get(%s): %v", id, err)
	}
	return a
}

// Structurally broken accounts are refused at write time.
func TestPutRejectsInvalidAccount(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	if err := m.Put(account("", "alice", 0, nil, 1)); !errors.Is(err, taskpkg.ErrInvalidAccount) {
		t.Fatalf("empty id must yield ErrInvalidAccount, got %v", err)
	}
	if err := m.Put(account("u1", "alice", -1, nil, 1)); !errors.Is(err, taskpkg.ErrInvalidAccount) {
		t.Fatalf("negative gold must yield ErrInvalidAccount, got %v", err)
	}
	if err := m.Put(account("u2", "bob", 0, map[string]int{"gem": -2}, 1)); !errors.Is(err, taskpkg.ErrInvalidAccount) {
		t.Fatalf("negative item count must yield ErrInvalidAccount, got %v", err)
	}
}

// Same account, unknown account and unknown policy are all rejected.
func TestPlanRejectsInvalidRequests(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 300, nil, 10))
	put(t, m, account("u2", "bob", 200, nil, 7))
	if _, err := m.Plan(taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u1", NicknamePolicy: taskpkg.PolicyKeepTarget,
	}, time.Unix(1700000000, 0)); !errors.Is(err, taskpkg.ErrSameAccount) {
		t.Fatalf("same account must yield ErrSameAccount, got %v", err)
	}
	if _, err := m.Plan(taskpkg.MergePlan{
		MergeID: "m2", SourceID: "ghost", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	}, time.Unix(1700000000, 0)); !errors.Is(err, taskpkg.ErrAccountNotFound) {
		t.Fatalf("unknown source must yield ErrAccountNotFound, got %v", err)
	}
	if _, err := m.Plan(taskpkg.MergePlan{
		MergeID: "m3", SourceID: "u1", TargetID: "u2", NicknamePolicy: "coin-flip",
	}, time.Unix(1700000000, 0)); !errors.Is(err, taskpkg.ErrInvalidPolicy) {
		t.Fatalf("unknown nickname policy must yield ErrInvalidPolicy, got %v", err)
	}
}

// Planning sums gold and items for the preview without touching accounts.
func TestPlanPreviewIsNonMutating(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 300, map[string]int{"potion": 2}, 10))
	put(t, m, account("u2", "bob", 200, map[string]int{"potion": 1, "gem": 1}, 7))
	preview := planMerge(t, m, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	})
	if preview.Target.ID != "u2" || preview.Target.Gold != 500 || preview.Target.Level != 10 {
		t.Fatalf("preview target = %+v, want u2 gold 500 level 10", preview.Target)
	}
	if preview.GoldMoved != 300 {
		t.Fatalf("preview gold moved = %d, want 300", preview.GoldMoved)
	}
	if preview.Target.Items["potion"] != 3 || preview.Target.Items["gem"] != 1 {
		t.Fatalf("preview items = %+v, want potion 3 gem 1", preview.Target.Items)
	}
	if preview.ItemsMerged["potion"] != 3 || preview.ItemsMerged["gem"] != 1 {
		t.Fatalf("preview ItemsMerged = %+v, want potion 3 gem 1", preview.ItemsMerged)
	}
	src := getAccount(t, m, "u1")
	if src.Gold != 300 || src.Level != 10 || src.Items["potion"] != 2 {
		t.Fatalf("source mutated by Plan: %+v", src)
	}
	dst := getAccount(t, m, "u2")
	if dst.Gold != 200 || dst.Items["potion"] != 1 || dst.Items["gem"] != 1 {
		t.Fatalf("target mutated by Plan: %+v", dst)
	}
	pending := m.Pending()
	if len(pending) != 1 || pending[0].MergeID != "m1" {
		t.Fatalf("Pending after Plan = %+v, want the new plan", pending)
	}
}

// keep_target and keep_source pick the obvious nickname when it is free.
func TestPlanNicknamePolicies(t *testing.T) {
	keepTarget := taskpkg.NewAccountMerge()
	put(t, keepTarget, account("u1", "bob", 100, nil, 1))
	put(t, keepTarget, account("u2", "alice", 100, nil, 1))
	preview := planMerge(t, keepTarget, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	})
	if preview.Nickname != "alice" || preview.Target.Nickname != "alice" {
		t.Fatalf("keep_target nickname = %q / %q, want alice", preview.Nickname, preview.Target.Nickname)
	}

	keepSource := taskpkg.NewAccountMerge()
	put(t, keepSource, account("u1", "bob", 100, nil, 1))
	put(t, keepSource, account("u2", "alice", 100, nil, 1))
	preview = planMerge(t, keepSource, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepSource,
	})
	if preview.Nickname != "bob" || preview.Target.Nickname != "bob" {
		t.Fatalf("keep_source nickname = %q / %q, want bob", preview.Nickname, preview.Target.Nickname)
	}
}

// The suffix policy walks up from <target>-2 until it finds a free name.
func TestPlanSuffixAvoidsCollisions(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "bob", 100, nil, 1))
	put(t, m, account("u2", "alice", 100, nil, 1))
	put(t, m, account("u3", "alice-2", 100, nil, 1))
	preview := planMerge(t, m, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicySuffix,
	})
	if preview.Nickname != "alice-3" {
		t.Fatalf("suffix nickname = %q, want alice-3", preview.Nickname)
	}
}

// keep_source fails loudly when a third account already owns the nickname.
func TestPlanRejectsTakenNickname(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "bob", 100, nil, 1))
	put(t, m, account("u2", "alice", 100, nil, 1))
	put(t, m, account("u3", "bob", 100, nil, 1))
	if _, err := m.Plan(taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepSource,
	}, time.Unix(1700000000, 0)); !errors.Is(err, taskpkg.ErrNicknameTaken) {
		t.Fatalf("taken nickname must yield ErrNicknameTaken, got %v", err)
	}
}

// A merged balance above MaxGold is refused at plan time.
func TestPlanRejectsGoldOverflow(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 200_000_000, nil, 1))
	put(t, m, account("u2", "bob", 900_000_000, nil, 1))
	if _, err := m.Plan(taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	}, time.Unix(1700000000, 0)); !errors.Is(err, taskpkg.ErrGoldOverflow) {
		t.Fatalf("gold overflow must yield ErrGoldOverflow, got %v", err)
	}
}

// A merged item stack above MaxStack is refused at plan time.
func TestPlanRejectsStackOverflow(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 100, map[string]int{"potion": 400}, 1))
	put(t, m, account("u2", "bob", 100, map[string]int{"potion": 600}, 1))
	if _, err := m.Plan(taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	}, time.Unix(1700000000, 0)); !errors.Is(err, taskpkg.ErrStackOverflow) {
		t.Fatalf("stack overflow must yield ErrStackOverflow, got %v", err)
	}
}

// Apply folds the source into the target and retires the source id.
func TestApplyMergesAccountsAndRetiresSource(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "bob", 300, map[string]int{"potion": 2}, 10))
	put(t, m, account("u2", "alice", 200, map[string]int{"potion": 1, "gem": 1}, 7))
	planMerge(t, m, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	})
	merged, err := m.Apply("m1", time.Unix(1700000100, 0))
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if merged.ID != "u2" || merged.Nickname != "alice" || merged.Gold != 500 || merged.Level != 10 {
		t.Fatalf("merged account = %+v", merged)
	}
	if merged.Items["potion"] != 3 || merged.Items["gem"] != 1 {
		t.Fatalf("merged items = %+v, want potion 3 gem 1", merged.Items)
	}
	if _, err := m.Get("u1"); !errors.Is(err, taskpkg.ErrAccountMerged) {
		t.Fatalf("retired source must yield ErrAccountMerged, got %v", err)
	}
	history := m.History()
	if len(history) != 1 {
		t.Fatalf("history length = %d, want 1", len(history))
	}
	if history[0].MergeID != "m1" || history[0].SourceID != "u1" || history[0].TargetID != "u2" || history[0].GoldMoved != 300 {
		t.Fatalf("history record = %+v", history[0])
	}
	if pending := m.Pending(); len(pending) != 0 {
		t.Fatalf("pending after apply = %+v, want empty", pending)
	}
	if _, err := m.Plan(taskpkg.MergePlan{
		MergeID: "m2", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	}, time.Unix(1700000200, 0)); !errors.Is(err, taskpkg.ErrAccountMerged) {
		t.Fatalf("planning with a retired source must yield ErrAccountMerged, got %v", err)
	}
}

// Applying the same merge twice returns the same account and never doubles it.
func TestApplyIsIdempotent(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 300, map[string]int{"potion": 2}, 10))
	put(t, m, account("u2", "bob", 200, map[string]int{"gem": 1}, 7))
	planMerge(t, m, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	})
	first, err := m.Apply("m1", time.Unix(1700000100, 0))
	if err != nil {
		t.Fatalf("first Apply: %v", err)
	}
	second, err := m.Apply("m1", time.Unix(1700000200, 0))
	if err != nil {
		t.Fatalf("replayed Apply: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("replayed Apply returned %+v, want %+v", second, first)
	}
	if second.Gold != 500 {
		t.Fatalf("gold after replay = %d, want 500", second.Gold)
	}
	if got := getAccount(t, m, "u2"); got.Gold != 500 || got.Items["potion"] != 2 || got.Items["gem"] != 1 {
		t.Fatalf("stored target after replay = %+v", got)
	}
	if history := m.History(); len(history) != 1 {
		t.Fatalf("history length after replay = %d, want 1", len(history))
	}
}

// Applying an unknown merge id fails.
func TestApplyUnknownPlanFails(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 300, nil, 10))
	put(t, m, account("u2", "bob", 200, nil, 7))
	if _, err := m.Apply("ghost", time.Unix(1700000100, 0)); !errors.Is(err, taskpkg.ErrPlanNotFound) {
		t.Fatalf("unknown plan must yield ErrPlanNotFound, got %v", err)
	}
}

// Cancelling a plan keeps both accounts untouched and blocks Apply.
func TestCancelPreventsApply(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 300, nil, 10))
	put(t, m, account("u2", "bob", 200, nil, 7))
	planMerge(t, m, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	})
	if err := m.Cancel("m1"); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if pending := m.Pending(); len(pending) != 0 {
		t.Fatalf("pending after cancel = %+v, want empty", pending)
	}
	if _, err := m.Apply("m1", time.Unix(1700000100, 0)); !errors.Is(err, taskpkg.ErrPlanCancelled) {
		t.Fatalf("apply after cancel must yield ErrPlanCancelled, got %v", err)
	}
	if err := m.Cancel("m1"); !errors.Is(err, taskpkg.ErrPlanCancelled) {
		t.Fatalf("double cancel must yield ErrPlanCancelled, got %v", err)
	}
	if got := getAccount(t, m, "u1"); got.Gold != 300 {
		t.Fatalf("source after cancel = %+v", got)
	}
	if got := getAccount(t, m, "u2"); got.Gold != 200 {
		t.Fatalf("target after cancel = %+v", got)
	}
}

// A committed merge cannot be cancelled.
func TestCancelAfterApplyRejected(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 300, nil, 10))
	put(t, m, account("u2", "bob", 200, nil, 7))
	planMerge(t, m, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	})
	if _, err := m.Apply("m1", time.Unix(1700000100, 0)); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if err := m.Cancel("m1"); !errors.Is(err, taskpkg.ErrMergeCommitted) {
		t.Fatalf("cancel after apply must yield ErrMergeCommitted, got %v", err)
	}
}

// A restart keeps pending plans, history and the idempotency of old applies.
func TestSnapshotRestoreKeepsPlansAndHistory(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 300, map[string]int{"potion": 2}, 10))
	put(t, m, account("u2", "bob", 200, map[string]int{"gem": 1}, 7))
	planMerge(t, m, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	})
	snap, err := m.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	if len(snap) == 0 {
		t.Fatal("Snapshot returned no bytes")
	}
	restored, err := taskpkg.NewAccountMergeFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewAccountMergeFromSnapshot: %v", err)
	}
	pending := restored.Pending()
	if len(pending) != 1 || pending[0].MergeID != "m1" || pending[0].SourceID != "u1" {
		t.Fatalf("restored pending = %+v", pending)
	}
	merged, err := restored.Apply("m1", time.Unix(1700000100, 0))
	if err != nil {
		t.Fatalf("Apply after restore: %v", err)
	}
	if merged.Gold != 500 || merged.Level != 10 || merged.Items["potion"] != 2 || merged.Items["gem"] != 1 {
		t.Fatalf("merged after restore = %+v", merged)
	}
	if history := restored.History(); len(history) != 1 || history[0].GoldMoved != 300 {
		t.Fatalf("history after restore = %+v", history)
	}
	again, err := restored.Apply("m1", time.Unix(1700000200, 0))
	if err != nil {
		t.Fatalf("replayed Apply after restore: %v", err)
	}
	if again.Gold != 500 {
		t.Fatalf("gold after replayed Apply = %d, want 500", again.Gold)
	}
	if _, err := taskpkg.NewAccountMergeFromSnapshot([]byte("{broken")); err == nil {
		t.Fatal("corrupt snapshot was accepted")
	}
}

// Concurrent applies of one plan merge exactly once.
func TestConcurrentApplyCreditsOnce(t *testing.T) {
	m := taskpkg.NewAccountMerge()
	put(t, m, account("u1", "alice", 300, nil, 10))
	put(t, m, account("u2", "bob", 200, nil, 7))
	planMerge(t, m, taskpkg.MergePlan{
		MergeID: "m1", SourceID: "u1", TargetID: "u2", NicknamePolicy: taskpkg.PolicyKeepTarget,
	})
	const workers = 8
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = m.Apply("m1", time.Unix(1700000100, 0))
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent Apply %d: %v", i, err)
		}
	}
	if got := getAccount(t, m, "u2"); got.Gold != 500 {
		t.Fatalf("target gold after concurrent applies = %d, want 500", got.Gold)
	}
	if history := m.History(); len(history) != 1 {
		t.Fatalf("history after concurrent applies has %d records, want 1", len(history))
	}
}
