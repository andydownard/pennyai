package main

// clerk_identity.go — the local user upsert for Rally sign-in.
//
// A verified Clerk session token is the whole identity: users.clerk_user_id is
// the join key and pennyai mints its own users.id. Nothing here calls another
// service.
//
// upsertClerkUser binds a verified Clerk identity to exactly one local users
// row in one transaction. A row already bound to one Clerk identity is NEVER
// rebound to another (the takeover guard).

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// errClerkAccountConflict means the local row we would have used is already
// bound to a DIFFERENT Clerk identity. Never resolved by overwriting: the user
// is told, and nothing is minted.
var errClerkAccountConflict = errors.New("local account is bound to a different Clerk identity")

func clerkEmail(id ClerkIdentity) string {
	return strings.ToLower(strings.TrimSpace(id.Email))
}

// upsertClerkUser returns the user and mappedBy: "clerk" for a repeat sign-in,
// "" for a new row.
//
// Match order is load-bearing and pinned by tests:
//
//	a. users.clerk_user_id == sub — repeat sign-in, idempotent
//	b. otherwise mint a new row
//
// Email and phone are never matching signals, and no phone is ever stored.
func upsertClerkUser(db *PGDB, id ClerkIdentity) (User, string, error) {
	if id.ClerkUserID == "" {
		return User{}, "", errors.New("upsertClerkUser: clerk user id is required")
	}

	// Load the serial-table catalog on the pool BEFORE any transaction opens.
	// Done lazily inside a transaction it needs a second connection while
	// holding the first, which deadlocks the pool under concurrent first
	// sign-ins. main() does this at boot; this keeps the function safe alone.
	if !db.skipConversion {
		db.loadSerialTables()
	}

	user, mappedBy, err := upsertClerkUserOnce(db, id)
	if errors.Is(err, errClerkAccountConflict) {
		// A concurrent first sign-in for the same sub can lose the race; step (a)
		// now finds the row the winner wrote, so retry once. A genuine conflict
		// fails the same way again and the original error stands.
		if retried, retriedBy, retryErr := upsertClerkUserOnce(db, id); retryErr == nil {
			return retried, retriedBy, nil
		}
	}
	return user, mappedBy, err
}

func upsertClerkUserOnce(db *PGDB, id ClerkIdentity) (User, string, error) {
	tx, err := db.Begin()
	if err != nil {
		return User{}, "", fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	commit := func(userID, mappedBy string) (User, string, error) {
		user, err := clerkFinalizeUser(tx, userID, id)
		if err != nil {
			return User{}, "", err
		}
		if err := tx.Commit(); err != nil {
			return User{}, "", fmt.Errorf("committing: %w", err)
		}
		return user, mappedBy, nil
	}

	// (a) Repeat sign-in: this Clerk identity is already bound locally.
	if existingID, ok := clerkLookupUserID(tx, "SELECT id FROM users WHERE clerk_user_id = ?", id.ClerkUserID); ok {
		return commit(existingID, "clerk")
	}

	// (b) Mint. INSERT OR IGNORE becomes ON CONFLICT DO NOTHING, so losing a
	// race on the Clerk unique index does not abort this transaction.
	newID := uuid.New().String()
	res, err := tx.Exec(
		"INSERT OR IGNORE INTO users (id, phone, email, name, clerk_user_id) VALUES (?, NULL, ?, ?, ?)",
		newID, clerkEmail(id), strings.TrimSpace(id.Name), id.ClerkUserID,
	)
	if err != nil {
		return User{}, "", fmt.Errorf("creating user: %w", err)
	}
	if affected, _ := res.RowsAffected(); affected == 0 {
		// A concurrent first sign-in won. Let the retry find its row via (a).
		return User{}, "", errClerkAccountConflict
	}
	if _, err := createSoloHouseholdTx(tx, newID); err != nil {
		return User{}, "", err
	}
	return commit(newID, "")
}

// clerkFinalizeUser records the auth method, refreshes what the token tells us
// about the user, tracks the login, and reads the user back. Shared by every
// branch so all of them behave identically.
func clerkFinalizeUser(tx dbExecer, userID string, id ClerkIdentity) (User, error) {
	// auth_methods has UNIQUE(provider, provider_id): the constraint is the
	// dedup guard, so this is safe to repeat.
	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'clerk', ?)",
		userID, id.ClerkUserID,
	); err != nil {
		return User{}, fmt.Errorf("recording clerk auth method: %w", err)
	}

	// Email and name follow the Rally account, but a name the user chose here
	// is theirs to keep.
	if email := clerkEmail(id); email != "" {
		if _, err := tx.Exec("UPDATE users SET email = ? WHERE id = ? AND email != ?", email, userID, email); err != nil {
			return User{}, fmt.Errorf("storing email: %w", err)
		}
	}
	if name := strings.TrimSpace(id.Name); name != "" {
		if _, err := tx.Exec("UPDATE users SET name = ? WHERE id = ? AND name = ''", name, userID); err != nil {
			return User{}, fmt.Errorf("storing name: %w", err)
		}
	}

	if _, err := tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", userID); err != nil {
		return User{}, fmt.Errorf("recording login: %w", err)
	}

	var user User
	var householdID sql.NullInt64
	var name, email sql.NullString
	err := tx.QueryRow(
		"SELECT id, COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE id = ?",
		userID,
	).Scan(&user.ID, &name, &email, &householdID, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("fetching user after clerk upsert: %w", err)
	}
	user.Name = name.String
	user.Email = email.String
	if householdID.Valid {
		user.HouseholdID = int(householdID.Int64)
	} else {
		// An older account may have no household yet.
		hID, err := createSoloHouseholdTx(tx, userID)
		if err != nil {
			return User{}, err
		}
		user.HouseholdID = hID
	}
	return user, nil
}

// clerkLookupUserID runs a single-column lookup. ok is false on no rows.
func clerkLookupUserID(tx dbExecer, query string, args ...interface{}) (string, bool) {
	var value string
	if err := tx.QueryRow(query, args...).Scan(&value); err != nil {
		return "", false
	}
	if value == "" {
		return "", false
	}
	return value, true
}
