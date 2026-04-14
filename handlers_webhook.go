package main

import (
	"crypto/hmac"
	"crypto/sha1"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// EMAIL INGESTION (SendGrid Inbound Parse)
// =============================================================================

// extractEmail parses an email address from formats like "Name <email>" or bare "email".
func extractEmail(from string) string {
	from = strings.TrimSpace(from)
	if idx := strings.Index(from, "<"); idx >= 0 {
		end := strings.Index(from, ">")
		if end > idx {
			return strings.TrimSpace(from[idx+1 : end])
		}
	}
	return strings.ToLower(from)
}

// validateTwilioSignature checks the X-Twilio-Signature header using HMAC-SHA1.
// See https://www.twilio.com/docs/usage/security#validating-requests
func validateTwilioSignature(r *http.Request, authToken, baseURL string) bool {
	sig := r.Header.Get("X-Twilio-Signature")
	if sig == "" {
		return false
	}

	// Build the full URL Twilio used (scheme + host + path)
	fullURL := baseURL + r.URL.Path

	// For POST, sort form params and append key=value pairs to the URL
	if err := r.ParseForm(); err != nil {
		return false
	}
	keys := make([]string, 0, len(r.PostForm))
	for k := range r.PostForm {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var buf strings.Builder
	buf.WriteString(fullURL)
	for _, k := range keys {
		buf.WriteString(k)
		buf.WriteString(r.PostForm.Get(k))
	}

	mac := hmac.New(sha1.New, []byte(authToken))
	mac.Write([]byte(buf.String()))
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(sig), []byte(expected))
}

// handleEmailWebhook handles POST /api/email — SendGrid Inbound Parse webhook.
func (s *Server) handleEmailWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Validate webhook secret (configured in SendGrid Inbound Parse URL as ?secret=...)
	if s.config.WebhookBaseURL != "" {
		secret := r.URL.Query().Get("secret")
		if secret == "" || secret != s.config.TwilioAuthToken {
			log.Printf("[email] invalid webhook secret — rejecting request")
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
			return
		}
	}

	// Always return 200 so SendGrid doesn't retry
	defer func() {
		if w.Header().Get("Content-Type") == "" {
			w.WriteHeader(http.StatusOK)
		}
	}()

	// Limit request body to 10 MB
	r.Body = http.MaxBytesReader(w, r.Body, 10*1024*1024)

	// Parse multipart form (SendGrid sends multipart)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		// Fall back to regular form parse
		if err := r.ParseForm(); err != nil {
			log.Printf("[email] error parsing form: %v", err)
			writeJSON(w, http.StatusOK, map[string]bool{"success": true})
			return
		}
	}

	from := extractEmail(r.FormValue("from"))
	text := strings.TrimSpace(r.FormValue("text"))

	log.Printf("[email] incoming from %s: %q", from, text)

	if from == "" {
		log.Printf("[email] no from address")
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Look up user by email
	var userID string
	var userPhoneNull sql.NullString
	err := s.db.QueryRow(
		"SELECT id, phone_number FROM users WHERE LOWER(email) = LOWER(?)",
		from,
	).Scan(&userID, &userPhoneNull)
	_ = userPhoneNull // phone may be null for non-phone users
	if err != nil {
		log.Printf("[email] unknown sender %s, ignoring", from)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Collect all image attachments (max 5 MB each)
	const maxImageSize = maxImageBytes
	var images []string
	if r.MultipartForm != nil && r.MultipartForm.File != nil {
		for _, headers := range r.MultipartForm.File {
			for _, h := range headers {
				if h.Size > maxImageSize {
					log.Printf("[email] skipping oversized image %s (%d bytes)", h.Filename, h.Size)
					continue
				}
				f, err := h.Open()
				if err != nil {
					continue
				}
				data, err := io.ReadAll(f)
				f.Close()
				if err != nil || len(data) == 0 {
					continue
				}
				ct := h.Header.Get("Content-Type")
				if strings.HasPrefix(ct, "image/") {
					images = append(images, fmt.Sprintf("data:%s;base64,%s", ct, base64.StdEncoding.EncodeToString(data)))
					if len(images) >= 5 {
						break
					}
				}
			}
		}
	}

	if text == "" && len(images) == 0 {
		log.Printf("[email] empty email from %s, ignoring", from)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Parse items with Claude — all images in one shot
	parsed, err := s.cachedParseItems(s.config.AnthropicAPIKey, text, images)
	if err != nil {
		log.Printf("[email] error parsing items for %s: %v", from, err)
		writeJSON(w, http.StatusOK, map[string]bool{"success": true})
		return
	}

	// Add items to user's list
	added, _ := addParsedItems(s.db, userID, parsed, "email")

	log.Printf("[email] added %d items for user %s from %s", len(added), userID, from)
	writeJSON(w, http.StatusOK, map[string]bool{"success": true})
}

// =============================================================================
// SMS WEBHOOK
//
// Twilio sends an HTTP POST to /api/sms when someone texts the Twilio number.
// Because Twilio sometimes splits image+text across two rapid webhook calls,
// we buffer messages per phone number for a configurable duration before
// processing them together.
// =============================================================================

// pendingMessage holds accumulated text and image data while we wait for
// additional webhook calls from the same phone number.
type pendingMessage struct {
	text   string
	images []string
	timer  *time.Timer
}

// messageBuffer is a mutex-protected map of phone → pendingMessage.
type messageBuffer struct {
	mu             sync.Mutex
	pending        map[string]*pendingMessage
	bufferDuration time.Duration
}

func newMessageBuffer(d time.Duration) *messageBuffer {
	return &messageBuffer{
		pending:        make(map[string]*pendingMessage),
		bufferDuration: d,
	}
}

// downloadTwilioMedia fetches an image from a Twilio MediaUrl (which requires
// Basic Auth) and returns a data URI string like "data:image/jpeg;base64,...".
func downloadTwilioMedia(mediaURL, contentType, accountSID, authToken string) string {
	req, err := http.NewRequest("GET", mediaURL, nil)
	if err != nil {
		log.Printf("[sms] error creating media request: %v", err)
		return ""
	}
	req.SetBasicAuth(accountSID, authToken)

	resp, err := twilioHTTPClient.Do(req)
	if err != nil {
		log.Printf("[sms] error downloading media: %v", err)
		return ""
	}
	defer resp.Body.Close()

	// Limit download to 5MB to prevent memory exhaustion
	const maxMediaSize = maxMediaBytes
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxMediaSize+1))
	if err != nil {
		log.Printf("[sms] error reading media body: %v", err)
		return ""
	}
	if len(data) > maxMediaSize {
		log.Printf("[sms] media too large (%d bytes), skipping", len(data))
		return ""
	}

	encoded := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", contentType, encoded)
}

