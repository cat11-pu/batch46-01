package task

import (
	"errors"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-e-05"
)

func defaultCaps() taskpkg.SurfaceCaps {
	return taskpkg.SurfaceCaps{
		MinImages: 2,
		MaxImages: 4,
		MinExtent: taskpkg.Extent{Width: 320, Height: 240},
		MaxExtent: taskpkg.Extent{Width: 2560, Height: 1440},
	}
}

func mustSwapchain(t *testing.T, caps taskpkg.SurfaceCaps) *taskpkg.SwapchainResize {
	t.Helper()
	s, err := taskpkg.NewSwapchainResize(caps)
	if err != nil {
		t.Fatalf("NewSwapchainResize(%+v): %v", caps, err)
	}
	return s
}

func wants(t *testing.T, s *taskpkg.SwapchainResize, width, height int) {
	t.Helper()
	extent, pending := s.PendingResize()
	if !pending {
		t.Fatalf("PendingResize reported no rebuild, want %dx%d", width, height)
	}
	if extent != (taskpkg.Extent{Width: width, Height: height}) {
		t.Fatalf("pending extent = %+v, want %dx%d", extent, width, height)
	}
}

// Invalid surface caps are rejected at construction time.
func TestNewRejectsBadCaps(t *testing.T) {
	bad := []taskpkg.SurfaceCaps{
		{MinImages: 0, MaxImages: 4, MinExtent: taskpkg.Extent{Width: 320, Height: 240}, MaxExtent: taskpkg.Extent{Width: 2560, Height: 1440}},
		{MinImages: 3, MaxImages: 2, MinExtent: taskpkg.Extent{Width: 320, Height: 240}, MaxExtent: taskpkg.Extent{Width: 2560, Height: 1440}},
		{MinImages: 1, MaxImages: 2, MinExtent: taskpkg.Extent{Width: 0, Height: 240}, MaxExtent: taskpkg.Extent{Width: 2560, Height: 1440}},
		{MinImages: 1, MaxImages: 2, MinExtent: taskpkg.Extent{Width: 320, Height: 240}, MaxExtent: taskpkg.Extent{Width: 100, Height: 100}},
	}
	for i, caps := range bad {
		if _, err := taskpkg.NewSwapchainResize(caps); !errors.Is(err, taskpkg.ErrBadCaps) {
			t.Fatalf("caps #%d error = %v, want ErrBadCaps", i, err)
		}
	}
	if _, err := taskpkg.NewSwapchainResize(defaultCaps()); err != nil {
		t.Fatalf("valid caps rejected: %v", err)
	}
}

// A fresh guard has no generation and no pending rebuild, and the first
// resize request makes one pending.
func TestInitialStateThenPending(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	info, ok := s.Current()
	if ok {
		t.Fatalf("Current reported %+v before any rebuild", info)
	}
	if s.Generation() != 0 {
		t.Fatalf("Generation = %d, want 0", s.Generation())
	}
	if s.Retired() != 0 {
		t.Fatalf("Retired = %d, want 0", s.Retired())
	}
	if _, pending := s.PendingResize(); pending {
		t.Fatalf("PendingResize reported a rebuild on a fresh guard")
	}
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	wants(t, s, 800, 600)
}

// Non-positive window sizes are rejected and leave the pending state alone.
func TestRequestResizeRejectsNonPositive(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(0, 600); !errors.Is(err, taskpkg.ErrBadExtent) {
		t.Fatalf("RequestResize(0, 600) error = %v, want ErrBadExtent", err)
	}
	if err := s.RequestResize(800, -1); !errors.Is(err, taskpkg.ErrBadExtent) {
		t.Fatalf("RequestResize(800, -1) error = %v, want ErrBadExtent", err)
	}
	if _, pending := s.PendingResize(); pending {
		t.Fatalf("rejected resize requests made a rebuild pending")
	}
}

