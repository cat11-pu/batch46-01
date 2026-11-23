package task

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	taskpkg "gsb/batch46-game-b-08"
)

var evNow = time.Date(2026, 5, 2, 9, 0, 0, 0, time.UTC)

func evHex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func evLink(prev, reportID, evidenceID, sha string, size int64) string {
	sum := sha256.Sum256([]byte(prev + "|" + reportID + "|" + evidenceID + "|" + sha + "|" + strconv.FormatInt(size, 10)))
	return hex.EncodeToString(sum[:])
}

func evConfig() taskpkg.Config {
	return taskpkg.Config{
		MaxEvidencePerReport: 3,
		MaxTotalBytes:        1000,
		ReporterDailyQuota:   5,
		Retention:            24 * time.Hour,
	}
}

func newGuard(t *testing.T) *taskpkg.EvidenceGuard {
	t.Helper()
	g := taskpkg.NewEvidenceGuard(evConfig())
	if err := g.OpenReport("r-1", "alice", "bob", "wintrade", taskpkg.SeverityHigh, evNow); err != nil {
		t.Fatalf("OpenReport r-1: %v", err)
	}
	if err := g.OpenReport("r-2", "alice", "carol", "abuse", taskpkg.SeverityMedium, evNow); err != nil {
		t.Fatalf("OpenReport r-2: %v", err)
	}
	return g
}

func evItem(id, content string, size int64) taskpkg.Evidence {
	return taskpkg.Evidence{ID: id, Kind: "screenshot", SHA256: evHex(content), Size: size, CapturedAt: evNow}
}

func evAdd(t *testing.T, g *taskpkg.EvidenceGuard, reportID string, e taskpkg.Evidence) {
	t.Helper()
	if _, err := g.AddEvidence(reportID, e, evNow); err != nil {
		t.Fatalf("AddEvidence(%s/%s): %v", reportID, e.ID, err)
	}
}

func TestOpenReportRejectsInvalidSeverity(t *testing.T) {
	g := taskpkg.NewEvidenceGuard(evConfig())
	for _, sev := range []taskpkg.Severity{0, 9} {
		if err := g.OpenReport("r-x", "alice", "bob", "cheat", sev, evNow); !errors.Is(err, taskpkg.ErrInvalidSeverity) {
			t.Fatalf("severity %d must yield ErrInvalidSeverity, got %v", sev, err)
		}
	}
	if err := g.OpenReport("r-ok", "alice", "bob", "cheat", taskpkg.SeverityLow, evNow); err != nil {
		t.Fatalf("valid severity rejected: %v", err)
	}
}

func TestAddEvidenceUnknownReport(t *testing.T) {
	g := newGuard(t)
	_, err := g.AddEvidence("r-404", evItem("ev-1", "alpha", 10), evNow)
	if !errors.Is(err, taskpkg.ErrUnknownReport) {
		t.Fatalf("unknown report must yield ErrUnknownReport, got %v", err)
	}
}

func TestAddEvidenceClosedReport(t *testing.T) {
	g := newGuard(t)
	if err := g.CloseReport("r-1", evNow); err != nil {
		t.Fatalf("CloseReport: %v", err)
	}
	_, err := g.AddEvidence("r-1", evItem("ev-1", "alpha", 10), evNow)
	if !errors.Is(err, taskpkg.ErrReportClosed) {
		t.Fatalf("closed report must yield ErrReportClosed, got %v", err)
	}
}

func TestDuplicateEvidenceID(t *testing.T) {
	g := newGuard(t)
	evAdd(t, g, "r-1", evItem("ev-1", "alpha", 10))
	_, err := g.AddEvidence("r-1", evItem("ev-1", "beta", 12), evNow)
	if !errors.Is(err, taskpkg.ErrDuplicateEvidence) {
		t.Fatalf("reused evidence id must yield ErrDuplicateEvidence, got %v", err)
	}
	if got := len(g.Chain("r-1")); got != 1 {
		t.Fatalf("chain length = %d, want 1", got)
	}
	if got := g.TotalBytes("r-1"); got != 10 {
		t.Fatalf("total bytes = %d, want 10", got)
	}
}

