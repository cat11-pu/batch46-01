package task

import "errors"

// Errors reported by PipelineCache. A compatibility change must be reported
// separately from a duplicate insert because the former invalidates work the
// driver already compiled while the latter is a caller bug.
var (
	ErrInvalidDesc       = errors.New("pipeline cache: descriptor misses a required field")
	ErrUnknownRenderPass = errors.New("pipeline cache: render pass not registered")
	ErrSubpassOutOfRange = errors.New("pipeline cache: subpass outside attachment range")
	ErrDuplicateEntry    = errors.New("pipeline cache: descriptor already cached")
	ErrDuplicateHandle   = errors.New("pipeline cache: handle already assigned")
	ErrCacheFull         = errors.New("pipeline cache: entry budget reached")
	ErrEvictRange        = errors.New("pipeline cache: evict count out of range")
	ErrVersionMismatch   = errors.New("pipeline cache: blob version mismatch")
	ErrInvalidLimit      = errors.New("pipeline cache: entry limit must be positive")

	errNotImplemented = errors.New("not implemented")
)

// PipelineDesc is the full graphics pipeline state that forms the cache key.
type PipelineDesc struct {
	VertexShader   string
	FragmentShader string
	VertexLayout   string
	RenderPass     string
	Subpass        uint32
	BlendState     string
	DepthState     string
}

// PipelineCache maps pipeline state to driver handles and keeps the
// render-pass compatibility table used to validate inserts.
type PipelineCache struct {
	version    string
	maxEntries int
}

// NewPipelineCache creates a cache for one driver/engine version. maxEntries
// is a hard cap on cached descriptors.
func NewPipelineCache(version string, maxEntries int) (*PipelineCache, error) {
	return &PipelineCache{version: version, maxEntries: maxEntries}, nil
}

// NewPipelineCacheFromBlob restores a cache serialised by Blob. A blob written
// by another version is rejected.
func NewPipelineCacheFromBlob(version string, maxEntries int, blob []byte) (*PipelineCache, error) {
	return nil, errNotImplemented
}

// RegisterRenderPass records the attachment count of a render pass. It returns
// the number of entries dropped because the compatibility changed; registering
// the same pass with the same attachment count changes nothing.
func (c *PipelineCache) RegisterRenderPass(name string, attachmentCount uint32) (int, error) {
	return 0, errNotImplemented
}

// Insert caches desc under handle. It validates the descriptor against the
// registered render pass before anything is stored.
func (c *PipelineCache) Insert(desc PipelineDesc, handle uint64) error {
	return errNotImplemented
}

// Lookup returns the handle of desc and marks it most recently used. A miss
// increments the miss counter, a hit increments the hit counter.
func (c *PipelineCache) Lookup(desc PipelineDesc) (uint64, bool) {
	return 0, false
}

// Evict removes the n least recently used entries and returns their
// descriptors in eviction order.
func (c *PipelineCache) Evict(n int) ([]PipelineDesc, error) {
	return nil, errNotImplemented
}

// InvalidateShader drops every entry that references shaderHash as its vertex
// or fragment shader and returns the number of removed entries.
func (c *PipelineCache) InvalidateShader(shaderHash string) int {
	return 0
}

// Len reports the number of cached entries.
func (c *PipelineCache) Len() int {
	return 0
}

// Hits reports the number of successful lookups.
func (c *PipelineCache) Hits() int {
	return 0
}

// Misses reports the number of failed lookups.
func (c *PipelineCache) Misses() int {
	return 0
}

// Blob serialises the version, entries, handles, counters and LRU order.
func (c *PipelineCache) Blob() ([]byte, error) {
	return nil, errNotImplemented
}