// A burst of resize events coalesces into the last requested size.
func TestRequestResizeCoalesces(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("first RequestResize: %v", err)
	}
	if err := s.RequestResize(1024, 768); err != nil {
		t.Fatalf("second RequestResize: %v", err)
	}
	wants(t, s, 1024, 768)
	wants(t, s, 1024, 768)
}

// A rebuild needs a pending change, and a lost swapchain counts as one.
func TestRebuildRequiresPendingChange(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	first, err := s.Rebuild(3, 1)
	if err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}
	if first.Generation != 1 || first.Extent != (taskpkg.Extent{Width: 800, Height: 600}) {
		t.Fatalf("first rebuild = %+v, want generation 1 at 800x600", first)
	}
	if _, err := s.Rebuild(3, 2); !errors.Is(err, taskpkg.ErrNoPendingResize) {
		t.Fatalf("Rebuild without a pending change error = %v, want ErrNoPendingResize", err)
	}
	if err := s.MarkOutOfDate(); err != nil {
		t.Fatalf("MarkOutOfDate: %v", err)
	}
	wants(t, s, 800, 600)
	second, err := s.Rebuild(3, 2)
	if err != nil {
		t.Fatalf("Rebuild after MarkOutOfDate: %v", err)
	}
	if second.Generation != 2 || second.Extent != (taskpkg.Extent{Width: 800, Height: 600}) {
		t.Fatalf("second rebuild = %+v, want generation 2 at 800x600", second)
	}
	if s.Retired() != 1 {
		t.Fatalf("Retired = %d, want 1", s.Retired())
	}
}

// The effective extent is clamped into the surface extent range.
func TestRebuildClampsToMaxExtent(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(5000, 4000); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	info, err := s.Rebuild(4, 1)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	want := taskpkg.Extent{Width: 2560, Height: 1440}
	if info.Extent != want {
		t.Fatalf("extent = %+v, want %+v", info.Extent, want)
	}
	if info.Images != 4 || info.Generation != 1 {
		t.Fatalf("rebuild = %+v, want generation 1 with 4 images", info)
	}
	current, ok := s.Current()
	if !ok || current != info {
		t.Fatalf("Current = %+v, %v, want the rebuilt info", current, ok)
	}
}

// The requested image count is clamped into [MinImages, MaxImages].
func TestRebuildClampsImages(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	info, err := s.Rebuild(9, 1)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if info.Images != 4 {
		t.Fatalf("Images = %d, want 4", info.Images)
	}
	if err := s.RequestResize(1024, 768); err != nil {
		t.Fatalf("second RequestResize: %v", err)
	}
	info, err = s.Rebuild(1, 2)
	if err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}
	if info.Images != 2 {
		t.Fatalf("Images = %d, want 2", info.Images)
	}
}

// At most one rebuild happens per frame and the frame number must increase.
func TestRebuildFrameOrder(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	if _, err := s.Rebuild(3, 5); err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}
	if err := s.RequestResize(1024, 768); err != nil {
		t.Fatalf("second RequestResize: %v", err)
	}
	if _, err := s.Rebuild(3, 5); !errors.Is(err, taskpkg.ErrFrameOrder) {
		t.Fatalf("second rebuild in the same frame error = %v, want ErrFrameOrder", err)
	}
	if _, err := s.Rebuild(3, 4); !errors.Is(err, taskpkg.ErrFrameOrder) {
		t.Fatalf("rebuild with a smaller frame error = %v, want ErrFrameOrder", err)
	}
	if s.Generation() != 1 {
		t.Fatalf("Generation = %d, want 1 after rejected rebuilds", s.Generation())
	}
	if current, _ := s.Current(); current.Extent != (taskpkg.Extent{Width: 800, Height: 600}) {
		t.Fatalf("current extent = %+v, want the old 800x600", current.Extent)
	}
	if _, err := s.Rebuild(3, 6); err != nil {
		t.Fatalf("Rebuild on a later frame: %v", err)
	}
}

