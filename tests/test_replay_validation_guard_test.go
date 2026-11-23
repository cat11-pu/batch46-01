package task

import (
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-c-05"
)

func replayConfig() taskpkg.Config {
	return taskpkg.Config{MaxPlayers: 4, MaxTicks: 32, Version: 1}
}

func testHeader() taskpkg.ReplayHeader {
	return taskpkg.ReplayHeader{
		MatchID:       "match-42",
		Version:       1,
		TickRate:      20,
		Players:       []string{"alpha", "bravo"},
		DurationTicks: 4,
	}
}

func chain(prev uint64, tick uint32, playerID, input string) uint64 {
	h := prev
	buf := make([]byte, 4, 4+len(playerID)+1+len(input))
	binary.LittleEndian.PutUint32(buf, tick)
	buf = append(buf, playerID...)
	buf = append(buf, 0x1F)
	buf = append(buf, input...)
	for _, b := range buf {
		h ^= uint64(b)
		h *= 1099511628211
	}
	return h
}

func openReplay(t *testing.T) *taskpkg.ReplayValidator {
	t.Helper()
	v := taskpkg.NewReplayValidator(replayConfig())
	if err := v.Open(testHeader()); err != nil {
		t.Fatalf("Open: %v", err)
	}
	return v
}

func appendFrame(t *testing.T, v *taskpkg.ReplayValidator, head *uint64, tick uint32, player, input string) {
	t.Helper()
	sum := chain(*head, tick, player, input)
	if err := v.Append(taskpkg.InputFrame{Tick: tick, PlayerID: player, Input: input, Checksum: sum}); err != nil {
		t.Fatalf("Append(tick=%d player=%s): %v", tick, player, err)
	}
	*head = sum
}

func appendFullWithHeads(t *testing.T, v *taskpkg.ReplayValidator) (uint64, []uint64) {
	t.Helper()
	head := uint64(0)
	heads := make([]uint64, 4)
	for tick := uint32(0); tick < 4; tick++ {
		for _, player := range []string{"alpha", "bravo"} {
			appendFrame(t, v, &head, tick, player, fmt.Sprintf("move-%d-%s", tick, player))
		}
		heads[tick] = head
	}
	return head, heads
}

// Duplicate players in the header must be rejected.
func TestOpenRejectsDuplicatePlayers(t *testing.T) {
	v := taskpkg.NewReplayValidator(replayConfig())
	h := testHeader()
	h.Players = []string{"alpha", "alpha"}
	if err := v.Open(h); !errors.Is(err, taskpkg.ErrDuplicatePlayer) {
		t.Fatalf("duplicate players error = %v, want ErrDuplicatePlayer", err)
	}
}

// A replay format newer than the server must be rejected.
func TestOpenRejectsUnsupportedVersion(t *testing.T) {
	v := taskpkg.NewReplayValidator(replayConfig())
	h := testHeader()
	h.Version = 2
	if err := v.Open(h); !errors.Is(err, taskpkg.ErrVersionUnsupported) {
		t.Fatalf("version 2 error = %v, want ErrVersionUnsupported", err)
	}
}

// Frames must not be accepted before a header is opened.
func TestAppendBeforeOpen(t *testing.T) {
	v := taskpkg.NewReplayValidator(replayConfig())
	err := v.Append(taskpkg.InputFrame{Tick: 0, PlayerID: "alpha", Input: "w", Checksum: chain(0, 0, "alpha", "w")})
	if !errors.Is(err, taskpkg.ErrNoReplay) {
		t.Fatalf("append before open error = %v, want ErrNoReplay", err)
	}
}

// Frames from players outside the header must be rejected.
func TestAppendUnknownPlayer(t *testing.T) {
	v := openReplay(t)
	err := v.Append(taskpkg.InputFrame{Tick: 0, PlayerID: "charlie", Input: "w", Checksum: chain(0, 0, "charlie", "w")})
	if !errors.Is(err, taskpkg.ErrUnknownPlayer) {
		t.Fatalf("unknown player error = %v, want ErrUnknownPlayer", err)
	}
}

// A frame at or beyond the declared duration must be rejected.
func TestAppendTickOutOfRange(t *testing.T) {
	v := openReplay(t)
	err := v.Append(taskpkg.InputFrame{Tick: 4, PlayerID: "alpha", Input: "w", Checksum: chain(0, 4, "alpha", "w")})
	if !errors.Is(err, taskpkg.ErrTickOutOfRange) {
		t.Fatalf("tick 4 error = %v, want ErrTickOutOfRange", err)
	}
}

