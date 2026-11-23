package task

import (
	"errors"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-b-07"
)

var draftStart = time.Date(2026, 4, 10, 20, 0, 0, 0, time.UTC)

func draftChampions() []taskpkg.Champion {
	return []taskpkg.Champion{
		{ID: "vi", Role: "fighter"},
		{ID: "ashe", Role: "marksman"},
		{ID: "lux", Role: "mage"},
		{ID: "ahri", Role: "mage"},
		{ID: "aatrox", Role: "fighter"},
		{ID: "braum", Role: "support"},
		{ID: "darius", Role: "fighter"},
		{ID: "ezreal", Role: "marksman"},
		{ID: "garen", Role: "fighter"},
		{ID: "janna", Role: "support"},
		{ID: "leona", Role: "support"},
		{ID: "malphite", Role: "tank"},
		{ID: "orianna", Role: "mage"},
		{ID: "sivir", Role: "marksman"},
		{ID: "thresh", Role: "support"},
	}
}

func newDraft(t *testing.T) *taskpkg.PvpDraft {
	t.Helper()
	d, err := taskpkg.NewPvpDraft(taskpkg.Config{
		BansPerTeam:  2,
		PicksPerTeam: 3,
		TurnTimeout:  30 * time.Second,
	}, draftChampions(), draftStart)
	if err != nil {
		t.Fatalf("NewPvpDraft: %v", err)
	}
	if d == nil {
		t.Fatalf("NewPvpDraft returned nil")
	}
	return d
}

func draftSubmit(t *testing.T, d *taskpkg.PvpDraft, id string, team taskpkg.Team, kind taskpkg.ActionKind, champ string, at time.Time) {
	t.Helper()
	if _, err := d.Submit(taskpkg.Action{ID: id, Team: team, Kind: kind, ChampionID: champ, At: at}, at); err != nil {
		t.Fatalf("Submit(%s/%s/%s): %v", id, kind, champ, err)
	}
}

// draftBans walks the four ban turns and returns the time of the last ban.
func draftBans(t *testing.T, d *taskpkg.PvpDraft) time.Time {
	t.Helper()
	base := draftStart
	draftSubmit(t, d, "b1", taskpkg.TeamBlue, taskpkg.KindBan, "ahri", base.Add(1*time.Second))
	draftSubmit(t, d, "b2", taskpkg.TeamRed, taskpkg.KindBan, "lux", base.Add(2*time.Second))
	draftSubmit(t, d, "b3", taskpkg.TeamBlue, taskpkg.KindBan, "ashe", base.Add(3*time.Second))
	draftSubmit(t, d, "b4", taskpkg.TeamRed, taskpkg.KindBan, "janna", base.Add(4*time.Second))
	return base.Add(4 * time.Second)
}

func TestBanPhaseRejectsPick(t *testing.T) {
	d := newDraft(t)
	_, err := d.Submit(taskpkg.Action{ID: "x1", Team: taskpkg.TeamBlue, Kind: taskpkg.KindPick, ChampionID: "aatrox", At: draftStart.Add(time.Second)}, draftStart.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrInvalidPhase) {
		t.Fatalf("pick during the ban phase must yield ErrInvalidPhase, got %v", err)
	}
	st := d.State()
	if st.Turn != 1 || st.Phase != taskpkg.PhaseBan || st.Team != taskpkg.TeamBlue {
		t.Fatalf("rejected pick advanced the draft: %+v", st)
	}
}

func TestDuplicateBanRejected(t *testing.T) {
	d := newDraft(t)
	draftSubmit(t, d, "b1", taskpkg.TeamBlue, taskpkg.KindBan, "ahri", draftStart.Add(time.Second))
	_, err := d.Submit(taskpkg.Action{ID: "b2", Team: taskpkg.TeamRed, Kind: taskpkg.KindBan, ChampionID: "ahri", At: draftStart.Add(2 * time.Second)}, draftStart.Add(2*time.Second))
	if !errors.Is(err, taskpkg.ErrAlreadyBanned) {
		t.Fatalf("re-banning ahri must yield ErrAlreadyBanned, got %v", err)
	}
}