// handleSMSWebhook handles POST /api/sms — the Twilio inbound SMS webhook.
// It buffers incoming messages per phone number and responds with empty TwiML.
func (s *Server) handleSMSWebhook(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}

	// Validate Twilio signature (skip in dev when no base URL configured)
	if s.config.WebhookBaseURL != "" {
		if !validateTwilioSignature(r, s.config.TwilioAuthToken, s.config.WebhookBaseURL) {
			log.Printf("[sms] invalid Twilio signature — rejecting request")
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Forbidden"})
			return
		}
	}

	if err := r.ParseForm(); err != nil {
		log.Printf("[sms] error parsing form: %v", err)
		writeBadRequest(w, "Bad request")
		return
	}

	from := r.FormValue("From")
	body := strings.TrimSpace(r.FormValue("Body"))
	numMedia, _ := strconv.Atoi(r.FormValue("NumMedia"))

	log.Printf("[sms] incoming from %s: numMedia=%d", maskPhone(from), numMedia)

	// CTIA / A2P 10DLC compliance keywords. Detect BEFORE buffering or any
	// Claude parsing — keyword messages must never be added to the user's
	// shopping list, never go to AI, and must respond synchronously with
	// the branded reply Twilio's vetting bot expects.
	if kind := classifySMSKeyword(body); kind != "" {
		s.handleSMSKeyword(from, kind)
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "<Response></Response>")
		return
	}

	// Download all image attachments (cap to prevent abuse)
	if numMedia > 10 {
		numMedia = 10
	}
	var newImages []string
	for i := 0; i < numMedia; i++ {
		mediaURL := r.FormValue(fmt.Sprintf("MediaUrl%d", i))
		contentType := r.FormValue(fmt.Sprintf("MediaContentType%d", i))
		if mediaURL != "" && strings.HasPrefix(contentType, "image/") {
			img := downloadTwilioMedia(
				mediaURL, contentType,
				s.config.TwilioAccountSID, s.config.TwilioAuthToken,
			)
			if img != "" {
				newImages = append(newImages, img)
			}
		}
	}

	// Buffer the message — group rapid-fire webhooks from the same phone
	s.smsBuffer.mu.Lock()
	pm, exists := s.smsBuffer.pending[from]
	if !exists {
		pm = &pendingMessage{}
		s.smsBuffer.pending[from] = pm
	}

	// Append text (with newline separator if both old and new have content)
	if body != "" {
		if pm.text != "" {
			pm.text += "\n" + body
		} else {
			pm.text = body
		}
	}
	// Collect all images
	pm.images = append(pm.images, newImages...)

	// Reset/start the debounce timer
	if pm.timer != nil {
		pm.timer.Stop()
	}
	pm.timer = time.AfterFunc(s.smsBuffer.bufferDuration, func() {
		s.processSMSMessage(from)
	})
	s.smsBuffer.mu.Unlock()

	// Respond with empty TwiML so Twilio doesn't retry
	w.Header().Set("Content-Type", "text/xml")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, "<Response></Response>")
}

