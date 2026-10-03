package main

// handlers_clerk.go — Rally sign-in endpoint and per-request resolver.
//
// POST /auth/clerk/exchange trades a verified Clerk session JWT for a normal
// pennyai session cookie. It is the ONLY place in the Clerk path that writes,
// and the only place that talks to timpager. Everything else is read-only.
//
// The endpoint ships DARK: with CLERK_ENABLED unset it answers 404, so a
// probe cannot even tell the route exists.

import (
	"errors"
	"log"
	"net/http"
	"time"
)

const (
	// clerkExchangeWindow / clerkExchangeMax: 20 exchanges per 10 minutes per IP.
	// Server-side and not bypassable from the client. Generous enough for a
	// household on one NAT, tight enough that the endpoint cannot be used to
	// probe the authority.
	clerkExchangeWindow = 10 * time.Minute
	clerkExchangeMax    = 20
)

// handleClerkExchange handles POST /auth/clerk/exchange.
//
// Order is deliberate: method → kill switch → rate limit → token → authority →
// local upsert → session. The rate limit sits ahead of verification so an
// attacker cannot make us do crypto or call timpager for free.
func (s *Server) handleClerkExchange(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Dark by default: reveal nothing while the feature is off.
	if !clerkIsEnabled() {
		writeNotFound(w, "Not found")
		return
	}

	if checkRateLimit(&s.clerkExchangeIPLimiter, getClientIP(r), clerkExchangeWindow, clerkExchangeMax) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{
			"error": "Too many sign-in attempts. Wait a few minutes and try again.",
		})
		return
	}

	// Bearer only. A __session cookie on the request is ignored even if present.
	token, err := extractClerkToken(r)
	if err != nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Sign-in could not be verified"})
		return
	}

	identity, err := VerifyClerkToken(r.Context(), token)
	if err != nil {
		// Reason class only — never the token.
		log.Printf("[clerk-auth] exchange rejected an unverified token (%T)", err)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Sign-in could not be verified"})
		return
	}

	// timpager is the authority on Rally identity. No answer, no account.
	rally, err := resolveRallyIdentity(r.Context(), s.config, nil, token, *identity)
	if err != nil {
		if errors.Is(err, errClerkAuthorityRejected) {
			log.Printf("[clerk-auth] authority rejected sub=%s", identity.ClerkUserID)
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "Sign-in could not be verified"})
			return
		}
		log.Printf("[clerk-auth] identity authority unavailable for sub=%s: %v", identity.ClerkUserID, err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error": "Sign-in is briefly unavailable. Try again in a moment.",
		})
		return
	}

	user, mappedBy, err := upsertClerkUser(s.db, *identity, rally)
	if err != nil {
		switch {
		case errors.Is(err, errClerkAccountConflict):
			writeConflict(w, "This account is already connected to a different Rally sign-in.")
		case errors.Is(err, errRallyIdentityMissing):
			writeConflict(w, "Your Rally account is still being set up. Try again in a moment.")
		default:
			dbError(w, err, "Failed to sign in")
		}
		return
	}

	sessionToken, err := s.sessions.CreateSession(user.ID, user.PhoneNumber, "clerk")
	if err != nil {
		dbError(w, err, "Failed to create session")
		return
	}
	setSessionCookie(w, s.config, sessionToken)

	log.Printf("[clerk-auth] user %s signed in via Rally (mapped_by=%s)", user.ID, mappedBy)

	resp := map[string]interface{}{
		"success": true,
		"user_id": user.ID,
	}
	// Same shape family as /auth/google: only say "merged" when an existing
	// local account was adopted, not on a repeat sign-in or a fresh account.
	if mappedBy != "" && mappedBy != "clerk" {
		resp["account_merged"] = true
		resp["merge_message"] = "Welcome back! I connected your Rally sign-in to your existing Groflo account."
	}
	writeJSON(w, http.StatusOK, resp)
}

// resolveClerkSession maps a verified Clerk identity to an existing local user.
// LOOKUP ONLY: no provisioning, no timpager call, no writes. A Clerk user with
// no local row is simply anonymous until they complete an exchange.
func (s *Server) resolveClerkSession(id ClerkIdentity) (*Session, error) {
	if id.ClerkUserID == "" {
		return nil, nil
	}
	var userID, phone string
	err := s.db.QueryRow(
		"SELECT id, COALESCE(phone,'') FROM users WHERE clerk_user_id = ?",
		id.ClerkUserID,
	).Scan(&userID, &phone)
	if err != nil {
		// No row yet is not an error — the request is anonymous.
		return nil, nil
	}
	return &Session{
		UserID:      userID,
		PhoneNumber: phone,
		AuthMethod:  "clerk",
		CreatedAt:   time.Now(),
	}, nil
}
