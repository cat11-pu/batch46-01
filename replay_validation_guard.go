package task

import "errors"

var errNotImplemented = errors.New("not implemented")

// Sentinel errors returned by ReplayValidator.
var (
	ErrInvalidHeader      = errors.New("replay: invalid header")
	ErrDuplicatePlayer    = errors.New("replay: duplicate player")
	ErrTooManyPlayers     = errors.New("replay: too many players")
	ErrVersionUnsupported = errors.New("replay: unsupported version")
	ErrAlreadyOpen        = errors.New("replay: a replay is already open")
	ErrNoReplay           = errors.New("replay: no replay is open")
	ErrUnknownPlayer      = errors.New("replay: unknown player")
	ErrTickOutOfRange     = errors.New("replay: tick outside the replay duration")
	ErrTickOutOfOrder     = errors.New("replay: tick older than the last frame")
	ErrDuplicateInput     = errors.New("replay: duplicate input for the player and tick")
	ErrChecksumMismatch   = errors.New("replay: checksum chain mismatch")
	ErrReplayIncomplete   = errors.New("replay: missing player input")
	ErrTickNotRecorded    = errors.New("replay: tick has no recorded input")
	ErrReplayTooLong      = errors.New("replay: exceeds the frame budget")
)

// Config controls which replays this server accepts.
type Config struct {
	// MaxPlayers caps the players in a header; zero means 8.
	MaxPlayers int
	// MaxTicks caps the replay duration; zero means 65536.
	MaxTicks uint32
	// Version is the only replay format version accepted; zero means 1.
	Version int
}

// ReplayHeader is the metadata block at the front of a replay.
type ReplayHeader struct {
	MatchID       string
	Version       int
	TickRate      int
	Players       []string
	DurationTicks uint32
}

// InputFrame is one player input recorded at one tick.
type InputFrame struct {
	Tick     uint32
	PlayerID string
	Input    string
	Checksum uint64
}

// ValidationReport is the outcome of sealing a replay.
type ValidationReport struct {
	Valid            bool
	Frames           int
	Ticks            uint32
	FirstMissingTick uint32
	HeadChecksum     uint64
}

// ReplayValidator checks that a recorded replay is complete and that every
// frame's checksum chains onto the previous frame. All exported methods are
// safe for concurrent use.
type ReplayValidator struct {
	cfg Config
}

// NewReplayValidator returns an empty validator configured by cfg.
func NewReplayValidator(cfg Config) *ReplayValidator {
	return &ReplayValidator{cfg: cfg}
}

// NewReplayValidatorFromSnapshot restores an open replay, including the
// checksum head, from bytes produced by Snapshot.
func NewReplayValidatorFromSnapshot(data []byte) (*ReplayValidator, error) {
	return nil, errNotImplemented
}

// Open starts validating the replay described by h. A second Open on the same
// validator yields ErrAlreadyOpen.
func (v *ReplayValidator) Open(h ReplayHeader) error {
	return errNotImplemented
}

// Append validates and records one frame. Frames must belong to a known
// player, lie inside the duration, never move to an older tick, appear at most
// once per player and tick, and carry the checksum produced by ChecksumChain
// from the previous frame's checksum.
func (v *ReplayValidator) Append(f InputFrame) error {
	return errNotImplemented
}

// Seal verifies that every tick in [0, DurationTicks) has exactly one input
// from every player.
func (v *ReplayValidator) Seal() (ValidationReport, error) {
	return ValidationReport{}, errNotImplemented
}

// HeadChecksum returns the checksum after the last recorded frame.
func (v *ReplayValidator) HeadChecksum() uint64 {
	return 0
}

// FrameCount reports how many frames have been recorded.
func (v *ReplayValidator) FrameCount() int {
	return 0
}

// Rewind drops every frame recorded after toTick and returns how many frames
// were removed. toTick must have at least one recorded frame.
func (v *ReplayValidator) Rewind(toTick uint32) (int, error) {
	return 0, errNotImplemented
}

// TickChecksum returns the checksum after the last frame recorded at tick.
func (v *ReplayValidator) TickChecksum(tick uint32) (uint64, error) {
	return 0, errNotImplemented
}

// PlayerInputs reports how many frames a player has contributed.
func (v *ReplayValidator) PlayerInputs(playerID string) int {
	return 0
}

// ChecksumChain folds one frame into prev. It consumes the 4-byte
// little-endian tick, then the player id bytes, then one 0x1F separator byte,
// then the input bytes. Each byte is XORed into the 64-bit state and the state
// is multiplied by 1099511628211.
func (v *ReplayValidator) ChecksumChain(prev uint64, tick uint32, playerID, input string) uint64 {
	return 0
}

// Snapshot serialises the open replay so a restart continues the same chain.
func (v *ReplayValidator) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
