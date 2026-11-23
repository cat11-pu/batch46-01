package task

import (
	"errors"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-f-04"
)

func testPortalConfig() taskpkg.PortalConfig {
	return taskpkg.PortalConfig{MaxRecursion: 3, MaxDrawCalls: 10}
}

func newPortalGuard(t *testing.T) *taskpkg.StencilPortal {
	t.Helper()
	g, err := taskpkg.NewStencilPortal(testPortalConfig())
	if err != nil {
		t.Fatalf("NewStencilPortal: %v", err)
	}
	return g
}

func mustAddRoom(t *testing.T, g *taskpkg.StencilPortal, id string) {
	t.Helper()
	if err := g.AddRoom(id); err != nil {
		t.Fatalf("AddRoom(%q): %v", id, err)
	}
}

func mustAddPortal(t *testing.T, g *taskpkg.StencilPortal, p taskpkg.Portal) {
	t.Helper()
	if err := g.AddPortal(p); err != nil {
		t.Fatalf("AddPortal(%q): %v", p.ID, err)
	}
}

func mustSetCamera(t *testing.T, g *taskpkg.StencilPortal, room string) {
	t.Helper()
	if err := g.SetCamera(room); err != nil {
		t.Fatalf("SetCamera(%q): %v", room, err)
	}
}

func visibleIDs(t *testing.T, g *taskpkg.StencilPortal) []string {
	t.Helper()
	ids, err := g.VisiblePortals()
	if err != nil {
		t.Fatalf("VisiblePortals: %v", err)
	}
	return ids
}

func mustSubmitPortals(t *testing.T, g *taskpkg.StencilPortal, ids ...string) taskpkg.SubmitResult {
	t.Helper()
	res, err := g.Submit(ids)
	if err != nil {
		t.Fatalf("Submit(%v): %v", ids, err)
	}
	return res
}

// Room ids are unique and non-empty.
func TestRoomRegistrationValidation(t *testing.T) {
	g := newPortalGuard(t)
	mustAddRoom(t, g, "atrium")
	if err := g.AddRoom("atrium"); !errors.Is(err, taskpkg.ErrRoomExists) {
		t.Fatalf("duplicate room must yield ErrRoomExists, got %v", err)
	}
	if err := g.AddRoom(""); !errors.Is(err, taskpkg.ErrInvalidRoom) {
		t.Fatalf("empty room must yield ErrInvalidRoom, got %v", err)
	}
	if stats := g.Stats(); stats.Rooms != 1 {
		t.Fatalf("want one room, got %+v", stats)
	}
}

// Portals must connect two distinct registered rooms and have unique ids.
func TestPortalStructuralValidation(t *testing.T) {
	g := newPortalGuard(t)
	mustAddRoom(t, g, "hall")
	mustAddRoom(t, g, "vault")
	mustAddPortal(t, g, taskpkg.Portal{ID: "gate", FromRoom: "hall", ToRoom: "vault"})
	if err := g.AddPortal(taskpkg.Portal{ID: "loop", FromRoom: "hall", ToRoom: "hall"}); !errors.Is(err, taskpkg.ErrSameRoom) {
		t.Fatalf("portal into the same room must yield ErrSameRoom, got %v", err)
	}
	if err := g.AddPortal(taskpkg.Portal{ID: "void", FromRoom: "hall", ToRoom: "cellar"}); !errors.Is(err, taskpkg.ErrRoomUnknown) {
		t.Fatalf("portal to an unknown room must yield ErrRoomUnknown, got %v", err)
	}
	if err := g.AddPortal(taskpkg.Portal{ID: "", FromRoom: "hall", ToRoom: "vault"}); !errors.Is(err, taskpkg.ErrInvalidPortal) {
		t.Fatalf("empty portal id must yield ErrInvalidPortal, got %v", err)
	}
	if err := g.AddPortal(taskpkg.Portal{ID: "gate", FromRoom: "vault", ToRoom: "hall"}); !errors.Is(err, taskpkg.ErrPortalExists) {
		t.Fatalf("duplicate portal must yield ErrPortalExists, got %v", err)
	}
	if stats := g.Stats(); stats.Portals != 1 {
		t.Fatalf("want one portal, got %+v", stats)
	}
}