func TestDuplicateEvidenceContentHash(t *testing.T) {
	g := newGuard(t)
	evAdd(t, g, "r-1", evItem("ev-1", "alpha", 10))
	_, err := g.AddEvidence("r-1", evItem("ev-2", "alpha", 10), evNow)
	if !errors.Is(err, taskpkg.ErrDuplicateEvidence) {
		t.Fatalf("reused content hash must yield ErrDuplicateEvidence, got %v", err)
	}
	if got := g.TotalBytes("r-1"); got != 10 {
		t.Fatalf("total bytes = %d, want 10", got)
	}
}

func TestEvidenceCountCapAtomic(t *testing.T) {
	g := newGuard(t)
	evAdd(t, g, "r-1", evItem("ev-1", "a", 10))
	evAdd(t, g, "r-1", evItem("ev-2", "b", 10))
	evAdd(t, g, "r-1", evItem("ev-3", "c", 10))
	_, err := g.AddEvidence("r-1", evItem("ev-4", "d", 10), evNow)
	if !errors.Is(err, taskpkg.ErrQuotaExceeded) {
		t.Fatalf("fourth item must yield ErrQuotaExceeded, got %v", err)
	}
	if got := len(g.Chain("r-1")); got != 3 {
		t.Fatalf("chain length = %d, want 3", got)
	}
	if got := g.TotalBytes("r-1"); got != 30 {
		t.Fatalf("total bytes = %d, want 30", got)
	}
}

func TestTotalBytesCapAtomic(t *testing.T) {
	cfg := evConfig()
	cfg.MaxTotalBytes = 1000
	g := taskpkg.NewEvidenceGuard(cfg)
	if err := g.OpenReport("r-big", "alice", "bob", "cheat", taskpkg.SeverityHigh, evNow); err != nil {
		t.Fatalf("OpenReport: %v", err)
	}
	evAdd(t, g, "r-big", evItem("ev-1", "big", 900))
	_, err := g.AddEvidence("r-big", evItem("ev-2", "big2", 200), evNow)
	if !errors.Is(err, taskpkg.ErrQuotaExceeded) {
		t.Fatalf("bytes over the cap must yield ErrQuotaExceeded, got %v", err)
	}
	if got := g.TotalBytes("r-big"); got != 900 {
		t.Fatalf("total bytes = %d, want 900", got)
	}
	if got := len(g.Chain("r-big")); got != 1 {
		t.Fatalf("chain length = %d, want 1", got)
	}
}

func TestReporterDailyQuotaAcrossReports(t *testing.T) {
	g := newGuard(t)
	evAdd(t, g, "r-1", evItem("ev-1", "a", 10))
	evAdd(t, g, "r-1", evItem("ev-2", "b", 10))
	evAdd(t, g, "r-1", evItem("ev-3", "c", 10))
	evAdd(t, g, "r-2", evItem("ev-4", "d", 10))
	evAdd(t, g, "r-2", evItem("ev-5", "e", 10))
	_, err := g.AddEvidence("r-2", evItem("ev-6", "f", 10), evNow)
	if !errors.Is(err, taskpkg.ErrQuotaExceeded) {
		t.Fatalf("sixth item for the reporter must yield ErrQuotaExceeded, got %v", err)
	}
	if got := g.ReporterDailyCount("alice", evNow); got != 5 {
		t.Fatalf("reporter daily count = %d, want 5", got)
	}
	next := evNow.Add(48 * time.Hour)
	if _, err := g.AddEvidence("r-2", evItem("ev-6", "f", 10), next); err != nil {
		t.Fatalf("next day upload must be accepted, got %v", err)
	}
	if got := g.ReporterDailyCount("alice", next); got != 1 {
		t.Fatalf("next day count = %d, want 1", got)
	}
}