// processSMSMessage is called by the debounce timer after messages have been
// buffered. It parses items with Claude, adds them to the user's list, and
// sends an SMS reply.
func (s *Server) processSMSMessage(phone string) {
	// Extract and remove the pending message
	s.smsBuffer.mu.Lock()
	pm, exists := s.smsBuffer.pending[phone]
	if !exists {
		s.smsBuffer.mu.Unlock()
		return
	}
	text := pm.text
	images := pm.images
	delete(s.smsBuffer.pending, phone)
	s.smsBuffer.mu.Unlock()

	if text == "" && len(images) == 0 {
		log.Printf("[sms] empty message from %s, skipping", maskPhone(phone))
		return
	}

	// Normalize the phone and look up (or create) the user.
	// Twilio sends E.164 format like "+15551234567" — strip to 10 digits.
	digits := strings.TrimPrefix(phone, "+1")
	user, err := upsertUser(s.db, digits)
	if err != nil {
		log.Printf("[sms] error upserting user for %s: %v", maskPhone(phone), err)
		return
	}

	// Parse items using the same Claude function the web app uses
	parsed, err := s.cachedParseItems(s.config.AnthropicAPIKey, text, images)
	if err != nil {
		log.Printf("[sms] error parsing items for %s: %v", maskPhone(phone), err)
		s.sendSMSReply(phone, "Sorry, I had trouble understanding that. Please try again!")
		return
	}

	if len(parsed) == 0 {
		log.Printf("[sms] no items parsed for %s", maskPhone(phone))
		s.sendSMSReply(phone, "I couldn't find any grocery items in your message. Try something like \"milk, eggs, bread\".")
		return
	}

	// Add each parsed item to the user's list
	added, skippedItems := addParsedItems(s.db, user.ID, parsed, "sms")
	var addedNames []string
	for _, item := range added {
		addedNames = append(addedNames, item.Name)
	}
	var updatedNames []string
	for _, si := range skippedItems {
		updatedNames = append(updatedNames, si.Parsed.Name)
	}

	// Build reply message
	var lines []string
	if len(addedNames) > 0 {
		lines = append(lines, "Added to your list:")
		for _, n := range addedNames {
			lines = append(lines, "- "+n)
		}
	}
	if len(updatedNames) > 0 {
		lines = append(lines, "Updated quantity:")
		for _, n := range updatedNames {
			lines = append(lines, "- "+n)
		}
	}

	if len(lines) > 0 {
		s.sendSMSReply(phone, strings.Join(lines, "\n"))
	}
}