// Visibility is breadth first and sorted by id inside a depth level.
func TestVisiblePortalsBreadthFirstOrder(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "A", ToRoom: "C"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p3", FromRoom: "B", ToRoom: "D"})
	mustSetCamera(t, g, "A")
	got := visibleIDs(t, g)
	want := []string{"p1", "p2", "p3"}
	if len(got) != len(want) {
		t.Fatalf("visible portals = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("visible portals = %v, want %v", got, want)
		}
	}
	if stats := g.Stats(); stats.Visible != 3 {
		t.Fatalf("visible counter = %d, want 3", stats.Visible)
	}
}

// A one-way portal only works from its front room; a two-sided one works both
// ways.
func TestOneWayAndTwoSidedTraversal(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "one-way", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "both", FromRoom: "C", ToRoom: "D", TwoSided: true})
	mustSetCamera(t, g, "B")
	if got := visibleIDs(t, g); len(got) != 0 {
		t.Fatalf("back of a one-way portal must not be traversable, got %v", got)
	}
	mustSetCamera(t, g, "D")
	got := visibleIDs(t, g)
	if len(got) != 1 || got[0] != "both" {
		t.Fatalf("two-sided portal must be traversable backwards, got %v", got)
	}
	mustSetCamera(t, g, "A")
	got = visibleIDs(t, g)
	if len(got) != 1 || got[0] != "one-way" {
		t.Fatalf("one-way portal must be traversable forwards, got %v", got)
	}
}

// Switching the camera recomputes visibility and changing to an unknown room
// fails without touching the previous camera.
func TestCameraSwitchUpdatesVisibility(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "C", ToRoom: "D"})
	mustSetCamera(t, g, "A")
	if got := visibleIDs(t, g); len(got) != 1 || got[0] != "p1" {
		t.Fatalf("camera A must see [p1], got %v", got)
	}
	if err := g.SetCamera("A"); err != nil {
		t.Fatalf("setting the same camera again must be idempotent, got %v", err)
	}
	if err := g.SetCamera("cellar"); !errors.Is(err, taskpkg.ErrRoomUnknown) {
		t.Fatalf("unknown camera room must yield ErrRoomUnknown, got %v", err)
	}
	mustSetCamera(t, g, "C")
	if got := visibleIDs(t, g); len(got) != 1 || got[0] != "p2" {
		t.Fatalf("camera C must see [p2], got %v", got)
	}
	if stats := g.Stats(); stats.Visible != 1 {
		t.Fatalf("visible counter = %d, want 1", stats.Visible)
	}
}

// A cycle must terminate and unreachable rooms must stay out.
func TestPortalCycleTerminatesAndUnreachableExcluded(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "B", ToRoom: "A"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p3", FromRoom: "C", ToRoom: "D"})
	mustSetCamera(t, g, "A")
	got := visibleIDs(t, g)
	if len(got) != 2 || got[0] != "p1" || got[1] != "p2" {
		t.Fatalf("cyclic graph must yield [p1 p2], got %v", got)
	}
	for _, id := range got {
		if id == "p3" {
			t.Fatalf("unreachable portal leaked into the visible set")
		}
	}
}

// The plan assigns depth-based stencil references and draw cost.
func TestRenderPlanStencilRefsByDepth(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "A", ToRoom: "C"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p3", FromRoom: "B", ToRoom: "D"})
	mustSetCamera(t, g, "A")
	plan, err := g.RenderPlan()
	if err != nil {
		t.Fatalf("RenderPlan: %v", err)
	}
	if plan.Truncated {
		t.Fatalf("plan with depth 1 must not be truncated")
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("plan has %d steps, want 3", len(plan.Steps))
	}
	want := []taskpkg.PlannedPortal{
		{PortalID: "p1", Depth: 0, StencilRef: 1, DrawCost: 1},
		{PortalID: "p2", Depth: 0, StencilRef: 1, DrawCost: 1},
		{PortalID: "p3", Depth: 1, StencilRef: 2, DrawCost: 2},
	}
	for i := range want {
		if plan.Steps[i] != want[i] {
			t.Fatalf("step %d = %+v, want %+v", i, plan.Steps[i], want[i])
		}
	}
}

