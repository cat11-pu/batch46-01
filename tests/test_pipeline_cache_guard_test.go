package task

import (
	"errors"
	"fmt"
	"sync"
	"testing"

	taskpkg "gsb/batch46-game-e-03"
)

func mustCache(t *testing.T, version string, maxEntries int) *taskpkg.PipelineCache {
	t.Helper()
	c, err := taskpkg.NewPipelineCache(version, maxEntries)
	if err != nil {
		t.Fatalf("NewPipelineCache(%q, %d): %v", version, maxEntries, err)
	}
	return c
}

// baseDesc is the opaque forward pipeline used by the default scene pass.
func baseDesc(renderPass string) taskpkg.PipelineDesc {
	return taskpkg.PipelineDesc{
		VertexShader:   "vs-basic",
		FragmentShader: "fs-basic",
		VertexLayout:   "vtx-pos",
		RenderPass:     renderPass,
		Subpass:        0,
		BlendState:     "opaque",
		DepthState:     "less",
	}
}

func registerMain(t *testing.T, c *taskpkg.PipelineCache) {
	t.Helper()
	if _, err := c.RegisterRenderPass("main", 2); err != nil {
		t.Fatalf("RegisterRenderPass(main, 2): %v", err)
	}
}

// A registered descriptor is stored once and looked up by its handle.
func TestInsertThenLookupHit(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	if _, err := c.RegisterRenderPass("", 2); !errors.Is(err, taskpkg.ErrInvalidDesc) {
		t.Fatalf("register with an empty name error = %v, want ErrInvalidDesc", err)
	}
	if _, err := c.RegisterRenderPass("main", 0); !errors.Is(err, taskpkg.ErrInvalidDesc) {
		t.Fatalf("register with zero attachments error = %v, want ErrInvalidDesc", err)
	}
	registerMain(t, c)
	desc := baseDesc("main")
	if err := c.Insert(desc, 42); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	handle, ok := c.Lookup(desc)
	if !ok {
		t.Fatalf("Lookup missed a descriptor that was just inserted")
	}
	if handle != 42 {
		t.Fatalf("Lookup handle = %d, want 42", handle)
	}
	if c.Hits() != 1 || c.Misses() != 0 || c.Len() != 1 {
		t.Fatalf("counters = hits %d misses %d len %d, want 1/0/1", c.Hits(), c.Misses(), c.Len())
	}
}

// A missing descriptor reports false and increments only the miss counter.
func TestLookupMissCounted(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	handle, ok := c.Lookup(baseDesc("main"))
	if ok {
		t.Fatalf("Lookup returned handle %d for an empty cache", handle)
	}
	if c.Misses() != 1 || c.Hits() != 0 {
		t.Fatalf("counters = hits %d misses %d, want 0/1", c.Hits(), c.Misses())
	}
}

// Every string field of the descriptor is mandatory.
func TestInsertRejectsIncompleteDesc(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	noVertex := baseDesc("main")
	noVertex.VertexShader = ""
	if err := c.Insert(noVertex, 1); !errors.Is(err, taskpkg.ErrInvalidDesc) {
		t.Fatalf("Insert without a vertex shader error = %v, want ErrInvalidDesc", err)
	}
	noBlend := baseDesc("main")
	noBlend.BlendState = ""
	if err := c.Insert(noBlend, 2); !errors.Is(err, taskpkg.ErrInvalidDesc) {
		t.Fatalf("Insert without a blend state error = %v, want ErrInvalidDesc", err)
	}
	if c.Len() != 0 || c.Hits() != 0 || c.Misses() != 0 {
		t.Fatalf("cache mutated by invalid descriptors: len=%d", c.Len())
	}
}

// Inserting against a render pass that was never registered is refused.
func TestInsertRequiresRegisteredRenderPass(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	if err := c.Insert(baseDesc("unregistered"), 5); !errors.Is(err, taskpkg.ErrUnknownRenderPass) {
		t.Fatalf("Insert for an unknown render pass error = %v, want ErrUnknownRenderPass", err)
	}
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0", c.Len())
	}
}

