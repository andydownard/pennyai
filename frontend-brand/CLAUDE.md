# Groflo Brand Portal (frontend-brand)

> The B2B dashboard at `brands.groflo.io` where CPG brand managers create deals, view competitive intel, and fund their accounts.

## Architecture

- **Frontend**: React/TypeScript single-page app (`App.tsx` + components), Vite build
- **Backend**: Same Go server as consumer app, routes prefixed with `/brand/`
- **Auth**: Email/password with `brand_users` table (separate from consumer `users`)
- **Build output**: `../static-brand/` (served by Go at `brands.groflo.io` subdomain)
- **Dev server**: `http://localhost:5174`, proxies `/brand/*` to Go backend at `:8080`

## Relationship to Consumer App (`frontend/`)

These are **two separate React apps** for **two separate audiences**:

| | Consumer App (`frontend/`) | Brand Portal (`frontend-brand/`) |
|---|---|---|
| **User** | Shoppers | CPG brand managers |
| **Design** | Mobile-first, Avenir font, warm tones | Desktop-first, system fonts, professional |
| **Auth** | Phone/SMS, Google OAuth, email | Email/password only |
| **DB tables** | `users`, `sessions` | `brand_users`, `brand_sessions` |
| **Port** | 5173 (dev) | 5174 (dev) |
| **Build dir** | `dist/` | `static-brand/` |

### What IS Shared
- Go backend (same binary, same DB)
- Database (brand portal reads from `swap_events`, `brand_deals`, etc.)
- Design principles (clean, minimal, trustworthy)
- TypeScript conventions (inline styles, no CSS framework)
- API patterns (`credentials: 'include'`, JSON bodies, error shapes)

### What is NOT Shared
- React components (no imports between `frontend/` and `frontend-brand/`)
- Design tokens (different color palette, different typography)
- Auth system (completely separate user tables and sessions)
- Build pipeline (separate Vite configs, separate `npm run build`)

## Design Tokens

The brand portal now uses the **canonical brand green** aligned with penny-brand-source.json:

```typescript
// Brand portal tokens (aligned with brand source 2026-03-09)
const BRAND = '#35644F'       // Canonical brand green (matches consumer app + website)
const BRAND_LIGHT = '#EEF7F2' // Light green for highlights (green-50 from brand source)
const TEXT = '#1A1A1A'         // Near-black for readability
const TEXT_SEC = '#666'        // Secondary text
const BORDER = '#E0E0E0'      // Light gray borders
const WHITE = '#FFF'
const BG = '#FAFAFA'           // Off-white background
const ERROR = '#D32F2F'        // Material red for errors
const FONT = "'Inter', -apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif"
```

**Design philosophy:** Think Stripe Dashboard or Linear, not consumer mobile app. Information-dense, desktop-primary, professional. Brand managers use this on laptops during work hours.

## Screens

| Screen | Route State | Description |
|--------|-------------|-------------|
| `landing` | Unauthenticated | "Put your product on every grocery list" landing page |
| `login` | Unauthenticated | Email/password sign-in |
| `register` | Unauthenticated | Brand account creation |
| `dashboard` | Authenticated | Metric cards, daily chart, quick actions |
| `deals` | Authenticated | List of deals with status, stats, pause/resume/delete |
| `create-deal` | Authenticated | Form to create a new swap deal |
| `intel` | Authenticated | Competitive Swap Intel — category search, swap-out data |
| `transactions` | Authenticated | Billing history, current balance |
| `deposit` | Authenticated | Add funds form |

## API Contract (Brand Portal Routes)

All routes prefixed with `/brand/`:

| Method | Path | Description |
|--------|------|-------------|
| POST | `/brand/register` | Create brand account |
| POST | `/brand/login` | Sign in |
| POST | `/brand/logout` | Sign out |
| GET | `/brand/me` | Current user + balance |
| GET | `/brand/dashboard` | Aggregate metrics (deals, swaps, spend) |
| GET | `/brand/deals` | List brand's deals with stats |
| POST | `/brand/deals` | Create a deal |
| PUT | `/brand/deals/:id` | Update deal (status, fields) |
| DELETE | `/brand/deals/:id` | Delete a deal |
| GET | `/brand/category-intel/:category` | Competitive intel for a category |
| GET | `/brand/transactions` | Transaction history |
| POST | `/brand/deposit` | Add funds (Stripe Checkout) |

## Coding Standards

### TypeScript
- Inline styles (same as consumer app — no CSS-in-JS libraries)
- Functional components with hooks
- `api<T>()` helper for typed fetch calls
- Error handling: catch → display in form, `UNAUTHORIZED` → redirect to login

### Component Structure
- Keep App.tsx as the router/layout shell
- Extract sub-components when they exceed ~100 lines
- Sub-components at bottom of App.tsx (like `CreateDealForm`, `IntelPanel`) are fine for now
- Extract to separate files when App.tsx exceeds ~1000 lines

### Patterns
- Screen navigation via `useState<Screen>` (same pattern as consumer app)
- Data loading via `useEffect` on screen change
- Forms: controlled inputs, `onSubmit` handlers, loading/error state
- `formatCents()` for all money display

## Testing

- Go backend tests cover brand portal API routes
- E2E tests (when added) should use a separate Playwright config
- Brand portal E2E tests run against `http://localhost:5174`

## What NOT to Do

- Do NOT import from `../frontend/src/` — the two frontends are independent
- Do NOT use the consumer app's design tokens (Avenir, #35644F, etc.)
- Do NOT add consumer features to the brand portal
- Do NOT share React components between the two apps
- Do NOT bolt brand portal screens onto the consumer App.tsx