// Portals at or beyond the recursion limit are dropped and unsubmittable.
func TestRecursionLimitTruncatesPlan(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "B", ToRoom: "C"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p3", FromRoom: "C", ToRoom: "D"})
	mustSetCamera(t, g, "A")
	if err := g.SetRecursionLimit(2); err != nil {
		t.Fatalf("SetRecursionLimit(2): %v", err)
	}
	plan, err := g.RenderPlan()
	if err != nil {
		t.Fatalf("RenderPlan: %v", err)
	}
	if !plan.Truncated {
		t.Fatalf("plan must be marked truncated at limit 2")
	}
	if len(plan.Steps) != 2 || plan.Steps[0].PortalID != "p1" || plan.Steps[1].PortalID != "p2" {
		t.Fatalf("limit 2 must keep p1 and p2, got %+v", plan.Steps)
	}
	if _, err := g.Submit([]string{"p3"}); !errors.Is(err, taskpkg.ErrPortalNotVisible) {
		t.Fatalf("truncated portal must yield ErrPortalNotVisible, got %v", err)
	}
	for _, bad := range []int{0, -2} {
		if err := g.SetRecursionLimit(bad); !errors.Is(err, taskpkg.ErrInvalidRecursion) {
			t.Fatalf("limit %d must yield ErrInvalidRecursion, got %v", bad, err)
		}
	}
}

// A plan that costs more than the frame budget admits nothing.
func TestSubmitBudgetExceededAdmitsNothing(t *testing.T) {
	g, err := taskpkg.NewStencilPortal(taskpkg.PortalConfig{MaxRecursion: 3, MaxDrawCalls: 2})
	if err != nil {
		t.Fatalf("NewStencilPortal: %v", err)
	}
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "A", ToRoom: "C"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p3", FromRoom: "B", ToRoom: "D"})
	mustSetCamera(t, g, "A")
	if _, err := g.Submit([]string{"p1", "p2", "p3"}); !errors.Is(err, taskpkg.ErrDrawBudget) {
		t.Fatalf("cost 4 over a budget of 2 must yield ErrDrawBudget, got %v", err)
	}
	if stats := g.Stats(); stats.Submitted != 0 {
		t.Fatalf("rejected submission must admit nothing, got %+v", stats)
	}
}

// Portal submission must follow the plan order.
func TestSubmitOutOfOrderRejected(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "A", ToRoom: "C"})
	mustSetCamera(t, g, "A")
	if _, err := g.Submit([]string{"p2", "p1"}); !errors.Is(err, taskpkg.ErrOrderViolation) {
		t.Fatalf("reverse order must yield ErrOrderViolation, got %v", err)
	}
	if stats := g.Stats(); stats.Submitted != 0 {
		t.Fatalf("rejected submission must admit nothing, got %+v", stats)
	}
}

// Portals outside the current plan cannot be submitted.
func TestSubmitInvisiblePortalRejected(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p9", FromRoom: "C", ToRoom: "D"})
	mustSetCamera(t, g, "A")
	if _, err := g.Submit([]string{"p9"}); !errors.Is(err, taskpkg.ErrPortalNotVisible) {
		t.Fatalf("invisible portal must yield ErrPortalNotVisible, got %v", err)
	}
}

// Duplicate ids and re-submissions are rejected.
func TestSubmitDuplicatePortalRejected(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "A", ToRoom: "C"})
	mustSetCamera(t, g, "A")
	if _, err := g.Submit([]string{"p1", "p1"}); !errors.Is(err, taskpkg.ErrDuplicatePortal) {
		t.Fatalf("duplicate portal in one submission must yield ErrDuplicatePortal, got %v", err)
	}
	mustSubmitPortals(t, g, "p1")
	if _, err := g.Submit([]string{"p1"}); !errors.Is(err, taskpkg.ErrAlreadySubmitted) {
		t.Fatalf("re-submission must yield ErrAlreadySubmitted, got %v", err)
	}
	if stats := g.Stats(); stats.Submitted != 1 {
		t.Fatalf("want one submitted portal, got %+v", stats)
	}
}

