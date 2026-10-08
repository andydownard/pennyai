package main

import (
	"sync"
	"testing"
	"time"
)

// Regression: the first INSERT through a fresh PGDB loads the catalog of
// integer-id tables. That load used to borrow a second pooled connection while
// the caller already held one inside a transaction, so a burst of concurrent
// first INSERTs that together held every connection in the pool deadlocked
// forever (the Clerk concurrent-first-sign-in test hung for the whole timeout).
func TestPostgres_FirstInsertBurstDoesNotDeadlockThePool(t *testing.T) {
	raw, _ := openTestDB(t)
	const conns = 4
	raw.SetMaxOpenConns(conns)
	fresh := WrapPG(raw) // catalog not loaded yet, as on a freshly started process

	done := make(chan error, conns)
	var wg sync.WaitGroup
	ready := make(chan struct{})
	for i := 0; i < conns; i++ {
		tx, err := fresh.Begin() // every connection in the pool is now in a transaction
		if err != nil {
			t.Fatalf("begin: %v", err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer tx.Rollback()
			<-ready
			res, err := tx.Exec("INSERT INTO households DEFAULT VALUES")
			if err == nil {
				var id int64
				if id, err = res.LastInsertId(); err == nil && id == 0 {
					t.Errorf("LastInsertId was 0: catalog load lost the serial table")
				}
			}
			done <- err
		}()
	}
	close(ready)

	finished := make(chan struct{})
	go func() { wg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(20 * time.Second):
		t.Fatal("deadlock: concurrent first INSERTs holding the whole pool never finished")
	}
	close(done)
	for err := range done {
		if err != nil {
			t.Errorf("insert failed: %v", err)
		}
	}
}
