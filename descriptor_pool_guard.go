package task

import "errors"

// Errors reported by DescriptorPool. Budget exhaustion is reported per
// descriptor type so the renderer can log which table ran out instead of
// guessing.
var (
	ErrInvalidConfig  = errors.New("descriptor pool: frame count, set budget and counts must be sane")
	ErrBadFrame       = errors.New("descriptor pool: frame index out of range")
	ErrNegativeCount  = errors.New("descriptor pool: descriptor count cannot be negative")
	ErrOutOfSets      = errors.New("descriptor pool: per-frame set budget exhausted")
	ErrOutOfUniforms  = errors.New("descriptor pool: uniform buffer budget exhausted")
	ErrOutOfStorages  = errors.New("descriptor pool: storage buffer budget exhausted")
	ErrOutOfSamplers  = errors.New("descriptor pool: sampler budget exhausted")
	ErrOutOfImages    = errors.New("descriptor pool: storage image budget exhausted")
	ErrUnknownSet     = errors.New("descriptor pool: unknown set id")
	ErrDoubleFree     = errors.New("descriptor pool: set already freed")
	ErrFrameBusy      = errors.New("descriptor pool: frame still has live sets")
	ErrFrameOrder     = errors.New("descriptor pool: frame reset out of order")
	ErrConfigMismatch = errors.New("descriptor pool: snapshot configuration mismatch")

	errNotImplemented = errors.New("not implemented")
)

// DescCounts is the descriptor demand of one set, or the per-frame budget of
// one pool when used as configuration.
type DescCounts struct {
	Uniforms int
	Storages int
	Samplers int
	Images   int
}

// SetInfo is the observable record of one allocated descriptor set.
type SetInfo struct {
	ID    string
	Frame int
	Need  DescCounts
	Freed bool
}

// DescriptorPool serves a ring of frames. Every frame owns an independent
// budget so that allocation never waits for GPU work on another frame.
type DescriptorPool struct {
	frames       int
	setsPerFrame int
	perFrame     DescCounts
}

// NewDescriptorPool creates a pool with frames * setsPerFrame set slots and
// the given per-frame descriptor budget.
func NewDescriptorPool(frames, setsPerFrame int, perFrame DescCounts) (*DescriptorPool, error) {
	return &DescriptorPool{frames: frames, setsPerFrame: setsPerFrame, perFrame: perFrame}, nil
}

// NewDescriptorPoolFromSnapshot restores allocations and the reset sequence
// from a snapshot. The pool configuration must match the snapshot.
func NewDescriptorPoolFromSnapshot(frames, setsPerFrame int, perFrame DescCounts, data []byte) (*DescriptorPool, error) {
	return nil, errNotImplemented
}

// AllocSet reserves one set in frame for need and returns its id. The id is
// "f<frame>-<slot>" with the lowest free slot starting at 1.
func (p *DescriptorPool) AllocSet(frame int, need DescCounts) (string, error) {
	return "", errNotImplemented
}

// FreeSet marks a set as unused by its frame. Descriptor budget is only
// returned to the frame by ResetFrame.
func (p *DescriptorPool) FreeSet(id string) error {
	return errNotImplemented
}

// ResetFrame reclaims every set of frame. Frames must be reset in ring order
// and every set of the frame must already be freed.
func (p *DescriptorPool) ResetFrame(frame int) error {
	return errNotImplemented
}

// Usage returns the descriptors currently charged to frame.
func (p *DescriptorPool) Usage(frame int) (DescCounts, error) {
	return DescCounts{}, errNotImplemented
}

// FreedPending counts sets of frame that are freed but not yet reset.
func (p *DescriptorPool) FreedPending(frame int) (int, error) {
	return 0, errNotImplemented
}

// ActiveSets counts sets that were allocated and not yet reset.
func (p *DescriptorPool) ActiveSets() int {
	return 0
}

// Faults counts allocation failures caused by exhausted budgets.
func (p *DescriptorPool) Faults() int {
	return 0
}

// Describe returns the record of one set.
func (p *DescriptorPool) Describe(id string) (SetInfo, error) {
	return SetInfo{}, errNotImplemented
}

// SetCount counts the sets charged to frame, freed or not.
func (p *DescriptorPool) SetCount(frame int) (int, error) {
	return 0, errNotImplemented
}

// Snapshot serialises the allocation table, reset sequence and fault counter.
func (p *DescriptorPool) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
