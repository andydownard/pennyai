package main

// clerk_identity.go — Rally identity resolution and the local user upsert for
// Rally sign-in (Clerk satellite).
//
// Two halves, in dependency order:
//
//  1. resolveRallyIdentity — a READ-ONLY call to timpager, the authority on
//     Rally identity. pennyai never invents a user id for a Clerk sign-in:
//     users.id must equal the rally_user_id so one person is one canonical user
//     across every Rally app. If the authority is unreachable or rejects the
//     token, nothing is created. Fail closed.
//
//  2. upsertClerkUser — one transaction that binds a verified Clerk identity to
//     exactly one local users row, mapping to an existing legacy account where
//     one is identifiable. A row already bound to one Clerk identity is NEVER
//     rebound to another (the takeover guard).

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// -------- Errors --------

// errClerkAccountConflict means the local row we would have used is already
// bound to a DIFFERENT Clerk identity, or the Rally id we were told to use is
// taken by a row that is not ours. Never resolved by overwriting: the user is
// told, and nothing is minted.
var errClerkAccountConflict = errors.New("local account is bound to a different Clerk identity")

// errRallyIdentityMissing means timpager returned no rally_user_id and no local
// row matched. We cannot mint, because users.id has to BE the rally_user_id.
var errRallyIdentityMissing = errors.New("no rally_user_id from the authority and no local account to map")

// errClerkAuthorityRejected means timpager refused the end-user token (401/403).
var errClerkAuthorityRejected = errors.New("authority rejected token")

// -------- 1. timpager identity resolve --------

const (
	clerkResolvePath    = "/api/v1/identity/resolve"
	clerkResolveTimeout = 10 * time.Second
)

// rallyIdentity is timpager's answer. Found is false — not an error — when the
// authority simply has no Rally user for this Clerk identity yet.
type rallyIdentity struct {
	RallyUserID string
	ClerkUserID string
	Email       string
	Phone       string
	Found       bool
}

// clerkResolveResponse mirrors timpager's POST /api/v1/identity/resolve 200 body.
// rally_user_id, email and phone are nullable, hence the pointers.
type clerkResolveResponse struct {
	RallyUserID *string `json:"rally_user_id"`
	ClerkUserID string  `json:"clerk_user_id"`
	Email       *string `json:"email"`
	Phone       *string `json:"phone"`
	Version     string  `json:"version"`
}

// resolveRallyIdentity asks timpager who this Clerk user is in Rally.
//
// Header discipline is load-bearing. timpager's routes/identity-v1.js reads the
// SERVICE key from X-API-Key; a service key sent as Authorization: Bearer makes
// it treat the request as a legacy, body-only resolve instead. When BOTH are
// present it verifies the Bearer as the end-user Clerk session JWT (fail-closed
// 401) and its own verified `sub` overrides whatever the body claimed — which is
// exactly the property we want, so we always send both.
//
// httpClient may be nil, in which case a client with a 10s timeout is used.
func resolveRallyIdentity(
	ctx context.Context,
	cfg Config,
	httpClient *http.Client,
	clerkJWT string,
	id ClerkIdentity,
) (rallyIdentity, error) {
	if strings.TrimSpace(cfg.TimpagerURL) == "" || strings.TrimSpace(cfg.TimpagerAPIKey) == "" {
		// Fail closed: without the authority we must not mint a user.
		return rallyIdentity{}, errors.New("timpager not configured (TIMPAGER_URL and TIMPAGER_API_KEY are both required for Rally sign-in)")
	}
	if clerkJWT == "" {
		return rallyIdentity{}, errors.New("resolveRallyIdentity: the end-user Clerk JWT is required")
	}
	if id.ClerkUserID == "" {
		return rallyIdentity{}, errors.New("resolveRallyIdentity: clerk user id is required")
	}

	// Only send fields we actually have. An empty phone/email could widen
	// timpager's clerk → phone → email match order.
	body := map[string]string{"clerk_user_id": id.ClerkUserID}
	if id.Phone != "" {
		body["phone"] = id.Phone
	}
	if id.Email != "" {
		body["email"] = id.Email
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return rallyIdentity{}, fmt.Errorf("encoding resolve request: %w", err)
	}

	client := httpClient
	if client == nil {
		client = &http.Client{Timeout: clerkResolveTimeout}
	}
	url := strings.TrimSuffix(cfg.TimpagerURL, "/") + clerkResolvePath

	// One retry, and only for a transport failure or a 5xx. A 4xx is a decision,
	// not a hiccup — retrying a 429 would only dig the limiter deeper.
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return rallyIdentity{}, fmt.Errorf("building resolve request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-API-Key", cfg.TimpagerAPIKey)
		req.Header.Set("Authorization", "Bearer "+clerkJWT)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("calling the Rally identity authority: %w", err)
			continue
		}

		switch {
		case resp.StatusCode == http.StatusOK:
			raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
			resp.Body.Close()
			if readErr != nil {
				lastErr = fmt.Errorf("reading resolve response: %w", readErr)
				continue
			}
			var decoded clerkResolveResponse
			if err := json.Unmarshal(raw, &decoded); err != nil {
				// A malformed body is never an empty success.
				return rallyIdentity{}, fmt.Errorf("decoding resolve response: %w", err)
			}
			return shapeRallyIdentity(decoded, id), nil

		case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
			resp.Body.Close()
			return rallyIdentity{}, fmt.Errorf("%w (status %d)", errClerkAuthorityRejected, resp.StatusCode)

		case resp.StatusCode == http.StatusTooManyRequests:
			resp.Body.Close()
			return rallyIdentity{}, fmt.Errorf("the Rally identity authority is rate limiting this server (status 429)")

		case resp.StatusCode >= 500:
			resp.Body.Close()
			lastErr = fmt.Errorf("the Rally identity authority returned status %d", resp.StatusCode)
			continue

		default:
			resp.Body.Close()
			return rallyIdentity{}, fmt.Errorf("unexpected status %d from the Rally identity authority", resp.StatusCode)
		}
	}
	return rallyIdentity{}, lastErr
}