// Portrait and landscape rotations swap the axes; other angles are refused.
func TestOrientationSwapsExtent(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	if err := s.SetOrientation(90); err != nil {
		t.Fatalf("SetOrientation(90): %v", err)
	}
	wants(t, s, 600, 800)
	if _, err := s.Rebuild(3, 1); err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	current, _ := s.Current()
	if current.Extent != (taskpkg.Extent{Width: 600, Height: 800}) {
		t.Fatalf("rebuilt extent = %+v, want 600x800", current.Extent)
	}
	if err := s.SetOrientation(45); !errors.Is(err, taskpkg.ErrBadOrientation) {
		t.Fatalf("SetOrientation(45) error = %v, want ErrBadOrientation", err)
	}
	if err := s.SetOrientation(180); err != nil {
		t.Fatalf("SetOrientation(180): %v", err)
	}
	wants(t, s, 800, 600)
}

// A DPI change scales the logical size; out-of-range scales are refused.
func TestScaleApplies(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(1280, 720); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	if err := s.SetScale(1.5); err != nil {
		t.Fatalf("SetScale(1.5): %v", err)
	}
	wants(t, s, 1920, 1080)
	if err := s.SetScale(0); !errors.Is(err, taskpkg.ErrBadScale) {
		t.Fatalf("SetScale(0) error = %v, want ErrBadScale", err)
	}
	if err := s.SetScale(5); !errors.Is(err, taskpkg.ErrBadScale) {
		t.Fatalf("SetScale(5) error = %v, want ErrBadScale", err)
	}
	wants(t, s, 1920, 1080)
	info, err := s.Rebuild(3, 1)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if info.Extent != (taskpkg.Extent{Width: 1920, Height: 1080}) {
		t.Fatalf("rebuilt extent = %+v, want 1920x1080", info.Extent)
	}
}

// Scale is applied before rotation, and the rotated result is clamped.
func TestScaleAndOrientationCombine(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	if err := s.SetScale(1.5); err != nil {
		t.Fatalf("SetScale: %v", err)
	}
	if err := s.SetOrientation(270); err != nil {
		t.Fatalf("SetOrientation: %v", err)
	}
	wants(t, s, 900, 1200)
	info, err := s.Rebuild(3, 1)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if info.Extent != (taskpkg.Extent{Width: 900, Height: 1200}) {
		t.Fatalf("rebuilt extent = %+v, want 900x1200", info.Extent)
	}
}

// Tiny windows are grown to the surface minimum extent.
func TestClampsToMinExtent(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(100, 50); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	info, err := s.Rebuild(2, 1)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if info.Extent != (taskpkg.Extent{Width: 320, Height: 240}) {
		t.Fatalf("extent = %+v, want 320x240", info.Extent)
	}
}

// Retired generations are released once, and the current one never is.
func TestReleaseRetiredGenerations(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	if _, err := s.Rebuild(3, 1); err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}
	if err := s.RequestResize(1024, 768); err != nil {
		t.Fatalf("second RequestResize: %v", err)
	}
	if _, err := s.Rebuild(3, 2); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}
	if s.Retired() != 1 {
		t.Fatalf("Retired = %d, want 1", s.Retired())
	}
	if err := s.Release(1); err != nil {
		t.Fatalf("Release(1): %v", err)
	}
	if s.Retired() != 0 {
		t.Fatalf("Retired = %d, want 0 after release", s.Retired())
	}
	if err := s.Release(1); !errors.Is(err, taskpkg.ErrStaleGeneration) {
		t.Fatalf("second Release(1) error = %v, want ErrStaleGeneration", err)
	}
	if err := s.Release(3); !errors.Is(err, taskpkg.ErrStaleGeneration) {
		t.Fatalf("Release(3) error = %v, want ErrStaleGeneration", err)
	}
	if err := s.Release(2); !errors.Is(err, taskpkg.ErrStaleGeneration) {
		t.Fatalf("Release(current) error = %v, want ErrStaleGeneration", err)
	}
	if s.Retired() != 0 {
		t.Fatalf("Retired = %d, want 0", s.Retired())
	}
}

