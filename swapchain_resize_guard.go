package task

import "errors"

// Errors reported by SwapchainResize. A stale generation is reported
// separately from a bad resize request because the former means the GPU is
// still presenting while the latter is a window-system glitch.
var (
	ErrBadCaps         = errors.New("swapchain: invalid surface caps")
	ErrBadExtent       = errors.New("swapchain: extent must be positive")
	ErrBadScale        = errors.New("swapchain: scale must be in (0, 4]")
	ErrBadOrientation  = errors.New("swapchain: orientation must be 0, 90, 180 or 270")
	ErrNoPendingResize = errors.New("swapchain: no rebuild is pending")
	ErrFrameOrder      = errors.New("swapchain: rebuild frame must increase")
	ErrStaleGeneration = errors.New("swapchain: generation is not a retired swapchain")
	ErrConfigMismatch  = errors.New("swapchain: snapshot caps mismatch")

	errNotImplemented = errors.New("not implemented")
)

// Extent is a swapchain image size in pixels.
type Extent struct {
	Width  int
	Height int
}

// SurfaceCaps is the part of the surface capabilities the rebuild honours.
type SurfaceCaps struct {
	MinImages int
	MaxImages int
	MinExtent Extent
	MaxExtent Extent
}

// SwapchainInfo describes one generation of the swapchain.
type SwapchainInfo struct {
	Generation uint64
	Extent     Extent
	Images     int
	Format     string
}

// SwapchainResize coalesces window resize, DPI and orientation events into one
// swapchain rebuild per frame and tracks retired generations.
type SwapchainResize struct {
	caps SurfaceCaps
}

// NewSwapchainResize creates a rebuild guard for the given surface caps.
func NewSwapchainResize(caps SurfaceCaps) (*SwapchainResize, error) {
	return &SwapchainResize{caps: caps}, nil
}

// NewSwapchainResizeFromSnapshot restores the pending resize state and the
// generation chain. The caps must match the snapshot.
func NewSwapchainResizeFromSnapshot(caps SurfaceCaps, data []byte) (*SwapchainResize, error) {
	return nil, errNotImplemented
}

// RequestResize records a logical window size. Later requests win, and the
// effective extent is only computed when it is needed.
func (s *SwapchainResize) RequestResize(width, height int) error {
	return errNotImplemented
}

// SetScale applies a DPI scale factor on top of the logical size.
func (s *SwapchainResize) SetScale(scale float64) error {
	return errNotImplemented
}

// SetOrientation applies a display rotation; 90 and 270 swap the axes.
func (s *SwapchainResize) SetOrientation(degrees int) error {
	return errNotImplemented
}

// PendingResize returns the extent the next rebuild would produce and whether
// a rebuild is required at all.
func (s *SwapchainResize) PendingResize() (Extent, bool) {
	return Extent{}, false
}

// MarkOutOfDate forces a rebuild with the current extent, as a driver would
// after losing the swapchain.
func (s *SwapchainResize) MarkOutOfDate() error {
	return errNotImplemented
}

// Rebuild creates the next swapchain generation for the pending size, clamps
// the image count into the surface caps and retires the previous generation.
func (s *SwapchainResize) Rebuild(desiredImages int, frame uint64) (SwapchainInfo, error) {
	return SwapchainInfo{}, errNotImplemented
}

// Current returns the active generation and whether one exists.
func (s *SwapchainResize) Current() (SwapchainInfo, bool) {
	return SwapchainInfo{}, false
}

// Generation returns the active generation number, or 0 before the first
// rebuild.
func (s *SwapchainResize) Generation() uint64 {
	return 0
}

// Retired counts generations that are no longer current but not yet released.
func (s *SwapchainResize) Retired() int {
	return 0
}

// Release drops a retired generation after the GPU finished presenting it.
func (s *SwapchainResize) Release(generation uint64) error {
	return errNotImplemented
}

// Snapshot serialises the resize state and the generation chain.
func (s *SwapchainResize) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
