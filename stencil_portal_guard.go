package task

import "errors"

// errNotImplemented is returned by every stub until the task is implemented.
var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by the portal guard.
var (
	ErrInvalidConfig    = errors.New("portal: invalid config")
	ErrInvalidRoom      = errors.New("portal: invalid room")
	ErrRoomExists       = errors.New("portal: room already registered")
	ErrRoomUnknown      = errors.New("portal: unknown room")
	ErrInvalidPortal    = errors.New("portal: invalid portal")
	ErrPortalExists     = errors.New("portal: portal already registered")
	ErrSameRoom         = errors.New("portal: both ends are the same room")
	ErrInvalidRecursion = errors.New("portal: invalid recursion limit")
	ErrPortalNotVisible = errors.New("portal: portal is not in the current plan")
	ErrOrderViolation   = errors.New("portal: submission order violates the plan")
	ErrDuplicatePortal  = errors.New("portal: duplicate portal in submission")
	ErrAlreadySubmitted = errors.New("portal: portal already submitted this frame")
	ErrDrawBudget       = errors.New("portal: draw budget exceeded")
	ErrCorruptSnapshot  = errors.New("portal: corrupt snapshot")
)

// PortalConfig configures a stencil portal guard. MaxRecursion caps the
// portal depth that may be drawn and MaxDrawCalls caps the per-frame draw
// cost.
type PortalConfig struct {
	MaxRecursion int
	MaxDrawCalls int
}

// Portal links two rooms. A one-way portal is traversable only from FromRoom;
// a two-sided portal is traversable in both directions.
type Portal struct {
	ID       string
	FromRoom string
	ToRoom   string
	TwoSided bool
}

// PlannedPortal is one step of the stencil render plan.
type PlannedPortal struct {
	PortalID   string
	Depth      int
	StencilRef int
	DrawCost   int
}

// Plan is the ordered portal render plan for the current camera.
type Plan struct {
	Steps     []PlannedPortal
	Truncated bool
}

// SubmitResult reports the work one Submit call admitted.
type SubmitResult struct {
	Portals  int
	DrawCost int
}

// PortalStats is a snapshot of guard counters.
type PortalStats struct {
	Rooms     int
	Portals   int
	Visible   int
	Submitted int
}

// StencilPortal tracks rooms, portals, the camera room and the per-frame
// submission state.
type StencilPortal struct {
	cfg PortalConfig
}

// NewStencilPortal returns a guard configured by cfg.
func NewStencilPortal(cfg PortalConfig) (*StencilPortal, error) {
	return &StencilPortal{cfg: cfg}, nil
}

// NewStencilPortalFromSnapshot restores the graph, camera, recursion limit and
// frame submission state from data produced by Snapshot.
func NewStencilPortalFromSnapshot(cfg PortalConfig, data []byte) (*StencilPortal, error) {
	return nil, errNotImplemented
}

// AddRoom registers a room.
func (g *StencilPortal) AddRoom(id string) error {
	return errNotImplemented
}

// AddPortal registers a portal between two registered rooms.
func (g *StencilPortal) AddPortal(p Portal) error {
	return errNotImplemented
}

// SetCamera sets the room the camera stands in.
func (g *StencilPortal) SetCamera(roomID string) error {
	return errNotImplemented
}

// VisiblePortals returns the portals reachable from the camera, breadth first
// by depth and sorted by ID inside a depth level. Every portal appears once.
func (g *StencilPortal) VisiblePortals() ([]string, error) {
	return nil, errNotImplemented
}

// RenderPlan turns the visible portals into stencil steps. Portals at or
// beyond the recursion limit are dropped and Truncated is set.
func (g *StencilPortal) RenderPlan() (Plan, error) {
	return Plan{}, errNotImplemented
}

// SetRecursionLimit changes the recursion limit used by RenderPlan.
func (g *StencilPortal) SetRecursionLimit(n int) error {
	return errNotImplemented
}

// Submit admits portals for the current frame in plan order and within the
// draw cost budget.
func (g *StencilPortal) Submit(portalIDs []string) (SubmitResult, error) {
	return SubmitResult{}, errNotImplemented
}

// ResetFrame clears the per-frame submission state.
func (g *StencilPortal) ResetFrame() error {
	return errNotImplemented
}

// Stats returns a copy of the current counters.
func (g *StencilPortal) Stats() PortalStats {
	return PortalStats{}
}

// Snapshot serialises the graph, camera, recursion limit and frame state.
func (g *StencilPortal) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