// Ticks must never move backwards.
func TestAppendOutOfOrderTick(t *testing.T) {
	v := openReplay(t)
	head := uint64(0)
	appendFrame(t, v, &head, 2, "alpha", "late")
	old := chain(head, 1, "alpha", "early")
	if err := v.Append(taskpkg.InputFrame{Tick: 1, PlayerID: "alpha", Input: "early", Checksum: old}); !errors.Is(err, taskpkg.ErrTickOutOfOrder) {
		t.Fatalf("older tick error = %v, want ErrTickOutOfOrder", err)
	}
}

// One player may contribute only one input per tick.
func TestAppendDuplicateInput(t *testing.T) {
	v := openReplay(t)
	head := uint64(0)
	appendFrame(t, v, &head, 0, "alpha", "w")
	dup := chain(head, 0, "alpha", "w")
	if err := v.Append(taskpkg.InputFrame{Tick: 0, PlayerID: "alpha", Input: "w", Checksum: dup}); !errors.Is(err, taskpkg.ErrDuplicateInput) {
		t.Fatalf("duplicate input error = %v, want ErrDuplicateInput", err)
	}
}

// A corrupted checksum must be detected before the frame is stored.
func TestAppendChecksumMismatch(t *testing.T) {
	v := openReplay(t)
	bad := chain(0, 0, "alpha", "w") + 1
	if err := v.Append(taskpkg.InputFrame{Tick: 0, PlayerID: "alpha", Input: "w", Checksum: bad}); !errors.Is(err, taskpkg.ErrChecksumMismatch) {
		t.Fatalf("bad checksum error = %v, want ErrChecksumMismatch", err)
	}
	if got := v.FrameCount(); got != 0 {
		t.Fatalf("corrupt frame was stored: %d", got)
	}
}

// A correctly chained replay must expose the expected head and tick checksums.
func TestValidChainHeadChecksum(t *testing.T) {
	v := openReplay(t)
	head, heads := appendFullWithHeads(t, v)
	if head == 0 {
		t.Fatalf("test chain produced a zero head")
	}
	if got := v.HeadChecksum(); got != head {
		t.Fatalf("HeadChecksum = %d, want %d", got, head)
	}
	if got := v.FrameCount(); got != 8 {
		t.Fatalf("FrameCount = %d, want 8", got)
	}
	tickSum, err := v.TickChecksum(1)
	if err != nil {
		t.Fatalf("TickChecksum(1): %v", err)
	}
	if tickSum != heads[1] {
		t.Fatalf("TickChecksum(1) = %d, want %d", tickSum, heads[1])
	}
	if got, want := v.ChecksumChain(0, 0, "alpha", "w"), chain(0, 0, "alpha", "w"); got != want {
		t.Fatalf("ChecksumChain = %d, want %d", got, want)
	}
}

// Sealing must point at the first tick with a missing player input.
func TestSealIncompleteReplay(t *testing.T) {
	v := openReplay(t)
	head := uint64(0)
	for tick := uint32(0); tick < 2; tick++ {
		for _, player := range []string{"alpha", "bravo"} {
			appendFrame(t, v, &head, tick, player, fmt.Sprintf("move-%d-%s", tick, player))
		}
	}
	appendFrame(t, v, &head, 2, "alpha", "move-2-alpha")
	for _, player := range []string{"alpha", "bravo"} {
		appendFrame(t, v, &head, 3, player, fmt.Sprintf("move-3-%s", player))
	}
	report, err := v.Seal()
	if !errors.Is(err, taskpkg.ErrReplayIncomplete) {
		t.Fatalf("seal error = %v, want ErrReplayIncomplete", err)
	}
	if report.Valid {
		t.Fatalf("incomplete replay reported valid")
	}
	if report.FirstMissingTick != 2 {
		t.Fatalf("FirstMissingTick = %d, want 2", report.FirstMissingTick)
	}
}

// A complete replay must seal with the expected frame and tick counts.
func TestSealCompleteReplay(t *testing.T) {
	v := openReplay(t)
	head, _ := appendFullWithHeads(t, v)
	report, err := v.Seal()
	if err != nil {
		t.Fatalf("Seal: %v", err)
	}
	if !report.Valid {
		t.Fatalf("complete replay reported invalid")
	}
	if report.Frames != 8 {
		t.Fatalf("Frames = %d, want 8", report.Frames)
	}
	if report.Ticks != 4 {
		t.Fatalf("Ticks = %d, want 4", report.Ticks)
	}
	if report.HeadChecksum != head {
		t.Fatalf("HeadChecksum = %d, want %d", report.HeadChecksum, head)
	}
}