func TestPickOfBannedChampionRejected(t *testing.T) {
	d := newDraft(t)
	draftBans(t, d)
	at := draftStart.Add(5 * time.Second)
	_, err := d.Submit(taskpkg.Action{ID: "p1", Team: taskpkg.TeamBlue, Kind: taskpkg.KindPick, ChampionID: "lux", At: at}, at)
	if !errors.Is(err, taskpkg.ErrChampionTaken) {
		t.Fatalf("picking a banned champion must yield ErrChampionTaken, got %v", err)
	}
}

func TestUnknownChampionRejected(t *testing.T) {
	d := newDraft(t)
	_, err := d.Submit(taskpkg.Action{ID: "b1", Team: taskpkg.TeamBlue, Kind: taskpkg.KindBan, ChampionID: "yuumi", At: draftStart.Add(time.Second)}, draftStart.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrUnknownChampion) {
		t.Fatalf("unknown champion must yield ErrUnknownChampion, got %v", err)
	}
}

func TestTurnOrderAlternates(t *testing.T) {
	d := newDraft(t)
	_, err := d.Submit(taskpkg.Action{ID: "b1", Team: taskpkg.TeamRed, Kind: taskpkg.KindBan, ChampionID: "ahri", At: draftStart.Add(time.Second)}, draftStart.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrNotYourTurn) {
		t.Fatalf("red acting on the blue turn must yield ErrNotYourTurn, got %v", err)
	}
	st := d.State()
	if st.Turn != 1 || st.Team != taskpkg.TeamBlue || len(st.Bans) != 0 {
		t.Fatalf("out-of-turn action changed the draft: %+v", st)
	}
}

func TestDuplicateActionIDRejected(t *testing.T) {
	d := newDraft(t)
	draftSubmit(t, d, "op-1", taskpkg.TeamBlue, taskpkg.KindBan, "ahri", draftStart.Add(time.Second))
	_, err := d.Submit(taskpkg.Action{ID: "op-1", Team: taskpkg.TeamRed, Kind: taskpkg.KindBan, ChampionID: "lux", At: draftStart.Add(2 * time.Second)}, draftStart.Add(2*time.Second))
	if !errors.Is(err, taskpkg.ErrDuplicateAction) {
		t.Fatalf("reused action id must yield ErrDuplicateAction, got %v", err)
	}
	st := d.State()
	if len(st.Bans) != 1 || st.Turn != 2 {
		t.Fatalf("duplicate action advanced the draft: %+v", st)
	}
}

func TestTimeoutBeforeDeadlineRejected(t *testing.T) {
	d := newDraft(t)
	if _, err := d.Timeout(draftStart.Add(10 * time.Second)); !errors.Is(err, taskpkg.ErrTurnNotExpired) {
		t.Fatalf("timeout before the deadline must yield ErrTurnNotExpired, got %v", err)
	}
	if st := d.State(); st.Turn != 1 || len(st.Bans) != 0 {
		t.Fatalf("early timeout changed the draft: %+v", st)
	}
}

func TestTimeoutAutoBansFirstAvailable(t *testing.T) {
	d := newDraft(t)
	st, err := d.Timeout(draftStart.Add(30 * time.Second))
	if err != nil {
		t.Fatalf("Timeout at the deadline: %v", err)
	}
	if len(st.Bans) != 1 || st.Bans[0] != "aatrox" {
		t.Fatalf("timeout must ban the sorted first champion aatrox, got %+v", st.Bans)
	}
	if st.Turn != 2 || st.Team != taskpkg.TeamRed || st.Phase != taskpkg.PhaseBan {
		t.Fatalf("timeout must advance to the red ban, got %+v", st)
	}
	h := d.History()
	if len(h) != 1 || h[0].ID != "auto-1" || h[0].Kind != taskpkg.KindBan || h[0].ChampionID != "aatrox" {
		t.Fatalf("auto action not recorded as auto-1: %+v", h)
	}
}

