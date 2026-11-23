package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-e-02"
)

func mustCache(t *testing.T, capacity int) *taskpkg.ShaderCache {
	t.Helper()
	c, err := taskpkg.NewShaderCache(capacity)
	if err != nil {
		t.Fatalf("NewShaderCache(%d): %v", capacity, err)
	}
	return c
}

// variant builds a vertex-shader key for the desktop target.
func variant(c *taskpkg.ShaderCache, source string) taskpkg.ShaderKey {
	return c.MakeKey("vs", source, "", "spirv-1.3")
}

// A stored variant is readable with its byte size, compile cost and one hit.
func TestPutAndGetRoundTrip(t *testing.T) {
	c := mustCache(t, 4096)
	key := variant(c, "src-bloom")
	if err := c.Put(key, 1024, 8.5); err != nil {
		t.Fatalf("Put: %v", err)
	}
	entry, ok := c.Get(key)
	if !ok {
		t.Fatalf("Get missed a variant that was just stored")
	}
	if entry.ByteSize != 1024 || entry.CompileMs != 8.5 {
		t.Fatalf("entry = %+v, want 1024 bytes and 8.5 ms", entry)
	}
	if entry.Hits != 1 {
		t.Fatalf("entry Hits = %d, want 1 after one Get", entry.Hits)
	}
	if c.SizeBytes() != 1024 || c.Len() != 1 {
		t.Fatalf("SizeBytes/Len = %d/%d, want 1024/1", c.SizeBytes(), c.Len())
	}
}

// Storing the same variant twice is rejected and keeps the original record.
func TestDuplicatePutRejected(t *testing.T) {
	c := mustCache(t, 4096)
	key := variant(c, "src-bloom")
	if err := c.Put(key, 1024, 8.5); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	if err := c.Put(key, 2048, 12); !errors.Is(err, taskpkg.ErrDuplicateKey) {
		t.Fatalf("duplicate Put error = %v, want ErrDuplicateKey", err)
	}
	entry, ok := c.Peek(key)
	if !ok {
		t.Fatalf("original variant disappeared after a rejected Put")
	}
	if entry.ByteSize != 1024 || entry.CompileMs != 8.5 {
		t.Fatalf("entry mutated by a rejected Put: %+v", entry)
	}
	if c.SizeBytes() != 1024 || c.Len() != 1 {
		t.Fatalf("SizeBytes/Len = %d/%d, want 1024/1", c.SizeBytes(), c.Len())
	}
}

// Zero and negative sizes are invalid; a variant bigger than the whole cache
// is rejected instead of triggering eviction.
func TestInvalidSizesRejected(t *testing.T) {
	c := mustCache(t, 1024)
	if err := c.Put(variant(c, "src-zero"), 0, 1); !errors.Is(err, taskpkg.ErrSizeInvalid) {
		t.Fatalf("zero-size Put error = %v, want ErrSizeInvalid", err)
	}
	if err := c.Put(variant(c, "src-negative"), -64, 1); !errors.Is(err, taskpkg.ErrSizeInvalid) {
		t.Fatalf("negative-size Put error = %v, want ErrSizeInvalid", err)
	}
	if err := c.Put(variant(c, "src-huge"), 2048, 1); !errors.Is(err, taskpkg.ErrTooLarge) {
		t.Fatalf("oversized Put error = %v, want ErrTooLarge", err)
	}
	if c.Len() != 0 || c.SizeBytes() != 0 {
		t.Fatalf("cache mutated by rejected Puts: len=%d size=%d", c.Len(), c.SizeBytes())
	}
}

// Filling a full cache evicts exactly the least recently used variant.
func TestAutoEvictsLRU(t *testing.T) {
	c := mustCache(t, 300)
	a, b, d := variant(c, "src-a"), variant(c, "src-b"), variant(c, "src-d")
	for _, key := range []taskpkg.ShaderKey{a, b, variant(c, "src-c")} {
		if err := c.Put(key, 100, 1); err != nil {
			t.Fatalf("Put %v: %v", key.Source, err)
		}
	}
	if _, ok := c.Get(a); !ok {
		t.Fatalf("Get(a) missed before eviction")
	}
	if err := c.Put(d, 100, 1); err != nil {
		t.Fatalf("Put over a full cache: %v", err)
	}
	if _, ok := c.Peek(b); ok {
		t.Fatalf("least recently used variant b survived")
	}
	for _, key := range []taskpkg.ShaderKey{a, d, variant(c, "src-c")} {
		if _, ok := c.Peek(key); !ok {
			t.Fatalf("variant %q was evicted unexpectedly", key.Source)
		}
	}
	if c.SizeBytes() != 300 || c.Len() != 3 {
		t.Fatalf("SizeBytes/Len = %d/%d, want 300/3", c.SizeBytes(), c.Len())
	}
}