func TestChainHashFormulaAndLinks(t *testing.T) {
	g := newGuard(t)
	evAdd(t, g, "r-1", evItem("ev-1", "alpha", 10))
	evAdd(t, g, "r-1", evItem("ev-2", "beta", 20))
	entries := g.Chain("r-1")
	if len(entries) != 2 {
		t.Fatalf("chain length = %d, want 2", len(entries))
	}
	if entries[0].PrevHash != "0000000000000000000000000000000000000000000000000000000000000000" {
		t.Fatalf("first PrevHash = %q, want 64 zeros", entries[0].PrevHash)
	}
	for i, e := range entries {
		if e.Seq != i+1 {
			t.Fatalf("entry %d seq = %d, want %d", i, e.Seq, i+1)
		}
		if want := evLink(e.PrevHash, e.ReportID, e.EvidenceID, e.SHA256, e.Size); e.Hash != want {
			t.Fatalf("entry %d hash = %q, want %q", i, e.Hash, want)
		}
		if i > 0 && e.PrevHash != entries[i-1].Hash {
			t.Fatalf("entry %d does not link to the previous hash", i)
		}
	}
	if err := g.VerifyChain("r-1"); err != nil {
		t.Fatalf("VerifyChain on a clean chain: %v", err)
	}
}

func TestChainReturnsCopy(t *testing.T) {
	g := newGuard(t)
	evAdd(t, g, "r-1", evItem("ev-1", "alpha", 10))
	chain := g.Chain("r-1")
	if len(chain) != 1 {
		t.Fatalf("chain length = %d, want 1", len(chain))
	}
	chain[0].EvidenceID = "tampered"
	again := g.Chain("r-1")
	if len(again) != 1 || again[0].EvidenceID != "ev-1" {
		t.Fatalf("chain mutation leaked: %+v", again)
	}
	if err := g.VerifyChain("r-1"); err != nil {
		t.Fatalf("VerifyChain after a caller copy was mutated: %v", err)
	}
}

func TestPurgeExpiredRemovesOldClosedReports(t *testing.T) {
	g := newGuard(t)
	evAdd(t, g, "r-1", evItem("ev-1", "alpha", 10))
	if err := g.CloseReport("r-1", evNow); err != nil {
		t.Fatalf("CloseReport: %v", err)
	}
	if got := g.PurgeExpired(evNow.Add(25 * time.Hour)); got != 1 {
		t.Fatalf("purged reports = %d, want 1", got)
	}
	if _, ok := g.Report("r-1"); ok {
		t.Fatalf("closed report survived the retention window")
	}
	if got := len(g.Chain("r-1")); got != 0 {
		t.Fatalf("purged chain length = %d, want 0", got)
	}
	if _, ok := g.Report("r-2"); !ok {
		t.Fatalf("open report must not be purged")
	}
}

func TestPurgeKeepsRecentAndOpenReports(t *testing.T) {
	g := newGuard(t)
	if err := g.CloseReport("r-1", evNow); err != nil {
		t.Fatalf("CloseReport: %v", err)
	}
	if got := g.PurgeExpired(evNow.Add(23 * time.Hour)); got != 0 {
		t.Fatalf("purged reports inside retention = %d, want 0", got)
	}
	if _, ok := g.Report("r-1"); !ok {
		t.Fatalf("recently closed report must survive")
	}
	openOnly := taskpkg.NewEvidenceGuard(evConfig())
	if err := openOnly.OpenReport("r-open", "bob", "carol", "cheat", taskpkg.SeverityLow, evNow); err != nil {
		t.Fatalf("OpenReport r-open: %v", err)
	}
	if got := openOnly.PurgeExpired(evNow.Add(100 * time.Hour)); got != 0 {
		t.Fatalf("purged open reports = %d, want 0", got)
	}
	if _, ok := openOnly.Report("r-open"); !ok {
		t.Fatalf("open report must survive any purge")
	}
}

