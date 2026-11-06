package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
)

// syncWriter is a goroutine-safe http.ResponseWriter + Flusher for SSE tests.
type syncWriter struct {
	mu      sync.Mutex
	header  http.Header
	status  int
	buf     strings.Builder
	flushes int
}

func newSyncWriter() *syncWriter { return &syncWriter{header: http.Header{}} }

func (w *syncWriter) Header() http.Header { return w.header }
func (w *syncWriter) WriteHeader(c int)   { w.mu.Lock(); w.status = c; w.mu.Unlock() }
func (w *syncWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.Write(b)
}
func (w *syncWriter) Flush() { w.mu.Lock(); w.flushes++; w.mu.Unlock() }
func (w *syncWriter) body() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

// noFlushWriter hides the Flusher interface.
type noFlushWriter struct{ http.ResponseWriter }

func countSSEClients(s *Server) int {
	n := 0
	s.sseClients.Range(func(_, _ any) bool { n++; return true })
	return n
}

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

func addSSEClient(s *Server, householdID int64, userID string) *sseClient {
	c := &sseClient{householdID: householdID, userID: userID, ch: make(chan []byte, 8)}
	s.sseClients.Store(c, struct{}{})
	return c
}

func drain(c *sseClient) []string {
	var out []string
	for {
		select {
		case m := <-c.ch:
			out = append(out, string(m))
		default:
			return out
		}
	}
}

func TestBroadcastListUpdate_HouseholdScope(t *testing.T) {
	s := &Server{}
	a := addSSEClient(s, 7, "u-a")
	b := addSSEClient(s, 7, "u-b")
	other := addSSEClient(s, 8, "u-c")

	s.broadcastListUpdate(7, "u-a")
	for name, c := range map[string]*sseClient{"a": a, "b": b} {
		got := drain(c)
		if len(got) != 1 || got[0] != "event: list-update\ndata: {}\n\n" {
			t.Errorf("client %s got %q", name, got)
		}
	}
	if got := drain(other); len(got) != 0 {
		t.Errorf("another household received %q", got)
	}
}

func TestBroadcastListUpdate_NoHouseholdMatchesByUserOnly(t *testing.T) {
	s := &Server{}
	mine := addSSEClient(s, 0, "u-a")
	theirs := addSSEClient(s, 0, "u-b")
	inHousehold := addSSEClient(s, 5, "u-a")

	s.broadcastListUpdate(0, "u-a")
	if len(drain(mine)) != 1 {
		t.Error("user's own client did not receive update")
	}
	if len(drain(theirs)) != 0 {
		t.Error("a different user received the update")
	}
	// Same user, stale household on the connection: still the caller's own tab.
	if len(drain(inHousehold)) != 1 {
		t.Error("the same user's other connection should receive the update")
	}
}

func TestBroadcastListUpdate_FullChannelDoesNotBlock(t *testing.T) {
	s := &Server{}
	c := &sseClient{householdID: 3, userID: "u", ch: make(chan []byte, 1)}
	s.sseClients.Store(c, struct{}{})
	solo := &sseClient{householdID: 0, userID: "u", ch: make(chan []byte, 1)}
	s.sseClients.Store(solo, struct{}{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 5; i++ {
			s.broadcastListUpdate(3, "u")
			s.broadcastListUpdate(0, "u")
		}
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("broadcast blocked on a full client channel")
	}
	if len(c.ch) != 1 || len(solo.ch) != 1 {
		t.Errorf("channel lengths %d, %d, want 1, 1", len(c.ch), len(solo.ch))
	}
}

func TestBroadcastCommandResult(t *testing.T) {
	s := &Server{}
	mine := addSSEClient(s, 1, "u-a")
	mineToo := addSSEClient(s, 1, "u-a") // second tab
	housemate := addSSEClient(s, 1, "u-b")

	s.broadcastCommandResult("u-a", map[string]interface{}{"reply": "Added milk", "n": 2})
	for _, c := range []*sseClient{mine, mineToo} {
		got := drain(c)
		if len(got) != 1 || !strings.HasPrefix(got[0], "event: command-result\ndata: {") || !strings.HasSuffix(got[0], "\n\n") ||
			!strings.Contains(got[0], `"reply":"Added milk"`) {
			t.Errorf("got %q", got)
		}
	}
	if got := drain(housemate); len(got) != 0 {
		t.Errorf("command result leaked to a housemate: %q", got)
	}

	// Unmarshalable result: nothing is sent.
	s.broadcastCommandResult("u-a", make(chan int))
	if got := drain(mine); len(got) != 0 {
		t.Errorf("unmarshalable result was sent: %q", got)
	}

	// Full channel does not block.
	full := &sseClient{userID: "u-z", ch: make(chan []byte)}
	s.sseClients.Store(full, struct{}{})
	done := make(chan struct{})
	go func() { s.broadcastCommandResult("u-z", "x"); close(done) }()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("blocked on full channel")
	}
}

