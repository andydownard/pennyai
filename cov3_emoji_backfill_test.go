package main

import "testing"

func listItemEmoji(t *testing.T, server *Server, id int64) string {
	t.Helper()
	var e string
	if err := server.db.QueryRow("SELECT emoji FROM list_items WHERE id = ?", id).Scan(&e); err != nil {
		t.Fatalf("read emoji: %v", err)
	}
	return e
}

func TestBackfillItemEmojis(t *testing.T) {
	server := newTestServer(t)

	// Empty table: no-op, no panic.
	backfillItemEmojis(server.db)

	_, userA := loginUser(t, server, "5550020001")
	_, userB := loginUser(t, server, "5550020002")

	banana := createListItem(t, server.db, userA, "Bananas", "Produce")
	milk := createListItem(t, server.db, userB, "Whole Milk", "Dairy & Eggs")
	stale := createListItem(t, server.db, userA, "Apples", "Produce")
	server.db.Exec("UPDATE list_items SET emoji = ? WHERE id = ?", "X", stale)
	custom := createListItem(t, server.db, userA, "Zzzz Unknown Thing", "Other")
	server.db.Exec("UPDATE list_items SET emoji = ? WHERE id = ?", "🎁", custom)

	wantBanana := inferItemEmoji("Bananas", "Produce")
	wantMilk := inferItemEmoji("Whole Milk", "Dairy & Eggs")
	wantApple := inferItemEmoji("Apples", "Produce")
	if wantBanana == "" || wantMilk == "" || wantApple == "" {
		t.Fatalf("test premise broken: inferred emojis %q %q %q", wantBanana, wantMilk, wantApple)
	}
	noInfer := inferItemEmoji("Zzzz Unknown Thing", "Other")

	backfillItemEmojis(server.db)

	if got := listItemEmoji(t, server, banana); got != wantBanana {
		t.Errorf("banana = %q, want %q", got, wantBanana)
	}
	if got := listItemEmoji(t, server, milk); got != wantMilk {
		t.Errorf("milk (other user) = %q, want %q", got, wantMilk)
	}
	if got := listItemEmoji(t, server, stale); got != wantApple {
		t.Errorf("stale emoji not refreshed: %q, want %q", got, wantApple)
	}
	// An item the keyword list cannot place keeps whatever it had, unless the
	// department fallback speaks for it.
	if noInfer == "" {
		if got := listItemEmoji(t, server, custom); got != "🎁" {
			t.Errorf("custom emoji overwritten with blank: %q", got)
		}
	}

	// Idempotent: a second run changes nothing.
	snap := map[int64]string{banana: wantBanana, milk: wantMilk, stale: wantApple}
	backfillItemEmojis(server.db)
	for id, want := range snap {
		if got := listItemEmoji(t, server, id); got != want {
			t.Errorf("after second run item %d = %q, want %q", id, got, want)
		}
	}
	var n int
	server.db.QueryRow("SELECT COUNT(*) FROM list_items").Scan(&n)
	if n != 4 {
		t.Errorf("row count changed to %d", n)
	}
}

func TestBackfillItemEmojis_QueryErrorIsSwallowed(t *testing.T) {
	server := newTestServer(t)
	server.db.Exec("DROP TABLE list_items CASCADE") // per-test database
	backfillItemEmojis(server.db)                    // must not panic
}