func TestCloseReportIdempotent(t *testing.T) {
	g := newGuard(t)
	if err := g.CloseReport("r-1", evNow); err != nil {
		t.Fatalf("first close: %v", err)
	}
	if err := g.CloseReport("r-1", evNow.Add(time.Minute)); err != nil {
		t.Fatalf("second close must be a no-op, got %v", err)
	}
	if err := g.CloseReport("r-404", evNow); !errors.Is(err, taskpkg.ErrUnknownReport) {
		t.Fatalf("closing an unknown report must yield ErrUnknownReport, got %v", err)
	}
	rep, ok := g.Report("r-1")
	if !ok || rep.Status != taskpkg.StatusClosed {
		t.Fatalf("report status = %+v, want closed", rep)
	}
}

func TestSnapshotRestoresChainAndDedupe(t *testing.T) {
	g := newGuard(t)
	evAdd(t, g, "r-1", evItem("ev-1", "alpha", 10))
	evAdd(t, g, "r-1", evItem("ev-2", "beta", 20))
	before := g.Chain("r-1")
	snap, err := g.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewEvidenceGuardFromSnapshot(evConfig(), snap)
	if err != nil {
		t.Fatalf("NewEvidenceGuardFromSnapshot: %v", err)
	}
	after := restored.Chain("r-1")
	if len(after) != len(before) {
		t.Fatalf("chain length after restore = %d, want %d", len(after), len(before))
	}
	for i := range before {
		if after[i].Hash != before[i].Hash || after[i].EvidenceID != before[i].EvidenceID {
			t.Fatalf("chain entry %d changed across restart", i)
		}
	}
	if err := restored.VerifyChain("r-1"); err != nil {
		t.Fatalf("VerifyChain after restore: %v", err)
	}
	if _, err := restored.AddEvidence("r-1", evItem("ev-1", "gamma", 30), evNow); !errors.Is(err, taskpkg.ErrDuplicateEvidence) {
		t.Fatalf("restart must remember evidence ids, got %v", err)
	}
	if _, err := restored.AddEvidence("r-1", evItem("ev-9", "alpha", 30), evNow); !errors.Is(err, taskpkg.ErrDuplicateEvidence) {
		t.Fatalf("restart must remember content hashes, got %v", err)
	}
	if got := restored.ReporterDailyCount("alice", evNow); got != 2 {
		t.Fatalf("reporter daily count after restore = %d, want 2", got)
	}
}

func TestConcurrentAddEvidenceRespectsCap(t *testing.T) {
	cfg := evConfig()
	cfg.MaxTotalBytes = 10000
	cfg.ReporterDailyQuota = 100
	g := taskpkg.NewEvidenceGuard(cfg)
	if err := g.OpenReport("r-race", "alice", "bob", "cheat", taskpkg.SeverityHigh, evNow); err != nil {
		t.Fatalf("OpenReport: %v", err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = g.AddEvidence("r-race", evItem("race-"+strconv.Itoa(i), "race-content-"+strconv.Itoa(i), 1), evNow)
		}(i)
	}
	wg.Wait()
	successes, capped := 0, 0
	for _, err := range errs {
		if err == nil {
			successes++
			continue
		}
		if errors.Is(err, taskpkg.ErrQuotaExceeded) {
			capped++
		}
	}
	if successes != 3 || capped != 5 {
		t.Fatalf("successes=%d capped=%d, want 3 and 5", successes, capped)
	}
	if got := len(g.Chain("r-race")); got != 3 {
		t.Fatalf("chain length = %d, want 3", got)
	}
	if err := g.VerifyChain("r-race"); err != nil {
		t.Fatalf("VerifyChain after concurrent uploads: %v", err)
	}
}