func TestHandleListStream_AuthMethodAndNoFlusher(t *testing.T) {
	server := newTestServer(t)
	cookie, _ := loginUser(t, server, "5550001111")

	rr := doRequest(server.handleListStream, "POST", "/api/list/stream", nil, []*http.Cookie{cookie})
	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST: got %d, want 405", rr.Code)
	}
	rr = doRequest(server.handleListStream, "GET", "/api/list/stream", nil, nil)
	if rr.Code != http.StatusUnauthorized {
		t.Errorf("no session: got %d, want 401", rr.Code)
	}
	if countSSEClients(server) != 0 {
		t.Error("unauthenticated request registered an SSE client")
	}

	req := httptest.NewRequest("GET", "/api/list/stream", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	server.handleListStream(noFlushWriter{rec}, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("no flusher: got %d, want 500", rec.Code)
	}
}

func TestHandleListStream_StreamsEventsAndCleansUp(t *testing.T) {
	server := newTestServer(t)
	cookie, uid := loginUser(t, server, "5550002222")
	hid := getUserHouseholdID(server.db, uid)
	if hid == 0 {
		t.Fatal("expected solo household")
	}

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/list/stream", nil).WithContext(ctx)
	req.AddCookie(cookie)
	w := newSyncWriter()
	done := make(chan struct{})
	go func() { server.handleListStream(w, req); close(done) }()

	waitFor(t, "client registration", func() bool { return countSSEClients(server) == 1 })

	server.broadcastListUpdate(hid, uid)
	server.broadcastCommandResult(uid, map[string]string{"reply": "hi"})
	waitFor(t, "events", func() bool {
		b := w.body()
		return strings.Contains(b, "event: list-update") && strings.Contains(b, "event: command-result")
	})

	// A different user's events never reach this stream.
	server.broadcastCommandResult("someone-else", map[string]string{"reply": "secret"})
	server.broadcastListUpdate(hid+9999, "someone-else")
	time.Sleep(50 * time.Millisecond)
	if strings.Contains(w.body(), "secret") {
		t.Error("stream received another user's command result")
	}
	if n := strings.Count(w.body(), "event: list-update"); n != 1 {
		t.Errorf("list-update delivered %d times, want 1", n)
	}

	w.mu.Lock()
	gotHeaders := w.header.Clone()
	gotStatus := w.status
	w.mu.Unlock()
	if gotStatus != http.StatusOK || gotHeaders.Get("Content-Type") != "text/event-stream" || gotHeaders.Get("Cache-Control") != "no-cache" {
		t.Errorf("headers/status wrong: %d %v", gotStatus, gotHeaders)
	}

	cancel()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("handler did not return after client disconnect")
	}
	if countSSEClients(server) != 0 {
		t.Error("client not unregistered after disconnect")
	}
}