// shapeRallyIdentity normalizes the authority's answer into the forms pennyai
// stores: phone as the bare 10 digits users.phone holds, email lower-cased.
func shapeRallyIdentity(decoded clerkResolveResponse, id ClerkIdentity) rallyIdentity {
	out := rallyIdentity{ClerkUserID: decoded.ClerkUserID}
	if out.ClerkUserID == "" {
		out.ClerkUserID = id.ClerkUserID
	}
	if decoded.RallyUserID != nil && *decoded.RallyUserID != "" {
		out.RallyUserID = *decoded.RallyUserID
		out.Found = true
	}
	if decoded.Email != nil {
		out.Email = strings.ToLower(strings.TrimSpace(*decoded.Email))
	}
	if decoded.Phone != nil {
		// normalizePhone yields +1XXXXXXXXXX; users.phone stores the 10 digits.
		if e164, ok := normalizePhone(*decoded.Phone); ok {
			out.Phone = strings.TrimPrefix(e164, "+1")
		}
	}
	return out
}

// -------- 2. local upsert --------

// upsertClerkUser binds a verified Clerk identity to exactly one local users
// row, in ONE transaction. Returns the user, and mappedBy naming which signal
// matched an existing legacy account ("" for a brand-new row, "clerk" for a
// repeat sign-in).
//
// Match order is load-bearing and pinned by tests:
//
//	a. users.clerk_user_id == sub      — repeat sign-in, idempotent
//	b. auth_methods google provider_id == google_sub   (strongest: a provider id)
//	c. users.phone == rally phone                      (verified by Twilio today)
//	d. email                                           (LAST — weakest signal)
//
// Email is accepted at all only because the authority has verified it, and it
// stays last. Soft-deleted rows (name LIKE '[merged into user%') are never
// candidates.
func upsertClerkUser(db *PGDB, id ClerkIdentity, rally rallyIdentity) (User, string, error) {
	if id.ClerkUserID == "" {
		return User{}, "", errors.New("upsertClerkUser: clerk user id is required")
	}

	user, mappedBy, err := upsertClerkUserOnce(db, id, rally)
	if errors.Is(err, errClerkAccountConflict) {
		// A concurrent first login for the same sub can lose the race on the
		// partial unique index; step (a) will now find the row the winner wrote,
		// so retry once. For a genuine takeover attempt the retry conflicts the
		// same way and the original error stands — correct, just not free, which
		// is acceptable on a path that is rate-limited to 20 per 10 minutes.
		if retried, retriedBy, retryErr := upsertClerkUserOnce(db, id, rally); retryErr == nil {
			return retried, retriedBy, nil
		}
	}
	return user, mappedBy, err
}

