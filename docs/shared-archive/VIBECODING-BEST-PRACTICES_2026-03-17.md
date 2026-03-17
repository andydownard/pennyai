# Vibe Coding Best Practices

A reference guide for building products from Tim's timpagers using the playbook developed by Andy Downard while building app.thrift.ly (PennyAI), incorporating frameworks from Garry Tan and coaching from Tim.

---

## 1. The Garry Tan Mega Plan Review Framework

Andy adopted Garry Tan's rigorous review framework for all development work. The core philosophy: **uncompromising scrutiny to catch every landmine before it explodes. No rubber-stamping.**

### Three Operating Modes

Once a mode is selected, **commitment is absolute. No silent drift.**

#### EXPANSION
- Build the cathedral. Dream bigger, push scope up.
- Ask: "What would make this 10x better for 2x the effort?"
- Default for greenfield products.
- Goal: ambitious, but achievable in 10-18 months runway.

#### HOLD SCOPE
- Accept boundaries. Review with maximum rigor.
- Bulletproof architecture, comprehensive edge cases, airtight error handling.
- Default for fixes, refactors, and vibe coding existing products.
- Goal: ship defensible code that scales to customers.

#### SCOPE REDUCTION
- Surgical minimalism. Find the absolute minimum viable version.
- Cut everything else ruthlessly.
- Default for under-resourced teams or tight timelines.
- Goal: prove the core insight with 30% of the effort.

### The 10 Review Dimensions

Every plan review must address all 10 areas. Use this checklist when designing new features:

#### 1. Architecture
- System design, component boundaries, dependencies
- Scaling plan: what breaks at 10x, 100x, 1000x users?
- Single points of failure and mitigation
- Security boundaries: what parts talk to the internet? What's internal?
- Data residency and privacy requirements
- Example: PennyAI's multi-auth (SMS + Google OAuth + email/password) auto-merges by email to handle account linkage.

#### 2. Error & Rescue Map
Create an exhaustive table. For every method that can fail:
- What specifically goes wrong? (exception class, trigger condition)
- Is it rescued? Where?
- What rescue action is taken? (retry, fallback, degrade, fail-open, fail-closed)
- What does the user see? (error message, retry button, silent recovery)
- Is it tested?

Example from PennyAI:
| Failure | Trigger | Rescue? | Action | User Sees | Tested? |
|---------|---------|---------|--------|-----------|---------|
| SMS rate limit exceeded | 5+ codes sent in 1h from same IP | Yes | Enqueue, retry in 60s | "Try again in 1 minute" | Yes |
| Google OAuth token invalid | Token expired or revoked | Yes | Redirect to login | "Sign in again" | Yes |
| Database connection pool exhausted | 100+ concurrent requests | Yes | Queue request, retry | "Server busy, retrying..." | Yes |

#### 3. Security & Threat Model
- Attack surfaces: where is user data exposed?
- Input validation: what can be forged, injected, or overflowed?
- Authorization: can user B see user A's data?
- Secrets management: where are API keys, database passwords, encryption keys stored?
- Injection vectors: SQL injection, XSS, CSRF, open redirects
- Audit logging: what gets logged for compliance?

Example mitigations from PennyAI:
- All user input validated and sanitized before database or HTML rendering
- Session tokens: 256-bit random, HttpOnly cookies, 30-day expiry
- Database: parameterized queries only, no string concatenation
- API keys: environment variables only, never in source code
- Rate limiting: sync.Map with sliding time windows per IP and user ID
- Audit log: every login, logout, data modification tracked with timestamp and user ID

#### 4. Data Flow & Edge Cases
Trace every interaction through **happy path + three shadow paths**:
- **Happy path:** User follows the golden flow. All data is valid, no errors, no concurrency issues.
- **Nil input path:** What if a required field is missing? `null`, `undefined`, empty string?
- **Empty/zero-length path:** What if a collection is empty? Array with 0 items? Empty string?
- **Upstream error path:** What if the dependency fails? API timeout? Database error? Network partition?

For each user interaction, also map:
- Double-click (does it create duplicate?)
- Navigate-away mid-action (is partial state saved? Recovered?)
- Slow connection (does UI hang? Show loading state?)
- Stale state (is cached data refreshed?)
- Back button (does it undo? Re-load?)

Example from PennyAI's shopping list item creation:
```
Happy path: User enters "milk", clicks add → item appears in list, database updated, Penny chimes in with suggestion
Nil input path: User clicks add with empty field → "Item can't be empty" error, focus stays on input
Empty list path: User has 0 items, Penny shows greeting and encouragement
Upstream error (DB down): User enters "milk", clicks add → "Can't save right now, retrying..." → successful save after 3s
Double-click: User enters "milk", clicks add twice → only 1 item created (idempotent key in database)
Navigate-away: User enters "milk", clicks add, navigates to profile before save completes → save continues in background, user returns to see item added
Slow connection: User on 2G, enters "milk", clicks add → spinner shows 5s, then success
Stale state: User A adds "milk", user B (same household) loads list before refetch → stale list shown → auto-refresh after 10s
Back button: User adds "milk", navigates to profile, clicks back → "milk" is still there (not lost)
```

