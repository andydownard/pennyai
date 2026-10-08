package main

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// checkRateLimit backs every sliding-window limiter. Concurrent callers must
// never be allowed past the limit.
func TestCheckRateLimit_ConcurrentCallersNeverExceedLimit(t *testing.T) {
	for round := 0; round < 50; round++ {
		var store sync.Map
		var allowed int64
		var wg sync.WaitGroup
		start := make(chan struct{})
		for i := 0; i < 64; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				<-start
				if !checkRateLimit(&store, "k", time.Hour, 10) {
					atomic.AddInt64(&allowed, 1)
				}
			}()
		}
		close(start)
		wg.Wait()
		if allowed != 10 {
			t.Fatalf("round %d: %d callers allowed, limit 10", round, allowed)
		}
	}
}