// ResetFrame clears the submission set but keeps the graph and camera.
func TestResetFrameAllowsResubmit(t *testing.T) {
	g := newPortalGuard(t)
	mustAddRoom(t, g, "A")
	mustAddRoom(t, g, "B")
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustSetCamera(t, g, "A")
	res := mustSubmitPortals(t, g, "p1")
	if res.Portals != 1 || res.DrawCost != 1 {
		t.Fatalf("unexpected submit result: %+v", res)
	}
	if err := g.ResetFrame(); err != nil {
		t.Fatalf("ResetFrame: %v", err)
	}
	if stats := g.Stats(); stats.Submitted != 0 {
		t.Fatalf("ResetFrame must clear submissions, got %+v", stats)
	}
	if got := visibleIDs(t, g); len(got) != 1 || got[0] != "p1" {
		t.Fatalf("ResetFrame must keep the camera, visible = %v", got)
	}
	mustSubmitPortals(t, g, "p1")
	if stats := g.Stats(); stats.Submitted != 1 {
		t.Fatalf("portal must be submittable after ResetFrame, got %+v", stats)
	}
}

// Graph, camera and frame progress must survive a restart.
func TestSnapshotRestorePreservesFrameState(t *testing.T) {
	g := newPortalGuard(t)
	for _, room := range []string{"A", "B", "C", "D"} {
		mustAddRoom(t, g, room)
	}
	mustAddPortal(t, g, taskpkg.Portal{ID: "p1", FromRoom: "A", ToRoom: "B"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p2", FromRoom: "A", ToRoom: "C"})
	mustAddPortal(t, g, taskpkg.Portal{ID: "p3", FromRoom: "B", ToRoom: "D"})
	mustSetCamera(t, g, "A")
	mustSubmitPortals(t, g, "p1")
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewStencilPortalFromSnapshot(testPortalConfig(), snap)
	if err != nil {
		t.Fatalf("NewStencilPortalFromSnapshot: %v", err)
	}
	got := visibleIDs(t, restored)
	want := []string{"p1", "p2", "p3"}
	if len(got) != len(want) {
		t.Fatalf("restored visible portals = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("restored visible portals = %v, want %v", got, want)
		}
	}
	if stats := restored.Stats(); stats.Submitted != 1 {
		t.Fatalf("restored submission count = %d, want 1", stats.Submitted)
	}
	if _, err := restored.Submit([]string{"p1"}); !errors.Is(err, taskpkg.ErrAlreadySubmitted) {
		t.Fatalf("restored frame must reject p1 again, got %v", err)
	}
	if res := mustSubmitPortals(t, restored, "p3"); res.DrawCost != 2 {
		t.Fatalf("p3 draw cost = %d, want 2", res.DrawCost)
	}
	if _, err := taskpkg.NewStencilPortalFromSnapshot(testPortalConfig(), []byte("nope")); !errors.Is(err, taskpkg.ErrCorruptSnapshot) {
		t.Fatalf("corrupt snapshot must yield ErrCorruptSnapshot, got %v", err)
	}
}

// The draw cost budget must hold under concurrent submissions.
func TestConcurrentSubmitBudgetRespected(t *testing.T) {
	g, err := taskpkg.NewStencilPortal(taskpkg.PortalConfig{MaxRecursion: 3, MaxDrawCalls: 2})
	if err != nil {
		t.Fatalf("NewStencilPortal: %v", err)
	}
	mustAddRoom(t, g, "A")
	const workers = 8
	for i := 0; i < workers; i++ {
		room := "B" + string(rune('0'+i))
		mustAddRoom(t, g, room)
		mustAddPortal(t, g, taskpkg.Portal{ID: "p" + string(rune('0'+i)), FromRoom: "A", ToRoom: room})
	}
	mustSetCamera(t, g, "A")
	var wg sync.WaitGroup
	errs := make([]error, workers)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(idx int) {
			defer wg.Done()
			_, errs[idx] = g.Submit([]string{"p" + string(rune('0'+idx))})
		}(i)
	}
	wg.Wait()
	accepted := 0
	rejected := 0
	for i := 0; i < workers; i++ {
		switch {
		case errs[i] == nil:
			accepted++
		case errors.Is(errs[i], taskpkg.ErrDrawBudget):
			rejected++
		default:
			t.Fatalf("unexpected error from worker %d: %v", i, errs[i])
		}
	}
	if accepted != 2 || rejected != 6 {
		t.Fatalf("want exactly 2 accepted and 6 rejected portals, got %d/%d", accepted, rejected)
	}
	if stats := g.Stats(); stats.Submitted != 2 {
		t.Fatalf("frame submission count = %d, want 2", stats.Submitted)
	}
}