#### 5. Code Quality
- **DRY (Don't Repeat Yourself):** Flag repetition aggressively. If code appears 3+ times, extract it.
- **Naming:** Functions and variables must be named so a stranger can read the code without comments.
- **Error patterns:** Consistent error handling. No `try/catch` with no handler. No swallowed exceptions.
- **Cyclomatic complexity:** Keep functions simple. Max 10 branches per function.
- **Over/under-engineering:** Is this the simplest code that solves the problem? Or are we adding abstractions that won't be needed?

Example from PennyAI:
- `resolveOrCreateUser()` centralizes account auto-merge logic (DRY)
- `handleListItemError()` centralizes error handling for all item mutations (DRY + consistent)
- Function names are explicit: `verifyPhoneCode()`, not `verify()`
- Error types are named: `ErrRateLimited`, `ErrInvalidToken`, not generic `ErrFailed`
- No magic numbers: `const SESSION_EXPIRY = 30 * 24 * 60 * 60 * 1000; // ms`

#### 6. Tests
Coverage map for **happy path**, **failure path**, and **edge cases**. Use test pyramid shape:
- Many unit tests (70%): pure functions, hooks, utilities
- Medium integration tests (20%): feature flows, API routes, database operations
- Few E2E tests (10%): critical user journeys, cross-browser

**Hard Rule: Test-to-Code Ratio >= 1.3x**

For every line of production code, at least 1.3 lines of test code. Measure: `lines of test code / lines of production code >= 1.3`.

What counts as test code:
- Unit tests for pure functions and custom React hooks
- Component render tests (mount, show content, respond to input)
- Integration tests for multi-step flows (signup, add item, share household)
- Edge case tests (empty arrays, special characters, rate limits, missing env vars)
- Accessibility tests (ARIA attributes, keyboard navigation, screen reader compat)

What does NOT count:
- Comments in test files
- Copy-pasted setup boilerplate without modification
- Snapshot tests with no assertions (lazy testing, high false-positive rate)

#### 7. Performance
- **N+1 queries:** Every loop that hits the database is a red flag. Use batch queries instead.
- **Memory usage:** Are large datasets held in memory? Stream them or paginate.
- **Caching:** What data is read frequently? Cache it. Invalidate cache on writes.
- **Slow paths:** Background jobs for heavy work (parsing, image processing, email sending).
- **Connection pool pressure:** How many concurrent requests can the database handle?

Example from PennyAI:
- Saved lists loaded once per session, cached in React context
- Item queries paginated (50 per page) instead of loading all at once
- Brand logo resolution: 3-layer cache (brand_logos → brands → brand_owners) to avoid redundant lookups
- SMS sending and email sending offloaded to background jobs (not blocking HTTP request)
- Database connection pool: 10 connections, rate limited to prevent exhaustion

#### 8. Observability
Structured logging, metrics, tracing, alerting, dashboards, and runbooks for each failure mode.
- **Logging:** Every login, logout, error, and critical action logged with timestamp, user ID, and context.
- **Metrics:** WM0-3 dashboard showing health, engagement, and revenue.
- **Tracing:** Distributed tracing for multi-step flows (user request → backend → database → response).
- **Alerting:** PagerDuty or Slack alerts for critical errors (database down, API timeout, spike in errors).
- **Dashboards:** Looker or custom dashboards for product (DAU, NPS, retention) and ops (error rate, latency, uptime).
- **Runbooks:** Step-by-step instructions for debugging and fixing common issues.

Example from PennyAI:
- Every request logged: `{timestamp, user_id, method, path, status, latency_ms, error}`
- WM metrics in Looker: DAU, 7-day retention, NPS, referral conversion
- Error alerting: Sentry integration, email on error spike
- On-call runbook: "Database connection pool exhausted: kill slow queries, scale up, then investigate leak"

#### 9. Deployment & Rollout
- **Migration compatibility:** New deployments must not break old databases or old API clients.
- **Feature flags:** Toggle new features on/off without redeploying.
- **Rollback procedures:** Every feature must have a 5-minute rollback plan (revert last commit, run migrations backward).
- **Smoke tests:** Critical user flows tested automatically post-deploy.
- **Gradual rollout:** Release to 5% of users first, monitor for 24h, then 50%, then 100%.

Example from PennyAI:
- Database migrations: always backward-compatible (`ALTER TABLE ADD COLUMN`, never `DROP TABLE`)
- Feature flags: `stripe_payments_enabled`, `admin_dashboard_visible`, `referral_system_active`
- Rollback: every deploy tagged in Git, can roll back to previous tag in 2 minutes
- Smoke tests: signup (all 3 auth flows), add item, share household, view settings
- Gradual rollout: new UI launched at 10% for 1 week, then 100%

#### 10. Long-Term Trajectory
- **Tech debt introduced:** Are we building on sand? Will this decision haunt us in 6 months?
- **Reversibility (scale 1-5):** Can we undo this decision if we're wrong? (5=easy undo, 1=impossible)
- **Knowledge concentration risk:** Does only one person know how this works?
- **Ecosystem fit:** Does this align with our chosen stack and team skills?
- **Six-month future:** Will this code be maintainable and scalable as we grow?

Example from PennyAI:
- SQLite choice: reversible (easy to migrate to Postgres if needed), low tech debt, low concentration risk
- Penny's constitutional AI: reversible (can swap out LLM provider), high ecosystem fit (using Claude), adds strategic value
- Multi-auth system: complex but necessary, reversible (can remove auth method without breaking others), moderate concentration risk (one person understands all auth flows)

### Issue Severity Labels

- **CRITICAL GAP** — Must fix before shipping. Blocks launch. Examples: no auth, no error handling, security vulnerability.
- **WARNING** — Should fix. Creates risk if ignored. Examples: N+1 query, missing test coverage, silent error.
- **OK** — Noted but acceptable for now. Examples: performance optimization, UI polish, documentation.

### Prime Directives

1. **Zero silent failures.** Every failure mode must be visible — to the system (logs, metrics), to the team (alerts), to the user (error message or recovery).
2. **Every error gets a name.** Don't say "handle errors." Name the specific exception (e.g., `ErrRateLimited`), what triggers it, what rescues it, what the user sees, and whether it's tested.
3. **Four shadow paths for all data flows.** Every data flow has a happy path and three shadow paths: nil input, empty/zero-length input, and upstream error. Trace all four.
4. **Edge cases are mandatory.** Every user-visible interaction: double-click, navigate-away-mid-action, slow connection, stale state, back button. Map them explicitly.
5. **Observability is scope, not afterthought.** Dashboards, alerts, and runbooks are first-class deliverables, not nice-to-have.
6. **Diagrams are mandatory.** No non-trivial flow goes undiagrammed. ASCII art for data flows, state machines, pipelines, dependency graphs. Code comments should include diagrams.
7. **Everything deferred gets written down.** Vague intentions are lies. TODOS.md or it doesn't exist. Link each TODO to the issue/PR that deferred it.
8. **Optimize for six months out, not just today.** Will this decision make sense when we have 10x more code, users, and features?

---

## 2. Testing Philosophy

### Hard Rule: Test-to-Code Ratio >= 1.3x

For every line of production code, at least 1.3 lines of test code must be written. This is **non-negotiable**.

**Measure:** `lines of test code / lines of production code >= 1.3`
- **Production code:** Any `.ts`, `.tsx`, `.go` file in `src/` or `handlers/` that ships to users
- **Test code:** Any file in `__tests__/`, `*.test.ts`, `*.test.tsx`, `*.spec.ts`, `*.spec.tsx`, `*_test.go`
- **Enforcement:** Check ratio before every PR merge. If ratio drops below 1.3, the PR is blocked until tests are added.

### What Counts as Test Code

1. **Unit tests** — Pure functions and custom hooks
   ```typescript
   // Example: testing a pure validation function
   describe('parseListItem', () => {
     it('parses "2 milk" as quantity:2 item:milk', () => {
       const result = parseListItem('2 milk');
       expect(result).toEqual({ quantity: 2, item: 'milk', urgent: false });
     });
     it('returns null for empty string', () => {
       expect(parseListItem('')).toBeNull();
     });
     it('handles special characters in item name', () => {
       const result = parseListItem('1 café au lait');
       expect(result.item).toBe('café au lait');
     });
   });
   ```

2. **Component render tests** — Does it mount? Does it show the right content?
   ```typescript
   describe('PennyAvatar', () => {
     it('renders avatar with name', () => {
       render(<PennyAvatar name="Penny" />);
       expect(screen.getByText('Penny')).toBeInTheDocument();
     });
     it('shows typewriter animation on mount', async () => {
       render(<PennyAvatar value="Hello" />);
       await waitFor(() => {
         expect(screen.getByText('Hell')).toBeInTheDocument();
       });
     });
   });
   ```

3. **Integration tests** — Multi-step flows
   ```typescript
   describe('signup flow', () => {
     it('creates user and session after phone verification', async () => {
       render(<SignupFlow />);
       // User enters phone
       await userEvent.type(screen.getByLabelText('Phone'), '4155551234');
       await userEvent.click(screen.getByText('Send Code'));
       // API returns code
       // User enters code
       await userEvent.type(screen.getByLabelText('Code'), '123456');
       await userEvent.click(screen.getByText('Verify'));
       // Assert user is logged in
       expect(window.location.pathname).toBe('/home');
     });
   });
   ```

4. **Edge case tests** — Empty inputs, special characters, missing env vars, error states
   ```typescript
   describe('edge cases', () => {
     it('handles empty list gracefully', () => {
       render(<ShoppingList items={[]} />);
       expect(screen.getByText(/no items yet/i)).toBeInTheDocument();
     });
     it('handles item names with SQL injection attempts', () => {
       const item = "'; DROP TABLE items; --";
       // Ensure it's stored safely and rendered safely
       const stored = saveItem(item);
       expect(stored.name).toBe(item);
       render(<ListItem item={stored} />);
       // Should not execute SQL
     });
   });
   ```

5. **Accessibility tests** — ARIA attributes, keyboard navigation, screen reader compat
   ```typescript
   describe('accessibility', () => {
     it('submit button is keyboard accessible', async () => {
       render(<SignupForm />);
       const button = screen.getByRole('button', { name: /submit/i });
       button.focus();
       await userEvent.keyboard('{Enter}');
       expect(screen.getByText(/success/i)).toBeInTheDocument();
     });
     it('form has proper ARIA labels', () => {
       render(<SignupForm />);
       expect(screen.getByLabelText('Email')).toBeInTheDocument();
       expect(screen.getByLabelText('Password')).toBeInTheDocument();
     });
   });
   ```

6. **API/Database tests** (Go backend)
   ```go
   func TestAddListItem(t *testing.T) {
     // Happy path: valid item added to database
     item, err := addListItem(testDB, userID, "milk")
     if err != nil {
       t.Fatalf("unexpected error: %v", err)
     }
     if item.Name != "milk" {
       t.Errorf("expected item name 'milk', got %q", item.Name)
     }

     // Shadow paths
     // Nil input path
     item, err = addListItem(testDB, userID, "")
     if err == nil {
       t.Errorf("expected error for empty item, got nil")
     }

     // Upstream error path: database error
     item, err = addListItem(testDBBroken, userID, "milk")
     if err == nil {
       t.Errorf("expected error when database is broken")
     }
   }
   ```

### What Does NOT Count

- **Comments in test files** — Explanation, not assertion
- **Copy-pasted setup boilerplate** — Without modification or new test cases
- **Snapshot tests with no assertions** — Snapshots change on every code refactor, creating false failures and encouraging "just update the snapshot"
- **Commented-out test code** — It's not running

### Test Pyramid Shape

Build a pyramid, not a diamond:

```
        E2E (10%)
       /      \
      /   Integration   \
     /        (20%)       \
    /          ____        \
   /  Unit Tests (70%)  \
  /________________________\
```

- **70% unit tests** — Fast (ms), reliable, test one thing
- **20% integration tests** — Medium speed (100ms-1s), test multi-step flows
- **10% E2E tests** — Slow (seconds), catch real bugs, run against staging only

### Automated UAT Pattern (From PennyAI)

When the server is running locally, run 4 parallel test agents:

#### Agent 1: New User Onboarding
- All 3 auth flows (phone/SMS, Google OAuth, email/password)
- Session creation and persistence
- Rate limiting (send 6 codes → blocked until window resets)
- Password reset flow
- Email verification (if required)
- Auto-merge accounts by email

#### Agent 2: Core Features
- Create a test user
- All list CRUD (create, read, update, delete)
- Natural language parsing ("2 milk" → quantity:2, item:milk)
- Saved lists (create, rename, delete, load)
- Settings (update preferences, enable/disable features)
- Edge cases: empty bodies, long strings (>1000 chars), special characters, SQL injection attempts, XSS payloads
- Cross-user access (user A cannot see user B's list without sharing)

#### Agent 3: Sharing & Viral Loops
- Create 2 users
- Invite flow (generate code, copy, send, join)
- Household sharing (both users see same list)
- Leave household (list becomes private again)
- Referral tracking (which user invited which?)
- NPS survey (respond to survey, cannot respond again)
- Admin auth gating (only admins see dashboard)

#### Agent 4: Frontend & Assets
- HTML structure (correct semantic markup)
- JavaScript bundle loads and runs
- Favicons and PWA icons render
- Security headers present (CSP, X-Frame-Options, etc.)
- SPA fallback (navigating to `/unknown` returns `index.html`)
- PWA manifest loads
- Responsive layout on mobile (375px) and desktop (1920px)
- Touch targets >= 44px for mobile

### Test Flakiness Prevention

**Flaky tests are worse than no tests.** They erode trust in the test suite.

- **Avoid `setTimeout()` in tests.** Use `waitFor()` and polling instead.
- **Mock external dependencies.** Don't hit real APIs in tests (except staging integration tests).
- **Use fixed seeds for randomness.** Deterministic tests catch real bugs.
- **Isolate tests.** Each test should not depend on previous tests running.
- **Clean up after tests.** Delete test users, reset database state, clear localStorage.

---

## 3. Architecture Patterns That Work

### The Stack That Works

**Backend:** Go + HTTP server
**Frontend:** React 19 + TypeScript + Vite
**Database:** SQLite (WAL mode) for <200 DAU, Postgres for >200 DAU
**Deployment:** Single binary (Go) + static assets (React)
**Auth:** Multi-auth (phone/SMS via Twilio, Google OAuth, email/password with bcrypt)
**Payments:** Stripe Elements for web, Stripe SDK for mobile
**AI:** Claude API (safe, constitutional, family-friendly)
**Infrastructure:** Railway or Heroku for simple deployments, AWS EC2 for more control

### Why This Stack

| Component | Why | Trade-offs |
|-----------|-----|-----------|
| **Go** | Fast, simple, excellent for APIs. Single binary deployment. Type-safe. Great standard library. | Learning curve if team is JavaScript-only. |
| **React/TypeScript** | Type safety catches bugs before runtime. Vite for fast dev. Large ecosystem. | Larger JavaScript bundle than alternatives. |
| **SQLite (WAL)** | Perfect for single-server apps up to ~200 DAU. Zero ops overhead. Full ACID compliance. No migration headaches. Backup is one file. | Can't scale to multi-server without workarounds. |
| **Twilio SMS** | Higher conversion for auth than email (SMS open rate ~98%, email ~20%). Reduces spam account creation. | Phone numbers are PII, requires careful handling. SMS delivery can be slow (1-30s). |
| **Stripe Elements** | Secure, PCI-compliant. Great UX. Handles 3D Secure automatically. | Fees: 2.9% + $0.30 per transaction. |
| **Claude API** | Constitutional AI is safe for family products. Best reasoning of any model. Can write code, analyze documents, etc. | Cost: $3/MTok input, $15/MTok output. Not real-time. |

### Multi-Auth Pattern

Auto-merge accounts by email to prevent duplicate users:

```go
func resolveOrCreateUser(db *sql.DB, email string, phone *sql.NullString) (*User, error) {
  // Try to find by email
  user, err := getUserByEmail(db, email)
  if err == nil {
    // Found by email, update phone if provided
    if phone.Valid {
      db.Exec("UPDATE users SET phone_number=? WHERE id=?", phone.String, user.ID)
    }
    return user, nil
  }

  // Not found, try phone
  if phone.Valid {
    user, err = getUserByPhone(db, phone.String)
    if err == nil {
      // Found by phone, update email
      db.Exec("UPDATE users SET email=? WHERE id=?", email, user.ID)
      return user, nil
    }
  }

  // Not found, create new user
  return createUser(db, email, phone)
}
```

Benefits:
- Users can sign up with phone, then add email later
- Users can sign up with Google (email), then add phone for SMS
- No duplicate accounts
- No account recovery flow needed

### Database Pattern: Backward-Compatible Migrations

Never drop tables or columns with existing data. Always use `ALTER TABLE ADD COLUMN`:

```sql
-- Migration 1: Create table
CREATE TABLE IF NOT EXISTS list_items (
  id INTEGER PRIMARY KEY,
  user_id INTEGER NOT NULL,
  name TEXT NOT NULL,
  created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Migration 2 (6 months later): Add column
ALTER TABLE list_items ADD COLUMN urgent BOOLEAN DEFAULT FALSE;

-- Migration 3 (1 year later): Add index
CREATE INDEX IF NOT EXISTS idx_list_items_user_id ON list_items(user_id);

-- NEVER do this in production:
-- DROP TABLE list_items;  -- Data loss!
-- ALTER TABLE list_items DROP COLUMN urgent;  -- Data loss!
```

Benefits:
- Old deployments can still read database (no downtime)
- New deployments can read/write new columns (no migration blocks)
- Rollback is safe (old code still works with new schema)

### Session Security

- **Token generation:** 256-bit random using `crypto/rand` (Go) or `crypto.getRandomValues()` (JavaScript)
- **Token storage:** HttpOnly cookies on web (cannot be stolen via XSS), localStorage for PWA with auto-refresh after 30 days
- **Token expiry:** 30-day sliding window (refreshed on each request)
- **Rate limiting:** Per-IP and per-user sliding window (e.g., 5 failed logins in 10 minutes → blocked for 1 hour)

```go
// Session token generation
func generateSessionToken() (string, error) {
  b := make([]byte, 32)
  if _, err := rand.Read(b); err != nil {
    return "", err
  }
  return hex.EncodeToString(b), nil
}

// Set HttpOnly cookie
http.SetCookie(w, &http.Cookie{
  Name:     "session",
  Value:    token,
  HttpOnly: true,
  Secure:   true, // HTTPS only
  SameSite: http.SameSiteLax,
  MaxAge:   30 * 24 * 60 * 60, // 30 days
})
```

### Rate Limiting Pattern

Use `sync.Map` with sliding time windows:

```go
var rateLimitMap sync.Map // map[string][]int64 (IP -> timestamps of attempts)

func isRateLimited(ip string, maxAttempts int, windowSeconds int) bool {
  now := time.Now().Unix()
  cutoff := now - int64(windowSeconds)

  val, _ := rateLimitMap.LoadOrStore(ip, []int64{})
  timestamps := val.([]int64)

  // Remove old timestamps outside window
  var recentTimestamps []int64
  for _, t := range timestamps {
    if t > cutoff {
      recentTimestamps = append(recentTimestamps, t)
    }
  }

  if len(recentTimestamps) >= maxAttempts {
    return true // Rate limited
  }

  // Add current attempt
  recentTimestamps = append(recentTimestamps, now)
  rateLimitMap.Store(ip, recentTimestamps)
  return false
}
```

### Nullable Fields in SQL

For optional fields like `phone_number`, use `sql.NullString`:

```go
type User struct {
  ID          int
  Email       string
  PhoneNumber sql.NullString // NOT string, use sql.NullString
}

// Reading:
rows, _ := db.Query("SELECT id, email, phone_number FROM users WHERE id=?", userID)
user := User{}
rows.Scan(&user.ID, &user.Email, &user.PhoneNumber)

// In SQL queries, always use COALESCE to handle NULL:
rows, _ := db.Query("SELECT id, COALESCE(phone_number,'') FROM users")
```

### Environment Variable Graceful Degradation

Features should degrade gracefully if optional env vars are missing:

```go
// Google OAuth is optional
googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
googleClientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

if googleClientID == "" || googleClientSecret == "" {
  log.Warn("Google OAuth disabled (GOOGLE_CLIENT_ID or GOOGLE_CLIENT_SECRET not set)")
  // Handler for /auth/google returns 501 Not Implemented
} else {
  // Set up Google OAuth
}

// Stripe is optional (for future revenue)
stripePublishableKey := os.Getenv("STRIPE_PUBLISHABLE_KEY")
stripeSecretKey := os.Getenv("STRIPE_SECRET_KEY")

if stripePublishableKey == "" || stripeSecretKey == "" {
  log.Warn("Stripe payments disabled")
  // Payment buttons hidden from UI
} else {
  // Set up Stripe
}
```

### AI Integration Pattern

For Claude API integration (e.g., Penny's personality):

```typescript
// Penny's system prompt (constitutional AI)
const PENNY_SYSTEM_PROMPT = `You are Penny, a cheerful grocery shopping assistant for families.
You help users build shopping lists, organize items, and save money.
Be warm, encouraging, and family-friendly. Avoid profanity.
When the list is empty, greet the user warmly.
After 5 minutes of inactivity, send a gentle nudge: "Want help with your list?"
Suggest items based on common household needs: milk, bread, eggs, etc.
Keep responses short (1-2 sentences).`;

// API call
const response = await fetch('https://api.anthropic.com/v1/messages', {
  method: 'POST',
  headers: {
    'x-api-key': process.env.ANTHROPIC_API_KEY,
    'content-type': 'application/json',
  },
  body: JSON.stringify({
    model: 'claude-opus-4-6',
    max_tokens: 200,
    system: PENNY_SYSTEM_PROMPT,
    messages: [
      {
        role: 'user',
        content: userMessage,
      },
    ],
  }),
});

// Streaming response (for typewriter effect)
for await (const chunk of response.body) {
  const text = new TextDecoder().decode(chunk);
  // Update UI character by character
}
```

Benefits of constitutional AI:
- Safe for families including children
- Consistent personality
- Can handle edge cases (profanity, inappropriate requests) automatically
- No moderation step needed

---

## 4. Tim's Winning Metrics Framework

Every vibe-coded product must be instrumented for Tim's Winning Metrics from Day 1. These are the only metrics that matter.

### WM0: Does It Work?

**Question:** Is the product live and usable?

**Measure:**
- Feature completion rate (% of planned features shipped)
- Error rate (% of requests that fail)
- Time-to-value (how long until user gets value?)

**For consumer:** Does the family thrive more after vs before?
**For B2B:** Does their income statement change materially?

**Success criteria:**
- Core flow works end-to-end (signup → use product → get value)
- Error rate < 1% (one error per 100 requests is acceptable)
- Time-to-value < 2 minutes (user gets value faster than they lose interest)

**Dashboard:**
```
WM0: Does It Work?
- Feature completion: 8/10 (80%)
- Error rate: 0.3% (healthy)
- Time-to-value: 1m 45s (great)
- Daily active users: 145
```

### WM1: NPS (Net Promoter Score)

**Question:** How likely are users to refer this product to friends?

**Measure:** "How likely are you to recommend [product] to friends?"
- Score 9-10 → Promoter (delighted, likely to recommend)
- Score 7-8 → Passive (satisfied but not excited)
- Score 0-6 → Detractor (unhappy, likely to bad-mouth)

**NPS = (# Promoters - # Detractors) / # Respondents × 100**

**Target:** > 70 (most successful products are 50-80)

**Follow-up:**
- If "very likely" → ask why → use in marketing copy and social proof
- If "very unlikely" → ask why → these are the bugs you need to fix

**Implementation:**
- Show survey after user gets value (e.g., after adding 5 list items)
- Make it optional (don't force)
- Follow up on detractors within 24h (customer service recovery)
- Use Delighted.com or custom in-app survey

**Dashboard:**
```
WM1: NPS
- NPS Score: 72 (great)
- Promoters: 45% (9-10)
- Passives: 35% (7-8)
- Detractors: 20% (0-6)
- Top reason for recommending: "Saves me $50/month"
- Top reason for not recommending: "UI is confusing"
```

### WM2: Retention Asymptote

**Question:** What % of users stay long-term?

**Measure:** Plot monthly cohorts. X-axis = months since acquisition, Y-axis = % retained.
- January 2024 cohort (darkest line): 100% → 60% → 45% → 40% → 38% → 38%
- February 2024 cohort: 100% → 58% → 43% → 38%
- March 2024 cohort: 100% → 55% → 40%
- April 2024 cohort: 100% → 50% (still fresh)

The asymptote (where the line flattens) tells you what % are long-term loyal.

**Target:** > 40% at 6+ months (40% of users become long-term loyal)

**Dashboard (Looker or custom):**
```
WM2: Retention Asymptote
January 2024:  [████████████████████] 45% (asymptote)
February 2024: [███████████████████] 40%
March 2024:    [██████████████████] 38%
April 2024:    [██████████████] 50% (still too fresh)
May 2024:      [████████] (just started, ignore)
```

**Actions based on asymptote:**
- If asymptote is 20%, users don't see long-term value. Fix it.
- If asymptote is 40%, good but can be better. Look for engagement features (streaks, goals, social).
- If asymptote is 60%+, exceptional. Scale the marketing.

### WM3: CAC Doubling Time (CACD)

**Question:** How fast does $1 of sales & marketing turn into $2 of gross margin?

**Measure:** Days or months from first marketing spend to gross margin payback.

**Formula:**
```
Gross Margin per customer = (Revenue - COGS) per customer
Customer Acquisition Cost (CAC) = Marketing spend / Customers acquired
CACD = (Revenue per customer / (Revenue - COGS per customer)) × (Marketing spend / Customers acquired)
     = Time for revenue to exceed CAC

Example:
- Marketing spend: $1,000/month
- Customers acquired: 100/month
- CAC = $10 per customer
- Revenue per customer (monthly): $5
- COGS per customer (monthly): $1
- Gross margin per customer = $5 - $1 = $4
- CACD = ($10 CAC / $4 monthly margin) × 30 days = 75 days = 2.5 months
```

**Target:**
- CACD < 8 months = good (5-8 month payback is standard)
- CACD < 6 months = very good
- CACD < 3 months = great
- CACD < 0 months (zero-day) = exceptional (revenue covers CAC immediately)

**For vibe-coded products with Guild rev share:** CACD should be zero-day because you pay 30% AFTER revenue is collected. Example:
```
- Guild takes 30% rev share (paid from collected revenue)
- You get 70% for product + ops
- COGS (Stripe fee, hosting): 15%
- Gross margin: 70% - 15% = 55% per dollar of revenue
- If customer acquires itself (viral k-factor > 1), CAC = $0
- CACD = zero-day (never, revenue covers costs immediately)
```

**For paid acquisition (Google Ads, Influencers):** Build in virality to shorten CACD.
```
- Viral k-factor = (% of users who invite) × (invites per user) × (% invited who click) × (% who convert)
- If k-factor > 1, each customer acquires > 1 new customer, and CACD flattens
- Example: k=1.15 means each customer brings 1.15 new customers (exponential growth)
- CACD becomes (marketing spend) / (total customers including viral) = much shorter
```

**Dashboard:**
```
WM3: CAC Doubling Time
- CAC: $8
- Monthly gross margin/customer: $4
- CACD: 60 days
- Viral k-factor: 1.08 (each customer acquires 1.08 new customers)
- Adjusted CACD (with viral): 45 days
```

### WM4: Scale

**Question:** Once WM0-3 work, can we pour fuel on the fire?

**Measure:** Revenue and growth rate.

**Milestones:**
- $10K MRR → seed round possible (not required)
- $40K MRR → seed round expected
- $3M ARR → Series A expected
- If CACD is short enough, skip rounds and self-fund

**Actions:**
- Once WM0-3 are proven, launch all 5 selling channels (see Section 7)
- Double marketing spend, watch CACD. If it stays short, you can scale 10x.
- Hire first employee at $40K MRR (this is your 6th person: Tim, Coach, Andy, AI workers, + 1 human)

### Building the WM Dashboard

**Week 1:** Instrument WM0-3 into the MVP
```typescript
// Example: log WM0 event (time-to-value)
const signupStartTime = localStorage.getItem('signupStartTime');
const now = Date.now();
const timeToValue = now - signupStartTime;
analytics.track('WM0_time_to_value', { duration_ms: timeToValue });

// Example: log WM1 event (NPS response)
analytics.track('WM1_nps_response', { score: 8, reason: 'Saves money' });

// Example: log WM2 event (user active)
analytics.track('WM2_user_active', { user_id, cohort_month: 'Jan2024' });

// Example: log WM3 event (revenue)
analytics.track('WM3_revenue', { customer_id, amount: 4.99, margin: 2.99 });
```

**Week 2:** Aggregate into Looker dashboard
```sql
-- WM0: Feature completion
SELECT
  COUNT(CASE WHEN feature_shipped THEN 1 END) as shipped,
  COUNT(*) as total,
  ROUND(100.0 * COUNT(CASE WHEN feature_shipped THEN 1 END) / COUNT(*), 1) as pct
FROM features;

-- WM1: NPS
SELECT
  COUNT(CASE WHEN nps_score >= 9 THEN 1 END) as promoters,
  COUNT(CASE WHEN nps_score >= 7 AND nps_score <= 8 THEN 1 END) as passives,
  COUNT(CASE WHEN nps_score <= 6 THEN 1 END) as detractors,
  ROUND(
    100.0 * (COUNT(CASE WHEN nps_score >= 9 THEN 1 END) - COUNT(CASE WHEN nps_score <= 6 THEN 1 END)) / NULLIF(COUNT(*), 0),
    0
  ) as nps_score
FROM nps_responses
WHERE created_at >= DATE_SUB(NOW(), INTERVAL 30 DAY);

-- WM2: Retention asymptote
SELECT
  cohort_month,
  ROUND(COUNT(CASE WHEN months_since_signup = 0 THEN 1 END) / NULLIF(COUNT(DISTINCT user_id), 0), 3) as m0,
  ROUND(COUNT(CASE WHEN months_since_signup = 1 THEN 1 END) / NULLIF(COUNT(DISTINCT user_id), 0), 3) as m1,
  ROUND(COUNT(CASE WHEN months_since_signup = 2 THEN 1 END) / NULLIF(COUNT(DISTINCT user_id), 0), 3) as m2,
  ROUND(COUNT(CASE WHEN months_since_signup >= 6 THEN 1 END) / NULLIF(COUNT(DISTINCT user_id), 0), 3) as asymptote
FROM user_activity
GROUP BY cohort_month;

-- WM3: CAC, margin, CACD
SELECT
  SUM(marketing_spend) / NULLIF(COUNT(DISTINCT customer_id), 0) as cac,
  ROUND(AVG(gross_margin), 2) as avg_monthly_margin,
  ROUND((SUM(marketing_spend) / NULLIF(COUNT(DISTINCT customer_id), 0)) / NULLIF(AVG(gross_margin), 0) * 30, 0) as cacd_days
FROM marketing_data
WHERE created_at >= DATE_SUB(NOW(), INTERVAL 90 DAY);
```

**Week 3:** Connect to Slack alerts
```
Post to #metrics Slack channel every Monday:
WM0: 80% features shipped, 0.3% error rate, 1m45s time-to-value
WM1: NPS 72 (45% promoters, 35% passives, 20% detractors)
WM2: 40% retention at 6+ months
WM3: $8 CAC, 60-day CACD, 1.08 viral k-factor
WM4: $12K MRR, growing 15% week-over-week
```

---

## 5. The MBO (Metrics-Based Optimization) Process

Tim's coaching framework for vibe-coded products. This is the weekly iteration loop that turns a broken MVP into a thriving product.

### Initial Team Structure

Per product:
- **Tim** — Coach, weekly MBO meetings, strategic direction
- **Founder/CEO + Vibe Coder** — Andy (wears both hats)
- **Analyzer/Iterator** — AI assistant (Claude) with task workers
- **Burn rate** — $35-50K/month
- **Runway** — $500K pre-seed = 10-18 months

### The Weekly Iteration Loop

**Every Monday:**

1. **Review WM0-3 metrics** — Pull last week's data (DAU, NPS, retention cohort, revenue/CAC)
   ```
   WM0: 120 DAU (up 8% WoW) ✓
   WM1: NPS 65 (target 70) — need to improve
   WM2: 35% asymptote (target 40%) — losing users after 3 months
   WM3: $8 CAC, 60-day CACD (target < 45 days) — marketing spend too high
   ```

2. **Identify the bottleneck** — What's NOT working? What's the #1 lever to pull?
   ```
   Bottleneck: NPS is 65, target is 70. Top complaint: "Onboarding is confusing"
   If we improve NPS to 70, we'll get more word-of-mouth (lower CACD, better WM3)
   ```

3. **Generate 3-5 a/b test ideas** — Look at what competitors do, don't reinvent
   ```
   Idea A: Interactive onboarding walkthrough (copy Duolingo's approach)
   Idea B: Penny greeting message on empty list (copy HeyBuddy's approach)
   Idea C: Simplified signup form (copy Discord's approach)
   Idea D: "Share your progress" reminder after 5 items added (copy Nike's approach)
   Idea E: In-app tutorial video (copy Canva's approach)
   ```

4. **Implement the highest-impact test** — Estimate impact and effort
   ```
   Idea A (walkthrough): High impact (50 companies did this), Medium effort (4h)
   Idea B (Penny greeting): High impact, Low effort (1h) → PICK THIS
   Idea C (simplified form): Medium impact, Low effort (2h)

   Pick Idea B: Best ROI (impact/effort = high/1h = 10)
   ```

5. **Measure results in 1 week** — Does it move the needle?
   ```
   Before: NPS 65, 120 DAU, 35% asymptote
   After (Penny greeting): NPS 68, 125 DAU (+4%), 37% asymptote (+2%)

   Result: +3 NPS, +4% DAU, +2% asymptote ✓ Keep it
   ```

6. **If it worked, keep it. If not, drop it and try next idea** — Fast iteration
   ```
   Idea B worked, so keep Penny greeting
   Next week, try Idea C (simplified form)
   ```

7. **Repeat every week for 3-6 months** — Compound the wins
   ```
   Week 1: +3 NPS (now 68)
   Week 2: +2 NPS (now 70) ✓ Target reached
   Week 3: +1% DAU (now 126)
   Week 4: -2% churn (retention goes 35% → 36%)
   ...repeat until WM3 is < 45 days
   ```

### A/B Test Framework

**Every test must have:**
1. **Control group** — 50% of new users get old version
2. **Treatment group** — 50% of new users get new version
3. **Metric to optimize** — Pick one (NPS, DAU, retention, revenue)
4. **Duration** — Min 100-200 users per group (1 week typical)
5. **Statistical significance** — p-value < 0.05 (95% confidence)

```typescript
// Example: A/B test for onboarding walkthrough
const useOnboardingVariant = () => {
  const userId = getCurrentUser().id;
  // Deterministic assignment: odd IDs = treatment, even = control
  const variant = userId % 2 === 0 ? 'control' : 'treatment';

  return {
    variant,
    showWalkthrough: variant === 'treatment',
  };
};

// Log the test assignment
analytics.track('onboarding_variant_assigned', {
  user_id: userId,
  variant: useOnboardingVariant().variant,
});

// After 1 week, compare metrics
SELECT
  variant,
  COUNT(DISTINCT user_id) as users,
  ROUND(AVG(nps_score), 1) as avg_nps,
  ROUND(SUM(revenue) / COUNT(DISTINCT user_id), 2) as arpu
FROM events
WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)
GROUP BY variant;
```

### Common Failure Modes to Avoid

| Failure | Why It Happens | How to Avoid |
|---------|---|---|
| Team too big, burn too hot | Hire before proving WM0-3 | Stay lean: 1-2 people until $40K MRR |
| Over-build MVP | Fear of "missing features" | Cut scope ruthlessly. Features can be added after. |
| Build MVP without channel | Launch without thinking about WM3 | Instrument CACD on Day 1. Test viral mechanics early. |
| No metrics on Day 1 | "We'll add analytics later" | No. WM0-3 must be trackable from launch. |
| Ignore data, follow gut | "Users say they want X" | A/B test everything. Data beats opinions. |
| Wrong metric to optimize | "Let's maximize DAU" | No. Optimize for retention asymptote or NPS first. DAU grows naturally from good retention. |
| Test too many ideas at once | "Run 10 tests in parallel" | No. Run 1 test per week, learn, then iterate. Confounds learnings. |
| Not talking to users | "We have metrics" | Talk to users: Why did you churn? What did you like? Metrics explain the "what," users explain the "why." |

---

## 6. Vibe Coding Workflow

The actual process for turning a timpager into a shipped product.

### Step 1: Tim Creates Timpager

Tim writes 1-2 page Google Doc with:
- Product idea (one sentence)
- Customer pain point (1-2 sentences)
- How it solves it (1-2 sentences)
- Rough feature list (10 items max)
- Success metrics (what would make you feel successful?)

Example:
```
# GrocerySync for Families

One sentence: A shared grocery list for families that splits shopping tasks and prevents duplicate buys.

Pain point: Families spend 30 minutes/week on shopping coordination. "Did you buy milk?" texts, duplicate buys, forgotten items, cart checkout at the store.

How it solves: Shared list, claim items, price syncing, check-off on mobile, Alexa integration.

Features:
1. Phone/SMS signup (fast for families)
2. Shared grocery list (edit from home)
3. Claim items (divide & conquer)
4. Price sync from Instacart/Walmart (AI parses prices)
5. Mobile check-off (aisle view at store)
6. Alexa integration ("Alexa, add milk to the list")
7. Weekly savings report ("You saved $15 this week")
8. Smart suggestions ("You bought eggs last week, need them again?")
9. Referral rewards (invite friends, earn $5 store credit)
10. Family history ("Mom bought eggs 4x this month")

Success metrics:
- 50+ families using by end of Month 3
- 4+ items per list per day (engagement)
- 80%+ purchase with referral link vs direct (viral)
```

### Step 2: Andy Reviews with Garry Tan Framework

Apply the 10 Review Dimensions (see Section 1):

```
EXPANSION mode (greenfield product)

1. Architecture ✓ — Single database (households table), multi-auth, shared list sync
2. Error & Rescue Map — Need timeout handling for Instacart API, Alexa skill failures
3. Security ✓ — Household members can only see their own family's lists
4. Data Flow — Map the 4 shadow paths for list item creation, sync conflicts
5. Code Quality — Will be clean, similar to PennyAI codebase
6. Tests — Need to test list sync on 2 devices, Alexa parsing edge cases
7. Performance — List queries should be cached (load list once, update via WebSocket)
8. Observability — NPS survey on check-off, track Alexa invocation success rate
9. Deployment — Feature flag for Alexa until we verify Skill submission
10. Long-Term — Reversible to multi-account (sibling adds own family)

Critical gaps:
- Need to handle concurrent edits (2 family members claim same item)
- Alexa API can fail, need graceful degradation
- Instacart API pricing is PII (Don't expose prices to unauthenticated users)

Recommendation: Proceed with EXPANSION mode, but defer Alexa until after launch. Keep it simple: shared list, claim items, price sync. Add Alexa in Month 2.
```

### Step 3: Map the Four Shadow Paths

For every data flow, trace happy path + 3 shadow paths:

```
Data flow: User adds "2 lbs chicken" to list

Happy path:
  User enters "2 lbs chicken" in input
  Clicks "Add"
  Item appears in list immediately (optimistic update)
  Server persists to database
  Other family members see item appear (WebSocket sync)
  Penny suggests "You bought chicken last week, budget: $10"

Nil input path:
  User clicks "Add" with empty input
  Show error: "Can't add empty item"
  Focus stays on input
  No item created

Empty list path:
  User has 0 items
  Enters "2 lbs chicken"
  Item appears as #1
  Penny shows greeting: "Hey, let's build your list!"

Upstream error path:
  User enters "2 lbs chicken"
  Clicks "Add"
  Network error (offline)
  Spinner shows 5s
  Show error: "Can't save right now, retrying..."
  Retry in background every 10s
  Once online, save succeeds
  Item appears in list

Double-click path:
  User enters "2 lbs chicken"
  Clicks "Add" twice quickly
  Only 1 item created (database has unique constraint on (household_id, item_name, created_at))
  UI shows 1 item
  Error message on 2nd click: "Just added that item"

Navigate-away path:
  User enters "2 lbs chicken"
  Clicks "Add"
  (Before response) User navigates to settings
  Item continues saving in background
  User returns to home
  Item is there (was saved)

Slow connection (2G):
  User enters "2 lbs chicken"
  Clicks "Add"
  Spinner for 10s
  Item appears with confidence meter (100% uploaded)
  User can continue adding more items

Stale state path:
  Mom (device A) adds "2 lbs chicken"
  Dad (device B) has list cached, hasn't synced in 5min
  Dad sees old list (0 items)
  Auto-refresh triggers after 10s
  Dad now sees "2 lbs chicken"

Back button path:
  User adds "2 lbs chicken"
  Navigates to settings
  Clicks back button in browser
  List still shows "2 lbs chicken"
  Not lost due to React Router
```

### Step 4: Write CLAUDE.md

Copy PennyAI's CLAUDE.md as template and customize:

```markdown
# GrocerySync for Families

## Project Context
- Stack: React 19 + TypeScript + Vite (frontend), Go (backend), SQLite (database)
- Deployment: Railway
- Pages: Signup, Home (shared list), Settings (household), Analytics

## Testing Requirements
Hard rule: test-to-code ratio >= 1.3x

## Architecture
- Multi-auth: phone/SMS + email/password
- Shared list across household
- Price sync from Instacart API
- WebSocket for real-time sync between devices

## Must-Have Features
- Phone/SMS signup
- Shared grocery list
- Claim items (household coordination)
- Price sync
- Mobile check-off
- Weekly savings report
- Referral rewards
- NPS survey

## Database Invariants
(List tables that must never be dropped)
- users
- households
- household_members
- list_items
- prices
- referrals

## API Contract
(List endpoints that must never change)
- POST /auth/send-code
- POST /auth/verify-code
- GET /api/list
- POST /api/list
- PATCH /api/list/{id}/claim
- GET /api/prices
- POST /api/referral/redeem
```

### Step 5: Build MVP Using AI Coding Tools

Use Claude Code, Cursor, or GitHub Copilot. Pair with a human vibe coder (Andy).

**Vibe coding checklist:**
- [ ] Architecture diagram ASCII art in `server.go` comments
- [ ] Error & Rescue Map in CLAUDE.md
- [ ] 4 shadow paths for each data flow traced in test cases
- [ ] Test-to-code ratio >= 1.3x before shipping
- [ ] All 10 Review Dimensions addressed
- [ ] No hardcoded secrets in source code
- [ ] Database migrations backward-compatible
- [ ] CI/CD pipeline runs tests on every commit

### Step 6: Instrument WM0-3 Metrics

Before launching, wire up analytics:

```typescript
// WM0: Feature completion and error rate
analytics.track('feature_used', { feature: 'add_item', success: true });
analytics.track('error_occurred', { feature: 'add_item', error: 'offline' });

// WM1: NPS survey
const showNPSSurvey = () => {
  const itemsAdded = getItemsAddedThisWeek();
  if (itemsAdded >= 5) {
    showModal('How likely to refer GrocerySync to friends?', 0, 10);
  }
};

// WM2: Retention cohorts
analytics.track('user_active', { cohort_month: 'Mar2024' });

// WM3: Revenue and CAC
analytics.track('revenue', { customer_id, amount: 4.99, margin: 2.99 });
analytics.track('referral_conversion', { referrer_id, referred_user_id });
```

### Step 7: Tim Does 20-40 Bug/Feature Submissions

Tim tests the MVP and creates a priority-ranked list of bugs and features to fix:

```
Priority 1 (Critical bugs):
B1: List doesn't sync when offline → check backend logs

B2: Claiming an item on phone A doesn't update phone B immediately → WebSocket issue

B3: SMS code sometimes doesn't arrive → Twilio rate limiting or timeout

Priority 2 (High-impact features):
F1: Alexa integration (defer to Month 2, but plan for it)

F2: Instacart price sync accuracy is 70%, should be 95% → improve parsing

F3: Household invite code should expire after 7 days → prevent stale codes

Priority 3 (Polish):
F4: Add animations to item check-off → delight users

F5: Dark mode support → table-stakes for modern apps
```

### Step 8: Weekly MBO Iteration (3-6 Months)

Follow the weekly loop (see Section 5):
1. Review WM0-3
2. Identify bottleneck
3. Generate 3-5 a/b test ideas
4. Implement highest-impact test
5. Measure results
6. Keep or drop
7. Repeat

Example sprint:
```
Week 1: NPS is 55 (target 70). Test onboarding walkthrough. Result: NPS +3 → 58
Week 2: NPS 58. Test Alexa demo video. Result: NPS +1 → 59 (not great). Drop.
Week 3: NPS 59. Test "share your savings" reminder. Result: NPS +4 → 63
Week 4: NPS 63. Test Instacart price sync UI. Result: NPS +3 → 66
Week 5: NPS 66. Test referral rewards $5 → $10. Result: NPS +2, revenue +25% → 66 keeps it
Week 6: NPS 66. Test Penny suggestions for common items. Result: NPS +5 → 71 ✓ Target!
```

### Step 9: Declare MVP When It Sings

When WM0-3 are healthy:
- WM0: Error rate < 1%, time-to-value < 2min
- WM1: NPS > 70
- WM2: 40%+ retention asymptote
- WM3: CACD < 6 months

Then shift to **vibe selling** (see Section 7).

---

## 7. Vibe Selling Channels (Post-MVP)

Once the MVP works and WM0-3 are proven, sell through 5 channels simultaneously.

### 1. Viral PLG (Product-Led Growth)

Close the viral loops already in the MVP:
- User A invites friends → each friend's invite link has referrer tracking → viral k-factor
- Example: GrocerySync shares "Come manage [household_name]'s list with me" via copylink
- Conversion: 20% of invitees sign up

**Measurement:**
```
k-factor = (% users who invite) × (invites per user) × (% invited who click) × (% who convert)
          = 40% × 2 invites × 60% click × 20% convert
          = 0.40 × 2 × 0.60 × 0.20
          = 0.096 (less than 1, viral loop is weak)

If k < 1, viral doesn't work. Need to improve one of these:
- More users inviting (improve sharing prompt visibility)
- More invites per user (share button after every milestone)
- Better click rate (make link attractive: "Come manage our list!")
- Better conversion (fast signup flow, no email needed)
```

**Actions to improve k-factor:**
- A/B test share button placement (after 1st item, 5th item, 1st week)
- A/B test share message ("Come manage our list" vs "Join my family" vs "We're saving money!")
- A/B test signup flow (reduce from 3 screens to 2)
- A/B test incentive ("Get 1 bonus item per referral" vs "$5 Instacart credit")

**Goal:** k-factor > 1.1 (exponential growth)

### 2. Incented Viral (Guild)

Guild is a rev-share network. You offer partners (Instacart, meal prep services, grocery delivery) a reason to send users to you.

**Model:**
- You get 70% of revenue
- Guild takes 30% (split between referrer + platform)
- Partners get 10% commission on referred users' purchases
- **Zero-day CAC:** You pay the referrer AFTER you collect revenue from the referred user

**Example copylink:**
```
https://grocerysync.com/refer/instacart-user?ref=instacart&user_id=bob123

Instacart embeds this link on their checkout confirmation:
"Save 20% on shared grocery lists → Join GrocerySync"

When user clicks:
1. User lands on GrocerySync signup (already in Instacart context)
2. User signs up and starts using list
3. User sees Instacart option to sync prices
4. User clicks "Sync prices from Instacart"
5. User makes purchase on Instacart
6. GrocerySync gets revenue ($4.99/month)
7. Guild pays Instacart 10% commission ($0.50) from that revenue
8. Your revenue: 70% × $4.99 = $3.49 (minus COGS)
```

**Partners to approach:**
- Instacart
- Amazon Fresh
- Walmart+
- Meal prep services (HelloFresh, EveryPlate)
- Grocery delivery (DoorDash, Uber Eats)

### 3. Scout-Assisted Referrals

Use AI to find ICP users (e.g., busy parents with 2+ kids) in your social graph and facilitate warm intros.

**Example:**
```
Mom user (Sarah) has list with "2 lbs chicken, eggs, bread"
AI finds Sarah's friends from LinkedIn/Facebook
AI identifies which friends are also busy parents (high chance of fit)
AI suggests: "Invite Emily to your list" with template message:
"Hey Emily, I've been using this app to manage our grocery list with my family.
Saves us 20 min/week on 'What did you buy?' texts. Thought you might like it!"

Sarah clicks "Copy message", sends via iMessage
Emily clicks link, signs up in 30s
Sarah gets $5 Instacart credit
```

**Setup:**
- Require LinkedIn/Facebook OAuth on signup (optional but recommended)
- Analyze friend list for ICP attributes (parent/age/location/income proxy)
- Suggest friends to invite, with personalized messaging templates
- Track referral attribution (who referred whom)

### 4. Tod-Assist Outreach

Use Claude API to draft personalized outreach to brand partners (Instacart, Walmart, etc.). Emphasize that GrocerySync users are their customers buying from competitors.

**Example email (drafted by Claude):**
```
From: Tim (CEO, GrocerySync)
To: partnerships@instacart.com
Subject: Co-marketing opportunity: reach busy families

Hi Instacart team,

GrocerySync is a shared shopping list app for families. Our users (target: busy parents in US metro areas) spend 30 min/week coordinating grocery shopping.

We've noticed that many of our users shop on competitors' apps (Amazon Fresh, Walmart+) instead of Instacart. We'd like to change that.

Proposal:
- We embed Instacart as a featured option in our price sync (vs competitors)
- We offer users 5% cashback on Instacart purchases (you pay the subsidy, we handle distribution)
- You get first-party data on family size, shopping preferences, purchase frequency

Result: You acquire high-intent shoppers at 30% lower CAC than paid ads, we generate revenue through referral commissions.

Would you be open to a 15-min call?

Best,
Tim
```

**Claude generates 3-5 versions of this, customized per partner**, then Tim picks the best one.

### 5. Tod-Sourced Human Reps

Recruit sales reps who earn 30% Guild commission (pure upside for you).

**Rep job description:**
```
GrocerySync Referral Rep

Earn 30% of all revenue from referred customers (lifetime).

Job: Refer busy families to GrocerySync shopping list.

Example:
- You refer Sarah (busy mom)
- Sarah signs up and pays $4.99/month
- You earn: 30% × $4.99 = $1.50/month for life
- If you refer 100 Sarahs, you earn: $1.50 × 100 = $150/month (passive income)

How to recruit: Talk to parents at PTA meetings, soccer practice, church groups, online parenting forums.

What to say:
"I found this app that saves families 20 min/week on grocery coordination. If you want to try it, use my link and you'll get 1 free week. If you like it, it's $4.99/month. Fair warning: I get a small commission, but I genuinely use this myself."

Support: We provide you with: landing page, social media assets, email templates, FAQ.

Why reps work: They reach people ads can't. They have social proof (friend recommended it). They're incentivized.
```

**To recruit reps:**
- Post on parenting Facebook groups
- Reach out to PTA leaders
- Contact online community managers (Reddit parenting subs, BumbleBlack, etc.)
- Start with 5-10 reps, see who sticks

**Tracking:**
```go
type Referral struct {
  ID          int       // unique
  RepID       int       // rep who referred
  UserID      int       // referred user
  Revenue     float64   // total revenue from referred user
  Commission  float64   // 30% of revenue
  CreatedAt   time.Time // when referred
  PaidOut     bool      // has rep been paid?
  PaidOutAt   time.Time // when paid
}

// Monthly payout (every 1st of month):
SELECT
  rep_id,
  SUM(commission) as total_commission,
  COUNT(DISTINCT user_id) as customers_referred
FROM referrals
WHERE paid_out = FALSE
  AND MONTH(created_at) < MONTH(NOW())
GROUP BY rep_id;

// Pay out via Stripe Connect or PayPal
```

---

## 8. Product Quality Checklist (Before Declaring MVP)

Use this checklist before shipping. Link each item to a test case.

- [ ] **All endpoints respond with correct status codes** — Test every endpoint (GET, POST, PATCH, DELETE) with valid and invalid inputs
- [ ] **All endpoints return correct JSON shapes** — Automated test compares response against schema
- [ ] **All screens render and are navigable** — E2E test for each screen (signup, home, settings, analytics)
- [ ] **Database migrations run cleanly** — Test on fresh database and on existing database from previous version
- [ ] **Test-to-code ratio >= 1.3x** — Calculate before PR merge, block if below
- [ ] **No silent failures** — Every error is logged and visible (Sentry, custom dashboard)
- [ ] **Auth works correctly** — All 3 flows tested (phone/SMS, Google OAuth, email/password)
- [ ] **Session security** — HttpOnly cookies, 256-bit tokens, 30-day expiry, rate limiting
- [ ] **WM0-3 metrics are instrumented** — Analytics events logged for all critical paths
- [ ] **NPS survey is implemented** — Shows after user gets value, tracks responses
- [ ] **Sharing/invite flow works** — Cross-browser tested (Chrome, Safari, Firefox, mobile)
- [ ] **Referral attribution tracks** — Can identify which user referred which, revenue correctly attributed
- [ ] **Rate limiting is in place** — SMS codes, logins, API requests all rate-limited
- [ ] **No hardcoded secrets** — API keys in environment variables only, `.env` in `.gitignore`
- [ ] **Error monitoring is live** — Sentry or equivalent integrated, alerts on error spike
- [ ] **CI/CD pipeline runs tests** — Every PR runs `npm test` and `go test ./...`, must pass before merge
- [ ] **CLAUDE.md exists** — Project context, architecture, must-haves, and refactoring rules documented

---

## 9. The Revenue Model Template

For every marketplace-style product (product + sellers + buyers):

### Unit Economics

```
For each $1 of revenue:
  10% → COGS (hosting, API costs, Stripe fees)
  30% → Guild rev share (paid AFTER collection = zero-day CACD)
  10% → Employees/operations (salaries, contractors, task workers)
  50% → Operating profit (runway extension, growth investment, founder payout)
```

### Key Ratios

| Metric | Formula | Target |
|--------|---------|--------|
| Value delivered | Price × 10 | 10x value-to-price |
| Price per customer | (Revenue / Customers) / Month | 10% of value delivered |
| Guild share | % of revenue | 30% (motivates partners, doesn't cannibalize) |
| COGS ratio | (Hosting + APIs + Payments) / Revenue | < 15% |
| CAC Doubling Time | (CAC / Monthly Margin) × 30 days | < 6 months for good |
| Viral k-factor | (% invite) × (invites/user) × (click%) × (convert%) | > 1.1 for exponential |
| Gross margin | Revenue - COGS | > 50% |
| Burn rate | (Payroll + Ops - Revenue) per month | $35-50K/month for vibe team |

### Example: GrocerySync

```
Assumptions:
- 1,000 active households
- 50% are paid customers ($4.99/month per household)
- 50% are free (no revenue)

Revenue = 1,000 × 50% × $4.99 = $2,495/month = ~$30K/year

COGS per household:
- Hosting (Go server, SQLite): $30/month ÷ 1,000 = $0.03
- Instacart API calls (price sync): $50/month ÷ 1,000 = $0.05
- Stripe fees (2.9% + $0.30): 2.9% × $4.99 + $0.30 = $0.44
- Total COGS: $0.52 per customer
- COGS ratio: $0.52 / $4.99 = 10% ✓ (target < 15%)

Gross profit per customer: $4.99 - $0.52 = $4.47
Guild share: 30% × $4.99 = $1.50 (paid to referrers/partners)
Your revenue per customer: $4.47 - $1.50 = $2.97
Payroll + ops: $40K/month = $40 per customer per month (assuming 1,000 customers)
Operating profit per customer: $2.97 - $40 = -$37.03 (loss)

At this scale, you're losing money. But:
- Growth should be 20% month-over-month (doubling every 5 months)
- At 2,500 customers: payroll = $16 per customer, profit = $2.97 - $16 = -$13.03 (better)
- At 5,000 customers: payroll = $8 per customer, profit = $2.97 - $8 = -$5.03 (break-even soon)
- At 10,000 customers: payroll = $4 per customer, profit = $2.97 - $4 = -$1.03 (breakeven!)
- At 20,000 customers: payroll = $2 per customer, profit = $2.97 - $2 = +$0.97 (profitable!)

Timeline to profitability: With 20% MoM growth, reach 20K customers in ~15 months.
Runway: $500K ÷ $40K/month = 12.5 months. Tight, but doable.
```

### Key Principle: "Profitable at First Sale"

Design the unit economics so that **from the first paying customer, gross margin covers COGS + Guild share, and you're marching toward break-even.**

If you have a loss-making unit, fix it immediately:
- Raise price (better if product is 10x value)
- Lower COGS (negotiate APIs, self-host instead of SaaS)
- Reduce Guild share (but this hurts partner motivation)

**Never assume you'll "figure out unit economics at scale."** By then you'll be out of runway.

---

## 10. What NOT to Do (Hard-Learned Lessons)

Learn from the mistakes others made:

| Mistake | Why It Fails | How to Avoid |
|---------|---|---|
| Build 9 products at once | Dilutes focus. Each product needs 3-6 months iteration. | Prove one product (WM0-3 healthy) before starting the next. |
| Build Guild payout system before revenue | Waste time on engineering that won't be used for 6 months. | Use Stripe Connect or PayPal API. It's not hard. Build when needed. |
| Use Postgres instead of SQLite | Adds ops burden (backups, monitoring, replicas). Not needed for <200 DAU. | Stick with SQLite. If you need to migrate later (at 200+ DAU), it's a 1-day job. |
| Build native apps before web app | 2x the engineering, 2x the bugs, harder to iterate. | Build PWA first. Native can come after you've proven product-market fit. |
| Over-abstract before you have 2+ products | "Let's build a shared component library!" Premature abstraction. | Keep code duplicated until you have 2+ products sharing code. Then extract. |
| Skip the weekly MBO iteration | "MVP is done, let's hire a team!" MVP is ALWAYS wrong. Iteration fixes it. | Commit 3-6 months to weekly iteration. This is where the magic happens. |
| Hire before WM0-3 are proven | "We need a designer, marketer, product manager." Burn rate goes 3x, runway goes 3x shorter. | Stay lean until $40K MRR. Then hire one person at a time. |
| Raise VC money when CACD is short | "Let's raise Series A!" If CACD is < 6 months, you're self-funding. Equity is expensive. | Only raise if you can't grow faster than capital allows. Otherwise, self-fund and keep upside. |
| Forget impact metrics | "DAU, revenue, ..." Metrics are good, but not enough. | Also measure impact: Are families saving money? Are parents less stressed? Align profit with purpose. |
| Ship undocumented architecture | "We'll document later." No one does. New team members get lost. | Document as you build. CLAUDE.md takes 1h. Saves 10h of onboarding. |
| No CI/CD pipeline | "We'll test manually." Manual testing is slow, error-prone, doesn't scale. | Wire up CI/CD on Day 1. `git push` → tests run → auto-deploy on pass. This takes 2h. |

---

## Summary: The Vibe Coding Playbook

1. **Use Garry Tan's rigorous review framework** (10 dimensions, 3 modes)
2. **Enforce test-to-code ratio >= 1.3x** (non-negotiable)
3. **Build with the stack that works** (Go + React + SQLite + Twilio)
4. **Instrument WM0-3 metrics from Day 1** (does it work, NPS, retention, CAC)
5. **Run the weekly MBO iteration loop** (identify bottleneck, A/B test, measure, repeat)
6. **Ship the MVP, then vibe sell** (5 channels: viral PLG, incented viral, scout-assisted, partner outreach, reps)
7. **Stay lean until $40K MRR** (then hire)
8. **Design unit economics to be profitable at first sale** (not at scale)
9. **Use this checklist before shipping** (18-point quality bar)
10. **Learn from these mistakes** (don't repeat them)

**The result:** Products that work, that grow, and that make money.

---

## Appendix: Example Files

### Example: CLAUDE.md (GrocerySync)

```markdown
# GrocerySync for Families

## Project Context
- **Stack:** React 19 + TypeScript + Vite (frontend), Go (backend), SQLite (database)
- **Routing:** React Router v7
- **Auth:** Phone/SMS (Twilio), Google OAuth, email/password
- **Payments:** Instacart API for price sync, Stripe for future subscriptions
- **Styling:** Vanilla CSS with custom properties
- **Architecture:** SPA with real-time sync via WebSocket
- **Pages:** Signup, Home (shared list), Settings (household), Analytics

## Testing Requirements
**Hard Rule: Test-to-Code Ratio >= 1.3x**

For every line of production code written, at least 1.3 lines of test code must be written.

## Refactoring Rules
### What IS allowed
- Splitting `App.tsx` into multiple React components
- Extracting shared styles
- Renaming internal functions
- Improving error handling
- Adding types/interfaces
- Reducing code duplication

### What is NOT allowed
- Changing API endpoints or request/response shapes
- Removing any screen or feature
- Dropping database tables
- Removing auth methods
- Removing sharing/invite flow
- Removing referral system
- Removing NPS survey

## Must-Have Features
- Phone/SMS signup (Twilio)
- Google OAuth
- Email/password
- Shared grocery list (household)
- Claim items (divide shopping)
- Price sync (Instacart API)
- Mobile check-off (store aisle view)
- Weekly savings report
- Referral rewards
- NPS survey
- Admin dashboard (future)

## API Contract (all endpoints must remain)
### Auth
- `POST /auth/send-code`
- `POST /auth/verify-code`
- `POST /auth/google`
- `POST /auth/email/register`
- `POST /auth/email/login`
- `POST /auth/logout`

### Lists
- `GET /api/list`
- `POST /api/list`
- `PATCH /api/list/{id}`
- `PATCH /api/list/{id}/claim`
- `DELETE /api/list/{id}`
- `POST /api/list/clear`

### Prices
- `GET /api/prices`
- `POST /api/prices/sync`

### Referrals
- `GET /api/referral`
- `POST /api/referral/redeem`

### NPS
- `POST /api/nps`

## Database Invariants (tables that must NOT be dropped)
1. `users`
2. `households`
3. `household_members`
4. `list_items`
5. `prices`
6. `referrals`
7. `nps_responses`
8. `sessions`

## Engineering Preferences
- Explicit over clever
- DRY violations flagged aggressively
- Well-tested code is non-negotiable
- Edge cases are mandatory
- Security required for every new codepath
- Diagrams mandatory for non-trivial flows
```

### Example: Error & Rescue Map (GrocerySync)

| Failure Mode | Trigger | Rescue? | Rescue Action | User Sees | Tested? |
|---|---|---|---|---|---|
| SMS code timeout | Code not delivered in 120s | Yes | Retry delivery, alert user | "Code taking longer, retrying..." | Yes |
| SMS rate limit | 5+ codes sent from same IP in 1h | Yes | Enqueue, retry in 60s | "Try again in 1 minute" | Yes |
| Instacart API timeout | Price sync takes >10s | Yes | Show cached price, retry in background | "$2.99 (from last check)" | Yes |
| Household invite expired | Invite code >7 days old | Yes | Redirect to settings, "Invite expired" | Generate new code | Yes |
| Concurrent claim conflict | 2 family members claim same item | Yes | First wins, 2nd gets "Already claimed by Mom" | Error message, item still claimable | Yes |
| Network offline | No internet for 30+ sec | Yes | Queue requests, sync when online | "Saving..." spinner, retry | Yes |
| Database error | Connection pool exhausted | Yes | Queue request, retry with exponential backoff | "Server busy, retrying..." | Yes |

---

---

## 2026-03-09 — CEO Panel Learning: Feature Prioritization & Activation

*Learning from unicorn CEO panel review of PennyAI, Rally, and Thriftly.*

### Core Insight: "Optimize for the Core, Not the Edge"

The panel (Brian Chesky, Whitney Wolfe Herd, Stewart Butterfield, Katrina Lake, Patrick Collison, Tobi Lütke, Sarah Friar) made consistent observations across three products:

**Pattern:** Teams ship features (dark mode, trip planner, feedback widget) before the core experience is magical.

**Recommendation:** Reverse the priority order. Build features only after the core experience passes the "11-star" test.

### The Three Gates Before Shipping Features

#### Gate 1: First-5-Minutes Magic (Stewart Butterfield)
**Question:** Can a new user with zero history feel "This just works" in 5 minutes?

**Why it matters:** First impression is retention and referral engine. If onboarding is friction, users leave before household members join.

**Test:** Recruit 10 new users (no Penny experience). Time to first list, first household invite, first positive emotion. Target: 80% feel magic by 5 minutes.

**Apply to all products:** What's the first-5-minutes promise for Rally (helper claiming first task)? For Thriftly website (signing up for campaign)?

#### Gate 2: 11-Star Moment (Brian Chesky)
**Question:** What makes a user cry, laugh, or tell the story for years?

**Examples of 11-star moments:**
- Airbnb: "I belonged anywhere" (belonging, not just booking)
- Slack: "We talk like we're in the same room" (presence, not chat history)
- Penny (hypothetical): "We never argued about groceries again" (harmony, not convenience)

**Why it matters:** 11-star moments drive brand loyalty, word-of-mouth, and defensibility against competitors who copy features.

**Process:**
1. Run focus group: "What would you tell a friend about [product]?"
2. Listen for emotional language (joy, relief, freedom, trust)
3. Lock ONE narrative (not multiple)
4. Every feature should ladder up to it

**Apply to all products:**
- PennyAI: "Stop asking 'Did you get X?'" (coordination harmony)
- Rally: "I control my time, I trust the pay" (autonomy + transparency)
- Thriftly: "Shoppers feel empowered, brands reach intent" (mutual benefit)

#### Gate 3: Data Flywheel Clarity (Katrina Lake)
**Question:** Does my product get meaningfully smarter as users grow?

**Why it matters:** Flywheels create defensibility. Competitors can copy features, but not data compounding.

**Examples:**
- Stitch Fix: More data → better personalization → higher repeat rate → more data
- Penny: More households → more behavior patterns → better brand suggestions → higher retention
- Rally: More tasks → better skill matching → better outcomes → more helpers

**Process:**
1. Chart data collection → model improvement → retention lift at each stage (months 0–12)
2. Define KPIs per stage (activation, accuracy, retention)
3. Align team to this timeline (not to feature count)

**Why this matters:** Shows team, investors, and users that product compounds, not just broadens.

---

### Practical Application: Pause & Reset Framework

**When you feel pressure to ship many features:**
1. **Step 1:** Ask "Does this ladder up to our 11-star moment?"
   - If yes: Ship it
   - If no: Question if it's needed

2. **Step 2:** Ask "Does this improve our first-5-minutes promise?"
   - If yes: Prioritize
   - If no: Deprioritize

3. **Step 3:** Ask "Does this tighten our data flywheel?"
   - If yes: Add to roadmap
   - If no: Move to "Nice-to-have" or "Post-launch"

**Applied to PennyAI's current roadmap:**
- Dark mode: Nice-to-have. Doesn't improve first-5-minutes, doesn't tighten flywheel.
- Trip planner: Interesting. Could improve 11-star moment (speed), but secondary to core coordination.
- Feedback widget: Important for learning, but ops burden outweighs current value.
- Referral system: Core. Enables flywheel (more households → more learning → better product).

---

### The Flywheel Template

Use this template to document your product's compounding logic:

```
MONTH 0–3: MVP
- Core features: [list]
- Data collected: [what signals?]
- User activation: X% feel magic

MONTH 3–6: First Compounding
- AI improves: [how?]
- Data signal: [what new insight?]
- Retention improves by: Y%

MONTH 6–12: Flywheel Tightens
- Network effect: [household? brand? store?]
- Data multiplier: [cross-household learning?]
- Differentiation: [what can competitors not copy?]

YEAR 2+: Defensibility
- Moat: [data, network, switching cost?]
- Expansion opportunity: [new market, new user type?]
```

**Example: PennyAI Flywheel**
```
MONTH 0–3: MVP (list + household)
- Core: SMS input, household sharing, brand memory
- Data: 100 households × 4 members = 400 users, 50 items/week
- Signal: Which brands each member prefers

MONTH 3–6: Brand Accuracy (per-member learning)
- AI improves: Claude learns "Sarah wants organic, Jamie wants budget"
- Data signal: 2,000 users × 200 items/month = 400K data points
- Retention: 45% D30 → 50% D30 (5pp lift from better suggestions)

MONTH 6–12: Meal Planning (cross-member patterns)
- Network: Household says "taco night" → Penny suggests 2x tomatoes, cilantro, lime
- Data multiplier: 10K households × 12 meal types = learned patterns
- Differentiation: Competitors can't learn household rituals from data

YEAR 2+: Multi-Store (cross-store optimization)
- Moat: Penny knows "Whole Foods vs. Costco prices" for your household
- Expansion: Grocery delivery, meal kits, recipe partnerships
```

---

### Anti-Pattern: Feature Sprawl Without Flywheel

**Sign you're in trouble:**
- Roadmap is long list of features (dark mode, trip planner, feedback, etc.)
- No narrative about how product gets *smarter*
- Team doesn't know why they're building something
- CEO has to justify every ship

**How to escape:**
1. Pause. Pick ONE flywheel narrative (not multiple).
2. Audit roadmap: keep items that tighten flywheel, cut the rest.
3. Communicate the flywheel to team in writing.
4. Every 2 weeks, ask: "Did this ship move the flywheel?"

---

### Shipping Discipline: The Three Tiers

**Tier 1: Unlocks Core Flywheel**
- Example: Per-member brand memory (improves first-5-minutes + tightens flywheel)
- Approval: Fast track, minimal review

**Tier 2: Improves 11-Star Moment**
- Example: Trip planner (makes shopping faster, supports "relief" narrative)
- Approval: Standard review, needs clear laddering to narrative

**Tier 3: "Nice-to-Have"**
- Example: Dark mode (nice, but doesn't move core metrics)
- Approval: Hold until Tier 1 & 2 are solid, then low-priority queue

---

### For Product Teams: Update Your ROADMAP.md

Every roadmap should include a "Data Flywheel" section. Example:

```markdown
## Data Flywheel (12-Month Compounding)

### Months 0–3: Foundation
- Households: 100 → 500
- Data points: 5K items/month
- KPI: Activation 70%, D7 retention 40%

### Months 3–6: Brand Learning
- Households: 500 → 2K
- Data points: 50K items/month
- KPI: Brand accuracy 75%, D7 retention 50%

### Months 6–12: Household Rituals
- Households: 2K → 10K
- Data points: 500K items/month
- KPI: Suggestion acceptance 60%, D30 retention 40%

### Year 2: Cross-Household Patterns
- Households: 10K → 50K
- Data points: 5M items/month
- KPI: Penny becomes essential (NPS > 70)
```

---

**End of document.**

---

## 2026-03-09 — Cross-Team Governance & Shipping Gates

### Core Rules

#### Rule 1: No Feature Ships Without Passing All 3 Gates (Across ALL Repos)

Every feature—in Penny, Rally, Thrift.ly—must pass three gates before shipping. These gates ensure team alignment, defensibility, and magical product experience.

**Gate 1: First-5-Minutes Magic (Stewart Butterfield)**
- Question: Does this help a new user feel "this just works" in 5 minutes, without prior context?
- Applies to: Onboarding, core workflows, accessibility
- Failure examples: Dark mode (doesn't help new users), complex household setup (slows down first 5 minutes)
- Success examples: Frictionless list creation, reasonable defaults, brand correction buttons

**Gate 2: 11-Star Moment (Brian Chesky)**
- Question: Does this ladder up to our core narrative? Does it contribute to the story that makes users cry/laugh/tell it for years?
- Applies to: Feature positioning, UI flow, copy, marketing
- Penny's 11-star moment: "You never ask 'Did you get X?' again" (coordination)
- Rally's 11-star moment: [TBD by Tim + Andy]
- Failure examples: Dark mode (nice, not magical), generic feedback widget (operational, not narrative)
- Success examples: Household learning, multi-channel input (SMS), brand integration (trust + habit)

**Gate 3: Data Flywheel (Katrina Lake)**
- Question: Does this tighten product compounding? Does it help the product get smarter over time?
- Applies to: Data collection, model improvement, personalization
- Failure examples: Dark mode (no data), trip planner alone (doesn't tighten household learning)
- Success examples: Brand memory per user, household rituals, cross-household pattern learning

**Decision rule:**
- Feature passes 2+ gates → Ship on standard timeline
- Feature passes 1 gate → Hold or redesign
- Feature passes 0 gates → Deprioritize to post-launch (after core magic is undeniable)

**Applied examples:**

| Feature | Gate 1 | Gate 2 | Gate 3 | Decision |
|---------|--------|--------|--------|----------|
| First-5-minutes rewrite | ✅ | ✅ | ✅ | **Ship immediately** |
| Dark mode | ❌ | ❌ | ❌ | **Hold until core is solid** |
| Referral system | ✅ | ✅ | ✅ | **Ship now** |
| Trip planner | ✅ | ✅ | ✅ | **Ship when designed** |
| Feedback widget | ❌ | ❌ | ✅ | **Hold (needs ops maturity)** |
| Multi-auth (email/SMS) | ✅ | ✅ | ✅ | **Shipped, core feature** |

---

#### Rule 2: Website Copy Changes Require Brand PM Sign-Off Within 24 Hours

**Who:** Brand PM (brand consistency guardian)
**When:** Every Thrift.ly page rewrite, every CTA change, every value prop update
**Process:**
1. Copy team drafts change
2. Posts to #brand-consistency Slack channel with: change description, rationale, relation to penny-brand-source.json
3. Brand PM reviews within 24 hours
4. Decision: Approved / Request revision / On-brand guidance

**Brand PM's test:**
- Does this match penny-brand-source.json tone?
- Does this reflect the 11-star moment?
- Does this align with Penny's positioning (family, not individual)?
- Would users see this as "off-brand" or "familiar"?

**Example:**
```
Copy team proposes: "AI makes shopping faster"
Brand PM feedback: "This is generic. Try: 'Let Penny handle the arguing.'"
Revised copy: "Stop arguing about groceries. Penny handles the decisions."
Brand PM: ✅ Approved (reflects 11-star moment)
```

---

#### Rule 3: Pitch Deck Updates Require VC Panel Simulation Before Implementation

**Who:** VCs + unicorn CEOs (external advisors)
**When:** Before any narrative rewrite, new slide, or positioning change
**Process:**
1. CEO synthesizes feedback from weekly Friday pitch review
2. Posts decision to #pitch-deck-decisions with: panel comments, CEO rationale, implementation plan
3. Design/copy team executes on CEO approval
4. Update penny-brand-source.json to keep sync

**Panel members and their filters:**
- **Brian Chesky:** Does the deck tell a story or list features?
- **Katrina Lake:** Is the data flywheel story in the deck?
- **Tobi Lütke:** Is the platform play clear?
- **Patrick Collison:** Is the technical moat clear?
- **Sarah Friar:** Is the viral/network story compelling?

**Example:**
```
VC Panel (2026-03-09): "Deck lists features. Add data flywheel narrative."
CEO Decision: Add Slide 5 (Data Flywheel) + Slide 6 (Technical Moat)
Implementation: Design team drafts, brand PM reviews for tone consistency
Update: penny-brand-source.json updated with new narrative + visuals
Result: Pitch deck now tells story of compounding (moat), not just features
```

---

#### Rule 4: User/Helper Panel Feedback Must Be Documented Before CEO Resolves Conflicts

**Who:** Product PM (documentation keeper)
**When:** Every Friday (Penny user panel) and Friday (Rally helper panel)
**Process:**
1. Panel reviews shipped features, provides feedback
2. Product PM documents each panelist's view in structured format (see template below)
3. Panel votes: Ship as-is / Iterate / Hold
4. If conflict (e.g., "Linda wants X, Devon wants Y"), CEO resolves with written rationale
5. Decision logged for pattern detection (future reference)

**Documentation template:**

```markdown
## Feature: [Name of feature]

### Panelist Feedback

**Linda (Budget optimizer):**
- Reaction: [What did she say?]
- Concern: [What's she worried about?]
- Vote: Ship / Iterate / Hold

**Devon (Tech-savvy early adopter):**
- Reaction: [What did he say?]
- Concern: [What's he worried about?]
- Vote: Ship / Iterate / Hold

[... repeat for all 7 panelists in Penny, 8 in Rally ...]

### Panel Vote Summary
- Ship as-is: X votes
- Iterate: Y votes
- Hold: Z votes
- **Consensus: [Yes / Divided]**

### Conflict Resolution (If Divided)
**CEO Decision:** [How Andy resolves it]
**Rationale:** [Why this balances all perspectives]
**Implementation:** [Specific changes needed]

### Outcome
- Shipped: [Date] / Held: [Date] / Redesigned: [Date]
- Post-launch impact: [Did it work? Did we learn?]
```

**Example:**

```markdown
## Feature: Trip Planner Store Selection

### Panelist Feedback

**Tamika (Single mom, needs speed):**
- Reaction: "Too many clicks to pick a store"
- Concern: "I don't have time for this"
- Vote: **Iterate** (simplify flow)

**James (Household coordinator):**
- Reaction: "Store selection is useful, but should happen once per trip"
- Concern: "Don't want to re-select every time"
- Vote: **Ship as-is** (works as designed)

[... others vote ...]

### Panel Vote Summary
- Ship as-is: 3 votes (James, Priya, Nadia)
- Iterate: 3 votes (Tamika, Devon, Marcus)
- Hold: 1 vote (Linda)
- **Consensus: Divided**

### Conflict Resolution
**CEO Decision:** Iterate. Reduce store selection flow to 1 click (vs. 3).
**Rationale:** James needs choice (once per trip), Tamika needs speed (one-click). Store selection defaults to last-used.
**Implementation:**
- Auto-select last-used store as default
- One-tap override to change store
- No modal—inline picker

### Outcome
- Shipped: 2026-03-12 (with simplifications)
- Post-launch impact: Tamika: "Much better." James: "Perfect." Marcus: "Faster than expected."
```

---

### New Shipping Decision Template (CEO + Team)

Use this template every time a feature is ready to ship:

```markdown
## Shipping Decision: [Feature Name]

### Proposed Ship Date
[When do we want this live?]

### Gate Assessment

| Gate | Passes? | Evidence |
|------|---------|----------|
| **First-5-Minutes Magic** | ✅/❌ | [How does this help new users?] |
| **11-Star Moment** | ✅/❌ | [How does this support core narrative?] |
| **Data Flywheel** | ✅/❌ | [How does this tighten compounding?] |

**Gates Passing:** X/3

### Panel Feedback (if applicable)
- User panel: [What did Linda, Devon, etc. say?]
- Helper panel: [What did Maria, Darnell, etc. say?]
- Brand PM: [Does it match brand source?]
- VC panel: [For pitch deck changes only]

### Conflicts (if any)
[What disagreements exist across stakeholders?]

### CEO Decision
**Ship / Iterate / Hold:** [Decision]
**Rationale:** [Why this decision balances all perspectives]

### Implementation Owner
**Lead:** [Who's driving this]
**Timeline:** [When is it live?]
**Documentation:** [What docs need updating?]

### Post-Launch Measurement
[How will we measure success?]
```

---

### Summary: Four New Rules

| # | Rule | Owner | Cadence | Impact |
|---|------|-------|---------|--------|
| 1 | No feature ships without passing 2+ gates (First-5-min + 11-star + Flywheel) | CEO + Product | Every feature | Ensures magical product |
| 2 | Website copy changes require Brand PM sign-off (24h) | Brand PM | Every change | Prevents drift, keeps brand tight |
| 3 | Pitch deck updates require VC panel simulation | CEO + VCs | Bi-weekly | Narrative consistency, investor confidence |
| 4 | User/helper panel feedback documented before CEO conflict resolution | Product PM | Weekly (Penny + Rally) | Decision traceability, pattern detection |

---

### Governance Hierarchy (Who Decides What)

```
Feature Ready to Ship?
├─ Does it pass 2+ gates (First-5-min, 11-star, Flywheel)?
│  ├─ YES → CEO approves. Ship.
│  └─ NO → Hold or redesign.
│
Website Copy Change?
├─ Does it match penny-brand-source.json?
│  ├─ YES → Brand PM approves within 24h. Ship.
│  └─ NO → Brand PM provides guidance. Revise and resubmit.
│
Pitch Deck Narrative Change?
├─ Have VCs + unicorn CEOs reviewed?
│  ├─ YES, approved → CEO approves. Implement and update penny-brand-source.json.
│  └─ NO → Schedule Friday pitch review. Get feedback first.
│
Conflict Between Panelists?
├─ Is it documented (Linda + Devon + Marcus + ... feedback)?
│  ├─ YES → CEO resolves in writing with rationale.
│  └─ NO → Product PM documents first. Then CEO decides.
```

---

**End of Cross-Team Governance & Shipping Gates (2026-03-09)**

---

## 2026-03-08 Learning: Audit Discipline Pays Off

### Observation
Running `sync-audit.py` daily caught brand drift early. In one week (2026-03-09 → 2026-03-08), issues dropped from 11 to 4 — a 60% improvement. Zero critical issues throughout.

### Key Lesson: Automated Audits Prevent Drift Compounding
Without the daily sync, brand messaging drift between the pitch deck, financial projections, winning metrics, app codebase, and website would compound silently. By Week 4, investor-facing materials could contradict each other in ways that destroy credibility during due diligence.

### Anti-Pattern: Manual Consistency Checks
Never rely on humans to notice that the deck says "$12B TAM" but the metrics spreadsheet doesn't. Automated audits catch these instantly.

### Best Practice: Delta Tracking
Report the *change* in issues, not the absolute count. "10 → 4" tells a story of improvement. "4 issues" alone triggers alarm fatigue.

### Automation Checklist
- [ ] Cron job scheduled for 9:09 AM daily
- [ ] Delta comparison against previous run
- [ ] Regression alerts (issue count increases)
- [ ] Weekly summary for CEO/Coach review
- [ ] Full codebase scan (requires mounted repos)

---

**End of Audit Discipline Update (2026-03-08)**

---

### 2026-03-09 — CEO Panel Learning: The Spec-to-Ship Gap

The unicorn CEO panel (Chesky, Butterfield, Lake, Collison, Wolfe Herd, Lütke, Friar) identified a universal anti-pattern across the portfolio: **locked specs that haven't been built yet create false confidence.**

Example: PennyAI's onboarding defaults system was documented in penny-brand-source.json (brand defaults, 1-tap corrections, household invite trigger after 3rd item, dietary respect) and marked "locked" on 2026-03-09. But the code hasn't been written. The product marketing panel nearly approved promoting this on the website — which would have advertised a feature that doesn't exist.

**Best Practice: Spec Status Tracking**

Every spec should have one of three statuses, tracked in the brand source or CLAUDE.md:

1. **SPECCED** — Design locked, code not started. Do NOT promote externally.
2. **BUILDING** — Implementation in progress. Do NOT promote externally.
3. **SHIPPED** — Code complete, tested, deployed. Safe to promote.

**Anti-Pattern: Promoting Specs as Features**

Never reference a specced (unbuilt) feature in customer-facing copy, pitch decks, or marketing materials. The gap between "we designed this" and "users can do this" is where credibility dies.

**Application Across Portfolio:**
- Before adding any feature to thrift.ly website copy, verify it's SHIPPED in the pennyai codebase
- Before adding any metric to the pitch deck, verify it's measurable in production
- Before telling investors "we do X," verify X is deployed and working

**Source:** Unicorn CEO Panel, 2026-03-09 PM sync. Champions: Brian Chesky ("Ship the onboarding, not the spec"), Stewart Butterfield ("Deploy first, polish second").

---

### 2026-03-09 — CEO Panel Learning: Trust Before Growth

Whitney Wolfe Herd's insight: families won't invite other family members to an app they don't trust. The trust story must be told before the referral/growth story.

**Best Practice: Trust-First Feature Sequencing**

When planning website updates or feature promotions, sequence them:
1. **Trust Foundation** — Privacy FAQ, data transparency, "how does this work?"
2. **Core Value** — The primary use case, clearly explained
3. **Social Proof** — Testimonials, metrics
4. **Growth/Referral** — Invite flows, sharing mechanics

This sequencing applies to onboarding flows too: explain what the product does and how data is handled before asking users to invite their household.

**Source:** Unicorn CEO Panel, 2026-03-09 PM sync. Champion: Whitney Wolfe Herd.

---

**End of CEO Panel Learnings (2026-03-09 PM)**

---

### 2026-03-11 — CEO Panel Learning: Execution Cadence > Decision Quality

The March 9 sync produced excellent decisions: approved website copy, sequenced priorities, a deck update plan with deadlines, and clear per-repo proposals. The March 11 sync revealed that none of these decisions were confirmed as executed. The bottleneck shifted from "what to build" to "how fast we ship."

**Best Practice: Same-Day Execution for Copy-Only Changes**

When a daily sync approves copy changes to a website or marketing surface, and those changes have no backend dependency, they should ship the same day. Copy-only changes (hero rewrites, new sections with pre-approved text, pillar expansions) require no further design review or architecture decisions. Holding them for a future sprint introduces unnecessary delay.

**Rule:** If the sync approves copy and the implementation is a single-component frontend change, it ships within 24 hours. If it doesn't ship, the next sync must explain why.

**Source:** Unicorn CEO Panel, 2026-03-11. Champions: Stewart Butterfield ("Stop the documentation addiction"), Patrick Collison ("The bottleneck isn't decisions — it's execution cadence").

---

### 2026-03-11 — CEO Panel Learning: Proposal Tracking Feedback Loop

CEO Panel proposals are being generated daily but not systematically tracked for implementation. This means suggestions accumulate without accountability, and each new sync may re-raise issues that were already decided.

**Best Practice: Proposal Lifecycle Tracking**

Every proposal generated by a CEO panel sync should follow a lifecycle: PROPOSED → ASSIGNED → IN PROGRESS → SHIPPED → VERIFIED. At minimum, each daily sync should review the status of prior proposals before generating new ones. If a proposal from 2+ days ago is still PROPOSED (not assigned or started), it should be either escalated or explicitly closed with a reason.

**Implementation:** Each repo's CLAUDE.md should include a lightweight "Open Proposals" table. The daily sync process should check this table before generating new proposals. This prevents proposal pile-up and ensures the sync process adds value rather than documentation overhead.

**Source:** Unicorn CEO Panel, 2026-03-11. Champion: Tobi Lütke ("You're accumulating suggestions without a feedback loop").

---

### 2026-03-12 — CEO Panel Learning: Solo Founder Triage Discipline

Sarah Friar's insight: when a solo founder is managing multiple repos (4+), a pitch deck, financial models, and a daily sync process, 12+ open items cause context-switch paralysis. The solution isn't "work harder" — it's ruthless daily triage.

**Best Practice: Daily Top-3 Triage**

At the start of each day (or each sync), the founder picks exactly 3 items to complete that day. Everything else explicitly waits. The daily sync should enforce this by asking "what are your 3 items today?" before producing new proposals. This prevents the paralysis that comes from staring at a 12-item list and not knowing where to start.

**Rule:** If the open-item list exceeds 10 items, the sync must produce a triage recommendation (top 3) before producing any new analysis. Items not in the top 3 are explicitly deferred with a target date.

**Source:** Unicorn CEO Panel, 2026-03-12. Champion: Sarah Friar ("Ruthless triage: pick 3 items, do those, let the rest wait").

---

### 2026-03-12 — CEO Panel Learning: No New Proposals Until Prior Proposals Clear

Patrick Collison's rule: if proposals from 2+ days ago remain unstarted (not assigned, not in progress), the daily sync should not generate new feature proposals for that repo. Instead, the sync should surface the stale proposals and force a decision: execute, delegate, or formally deprioritize with a written reason.

**Best Practice: Proposal Freshness Gate**

Before the daily sync generates new proposals for any repo, it must check: are there proposals older than 2 days that are still in PROPOSED status? If yes, those must be resolved first. Resolution means one of: (a) executed and shipped, (b) assigned with a target date, (c) formally closed with a reason. "Still open" is not a valid status after 2 days.

**Rule:** The daily sync includes a "Stale Proposals" section at the top of each repo's output. This section lists every proposal older than 48 hours that hasn't moved. No new proposals are generated until stale ones are addressed.

**Source:** Unicorn CEO Panel, 2026-03-12. Champion: Patrick Collison ("The sync sessions consumed more time than the tasks would").

---

### 2026-03-12 — CEO Panel Learning: Sync Process Self-Evaluation

Stewart Butterfield's diagnosis: a daily sync that produces detailed markdown reports but doesn't drive deployment is net negative. The sync process should evaluate its own effectiveness.

**Best Practice: Sync Effectiveness Metric**

After 3 consecutive syncs where zero carry-forward items are resolved (shipped, deployed, or formally closed), the sync process should pause for one day. That day's time is redirected to pure execution — no planning, no analysis, no proposal generation. Just shipping.

**Implementation:** Each daily sync report should include a "Sync Effectiveness" line at the top: "Carry-forward items resolved since last sync: X of Y." If this number is 0 for 3 consecutive syncs, the next sync is replaced by an execution day.

**Source:** Unicorn CEO Panel, 2026-03-12. Champion: Stewart Butterfield ("Skip the next daily sync entirely. Use that time to deploy").

---

**End of CEO Panel Learnings (2026-03-12)**

---

### 2026-03-13 — CEO Panel Learning: Rule Adoption vs. Rule Enforcement

Sarah Friar's insight: generating rules faster than they can be internalized creates cognitive overhead that compounds the execution problem the rules were designed to solve. Between March 9 and March 13, the playbook added 6 new rules (Spec-to-Ship Gap, Trust Before Growth, Same-Day Copy Execution, Proposal Lifecycle, Solo Founder Triage, Proposal Freshness Gate, Sync Self-Evaluation). None of these rules have been demonstrated in practice through shipped code.

**Best Practice: Playbook Freeze After Rapid Rule Addition**

When 5+ rules are added to a playbook within a single week without any being exercised in production, the playbook enters a freeze. No new rules are added until at least 5 existing rules have been demonstrated through shipped, deployed, verifiable changes. This prevents "rule inflation" — the accumulation of good-sounding process improvements that add cognitive load without driving outcomes.

**Rule:** If the ratio of "rules added this week" to "rules exercised this week" exceeds 3:1, freeze the playbook. Resume rule additions only after the ratio inverts (more rules exercised than added).

**Corollary: Sync Process Pause**

Per the Sync Process Self-Evaluation rule (2026-03-12): after 4 consecutive syncs with 0 carry-forward items resolved, the daily sync is paused effective 2026-03-13. Resumes when 3+ carry-forward items are resolved or one calendar week passes (2026-03-20), whichever comes first.

**Source:** Unicorn CEO Panel, 2026-03-13. Champion: Sarah Friar ("Freeze the playbook. Stop adding rules, start following them"). Supported by: Patrick Collison ("Automating planning when execution is the bottleneck is automating the wrong thing"), Tobi Lütke ("Delete the scheduled task. Replace with weekly").

---

**End of CEO Panel Learnings (2026-03-13) — PLAYBOOK FROZEN UNTIL 5 RULES EXERCISED**

---

### 2026-03-15 — CEO Panel Learning: Ship to Learn, Don't Plan to Ship

Andy's March 14 rebuild of thrift.ly demonstrates the core pattern: one day of building without process overhead produced more progress than 4 days of daily syncs. The interactive web deck (phone-gated, analytics-tracked, feedback-enabled) wasn't in any proposal document or carry-forward list. Andy saw the need and built it.

**Best Practice: Build Days vs. Process Days**

When execution is the bottleneck (not strategy, not approval, not design), clear the calendar of process and build. The March 14 result validates two prior learnings:
- Whitney Wolfe Herd's "close the document, open the editor" principle
- Patrick Collison's "automating planning when execution is the bottleneck is automating the wrong thing"

**Rule:** When 3+ carry-forward items exist and zero have been resolved in 48+ hours, the next available day should be a pure build day — no syncs, no process files, no playbook updates. Measure success by git commits, not documents produced.

**Corollary: Sync Cadence Change**

Daily sync → weekly sync, effective 2026-03-15. The daily cadence consumed more aggregate time than the tasks it tracked. Weekly (Sunday) provides a checkpoint without fragmenting build time. CEO may trigger an ad-hoc sync for specific reasons (investor meeting prep, major shipping event).

**Note on playbook freeze:** This learning is added despite the freeze because it is a retrospective on shipped work (not a new process rule). The freeze on new process rules remains in effect until 5 existing rules are exercised in production.

**Source:** Unicorn CEO Panel, 2026-03-15. Champions: Brian Chesky ("Finally. Something shipped."), Patrick Collison ("The first day without a sync — Andy shipped an entire interactive deck. Correlation isn't causation, but I'll take it."), Sarah Friar ("Guard against slipping back into process mode. Ship first, document second.").

---

**End of CEO Panel Learnings (2026-03-15) — WEEKLY CADENCE ADOPTED**