func TestFullDraftReachesDone(t *testing.T) {
	d := newDraft(t)
	base := draftBans(t, d)
	draftSubmit(t, d, "p1", taskpkg.TeamBlue, taskpkg.KindPick, "aatrox", base.Add(1*time.Second))
	draftSubmit(t, d, "p2", taskpkg.TeamRed, taskpkg.KindPick, "braum", base.Add(2*time.Second))
	draftSubmit(t, d, "p3", taskpkg.TeamBlue, taskpkg.KindPick, "darius", base.Add(3*time.Second))
	draftSubmit(t, d, "p4", taskpkg.TeamRed, taskpkg.KindPick, "ezreal", base.Add(4*time.Second))
	draftSubmit(t, d, "p5", taskpkg.TeamBlue, taskpkg.KindPick, "garen", base.Add(5*time.Second))
	draftSubmit(t, d, "p6", taskpkg.TeamRed, taskpkg.KindPick, "malphite", base.Add(6*time.Second))
	st := d.State()
	if st.Phase != taskpkg.PhaseDone {
		t.Fatalf("phase after the last pick = %q, want done", st.Phase)
	}
	if len(st.BluePicks) != 3 || len(st.RedPicks) != 3 {
		t.Fatalf("pick counts blue=%d red=%d, want 3 and 3", len(st.BluePicks), len(st.RedPicks))
	}
	if _, err := d.Submit(taskpkg.Action{ID: "p7", Team: taskpkg.TeamBlue, Kind: taskpkg.KindPick, ChampionID: "sivir", At: base.Add(7 * time.Second)}, base.Add(7*time.Second)); !errors.Is(err, taskpkg.ErrDraftClosed) {
		t.Fatalf("action after the draft finished must yield ErrDraftClosed, got %v", err)
	}
}

func TestAvailableFiltersRoleAndUsed(t *testing.T) {
	d := newDraft(t)
	got := d.Available("mage")
	want := []string{"ahri", "lux", "orianna"}
	if len(got) != len(want) {
		t.Fatalf("available mages = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("available mages = %v, want %v", got, want)
		}
	}
	draftSubmit(t, d, "b1", taskpkg.TeamBlue, taskpkg.KindBan, "ahri", draftStart.Add(time.Second))
	draftSubmit(t, d, "b2", taskpkg.TeamRed, taskpkg.KindBan, "lux", draftStart.Add(2*time.Second))
	got = d.Available("mage")
	if len(got) != 1 || got[0] != "orianna" {
		t.Fatalf("available mages after bans = %v, want [orianna]", got)
	}
	if fighters := d.Available("fighter"); len(fighters) != 4 {
		t.Fatalf("available fighters = %v, want four", fighters)
	}
	if all := d.Available(""); len(all) != 13 {
		t.Fatalf("available champions = %d, want 13", len(all))
	}
}

func TestAvailableReturnsCopy(t *testing.T) {
	d := newDraft(t)
	got := d.Available("mage")
	if len(got) != 3 || got[0] != "ahri" {
		t.Fatalf("available mages = %v, want [ahri lux orianna]", got)
	}
	got[0] = "zzz"
	again := d.Available("mage")
	if len(again) != 3 || again[0] != "ahri" {
		t.Fatalf("caller mutation leaked into the champion pool: %v", again)
	}
}

func TestStateReturnsCopy(t *testing.T) {
	d := newDraft(t)
	draftSubmit(t, d, "b1", taskpkg.TeamBlue, taskpkg.KindBan, "ahri", draftStart.Add(time.Second))
	st := d.State()
	if len(st.Bans) != 1 {
		t.Fatalf("state bans = %v, want one entry", st.Bans)
	}
	st.Bans[0] = "zzz"
	if again := d.State(); len(again.Bans) != 1 || again.Bans[0] != "ahri" {
		t.Fatalf("state mutation leaked into the draft: %+v", again)
	}
	champs := d.Champions()
	if len(champs) != 15 {
		t.Fatalf("champion pool size = %d, want 15", len(champs))
	}
	champs[0].ID = "zzz"
	if again := d.Champions(); again[0].ID == "zzz" {
		t.Fatalf("champion pool mutation leaked")
	}
}