// A pinned variant is never chosen by automatic eviction; when pinned entries
// occupy the whole cache, the Put fails without changing anything.
func TestPinnedSurvivesAutoEvict(t *testing.T) {
	c := mustCache(t, 200)
	a, b, held := variant(c, "src-a"), variant(c, "src-b"), variant(c, "src-held")
	if err := c.Put(a, 100, 1); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := c.Pin(a); err != nil {
		t.Fatalf("Pin a: %v", err)
	}
	if err := c.Put(b, 100, 1); err != nil {
		t.Fatalf("Put b: %v", err)
	}
	if err := c.Pin(b); err != nil {
		t.Fatalf("Pin b: %v", err)
	}
	if err := c.Put(held, 100, 1); !errors.Is(err, taskpkg.ErrCacheFull) {
		t.Fatalf("Put with a pinned cache error = %v, want ErrCacheFull", err)
	}
	if _, ok := c.Peek(held); ok {
		t.Fatalf("rejected variant was inserted anyway")
	}
	for _, key := range []taskpkg.ShaderKey{a, b} {
		if _, ok := c.Peek(key); !ok {
			t.Fatalf("pinned variant %q was evicted", key.Source)
		}
	}
	if c.SizeBytes() != 200 || c.Len() != 2 {
		t.Fatalf("SizeBytes/Len = %d/%d, want 200/2", c.SizeBytes(), c.Len())
	}
}

// Pinning or unpinning an unknown variant reports a missing key.
func TestPinUnknownRejected(t *testing.T) {
	c := mustCache(t, 1024)
	missing := variant(c, "src-missing")
	if err := c.Pin(missing); !errors.Is(err, taskpkg.ErrKeyNotFound) {
		t.Fatalf("Pin unknown error = %v, want ErrKeyNotFound", err)
	}
	if err := c.Unpin(missing); !errors.Is(err, taskpkg.ErrKeyNotFound) {
		t.Fatalf("Unpin unknown error = %v, want ErrKeyNotFound", err)
	}
}

// Unpinning a variant makes it the next eviction candidate again.
func TestUnpinMakesEvictable(t *testing.T) {
	c := mustCache(t, 100)
	a, d := variant(c, "src-a"), variant(c, "src-d")
	if err := c.Put(a, 100, 1); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := c.Pin(a); err != nil {
		t.Fatalf("Pin a: %v", err)
	}
	if err := c.Put(d, 100, 1); !errors.Is(err, taskpkg.ErrCacheFull) {
		t.Fatalf("Put with a pinned cache error = %v, want ErrCacheFull", err)
	}
	if err := c.Unpin(a); err != nil {
		t.Fatalf("Unpin a: %v", err)
	}
	if err := c.Put(d, 100, 1); err != nil {
		t.Fatalf("Put after unpin: %v", err)
	}
	if _, ok := c.Peek(a); ok {
		t.Fatalf("unpinned and least recently used variant survived")
	}
	if _, ok := c.Peek(d); !ok {
		t.Fatalf("new variant missing after a successful Put")
	}
	if c.SizeBytes() != 100 || c.Len() != 1 {
		t.Fatalf("SizeBytes/Len = %d/%d, want 100/1", c.SizeBytes(), c.Len())
	}
}

// An evict request outside [1, Len] is rejected and keeps the cache intact.
func TestEvictCountOutOfRange(t *testing.T) {
	c := mustCache(t, 100)
	if err := c.Put(variant(c, "src-only"), 100, 1); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, err := c.Evict(0); !errors.Is(err, taskpkg.ErrEvictRange) {
		t.Fatalf("Evict(0) error = %v, want ErrEvictRange", err)
	}
	if _, err := c.Evict(2); !errors.Is(err, taskpkg.ErrEvictRange) {
		t.Fatalf("Evict(2) error = %v, want ErrEvictRange", err)
	}
	if c.Len() != 1 || c.SizeBytes() != 100 {
		t.Fatalf("cache mutated by a rejected Evict: len=%d size=%d", c.Len(), c.SizeBytes())
	}
}