// Rewinding must drop later frames and restore the earlier chain head.
func TestRewindTruncatesFrames(t *testing.T) {
	v := openReplay(t)
	_, heads := appendFullWithHeads(t, v)
	removed, err := v.Rewind(1)
	if err != nil {
		t.Fatalf("Rewind(1): %v", err)
	}
	if removed != 4 {
		t.Fatalf("removed = %d, want 4", removed)
	}
	if got := v.FrameCount(); got != 4 {
		t.Fatalf("FrameCount = %d, want 4", got)
	}
	if got := v.HeadChecksum(); got != heads[1] {
		t.Fatalf("HeadChecksum after rewind = %d, want %d", got, heads[1])
	}
}

// Rewinding to a tick with no input must be rejected.
func TestRewindUnknownTick(t *testing.T) {
	v := openReplay(t)
	appendFullWithHeads(t, v)
	if _, err := v.Rewind(7); !errors.Is(err, taskpkg.ErrTickNotRecorded) {
		t.Fatalf("Rewind(7) error = %v, want ErrTickNotRecorded", err)
	}
}

// A restarted process must continue the checksum chain from the snapshot.
func TestSnapshotRestoreContinuesChain(t *testing.T) {
	v := openReplay(t)
	head := uint64(0)
	for tick := uint32(0); tick < 2; tick++ {
		for _, player := range []string{"alpha", "bravo"} {
			appendFrame(t, v, &head, tick, player, fmt.Sprintf("move-%d-%s", tick, player))
		}
	}
	snap, err := v.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewReplayValidatorFromSnapshot(snap)
	if err != nil {
		t.Fatalf("NewReplayValidatorFromSnapshot: %v", err)
	}
	if got := restored.HeadChecksum(); got != head {
		t.Fatalf("restored head = %d, want %d", got, head)
	}
	for tick := uint32(2); tick < 4; tick++ {
		for _, player := range []string{"alpha", "bravo"} {
			appendFrame(t, restored, &head, tick, player, fmt.Sprintf("move-%d-%s", tick, player))
		}
	}
	full := openReplay(t)
	wantHead, _ := appendFullWithHeads(t, full)
	if got := restored.HeadChecksum(); got != wantHead {
		t.Fatalf("head after restore = %d, want %d", got, wantHead)
	}
	if got := restored.FrameCount(); got != 8 {
		t.Fatalf("restored FrameCount = %d, want 8", got)
	}
	if got := restored.PlayerInputs("bravo"); got != 4 {
		t.Fatalf("bravo inputs = %d, want 4", got)
	}
}

// Concurrent readers must see a consistent chain while frames are appended.
func TestConcurrentAppendAndRead(t *testing.T) {
	h := testHeader()
	h.Players = []string{"alpha", "bravo", "charlie", "delta"}
	v := taskpkg.NewReplayValidator(replayConfig())
	if err := v.Open(h); err != nil {
		t.Fatalf("Open: %v", err)
	}
	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				v.FrameCount()
				v.HeadChecksum()
				v.PlayerInputs("alpha")
			}
		}()
	}
	appendErr := make(chan error, 1)
	go func() {
		head := uint64(0)
		for tick := uint32(0); tick < 4; tick++ {
			for _, player := range h.Players {
				sum := chain(head, tick, player, fmt.Sprintf("move-%d-%s", tick, player))
				if err := v.Append(taskpkg.InputFrame{Tick: tick, PlayerID: player, Input: fmt.Sprintf("move-%d-%s", tick, player), Checksum: sum}); err != nil {
					appendErr <- err
					return
				}
				head = sum
			}
		}
		appendErr <- nil
	}()
	if err := <-appendErr; err != nil {
		close(stop)
		wg.Wait()
		t.Fatalf("concurrent Append: %v", err)
	}
	close(stop)
	wg.Wait()
	if got := v.FrameCount(); got != 16 {
		t.Fatalf("FrameCount = %d, want 16", got)
	}
	wantHead := uint64(0)
	for tick := uint32(0); tick < 4; tick++ {
		for _, player := range h.Players {
			wantHead = chain(wantHead, tick, player, fmt.Sprintf("move-%d-%s", tick, player))
		}
	}
	if got := v.HeadChecksum(); got != wantHead {
		t.Fatalf("HeadChecksum = %d, want %d", got, wantHead)
	}
	for _, player := range h.Players {
		if got := v.PlayerInputs(player); got != 4 {
			t.Fatalf("PlayerInputs(%q) = %d, want 4", player, got)
		}
	}
}
