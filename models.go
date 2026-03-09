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
	BlueCartAPIKey      string
	ReportingDBPath     string
	FeedbackPhones      []string // phone numbers allowed to submit feedback
	GitHubPAT           string   // personal access token for creating issues
	GitHubRepo          string   // "owner/repo" format
}

// FeedbackRequest represents a founder feedback item destined for the roadmap.
type FeedbackRequest struct {
	ID             int    `json:"id"`
	UserID         int    `json:"user_id"`
	Body           string `json:"body"`
	Type           string `json:"type"`
	Urgency        string `json:"urgency"`
	Status         string `json:"status"`
	AdminNotes     string `json:"admin_notes,omitempty"`
	GitHubIssueURL string `json:"github_issue_url,omitempty"`
	CreatedAt      string `json:"created_at"`
	UpdatedAt      string `json:"updated_at,omitempty"`
	UserName       string `json:"user_name,omitempty"`
	UserPhone      string `json:"user_phone,omitempty"`
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
	Merged      bool // transient: true when resolveOrCreateUser linked a new auth method to existing account
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
	ClaimedByID      *int   `json:"claimed_by_id"`
	ClaimedByName    string `json:"claimed_by_name"`
	SwapSavingsCents int    `json:"swap_savings_cents"`
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
	PendingSwap          *PendingSwap          `json:"pending_swap,omitempty"`
	SwapCreditsCents     int                   `json:"swap_credits_cents,omitempty"`
	DuplicateWarnings    []DuplicateWarning    `json:"duplicate_warnings,omitempty"`
}

// DuplicateWarning indicates another household member recently added the same item.
type DuplicateWarning struct {
	ItemName  string `json:"item_name"`
	AddedBy   string `json:"added_by"`   // first name of the other member
	HoursAgo int    `json:"hours_ago"`   // how many hours ago they added it
}

// BrandDisambiguation represents a brand choice the user needs to make.
type BrandDisambiguation struct {
	ItemID          int      `json:"item_id"`
	ProductCategory string   `json:"product_category"`
	CurrentBrand    string   `json:"current_brand"`
	Alternatives    []string `json:"alternatives"`
	AutoAssigned    bool     `json:"auto_assigned,omitempty"`
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
	reportDB     *sql.DB // read-only clone for admin analytics (nil if unavailable)
	sessions     *SessionStore
	twilio       TwilioClient
	parseItems   func(apiKey, text string, images []string) ([]ParsedItem, error)
	parseCommand func(apiKey, text string, items []ListItem, name, learningContext string) (*ParsedCommand, error)
	parseReceipt func(apiKey string, images []string, currentItems []ListItem) ([]string, error)
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
	// Per-IP password reset/magic link rate limiter: IP -> []time.Time (sliding window, 5 req/15min)
	passwordResetIPLimiter sync.Map
	// Per-user earnings upload rate limiter: userID -> *rateBucket (5 uploads/hour)
	earningsUploadLimiter sync.Map

	// googleTokenInfoURL overrides the Google tokeninfo endpoint (for testing).
	// When empty, defaults to "https://oauth2.googleapis.com/tokeninfo".
	googleTokenInfoURL string
}

// BrandDeal represents an active swap deal from the brand_deals table.
type BrandDeal struct {
	ID                  int    `json:"id"`
	CompetitorProduct   string `json:"competitor_product"`
	OfferBrand          string `json:"offer_brand"`
	OfferProduct        string `json:"offer_product"`
	DiscountCents       int    `json:"discount_cents"`
	TotalBudgetCents    int    `json:"daily_budget_cents"` // JSON tag kept for backward compat (was daily_budget_cents, renamed B15)
	BudgetSpentCents    int    `json:"budget_spent_cents"`
	Status              string `json:"status"`
	StartsAt            string `json:"starts_at"`
	EndsAt              string `json:"ends_at"`
	CreatedAt           string `json:"created_at"`
	// Price comparison & why data (M1 swap card redesign)
	OfferPriceCents     int    `json:"offer_price_cents"`
	CompetitorPriceCents int   `json:"competitor_price_cents"`
	OfferSize           string `json:"offer_size"`
	CompetitorSize      string `json:"competitor_size"`
	OfferUnitPrice      string `json:"offer_unit_price"`
	CompetitorUnitPrice string `json:"competitor_unit_price"`
	UnitLabel           string `json:"unit_label"`
	WhyBullets          string `json:"why_bullets"`
	AllergenNotes       string `json:"allergen_notes"`
	// Sprint 3: swap type expansion
	DealType         string `json:"deal_type"`
	QuantityOffered  int    `json:"quantity_offered"`
	TrialProductName string `json:"trial_product_name"`
	TrialSize        string `json:"trial_size"`
	CampaignTag      string `json:"campaign_tag"`
}

