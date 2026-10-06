package embeddedpostgres

import (
	"bytes"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"
)

// Resource retention is checked after real process lifecycles, including all
// supervisors and pipe readers. A small allowance covers the Go test runtime.
func TestLifecycleResourceRetention(t *testing.T) {
	c := integrationConfig(t)
	pg := NewDatabase(c)
	// Warm library/runtime state before taking the baseline.
	if e := pg.Start(); e != nil {
		t.Fatal(e)
	}
	if e := pg.Close(); e != nil {
		t.Fatal(e)
	}
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	before := runtime.NumGoroutine()
	var baseline runtime.MemStats
	runtime.ReadMemStats(&baseline)
	for range 5 {
		if e := pg.Start(); e != nil {
			t.Fatal(e)
		}
		work := pg.Info().WorkDir
		if e := pg.Close(); e != nil {
			t.Fatal(e)
		}
		if _, e := os.Stat(work); !os.IsNotExist(e) {
			t.Fatal("workspace retained", e)
		}
	}
	deadline := time.Now().Add(3 * time.Second)
	for runtime.NumGoroutine() > before+2 && time.Now().Before(deadline) {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
	}
	if runtime.NumGoroutine() > before+2 {
		var stacks bytes.Buffer
		pprof.Lookup("goroutine").WriteTo(&stacks, 1)
		t.Fatalf("lifecycle goroutines retained:\n%s", &stacks)
	}
	runtime.GC()
	var after runtime.MemStats
	runtime.ReadMemStats(&after)
	if after.HeapAlloc > baseline.HeapAlloc+(16<<20) {
		t.Fatalf("unexpected retained heap: before=%d after=%d", baseline.HeapAlloc, after.HeapAlloc)
	}
}
