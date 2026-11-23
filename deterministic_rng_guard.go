package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by DeterministicRng.
var (
	ErrInvalidStream   = errors.New("rng: invalid stream name")
	ErrStreamExists    = errors.New("rng: stream already exists")
	ErrUnknownStream   = errors.New("rng: unknown stream")
	ErrStreamLimit     = errors.New("rng: stream limit reached")
	ErrDrawBudget      = errors.New("rng: draw budget exhausted")
	ErrRangeInvalid    = errors.New("rng: invalid draw range")
	ErrStaleCheckpoint = errors.New("rng: stale checkpoint")
)

// Config controls the seed and the hard limits of one lockstep match.
type Config struct {
	// Seed is the match seed. A stream's initial state depends on Seed and the
	// stream name only, never on draw order or on other streams.
	Seed uint64
	// MaxStreams caps the number of named streams; zero means 32.
	MaxStreams int
	// MaxDrawsPerStream caps the number of draws per stream; zero means 1<<24.
	MaxDrawsPerStream uint64
}

// StreamState is the persisted state of a single stream.
type StreamState struct {
	Name   string
	Cursor uint64
	State  uint64
	Digest uint64
}

// Checkpoint captures every stream at one tick boundary. Epoch identifies the
// generation the checkpoint was taken from; a checkpoint may be restored at
// most once.
type Checkpoint struct {
	Epoch   uint64
	Streams []StreamState
}

// DeterministicRng provides independent, reproducible random streams for a
// lockstep match. Every exported method is safe for concurrent use.
type DeterministicRng struct {
	cfg Config
}

// NewDeterministicRng returns an empty generator configured by cfg.
func NewDeterministicRng(cfg Config) *DeterministicRng {
	return &DeterministicRng{cfg: cfg}
}

// NewDeterministicRngFromSnapshot restores a generator, including stream
// cursors, internal states and digests, from bytes produced by Snapshot.
func NewDeterministicRngFromSnapshot(data []byte) (*DeterministicRng, error) {
	return nil, errNotImplemented
}

// AddStream registers a named stream. The name must be non-empty, unique and
// within Config.MaxStreams.
func (r *DeterministicRng) AddStream(name string) error {
	return errNotImplemented
}

// Next returns the next value of stream. Drawing from one stream never changes
// what another stream returns.
func (r *DeterministicRng) Next(stream string) (uint32, error) {
	return 0, errNotImplemented
}

// DrawRange returns a value in the inclusive interval [low, high]. An inverted
// interval yields ErrRangeInvalid; low == high returns low without consuming a
// draw.
func (r *DeterministicRng) DrawRange(stream string, low, high int32) (int32, error) {
	return 0, errNotImplemented
}

// Consumed reports how many values have been drawn from stream.
func (r *DeterministicRng) Consumed(stream string) (uint64, error) {
	return 0, errNotImplemented
}

// Checkpoint returns a copy of every stream's cursor, state and digest.
func (r *DeterministicRng) Checkpoint() (Checkpoint, error) {
	return Checkpoint{}, errNotImplemented
}

// Restore rewinds every stream to cp. A checkpoint from an older generation is
// rejected with ErrStaleCheckpoint; each checkpoint restores at most once.
func (r *DeterministicRng) Restore(cp Checkpoint) error {
	return errNotImplemented
}

// Fork derives a new stream dst from the current state of src without
// advancing src. The derived sequence is deterministic for equal parent
// states.
func (r *DeterministicRng) Fork(src, dst string) error {
	return errNotImplemented
}

// StreamDigest returns a running digest over the values drawn from stream, in
// draw order.
func (r *DeterministicRng) StreamDigest(stream string) (uint64, error) {
	return 0, errNotImplemented
}

// ResetStream returns stream to its freshly created state.
func (r *DeterministicRng) ResetStream(stream string) error {
	return errNotImplemented
}

// Snapshot serialises the whole generator so a restarted process resumes the
// exact same sequences.
func (r *DeterministicRng) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
