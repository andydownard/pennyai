package main

import (
	"database/sql"
	"sync"
	"time"
)

// =============================================================================
// CONFIGURATION
// =============================================================================

type Config struct {
	TwilioAccountSID    string
	TwilioAuthToken     string
	TwilioVerifyService string
	TwilioPhoneNumber   string
	AnthropicAPIKey     string
	AdminPhone          string
	LogoDevToken        string
	LogoDevSecret       string
	GooglePlacesAPIKey  string
	NPSIntervalDays     int
	Port                string
	DBPath              string
	WebhookBaseURL      string
	SecureCookies       bool
	TestMode            bool
	GoogleClientID      string
	SendGridAPIKey      string
}

// =============================================================================
// DOMAIN TYPES
// =============================================================================

// User represents a row in the users table.
type User struct {
	ID          int
	PhoneNumber string
	Name        string
	Email       string
	HouseholdID int
	CreatedAt   string
}

// Session represents an authenticated user session.
type Session struct {
	UserID      int
	PhoneNumber string // may be empty for non-phone auth
	AuthMethod  string // "phone", "google", or "email"
	CreatedAt   time.Time
}

// ListItem is the full representation of a grocery list item.
type ListItem struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	Brand         string `json:"brand"`
	Department    string `json:"department"`
	Quantity      int    `json:"quantity"`
	Size          string `json:"size"`
	Notes         string `json:"notes"`
	Source        string `json:"source"`
	Checked       bool   `json:"checked"`
	Urgent        bool   `json:"urgent"`
	CreatedAt     string `json:"created_at"`
	AddedByName   string `json:"added_by_name"`
	AddedByPhone  string `json:"added_by_phone"`
	ClaimedByID   *int   `json:"claimed_by_id"`
	ClaimedByName string `json:"claimed_by_name"`
}

// SkippedItem represents a parsed item that was not added because it already
// exists on the user's list. The frontend can offer to increase quantity.
type SkippedItem struct {
	Parsed     ParsedItem `json:"parsed"`
	ExistingID int        `json:"existing_id"`
}

// SavedListSummary is a lightweight saved list for display in chat.
type SavedListSummary struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	ItemCount int    `json:"item_count"`
}

// ParseResponse is the response format for the parse-and-add endpoint.
type ParseResponse struct {
	Items                []ListItem            `json:"items"`
	Skipped              []SkippedItem         `json:"skipped"`
	Added                int                   `json:"added"`
	Removed              []ListItem            `json:"removed,omitempty"`
	Updated              []ListItem            `json:"updated,omitempty"`
	Checked              []ListItem            `json:"checked,omitempty"`
	Action               string                `json:"action,omitempty"`
	Message              string                `json:"message,omitempty"`
	BrandDisambiguations []BrandDisambiguation `json:"brand_disambiguations,omitempty"`
	SavedLists           []SavedListSummary    `json:"saved_lists,omitempty"`
}

// BrandDisambiguation represents a brand choice the user needs to make.
type BrandDisambiguation struct {
	ItemID          int      `json:"item_id"`
	ProductCategory string   `json:"product_category"`
	CurrentBrand    string   `json:"current_brand"`
	Alternatives    []string `json:"alternatives"`
}

// cachedLogo is a brand logo held in the in-memory logo cache.
type cachedLogo struct {
	data        []byte
	contentType string
}

// rateBucket is a thread-safe sliding window rate limiter bucket.
type rateBucket struct {
	mu    sync.Mutex
	times []time.Time
}

// Server holds all runtime state for the HTTP server.
type Server struct {
	config       Config
	db           *sql.DB
	sessions     *SessionStore
	twilio       TwilioClient
	parseItems   func(apiKey, text string, images []string) ([]ParsedItem, error)
	parseCommand func(apiKey, text string, items []ListItem, name string) (*ParsedCommand, error)
	smsBuffer    *messageBuffer
	logoCache    sync.Map // domain -> *cachedLogo
	parseCache   sync.Map // normalized text -> *parseCacheEntry

	// Cached LLM-generated Penny quotes, keyed by context (login, chime, done)
	pennyQuoteMu   sync.Mutex
	pennyQuotes    map[string][]string
	pennyQuoteLast time.Time // rate limit: one LLM call per 10s max

	// OTP rate limiter: phone (E.164) -> last send time
	otpLimiter sync.Map
	// Per-IP OTP rate limiter: IP -> []time.Time (sliding window)
	otpIPLimiter sync.Map
	// Per-user parse rate limiter: userID -> *rateBucket (sliding window, 30 req/hour)
	parseLimiter sync.Map
	// Per-IP email login rate limiter: IP -> []time.Time (sliding window, 10 req/10min)
	emailLoginIPLimiter sync.Map
}

// itemData is an internal helper for collecting items to save to a saved list.
type itemData struct {
	Name       string
	Brand      string
	Department string
	Quantity   int
	Size       string
	Notes      string
}