// =============================================================================
// getLearnedDepartmentOrder
// =============================================================================

func seedDeptEvent(t *testing.T, server *Server, userID, store, dept, tripDate string, at time.Time) {
	t.Helper()
	if _, err := server.db.Exec(
		"INSERT INTO department_order_events (user_id, store_name, department, checked_at, trip_date) VALUES (?, ?, ?, ?, ?)",
		userID, store, dept, at, tripDate); err != nil {
		t.Fatalf("seed: %v", err)
	}
}

func TestGetLearnedDepartmentOrder(t *testing.T) {
	server := newTestServer(t)
	_, uid := loginUser(t, server, "5550003333")
	_, other := loginUser(t, server, "5550004444")
	base := time.Now().Add(-72 * time.Hour)

	// Fewer than 3 trips: not learned, but the count is reported.
	for i, d := range []string{"2026-01-01", "2026-01-02"} {
		seedDeptEvent(t, server, uid, "Kroger", "Produce", d, base.Add(time.Duration(i)*time.Hour))
	}
	order, trips, ok := getLearnedDepartmentOrder(server.db, uid, "Kroger")
	if ok || order != nil || trips != 2 {
		t.Errorf("2 trips: order=%v trips=%d ok=%v", order, trips, ok)
	}

	// Three trips, Dairy always before Produce before Bakery.
	for i, d := range []string{"2026-01-01", "2026-01-02", "2026-01-03"} {
		at := base.Add(time.Duration(i) * 24 * time.Hour)
		seedDeptEvent(t, server, uid, "Aldi", "Dairy", d, at)
		seedDeptEvent(t, server, uid, "Aldi", "Produce", d, at.Add(time.Minute))
		seedDeptEvent(t, server, uid, "Aldi", "Bakery", d, at.Add(2*time.Minute))
		// Repeats within a trip count only once, at first sighting.
		seedDeptEvent(t, server, uid, "Aldi", "Dairy", d, at.Add(3*time.Minute))
	}
	order, trips, ok = getLearnedDepartmentOrder(server.db, uid, "Aldi")
	if !ok || trips != 3 || !reflect.DeepEqual(order, []string{"Dairy", "Produce", "Bakery"}) {
		t.Errorf("order=%v trips=%d ok=%v", order, trips, ok)
	}

	// Another user's identical store has no data; theirs never bleeds in.
	order, trips, ok = getLearnedDepartmentOrder(server.db, other, "Aldi")
	if ok || order != nil || trips != 0 {
		t.Errorf("other user: order=%v trips=%d ok=%v", order, trips, ok)
	}
	// Another store for the same user is also empty.
	if _, trips, ok = getLearnedDepartmentOrder(server.db, uid, "Costco"); ok || trips != 0 {
		t.Errorf("other store: trips=%d ok=%v", trips, ok)
	}

	// Events with a blank trip date are ignored.
	for i := 0; i < 4; i++ {
		seedDeptEvent(t, server, other, "Safeway", "Deli", "", base.Add(time.Duration(i)*time.Hour))
	}
	if _, trips, ok = getLearnedDepartmentOrder(server.db, other, "Safeway"); ok || trips != 0 {
		t.Errorf("blank trip dates: trips=%d ok=%v", trips, ok)
	}

	// Idempotent read.
	again, _, _ := getLearnedDepartmentOrder(server.db, uid, "Aldi")
	if !reflect.DeepEqual(again, []string{"Dairy", "Produce", "Bakery"}) {
		t.Errorf("second read differs: %v", again)
	}
}

func TestGetLearnedDepartmentOrder_QueryErrorIsNotLearned(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("DROP TABLE department_order_events") // test-only, per-test database
	order, trips, ok := getLearnedDepartmentOrder(server.db, "u", "s")
	if ok || order != nil || trips != 0 {
		t.Errorf("order=%v trips=%d ok=%v", order, trips, ok)
	}
}