// PendingSwap represents a swap offer matched to a list item.
type PendingSwap struct {
	DealID        int    `json:"deal_id"`
	ItemID        int    `json:"item_id"`
	OfferBrand    string `json:"offer_brand"`
	OfferProduct  string `json:"offer_product"`
	DiscountCents int    `json:"discount_cents"`
	// Price comparison fields (zero values = graceful degradation)
	OfferPriceCents    int    `json:"offer_price_cents"`
	OriginalPriceCents int    `json:"original_price_cents"`
	OfferSize          string `json:"offer_size"`
	OriginalSize       string `json:"original_size"`
	OfferUnitPrice     string `json:"offer_unit_price"`
	OriginalUnitPrice  string `json:"original_unit_price"`
	SavingsPercent     int    `json:"savings_percent"`
	// Why panel
	WhyBullets      []string `json:"why_bullets"`
	AllergenWarning string   `json:"allergen_warning,omitempty"`
	// Sprint 3: swap type expansion
	DealType         string `json:"deal_type"`
	QuantityOffered  int    `json:"quantity_offered,omitempty"`
	TrialProductName string `json:"trial_product_name,omitempty"`
	TrialSize        string `json:"trial_size,omitempty"`
	CampaignTag      string `json:"campaign_tag,omitempty"`
	EndsAt           string `json:"ends_at,omitempty"`
	// Store-aware pricing
	StoreName string `json:"store_name,omitempty"`
	// v1 Stretch: rich product descriptions
	ProductDescription string   `json:"product_description,omitempty"`
	KeyBenefits        []string `json:"key_benefits,omitempty"`
	BrandStory         string   `json:"brand_story,omitempty"`
}

// ProfilePhoto represents a user's avatar image stored as a BLOB.
type ProfilePhoto struct {
	UserID      int    `json:"user_id"`
	PhotoData   []byte `json:"-"`
	ContentType string `json:"content_type"`
	UpdatedAt   string `json:"updated_at"`
}

// EarningTransaction represents a single swap rebate for the earnings screen.
type EarningTransaction struct {
	ID                int    `json:"id"`
	OfferBrand        string `json:"offer_brand"`
	OfferProduct      string `json:"offer_product"`
	CompetitorProduct string `json:"competitor_product"`
	OriginalItemName  string `json:"original_item_name"`
	AmountCents       int    `json:"amount_cents"`
	Status            string `json:"status"`
	CreatedAt         string `json:"created_at"`
	AttachmentPath    string `json:"attachment_path"`
}

// EarningsResponse is the JSON response for GET /api/earnings.
type EarningsResponse struct {
	Transactions  []EarningTransaction `json:"transactions"`
	TotalCents    int                  `json:"total_cents"`
	PendingCents  int                  `json:"pending_cents"`
	PaidCents     int                  `json:"paid_cents"`
	DisplayAmount string               `json:"display_amount"`
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

// =============================================================================
// REQUEST TYPES — named structs for JSON request decoding
// =============================================================================

// --- Auth requests ---

type GoogleAuthRequest struct {
	Credential string `json:"credential"`
}

type GoogleTokenInfo struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"`
	Name          string `json:"name"`
	Aud           string `json:"aud"`
	Picture       string `json:"picture"`
}

type EmailRegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type EmailLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type EmailOnlyRequest struct {
	Email string `json:"email"`
}

type PasswordResetRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

type PhoneRequest struct {
	PhoneNumber string `json:"phone_number"`
}

type VerifyCodeRequest struct {
	PhoneNumber string `json:"phone_number"`
	Code        string `json:"code"`
}

type AuthLinkRequest struct {
	Provider string `json:"provider"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	Code     string `json:"code"`
}

// --- List requests ---

type ParseAndAddRequest struct {
	Text           string   `json:"text"`
	Image          string   `json:"image"`
	Images         []string `json:"images"`
	SwapDealID     int      `json:"swap_deal_id"`
	SwapItemID     int      `json:"swap_item_id"`
	IsReceipt      bool     `json:"is_receipt"`
	DeclineReason  string   `json:"decline_reason"`
	TimeToActionMs int      `json:"time_to_action_ms"`
	WhyTapped      bool     `json:"why_tapped"`
}

type ListAddRequest struct {
	Name       string `json:"name"`
	Department string `json:"department"`
	Quantity   int    `json:"quantity"`
}

type ListEditRequest struct {
	Name       *string `json:"name"`
	Brand      *string `json:"brand"`
	Department *string `json:"department"`
	Quantity   *int    `json:"quantity"`
	Size       *string `json:"size"`
	Notes      *string `json:"notes"`
}

type BrandResolveRequest struct {
	ItemID          int    `json:"item_id"`
	Brand           string `json:"brand"`
	ProductCategory string `json:"product_category"`
}

// --- Feedback requests ---

type FeedbackSubmitRequest struct {
	Body    string `json:"body"`
	Type    string `json:"type"`
	Urgency string `json:"urgency"`
}

type FeedbackTriageRequest struct {
	Status     *string `json:"status"`
	AdminNotes *string `json:"admin_notes"`
}

// --- Saved list requests ---

type SavedListCreateRequest struct {
	Name  string `json:"name"`
	Items []struct {
		Name       string `json:"name"`
		Brand      string `json:"brand"`
		Department string `json:"department"`
		Quantity   int    `json:"quantity"`
		Size       string `json:"size"`
		Notes      string `json:"notes"`
	} `json:"items"`
}

type SavedListMergeRequest struct {
	Items []struct {
		Name       string `json:"name"`
		Brand      string `json:"brand"`
		Department string `json:"department"`
		Quantity   int    `json:"quantity"`
		Size       string `json:"size"`
		Notes      string `json:"notes"`
	} `json:"items"`
}