// A snapshot restores the generation chain and the pending state, and refuses
// caps that differ from the ones it was written with.
func TestSnapshotRestoresChain(t *testing.T) {
	s := mustSwapchain(t, defaultCaps())
	if err := s.RequestResize(800, 600); err != nil {
		t.Fatalf("RequestResize: %v", err)
	}
	if _, err := s.Rebuild(3, 1); err != nil {
		t.Fatalf("first Rebuild: %v", err)
	}
	if err := s.RequestResize(1024, 768); err != nil {
		t.Fatalf("second RequestResize: %v", err)
	}
	if _, err := s.Rebuild(3, 2); err != nil {
		t.Fatalf("second Rebuild: %v", err)
	}
	blob, err := s.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewSwapchainResizeFromSnapshot(defaultCaps(), blob)
	if err != nil {
		t.Fatalf("NewSwapchainResizeFromSnapshot: %v", err)
	}
	if restored.Generation() != 2 || restored.Retired() != 1 {
		t.Fatalf("restored generation/retired = %d/%d, want 2/1", restored.Generation(), restored.Retired())
	}
	current, ok := restored.Current()
	if !ok || current.Extent != (taskpkg.Extent{Width: 1024, Height: 768}) || current.Images != 3 {
		t.Fatalf("restored current = %+v, %v, want 1024x768 with 3 images", current, ok)
	}
	if _, err := restored.Rebuild(3, 3); !errors.Is(err, taskpkg.ErrNoPendingResize) {
		t.Fatalf("restored Rebuild without a pending change error = %v, want ErrNoPendingResize", err)
	}
	if err := restored.RequestResize(1280, 720); err != nil {
		t.Fatalf("restored RequestResize: %v", err)
	}
	info, err := restored.Rebuild(3, 10)
	if err != nil {
		t.Fatalf("restored Rebuild: %v", err)
	}
	if info.Generation != 3 {
		t.Fatalf("generation after restore = %d, want 3", info.Generation)
	}
	other := defaultCaps()
	other.MaxImages = 6
	if _, err := taskpkg.NewSwapchainResizeFromSnapshot(other, blob); !errors.Is(err, taskpkg.ErrConfigMismatch) {
		t.Fatalf("restore with other caps error = %v, want ErrConfigMismatch", err)
	}
}

// Concurrent resize events collapse into one pending rebuild for the next
// frame.
func TestConcurrentResizeRequests(t *testing.T) {
	const workers = 20
	s := mustSwapchain(t, defaultCaps())
	sizes := []taskpkg.Extent{
		{Width: 800, Height: 600},
		{Width: 1024, Height: 768},
		{Width: 1280, Height: 720},
		{Width: 1600, Height: 900},
	}
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			size := sizes[slot%len(sizes)]
			errs[slot] = s.RequestResize(size.Width, size.Height)
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent RequestResize #%d: %v", i, err)
		}
	}
	extent, pending := s.PendingResize()
	if !pending {
		t.Fatalf("PendingResize reported nothing after resize events")
	}
	known := false
	for _, size := range sizes {
		if extent == size {
			known = true
		}
	}
	if !known {
		t.Fatalf("pending extent %+v is not one of the requested sizes", extent)
	}
	info, err := s.Rebuild(3, 1)
	if err != nil {
		t.Fatalf("Rebuild: %v", err)
	}
	if info.Generation != 1 || info.Extent != extent {
		t.Fatalf("rebuild = %+v, want generation 1 at %+v", info, extent)
	}
	if _, err := s.Rebuild(3, 1); !errors.Is(err, taskpkg.ErrFrameOrder) {
		t.Fatalf("second rebuild in the same frame error = %v, want ErrFrameOrder", err)
	}
}