func TestSnapshotRestoresDraft(t *testing.T) {
	d := newDraft(t)
	draftBans(t, d)
	at := draftStart.Add(5 * time.Second)
	draftSubmit(t, d, "p1", taskpkg.TeamBlue, taskpkg.KindPick, "aatrox", at)
	snap, err := d.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewPvpDraftFromSnapshot(taskpkg.Config{
		BansPerTeam:  2,
		PicksPerTeam: 3,
		TurnTimeout:  30 * time.Second,
	}, draftChampions(), snap)
	if err != nil {
		t.Fatalf("NewPvpDraftFromSnapshot: %v", err)
	}
	st := restored.State()
	if st.Phase != taskpkg.PhasePick || st.Turn != 6 || st.Team != taskpkg.TeamRed {
		t.Fatalf("restored state = %+v, want pick turn 6 red", st)
	}
	if len(st.Bans) != 4 || len(st.BluePicks) != 1 || st.BluePicks[0] != "aatrox" {
		t.Fatalf("restored picks/bans = %+v", st)
	}
	if want := at.Add(30 * time.Second); !restored.Deadline().Equal(want) {
		t.Fatalf("restored deadline = %v, want %v", restored.Deadline(), want)
	}
	_, err = restored.Submit(taskpkg.Action{ID: "b1", Team: taskpkg.TeamRed, Kind: taskpkg.KindPick, ChampionID: "braum", At: at.Add(time.Second)}, at.Add(time.Second))
	if !errors.Is(err, taskpkg.ErrDuplicateAction) {
		t.Fatalf("restart must remember accepted action ids, got %v", err)
	}
	draftSubmit(t, restored, "p2", taskpkg.TeamRed, taskpkg.KindPick, "braum", at.Add(2*time.Second))
	if h := restored.History(); len(h) != 6 {
		t.Fatalf("history after restore and pick = %d, want 6", len(h))
	}
}

func TestConcurrentSubmitSingleWinner(t *testing.T) {
	d := newDraft(t)
	champs := []string{"aatrox", "ahri", "ashe", "braum"}
	var wg sync.WaitGroup
	errs := make([]error, len(champs))
	at := draftStart.Add(time.Second)
	for i, champ := range champs {
		wg.Add(1)
		go func(i int, champ string) {
			defer wg.Done()
			_, errs[i] = d.Submit(taskpkg.Action{
				ID:         "c" + champ,
				Team:       taskpkg.TeamBlue,
				Kind:       taskpkg.KindBan,
				ChampionID: champ,
				At:         at,
			}, at)
		}(i, champ)
	}
	wg.Wait()
	successes := 0
	for _, err := range errs {
		if err == nil {
			successes++
		}
	}
	if successes != 1 {
		t.Fatalf("concurrent submissions succeeded %d times, want exactly 1", successes)
	}
	st := d.State()
	if st.Turn != 2 || len(st.Bans) != 1 {
		t.Fatalf("draft state after the race = %+v, want turn 2 with one ban", st)
	}
}

func TestDeadlineResetsOnAcceptedAction(t *testing.T) {
	d := newDraft(t)
	if want := draftStart.Add(30 * time.Second); !d.Deadline().Equal(want) {
		t.Fatalf("initial deadline = %v, want %v", d.Deadline(), want)
	}
	at := draftStart.Add(5 * time.Second)
	draftSubmit(t, d, "b1", taskpkg.TeamBlue, taskpkg.KindBan, "ahri", at)
	if want := at.Add(30 * time.Second); !d.Deadline().Equal(want) {
		t.Fatalf("deadline after a ban = %v, want %v", d.Deadline(), want)
	}
	if _, err := d.Timeout(at.Add(29 * time.Second)); !errors.Is(err, taskpkg.ErrTurnNotExpired) {
		t.Fatalf("timeout before the new deadline must be rejected, got %v", err)
	}
}