// An explicit eviction that cannot be satisfied by unpinned entries evicts
// nothing at all.
func TestEvictIsAllOrNothing(t *testing.T) {
	c := mustCache(t, 400)
	a, b, d := variant(c, "src-a"), variant(c, "src-b"), variant(c, "src-d")
	if err := c.Put(a, 100, 1); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := c.Pin(a); err != nil {
		t.Fatalf("Pin a: %v", err)
	}
	for _, key := range []taskpkg.ShaderKey{b, d} {
		if err := c.Put(key, 100, 1); err != nil {
			t.Fatalf("Put %q: %v", key.Source, err)
		}
	}
	if _, err := c.Evict(3); !errors.Is(err, taskpkg.ErrPinnedBudget) {
		t.Fatalf("Evict(3) with one pinned entry error = %v, want ErrPinnedBudget", err)
	}
	if c.Len() != 3 || c.SizeBytes() != 300 {
		t.Fatalf("cache mutated by a failed Evict: len=%d size=%d", c.Len(), c.SizeBytes())
	}
	keys, err := c.Evict(2)
	if err != nil {
		t.Fatalf("Evict(2): %v", err)
	}
	if len(keys) != 2 || keys[0] != b || keys[1] != d {
		t.Fatalf("Evict(2) = %v, want [%q %q]", keys, b.Source, d.Source)
	}
	if c.Len() != 1 {
		t.Fatalf("Len after Evict(2) = %d, want 1", c.Len())
	}
}

// Eviction takes variants in least-recently-used order, and Get refreshes a
// variant's position while Peek does not.
func TestEvictOrderFollowsUsage(t *testing.T) {
	c := mustCache(t, 400)
	a, b, d := variant(c, "src-a"), variant(c, "src-b"), variant(c, "src-d")
	for _, key := range []taskpkg.ShaderKey{a, b, d} {
		if err := c.Put(key, 100, 1); err != nil {
			t.Fatalf("Put %q: %v", key.Source, err)
		}
	}
	if _, ok := c.Get(a); !ok {
		t.Fatalf("Get a missed")
	}
	if _, ok := c.Peek(b); !ok {
		t.Fatalf("Peek b missed")
	}
	order := c.EvictionOrder()
	if len(order) != 3 || order[0] != b || order[1] != d || order[2] != a {
		t.Fatalf("EvictionOrder = %v, want b d a", order)
	}
	keys, err := c.Evict(1)
	if err != nil {
		t.Fatalf("Evict(1): %v", err)
	}
	if len(keys) != 1 || keys[0] != b {
		t.Fatalf("Evict(1) = %v, want b", keys)
	}
}

// Peek inspects an entry without counting a hit, so repeated probing of the
// cache does not distort the hit rate.
func TestPeekDoesNotCountHits(t *testing.T) {
	c := mustCache(t, 1024)
	a := variant(c, "src-a")
	if err := c.Put(a, 128, 2); err != nil {
		t.Fatalf("Put: %v", err)
	}
	for i := 0; i < 3; i++ {
		entry, ok := c.Peek(a)
		if !ok {
			t.Fatalf("Peek %d missed", i)
		}
		if entry.Hits != 0 {
			t.Fatalf("Peek incremented Hits to %d", entry.Hits)
		}
	}
	if got := c.HitRate(); got != 0 {
		t.Fatalf("HitRate after peeks = %v, want 0", got)
	}
	if _, ok := c.Get(a); !ok {
		t.Fatalf("Get after peeks missed")
	}
	entry, _ := c.Peek(a)
	if entry.Hits != 1 {
		t.Fatalf("entry Hits = %d, want 1 after one Get", entry.Hits)
	}
	if got := c.HitRate(); got != 1 {
		t.Fatalf("HitRate = %v, want 1", got)
	}
}

// Hit rate counts every lookup and entries keep their per-variant hit count.
func TestHitRateAccounting(t *testing.T) {
	c := mustCache(t, 1024)
	a := variant(c, "src-a")
	if err := c.Put(a, 128, 2); err != nil {
		t.Fatalf("Put: %v", err)
	}
	if _, ok := c.Get(a); !ok {
		t.Fatalf("first Get(a) missed")
	}
	if _, ok := c.Get(variant(c, "src-b")); ok {
		t.Fatalf("Get of an uncached variant hit")
	}
	if _, ok := c.Get(a); !ok {
		t.Fatalf("second Get(a) missed")
	}
	if _, ok := c.Get(variant(c, "src-c")); ok {
		t.Fatalf("Get of a second uncached variant hit")
	}
	if got := c.HitRate(); got != 0.5 {
		t.Fatalf("HitRate = %v, want 0.5", got)
	}
	entry, ok := c.Peek(a)
	if !ok || entry.Hits != 2 {
		t.Fatalf("entry = %+v (ok=%v), want Hits 2", entry, ok)
	}
}