// sendSMSReply sends an SMS from the configured Twilio phone number.
func (s *Server) sendSMSReply(to, body string) {
	if s.config.TwilioPhoneNumber == "" {
		log.Printf("[sms] cannot send reply — TWILIO_PHONE_NUMBER not configured")
		return
	}
	if err := s.twilio.SendSMS(s.config.TwilioPhoneNumber, to, body); err != nil {
		log.Printf("[sms] error sending reply to %s: %v", to, err)
	}
}

// =============================================================================
// CTIA / Twilio A2P 10DLC keyword handling
// =============================================================================

// classifySMSKeyword returns "stop", "help", "start", or "" based on the body.
// Match is exact (case-insensitive, single word). Embedded usage like "I need
// help with eggs" does NOT trigger HELP — that's a list item the user wants
// added. Only an SMS body that is exactly the keyword qualifies.
//
// Keyword sets per CTIA Messaging Principles:
//
//	stop:  STOP, STOPALL, UNSUBSCRIBE, CANCEL, END, QUIT
//	help:  HELP, INFO
//	start: START, UNSTOP
//
// "YES" is intentionally NOT a START keyword. Penny uses YES/NO as conversational
// confirmation in restock nudges and swap offers; treating it as a global opt-in
// would break those flows.
func classifySMSKeyword(body string) string {
	upper := strings.ToUpper(strings.TrimSpace(body))
	switch upper {
	case "STOP", "STOPALL", "UNSUBSCRIBE", "CANCEL", "END", "QUIT":
		return "stop"
	case "HELP", "INFO":
		return "help"
	case "START", "UNSTOP":
		return "start"
	}
	return ""
}

// handleSMSKeyword routes a CTIA keyword to the appropriate side effect
// (consent state change) and sends the branded reply. Synchronous: does NOT
// buffer, does NOT go to Claude, does NOT touch the user's shopping list.
func (s *Server) handleSMSKeyword(from, kind string) {
	digits := strings.TrimPrefix(from, "+1")
	switch kind {
	case "stop":
		// Clear consent so canSendSMS() returns false. Twilio also handles
		// the carrier-level STOP at the platform; this branded reply is
		// the application-level confirmation Penny owes the user.
		if _, err := s.db.Exec(
			`UPDATE users SET sms_consent_at = NULL WHERE phone_number = ?`, digits,
		); err != nil {
			log.Printf("[sms-keyword] STOP consent clear failed for %s: %v", maskPhone(from), err)
		}
		log.Printf("[sms-keyword] STOP from %s — consent cleared", maskPhone(from))
		s.sendSMSReply(from,
			"You're unsubscribed from Penny / Groflo. You won't get any more texts. "+
				"Reply START to opt back in anytime.")

	case "help":
		log.Printf("[sms-keyword] HELP from %s", maskPhone(from))
		s.sendSMSReply(from,
			"Hi — I'm Penny, your Groflo shopping list keeper. Text me items "+
				"to add to your list. Reply STOP to opt out. Msg & data rates may apply. "+
				"Help: support@groflo.io")

	case "start":
		// Re-opt-in. Set consent timestamp to NOW with placeholder IP/UA
		// (carrier-level inbound has no client IP). Twilio carrier-level
		// START handles the actual re-subscription; this is the app's record.
		if _, err := s.db.Exec(
			`UPDATE users SET sms_consent_at = CURRENT_TIMESTAMP,
			   sms_consent_ip = 'sms-keyword',
			   sms_consent_user_agent = 'twilio-inbound'
			 WHERE phone_number = ?`, digits,
		); err != nil {
			log.Printf("[sms-keyword] START consent set failed for %s: %v", maskPhone(from), err)
		}
		log.Printf("[sms-keyword] START from %s — consent restored", maskPhone(from))
		s.sendSMSReply(from,
			"Welcome back. I'm Penny — text me anything you need on your "+
				"shopping list and I'll add it. Reply STOP anytime to opt out.")
	}
}

// =============================================================================