func upsertClerkUserOnce(db *PGDB, id ClerkIdentity, rally rallyIdentity) (User, string, error) {
	tx, err := db.Begin()
	if err != nil {
		return User{}, "", fmt.Errorf("starting transaction: %w", err)
	}
	defer tx.Rollback()

	// (a) Repeat sign-in: this Clerk identity is already bound locally.
	if existingID, ok := clerkLookupUserID(tx, "SELECT id FROM users WHERE clerk_user_id = ?", id.ClerkUserID); ok {
		user, err := clerkFinalizeUser(tx, existingID, id.ClerkUserID)
		if err != nil {
			return User{}, "", err
		}
		if err := tx.Commit(); err != nil {
			return User{}, "", fmt.Errorf("committing: %w", err)
		}
		return user, "clerk", nil
	}

	// (b)–(d) Look for a legacy account to adopt, strongest signal first.
	candidateID, mappedBy := clerkFindLegacyCandidate(tx, id, rally)

	if candidateID != "" {
		// Atomic conditional UPDATE, not check-then-act: the WHERE clause is the
		// guard. Binding only succeeds while the row holds no Clerk identity.
		res, err := tx.Exec(
			"UPDATE users SET clerk_user_id = ? WHERE id = ? AND clerk_user_id IS NULL",
			id.ClerkUserID, candidateID,
		)
		if err != nil {
			return User{}, "", fmt.Errorf("binding clerk identity to user %s: %w", candidateID, err)
		}
		affected, _ := res.RowsAffected()
		if affected == 0 {
			// Someone bound this row first. If it was us, fine; if it was another
			// Clerk identity, this is a takeover attempt and we refuse.
			bound, _ := clerkLookupUserID(tx, "SELECT COALESCE(clerk_user_id,'') FROM users WHERE id = ?", candidateID)
			if bound != id.ClerkUserID {
				log.Printf("[clerk-auth] refusing to rebind user %s: already linked to another Rally sign-in", candidateID)
				return User{}, "", errClerkAccountConflict
			}
		}
		clerkWarnOnIDMismatch(candidateID, rally)

		user, err := clerkFinalizeUser(tx, candidateID, id.ClerkUserID)
		if err != nil {
			return User{}, "", err
		}
		if err := tx.Commit(); err != nil {
			return User{}, "", fmt.Errorf("committing: %w", err)
		}
		return user, mappedBy, nil
	}

	// No legacy account. Mint — but only with the authority's id.
	if !rally.Found || rally.RallyUserID == "" {
		return User{}, "", errRallyIdentityMissing
	}

	// users.id IS the rally_user_id. ON CONFLICT DO NOTHING (via the repo's
	// INSERT OR IGNORE, converted by pgcompat) so a concurrent first login
	// does not abort this transaction.
	if _, err := tx.Exec(
		"INSERT OR IGNORE INTO users (id, phone, email, name, clerk_user_id) VALUES (?, NULL, ?, '', ?)",
		rally.RallyUserID, rally.Email, id.ClerkUserID,
	); err != nil {
		// A unique violation on idx_users_clerk_user_id means a concurrent first
		// login won. Surface as a conflict; upsertClerkUser retries step (a) once.
		return User{}, "", fmt.Errorf("%w: %v", errClerkAccountConflict, err)
	}

	newID, ok := clerkLookupUserID(tx, "SELECT id FROM users WHERE clerk_user_id = ?", id.ClerkUserID)
	if !ok {
		// The id existed already but is bound to something else.
		log.Printf("[clerk-auth] rally_user_id already present locally under a different sign-in")
		return User{}, "", errClerkAccountConflict
	}

	// Set the phone separately and only when no other row holds it, so the
	// UNIQUE(phone) constraint can never abort the mint.
	if rally.Phone != "" {
		if _, err := tx.Exec(
			"UPDATE users SET phone = ? WHERE id = ? AND phone IS NULL AND NOT EXISTS (SELECT 1 FROM users WHERE phone = ?)",
			rally.Phone, newID, rally.Phone,
		); err != nil {
			log.Printf("[clerk-auth] could not set phone on new user %s: %v", newID, err)
		}
	}

	if _, err := createSoloHouseholdTx(tx, newID); err != nil {
		return User{}, "", err
	}

	user, err := clerkFinalizeUser(tx, newID, id.ClerkUserID)
	if err != nil {
		return User{}, "", err
	}
	if err := tx.Commit(); err != nil {
		return User{}, "", fmt.Errorf("committing: %w", err)
	}
	return user, "", nil
}