// A subpass index must stay below the attachment count of its render pass.
func TestSubpassOutOfRange(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	if _, err := c.RegisterRenderPass("gbuffer", 2); err != nil {
		t.Fatalf("RegisterRenderPass(gbuffer, 2): %v", err)
	}
	bad := baseDesc("gbuffer")
	bad.Subpass = 2
	if err := c.Insert(bad, 10); !errors.Is(err, taskpkg.ErrSubpassOutOfRange) {
		t.Fatalf("Insert with subpass 2 of 2 error = %v, want ErrSubpassOutOfRange", err)
	}
	good := baseDesc("gbuffer")
	good.Subpass = 1
	if err := c.Insert(good, 11); err != nil {
		t.Fatalf("Insert with subpass 1 of 2: %v", err)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
}

// The same descriptor cannot be stored twice; the first handle wins.
func TestDuplicateDescRejected(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	desc := baseDesc("main")
	if err := c.Insert(desc, 7); err != nil {
		t.Fatalf("first Insert: %v", err)
	}
	if err := c.Insert(desc, 8); !errors.Is(err, taskpkg.ErrDuplicateEntry) {
		t.Fatalf("duplicate Insert error = %v, want ErrDuplicateEntry", err)
	}
	handle, ok := c.Lookup(desc)
	if !ok || handle != 7 {
		t.Fatalf("Lookup = (%d, %v), want (7, true)", handle, ok)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
}

// Two distinct descriptors cannot share one driver handle.
func TestDuplicateHandleRejected(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	first := baseDesc("main")
	second := baseDesc("main")
	second.VertexLayout = "vtx-inst"
	if err := c.Insert(first, 100); err != nil {
		t.Fatalf("first Insert: %v", err)
	}
	if err := c.Insert(second, 100); !errors.Is(err, taskpkg.ErrDuplicateHandle) {
		t.Fatalf("Insert with a reused handle error = %v, want ErrDuplicateHandle", err)
	}
	if _, ok := c.Lookup(second); ok {
		t.Fatalf("descriptor with a duplicate handle was stored anyway")
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
}

// The configured entry limit is a hard cap.
func TestCacheFullAtLimit(t *testing.T) {
	c := mustCache(t, "engine-1", 2)
	registerMain(t, c)
	for i := 0; i < 2; i++ {
		desc := baseDesc("main")
		desc.VertexLayout = fmt.Sprintf("vtx-%d", i)
		if err := c.Insert(desc, uint64(100+i)); err != nil {
			t.Fatalf("Insert #%d: %v", i, err)
		}
	}
	third := baseDesc("main")
	third.VertexLayout = "vtx-third"
	if err := c.Insert(third, 200); !errors.Is(err, taskpkg.ErrCacheFull) {
		t.Fatalf("Insert past the limit error = %v, want ErrCacheFull", err)
	}
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
}

// Registering the same render pass with the same attachment count is a no-op.
func TestReRegisterSameCompatKeepsEntries(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	for i := 0; i < 2; i++ {
		desc := baseDesc("main")
		desc.DepthState = fmt.Sprintf("depth-%d", i)
		if err := c.Insert(desc, uint64(10+i)); err != nil {
			t.Fatalf("Insert #%d: %v", i, err)
		}
	}
	dropped, err := c.RegisterRenderPass("main", 2)
	if err != nil {
		t.Fatalf("re-register: %v", err)
	}
	if dropped != 0 {
		t.Fatalf("re-register dropped %d entries, want 0", dropped)
	}
	if c.Len() != 2 {
		t.Fatalf("Len = %d, want 2", c.Len())
	}
}

// Changing the attachment count invalidates exactly that render pass.
func TestCompatChangeInvalidatesRenderPass(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	if _, err := c.RegisterRenderPass("shadow", 1); err != nil {
		t.Fatalf("RegisterRenderPass(shadow, 1): %v", err)
	}
	for i := 0; i < 2; i++ {
		desc := baseDesc("main")
		desc.FragmentShader = fmt.Sprintf("fs-main-%d", i)
		if err := c.Insert(desc, uint64(20+i)); err != nil {
			t.Fatalf("Insert main #%d: %v", i, err)
		}
	}
	shadow := baseDesc("shadow")
	if err := c.Insert(shadow, 30); err != nil {
		t.Fatalf("Insert shadow: %v", err)
	}
	dropped, err := c.RegisterRenderPass("main", 3)
	if err != nil {
		t.Fatalf("re-register with new compat: %v", err)
	}
	if dropped != 2 {
		t.Fatalf("compat change dropped %d entries, want 2", dropped)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
	if _, ok := c.Lookup(shadow); !ok {
		t.Fatalf("shadow entry disappeared after main compat change")
	}
}

// Shader hot reload invalidates every entry that references the new hash.
func TestInvalidateShaderHash(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	entries := []struct {
		vs, fs string
		handle uint64
	}{{"vs-a", "fs-x", 1}, {"vs-a", "fs-y", 2}, {"vs-b", "fs-a", 3}}
	for _, e := range entries {
		desc := baseDesc("main")
		desc.VertexShader = e.vs
		desc.FragmentShader = e.fs
		if err := c.Insert(desc, e.handle); err != nil {
			t.Fatalf("Insert %s/%s: %v", e.vs, e.fs, err)
		}
	}
	if got := c.InvalidateShader("vs-a"); got != 2 {
		t.Fatalf("InvalidateShader(vs-a) = %d, want 2", got)
	}
	if c.Len() != 1 {
		t.Fatalf("Len after invalidating vs-a = %d, want 1", c.Len())
	}
	if got := c.InvalidateShader("fs-a"); got != 1 {
		t.Fatalf("InvalidateShader(fs-a) = %d, want 1", got)
	}
	if got := c.InvalidateShader("vs-unknown"); got != 0 {
		t.Fatalf("InvalidateShader(unknown) = %d, want 0", got)
	}
	if c.Len() != 0 {
		t.Fatalf("Len = %d, want 0", c.Len())
	}
}

// An evict request outside [1, Len] is rejected without touching the cache.
func TestEvictRange(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	if err := c.Insert(baseDesc("main"), 1); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if _, err := c.Evict(0); !errors.Is(err, taskpkg.ErrEvictRange) {
		t.Fatalf("Evict(0) error = %v, want ErrEvictRange", err)
	}
	if _, err := c.Evict(2); !errors.Is(err, taskpkg.ErrEvictRange) {
		t.Fatalf("Evict(2) error = %v, want ErrEvictRange", err)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
}

// Eviction follows the least-recently-used order, and a hit refreshes it.
func TestEvictLRUOrder(t *testing.T) {
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	descs := make([]taskpkg.PipelineDesc, 3)
	for i := range descs {
		descs[i] = baseDesc("main")
		descs[i].VertexLayout = fmt.Sprintf("vtx-%d", i)
		if err := c.Insert(descs[i], uint64(50+i)); err != nil {
			t.Fatalf("Insert #%d: %v", i, err)
		}
	}
	if _, ok := c.Lookup(descs[0]); !ok {
		t.Fatalf("Lookup of the first descriptor missed")
	}
	dropped, err := c.Evict(2)
	if err != nil {
		t.Fatalf("Evict(2): %v", err)
	}
	if len(dropped) != 2 || dropped[0] != descs[1] || dropped[1] != descs[2] {
		t.Fatalf("Evict(2) = %v, want descriptors 1 and 2", dropped)
	}
	if c.Len() != 1 {
		t.Fatalf("Len = %d, want 1", c.Len())
	}
	if _, ok := c.Lookup(descs[0]); !ok {
		t.Fatalf("recently used descriptor was evicted")
	}
	if c.Hits() != 2 {
		t.Fatalf("Hits = %d, want 2", c.Hits())
	}
}

// A blob keeps entries, handles, counters and LRU order, and rejects a blob
// written by another engine version.
func TestBlobRoundTrip(t *testing.T) {
	c := mustCache(t, "engine-2", 8)
	registerMain(t, c)
	first := baseDesc("main")
	second := baseDesc("main")
	second.VertexLayout = "vtx-inst"
	if err := c.Insert(first, 601); err != nil {
		t.Fatalf("Insert first: %v", err)
	}
	if err := c.Insert(second, 602); err != nil {
		t.Fatalf("Insert second: %v", err)
	}
	if _, ok := c.Lookup(first); !ok {
		t.Fatalf("Lookup before snapshot missed")
	}
	blob, err := c.Blob()
	if err != nil {
		t.Fatalf("Blob: %v", err)
	}
	restored, err := taskpkg.NewPipelineCacheFromBlob("engine-2", 8, blob)
	if err != nil {
		t.Fatalf("NewPipelineCacheFromBlob: %v", err)
	}
	if restored.Len() != 2 || restored.Hits() != 1 || restored.Misses() != 0 {
		t.Fatalf("restored counters = len %d hits %d misses %d, want 2/1/0", restored.Len(), restored.Hits(), restored.Misses())
	}
	if handle, ok := restored.Lookup(first); !ok || handle != 601 {
		t.Fatalf("restored Lookup = (%d, %v), want (601, true)", handle, ok)
	}
	dropped, err := restored.Evict(1)
	if err != nil {
		t.Fatalf("restored Evict(1): %v", err)
	}
	if len(dropped) != 1 || dropped[0] != second {
		t.Fatalf("restored Evict(1) = %v, want the second descriptor", dropped)
	}
	if _, err := taskpkg.NewPipelineCacheFromBlob("engine-3", 8, blob); !errors.Is(err, taskpkg.ErrVersionMismatch) {
		t.Fatalf("restore with another version error = %v, want ErrVersionMismatch", err)
	}
}

// Concurrent lookups of a cached pipeline all succeed and are counted.
func TestConcurrentLookups(t *testing.T) {
	const workers = 50
	c := mustCache(t, "engine-1", 8)
	registerMain(t, c)
	desc := baseDesc("main")
	if err := c.Insert(desc, 42); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	handles := make([]uint64, workers)
	oks := make([]bool, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(slot int) {
			defer wg.Done()
			handles[slot], oks[slot] = c.Lookup(desc)
		}(i)
	}
	wg.Wait()
	for i := 0; i < workers; i++ {
		if !oks[i] || handles[i] != 42 {
			t.Fatalf("worker %d Lookup = (%d, %v), want (42, true)", i, handles[i], oks[i])
		}
	}
	if c.Hits() != workers || c.Misses() != 0 {
		t.Fatalf("counters = hits %d misses %d, want %d/0", c.Hits(), c.Misses(), workers)
	}
}