// The stage is normalised to lower case while defines stay part of identity.
func TestKeyNormalisationAndDefines(t *testing.T) {
	c := mustCache(t, 1024)
	upper := c.MakeKey("VS", "src-a", "", "spirv-1.3")
	lower := c.MakeKey("vs", "src-a", "", "spirv-1.3")
	if upper != lower {
		t.Fatalf("MakeKey is case sensitive on stage: %+v vs %+v", upper, lower)
	}
	defined := c.MakeKey("vs", "src-a", "QUALITY=HIGH", "spirv-1.3")
	if upper == defined {
		t.Fatalf("keys with different defines compare equal: %+v", upper)
	}
	if err := c.Put(upper, 64, 1); err != nil {
		t.Fatalf("Put upper: %v", err)
	}
	if err := c.Put(lower, 64, 1); !errors.Is(err, taskpkg.ErrDuplicateKey) {
		t.Fatalf("Put after stage normalisation error = %v, want ErrDuplicateKey", err)
	}
	if err := c.Put(defined, 64, 2); err != nil {
		t.Fatalf("Put with different defines: %v", err)
	}
	if _, ok := c.Peek(defined); !ok {
		t.Fatalf("variant with its own defines is missing")
	}
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
}

// A snapshot keeps entries, compile cost, hits and LRU order, and refuses a
// cache whose capacity differs from the snapshot.
func TestSnapshotRestoresLRUAndHits(t *testing.T) {
	c := mustCache(t, 512)
	a := variant(c, "src-a")
	if err := c.Put(a, 100, 2); err != nil {
		t.Fatalf("Put a: %v", err)
	}
	if err := c.Put(variant(c, "src-b"), 100, 4); err != nil {
		t.Fatalf("Put b: %v", err)
	}
	if _, ok := c.Get(a); !ok {
		t.Fatalf("Get a missed")
	}
	blob, err := c.Snapshot()
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	restored, err := taskpkg.NewShaderCacheFromSnapshot(512, blob)
	if err != nil {
		t.Fatalf("NewShaderCacheFromSnapshot: %v", err)
	}
	if restored.Len() != 2 || restored.SizeBytes() != 200 {
		t.Fatalf("restored len/size = %d/%d, want 2/200", restored.Len(), restored.SizeBytes())
	}
	if got := restored.TotalCompileMs(); got != 6 {
		t.Fatalf("restored TotalCompileMs = %v, want 6", got)
	}
	if got := restored.HitRate(); got != 1 {
		t.Fatalf("restored HitRate = %v, want 1", got)
	}
	order := restored.EvictionOrder()
	if len(order) != 2 || order[0].Source != "src-b" || order[1].Source != "src-a" {
		t.Fatalf("restored EvictionOrder = %v, want b before a", order)
	}
	d, ok := restored.Get(a)
	if !ok || d.Hits != 2 {
		t.Fatalf("restored entry = %+v (ok=%v), want Hits 2", d, ok)
	}
	if _, err := taskpkg.NewShaderCacheFromSnapshot(1024, blob); !errors.Is(err, taskpkg.ErrCapacityMismatch) {
		t.Fatalf("restore with another capacity error = %v, want ErrCapacityMismatch", err)
	}
}

// Concurrent inserts of distinct variants are all accepted and their lookups
// are counted exactly.
func TestConcurrentPutAndGet(t *testing.T) {
	const workers = 50
	c := mustCache(t, 8000)
	errs := make([]error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			key := c.MakeKey("fs", fmt.Sprintf("src-%02d", slot), "", "spirv-1.3")
			if err := c.Put(key, 64, 1); err != nil {
				errs[slot] = err
				return
			}
			if _, ok := c.Get(key); !ok {
				errs[slot] = fmt.Errorf("Get missed %s", key.Source)
			}
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("worker %d: %v", i, err)
		}
	}
	if c.Len() != workers {
		t.Fatalf("Len = %d, want %d", c.Len(), workers)
	}
	if c.SizeBytes() != workers*64 {
		t.Fatalf("SizeBytes = %d, want %d", c.SizeBytes(), workers*64)
	}
	if got := c.TotalCompileMs(); got != workers {
		t.Fatalf("TotalCompileMs = %v, want %d", got, workers)
	}
	if got := c.HitRate(); got != 1 {
		t.Fatalf("HitRate = %v, want 1", got)
	}
}