// clerkFindLegacyCandidate applies the mapping order. Returns ("", "") when no
// existing account can be identified.
func clerkFindLegacyCandidate(tx dbExecer, id ClerkIdentity, rally rallyIdentity) (string, string) {
	// (b) google_sub — a provider id, the strongest signal. Only present when
	// the primary instance adds the custom session-token claim.
	if id.GoogleSub != "" {
		if uid, ok := clerkLookupUserID(tx,
			"SELECT user_id FROM auth_methods WHERE provider = 'google' AND provider_id = ?", id.GoogleSub); ok {
			return uid, "google_sub"
		}
	}

	// (c) phone — pennyai's own join key for phone users.
	if rally.Phone != "" {
		if uid, ok := clerkLookupUserID(tx, "SELECT id FROM users WHERE phone = ?", rally.Phone); ok {
			return uid, "phone"
		}
	}

	// (d) email — last, and only because the authority verified it. Soft-deleted
	// rows are excluded.
	if rally.Email != "" {
		if uid, ok := clerkLookupUserID(tx,
			"SELECT id FROM users WHERE LOWER(email) = LOWER(?) AND email != '' AND name NOT LIKE '[merged into user%'",
			rally.Email); ok {
			return uid, "email"
		}
		// The join to users is not decoration: without it a soft-deleted account
		// whose email lives in auth_methods would still be adopted here, right
		// after the users lookup above deliberately excluded it.
		if uid, ok := clerkLookupUserID(tx,
			`SELECT am.user_id FROM auth_methods am
			 JOIN users u ON u.id = am.user_id
			 WHERE am.provider IN ('google','email')
			   AND LOWER(am.provider_id) = LOWER(?)
			   AND u.name NOT LIKE '[merged into user%'`,
			rally.Email); ok {
			return uid, "email"
		}
	}

	return "", ""
}

// clerkFinalizeUser records the auth method, tracks the login, and reads the
// user back. Shared by every branch so all of them behave identically.
func clerkFinalizeUser(tx dbExecer, userID, clerkUserID string) (User, error) {
	// auth_methods already has UNIQUE(provider, provider_id), so this is safe to
	// repeat — the dedup guard is the constraint, not a prior SELECT.
	tx.Exec("INSERT OR IGNORE INTO auth_methods (user_id, provider, provider_id) VALUES (?, 'clerk', ?)", userID, clerkUserID)

	if _, err := tx.Exec("UPDATE users SET last_active = CURRENT_TIMESTAMP, login_count = login_count + 1 WHERE id = ?", userID); err != nil {
		log.Printf("[clerk-auth] could not update login count for user %s: %v", userID, err)
	}

	var user User
	var householdID sql.NullInt64
	var phone, name, email sql.NullString
	err := tx.QueryRow(
		"SELECT id, COALESCE(phone,''), COALESCE(name,''), COALESCE(email,''), household_id, created_at FROM users WHERE id = ?",
		userID,
	).Scan(&user.ID, &phone, &name, &email, &householdID, &user.CreatedAt)
	if err != nil {
		return User{}, fmt.Errorf("fetching user after clerk upsert: %w", err)
	}
	user.PhoneNumber = phone.String
	user.Name = name.String
	user.Email = email.String
	if householdID.Valid {
		user.HouseholdID = int(householdID.Int64)
	}

	// A legacy row may have no household (created before households existed).
	if !householdID.Valid {
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

// clerkWarnOnIDMismatch logs when a matched legacy row keeps a locally minted
// users.id that differs from its rally_user_id. We deliberately do NOT re-key:
// users.id is referenced by foreign keys across every table, so re-keying is a
// separate migration decision (see the plan's Open Items). No PII is logged.
func clerkWarnOnIDMismatch(localID string, rally rallyIdentity) {
	if rally.Found && rally.RallyUserID != "" && rally.RallyUserID != localID {
		log.Printf("[clerk-reconcile] local users.id %s differs from rally_user_id %s — not re-keyed (FK fan-out; separate migration)", localID, rally.RallyUserID)
	}
}
