package task

import "errors"

// Errors reported by ShaderCache. Eviction failures must be distinguishable
// from bad input because the renderer logs the former as a cache-pressure
// alert and the latter as an authoring bug.
var (
	ErrDuplicateKey     = errors.New("shader cache: key already present")
	ErrKeyNotFound      = errors.New("shader cache: key not found")
	ErrSizeInvalid      = errors.New("shader cache: entry size must be positive")
	ErrTooLarge         = errors.New("shader cache: entry exceeds cache capacity")
	ErrPinnedBudget     = errors.New("shader cache: not enough unpinned entries to evict")
	ErrEvictRange       = errors.New("shader cache: evict count out of range")
	ErrCacheFull        = errors.New("shader cache: pinned entries fill the cache")
	ErrBadCapacity      = errors.New("shader cache: capacity must be positive")
	ErrCapacityMismatch = errors.New("shader cache: snapshot capacity mismatch")

	errNotImplemented = errors.New("not implemented")
)

// ShaderKey identifies one compiled shader variant. Every field participates
// in equality; Stage is compared case-insensitively via MakeKey.
type ShaderKey struct {
	Stage   string
	Source  string
	Defines string
	Target  string
}

// Entry is the bookkeeping record of one cached shader.
type Entry struct {
	Key       ShaderKey
	ByteSize  int
	CompileMs float64
	Hits      int
	Pinned    bool
}

// ShaderCache is a byte-budgeted LRU cache of compiled shader variants.
type ShaderCache struct {
	capacityBytes int
}

// NewShaderCache creates a cache whose entries may occupy at most
// capacityBytes bytes in total.
func NewShaderCache(capacityBytes int) (*ShaderCache, error) {
	return &ShaderCache{capacityBytes: capacityBytes}, nil
}

// NewShaderCacheFromSnapshot restores entries, hit counters and LRU order from
// a snapshot produced by Snapshot. The capacity must match the snapshot.
func NewShaderCacheFromSnapshot(capacityBytes int, data []byte) (*ShaderCache, error) {
	return nil, errNotImplemented
}

// MakeKey builds a ShaderKey, lower-casing the stage so that "VS" and "vs"
// address the same entry.
func (c *ShaderCache) MakeKey(stage, source, defines, target string) ShaderKey {
	return ShaderKey{}
}

// Put inserts a new entry. It evicts least-recently-used unpinned entries to
// make room and never mutates the cache when it returns an error.
func (c *ShaderCache) Put(key ShaderKey, byteSize int, compileMs float64) error {
	return errNotImplemented
}

// Get returns the entry and marks it most recently used. The returned entry
// already counts this lookup in Hits.
func (c *ShaderCache) Get(key ShaderKey) (Entry, bool) {
	return Entry{}, false
}

// Peek returns the entry without touching its LRU position or hit counter.
func (c *ShaderCache) Peek(key ShaderKey) (Entry, bool) {
	return Entry{}, false
}

// Pin protects an entry from automatic and explicit eviction.
func (c *ShaderCache) Pin(key ShaderKey) error {
	return errNotImplemented
}

// Unpin returns a pinned entry to the evictable set.
func (c *ShaderCache) Unpin(key ShaderKey) error {
	return errNotImplemented
}

// Evict removes the n least-recently-used unpinned entries and returns their
// keys in eviction order. It is all-or-nothing.
func (c *ShaderCache) Evict(n int) ([]ShaderKey, error) {
	return nil, errNotImplemented
}

// EvictionOrder lists every cached key from least to most recently used.
func (c *ShaderCache) EvictionOrder() []ShaderKey {
	return nil
}

// Len reports the number of cached entries.
func (c *ShaderCache) Len() int {
	return 0
}

// SizeBytes reports the total byte size of all cached entries.
func (c *ShaderCache) SizeBytes() int {
	return 0
}

// HitRate reports hits divided by hits plus misses, and 0 before any lookup.
func (c *ShaderCache) HitRate() float64 {
	return 0
}

// TotalCompileMs reports the summed compile cost of the cached entries.
func (c *ShaderCache) TotalCompileMs() float64 {
	return 0
}

// Snapshot serialises entries, hit counters and LRU order.
func (c *ShaderCache) Snapshot() ([]byte, error) {
	return nil, errNotImplemented
}
