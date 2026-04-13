# Consider for Roadmap

Items below were submitted via "Instructions for Product Team" for consideration.
Claude Code should evaluate these when planning future work.

## Resolved

- ~~[NORMAL] 2026-03-08 21:43 UTC — App is sloppy on iPhone, dragging stuff~~ — Fixed: touch-action:manipulation, user-select:none, compact layout
- ~~[NORMAL] 2026-03-08 21:46 UTC — Email sign in doesn't work~~ — Fixed: -webkit-appearance:none + autofill style overrides for iOS Safari
- ~~[NORMAL] 2026-03-08 21:55 UTC — "flour"/"rigatoni" not recognized~~ — Fixed: explicit AI parser instruction for single-word grocery items
- ~~[NORMAL] 2026-03-08 21:55 UTC — Type corrections directly in chat~~ — Already supported: chat input handles edits ("change X to Y"), placeholder says "Add, edit, or ask Penny"
- ~~[NORMAL] 2026-03-08 21:57 UTC — Section labels take too much vertical space~~ — Fixed: compact dept headers (11px font, 4px padding, 16px emoji, 32px icon area)
- ~~[NORMAL] 2026-03-08 22:00 UTC — Bottom UI too tall~~ — Fixed: Penny text hides when keyboard open, compact chat bar, 48px tab bar
- ~~[NORMAL] 2026-03-09 12:01 UTC — Penny says intro greeting even when list has items~~ — Fixed: Penny now says "What can I add to your list?" when user has existing items
- ~~[NORMAL] 2026-03-09 12:06 UTC — NPS showing every time~~ — Fixed: switched sessionStorage→localStorage so dismiss persists across reloads
- ~~[NORMAL] 2026-03-09 12:09 UTC — Share button text unclear~~ — Fixed: "Invite someone to your shared grocery list"
- ~~[NORMAL] 2026-03-09 12:13 UTC — Shared list says "Someone's shopping list"~~ — Fixed: shows "Shopping List" when no sharer name, shows "[Name]'s Shopping List" when name available
- ~~[NORMAL] 2026-03-09 12:11 UTC — Share list should let recipient join without leaving page~~ — Fixed: inline phone auth flow on shared list page (enter phone → verify code → auto-join household)
- ~~[NORMAL] 2026-03-09 12:06 UTC — Bug reporting button on all pages~~ — Fixed: BugReportFAB component (subtle bug icon bottom-left) on all authenticated screens, bottom sheet bug report form
- ~~[NORMAL] 2026-03-08 21:50 UTC — "More automatic, more delightful, fewer taps"~~ — Accepted as **F1: UX Friction Audit** (P1, Size M) in ROADMAP.md. Targets: auto-submit for unambiguous items, swipe-to-check, reduced brand confirmation for commodity items. Sprint-ready.
- ~~[NORMAL] 2026-03-08 21:58 UTC — Drag sections to match store order~~ — Accepted as **F3: Auto-Learn Store Department Order** (P2, Size M) in ROADMAP.md. Redesigned from manual drag to auto-learn (tracks check-off sequence per store, reorders after 3+ trips). Manual drag fails Gate 3; auto-learn passes it. Needs UX design round before sprint.
- ~~[NORMAL] 2026-03-10 12:46 UTC — F7: Bug report button on all pages~~ — Accepted as P3 audit task — verify BugReportFAB renders on all 12 screens + overlay states. BugReportFAB was already implemented (see earlier resolved item from 2026-03-09 12:06); this is a follow-up to audit coverage completeness.
- ~~[NORMAL] 2026-03-10 12:47 UTC — F9: NPS popup showing every time~~ — Fixed: npsMinLogins 3->10, localStorage dismiss changed to 90-day timestamp expiry. Server-side guard recommended.

## Remaining

### [NORMAL] 2026-03-08 21:56 UTC — F2: Show actual product images
You should show actual images of the product (either the full item or a close-up) for the icon. Generic images or icons are not helpful.

**Product team decision:** Deferred. Requires product image API integration (Kroger, Walmart, or Open Food Facts). Currently using brand logos which are available from our 3-layer brand logo cache. Product photos are a meaningful upgrade but depend on retailer API partnerships (see Kroger/Walmart API items in Future section of ROADMAP.md). Revisit when retailer integrations ship.

---
_Device: Screen: home | Viewport: 403x395 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 26_3_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/145.0.7632.108 Mobile/15E148 Safari/604.1 | Dark: light | Items: 10_
_Submitted: 2026-03-08 21:56 UTC_

### [NORMAL] 2026-03-09 12:02 UTC — F4: Brand self-serve deal entry
have penny reach out to brands and let them self serve enter deals to get me to buy their brand

**Product team decision:** Already shipped. Brand Portal v0 (M4) at brands.thrift.ly provides self-serve deal creation, budget management, and competitive intel. No further action needed — this is the core product.

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 17_
_Submitted: 2026-03-09 12:02 UTC_

### [NORMAL] 2026-03-09 12:02 UTC — F5: Groflo coupon integration
load groflo deals if they have an mcp

**Product team decision:** Deferred to Phase 4+. Already tracked as "Groflo Coupon Integration" in Nice-to-Have section of ROADMAP.md. Would automate deal supply but not critical while deals are manually seeded and brand portal handles self-serve. Revisit when deal supply becomes the bottleneck.

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 17_
_Submitted: 2026-03-09 12:02 UTC_

### [NORMAL] 2026-03-09 12:03 UTC — F6: Kroger MCP for correct logos
pull correct logo from the kroger mcp.  https://www.dropbox.com/scl/fi/ajkxjmmzmnjl5mhni8ggi/Screenshot-2026-03-09-at-7.02.31-AM.png?rlkey=axbqd0w28xmjajz1z5czb7mp4&dl=0

**Product team decision:** Deferred. Depends on Kroger API Integration (tracked in Future section of ROADMAP.md). Would provide accurate product images/logos per retailer. Currently using our 3-layer brand logo cache which covers brand logos but not SKU-level product images. Revisit alongside F2 when retailer API partnerships ship.

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 17_
_Submitted: 2026-03-09 12:03 UTC_


### [NORMAL] 2026-03-09 12:14 UTC — Scout on share page
add scout to the share page as 4th option to help get folks to get us to our ICPs (consumers and brand managers)

**Product team decision:** Rejected for share page per F6 review. B2B acquisition tools don’t belong on consumer share page. Scout is a separate strategic initiative for brand manager outreach, not a consumer-facing feature.

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 18_
_Submitted: 2026-03-09 12:14 UTC_

- ~~[NORMAL] 2026-03-10 12:46 UTC — F8: Household SMS notifications~~ — Phase 1 shipped: invite flow now shows phone input + "Send SMS invite" button. Phase 2 shipped: daily digest SMS with TCPA-compliant opt-in toggle, 20-hour dedup window, hourly ticker. ✅ Done

### [NORMAL] 2026-03-10 12:49 UTC — F10: Brand outreach for Penny Swaps
please reach out to brand managers for the items in my lists to my brands and competitve brands for each item on my list and get me Penny Swap deals for cheaper and healthier options

**Product team decision:** Deferred (duplicate of F4/Tod/Scout). B2B outreach automation is a separate strategic initiative, not a consumer app feature. Already covered by Brand Portal v0 (M4) at brands.thrift.ly for self-serve deal creation.

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 18_
_Submitted: 2026-03-10 12:49 UTC_

- ~~[NORMAL] 2026-03-10 16:07 UTC — No save button on profile page~~ — By design: profile fields auto-save on blur (500ms debounce). No explicit save button needed.

- ~~[NORMAL] 2026-03-15 13:20 UTC — Waterloo grape picture wrong~~ — Fixed: "Waterloo" was missing from the brands table in schema.go. Added with domain "waterloodrinks.com" so logo resolution fetches the correct sparkling water brand logo instead of falling back to "waterloo.com".

- ~~[NORMAL] 2026-03-15 13:23 UTC — Add "Brands" button to consumer app~~ — Fixed: Added "For Brands" link in NavMenu (bottom, above close hint) that opens brand.thrift.ly in a new tab. Subtle external-link icon indicates it opens a new window.

### [NORMAL] 2026-03-15 13:24 UTC — F11: Brand Portal phone/SMS login
[Brand Portal] let brands login with cell and texted code too, then confirm email from the brand.

**Product team decision:** Deferred. Requires extending brand auth system with phone verification flow (Twilio integration for brand portal), plus email confirmation step. Currently email/password is sufficient for B2B users. Phone login makes more sense when brand portal has mobile-optimized workflows. Revisit when brand portal usage grows beyond 20 active brands.

---
_Submitted: 2026-03-15 13:24 UTC_

### [NORMAL] 2026-03-15 13:25 UTC — F12: Consumer demand signals for brands
[Brand Portal] show all the items that are in shopping lists from consumers so far just like the consumer side, and have a "make offer" button next to each one that takes them to the create new deal page.

**Product team decision:** Accepted as **F12: Brand Market Intelligence Dashboard** (P2, Size L) for ROADMAP.md. Passes Gate 3 — demand signals from real shopper lists are the core data flywheel for B2B. Shows brands what products shoppers actually buy, aggregated anonymously. "Make Offer" button pre-fills the create deal form. Needs privacy review (aggregate only, no PII). Add to Phase 2 brand portal work.

---
_Submitted: 2026-03-15 13:25 UTC_

### [NORMAL] 2026-03-15 13:26 UTC — F13: Kroger MCP preload for brand products
[Brand Portal] preload this with the kroger mcp lookup of the brands that cpg company offers.

**Product team decision:** Deferred. Depends on Kroger API Integration (already tracked in Future section of ROADMAP.md). Would auto-populate a brand's product catalog from Kroger's database when they sign up. Revisit when Kroger API partnership ships.

---
_Submitted: 2026-03-15 13:26 UTC_

### [NORMAL] 2026-03-15 13:28 UTC — F14: Deal type radio buttons
[Brand Portal] have three radio buttons for the deal type to select. free sample. buy one get one free. $ discount.

**Product team decision:** Already exists. The brand portal create deal form already has a deal type selector with 4 options: SKU Swap ($ discount), BOGO (buy one get one), Trial (free sample), and Seasonal. Maps exactly to the request: "$ discount" = SKU Swap, "buy one get one free" = BOGO, "free sample" = Trial. No changes needed.

---
_Submitted: 2026-03-15 13:28 UTC_

### [NORMAL] 2026-03-15 13:29 UTC — F15: Brand product analytics view
[Brand Portal] list all items for all brands here, just like the shopping list view, and i can click on any one and see the details we have on it, including for the receipts which items are frequently bought together.

**Product team decision:** Deferred to Enterprise Brand Portal (already tracked in Future section of ROADMAP.md). Requires receipt data aggregation, market basket analysis, and SKU-level analytics. This is a premium analytics feature for P&G-tier brands. Revisit when receipt scanning (fully_free campaign type) generates enough data for meaningful "frequently bought together" insights.

---
_Submitted: 2026-03-15 13:29 UTC_

### [NORMAL] 2026-03-15 13:29 UTC — F16: Brand portal team sharing
[Brand Portal] have the same share button on this as the consumer side. let folks share it to their team, and share it to anyone.

**Product team decision:** Deferred to Enterprise Brand Portal (already tracked in Future section of ROADMAP.md as "Multi-user team accounts, SSO, invoicing"). Team sharing requires role-based access (admin, analyst, viewer), invite management, and shared deal ownership. Target: after 50+ paying brands.

---
_Submitted: 2026-03-15 13:29 UTC_

### [NORMAL] 2026-03-15 13:30 UTC — F17: Deal share links with affiliate tracking (Klaviyo)
[Brand Portal] when i create a deal, let me copylink to send it in klayvio to my current customer base. those share links are embedded with my cell# so I get affiliate credit for users signed through it.

**Product team decision:** Accepted as **F17: Affiliate Deal Share Links** (P3, Size M) for ROADMAP.md. Concept: each deal gets a shareable link with embedded referrer tracking (brand_user_id or phone). When a consumer signs up through that link, the referrer gets affiliate credit. Integrates with existing referral system. Passes Gate 3 (viral distribution data). Needs design for attribution model and payout terms. Add to Phase 3.

---
_Submitted: 2026-03-15 13:30 UTC_

- ~~[NORMAL] 2026-03-16 11:51 UTC — Earnings amount mismatch between header badge and dashboard~~ — Fixed: header badge used `swap_credits_cents` (confirmed only) while earnings dashboard summed ALL rebates including `pending_purchase`. Added `onEarningsLoaded` callback to sync the header badge with the earnings dashboard total when earnings screen loads.

---
_Device: Screen: earnings | Viewport: 394x374 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1 | Dark: light | Items: 15_
_Submitted: 2026-03-16 11:51 UTC_

## [NORMAL] 2026-03-19 10:52 UTC
have the url in the browser for my shopping list have my affiliate code embeeded so i can share it

---
_Device: Screen: home | Viewport: 1710x906 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 | Dark: dark | Items: 22_
_Submitted: 2026-03-19 10:52 UTC_

## [NORMAL] 2026-04-09 18:11 UTC
would love to have the share&earn button

---
_Device: Screen: home | Viewport: 1710x904 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 | Dark: light | Items: 12_
_Submitted: 2026-04-09 18:11 UTC_

## [NORMAL] 2026-04-09 18:12 UTC
need a button to share my list with those in my house who shop together

---
_Device: Screen: home | Viewport: 1710x904 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 | Dark: light | Items: 12_
_Submitted: 2026-04-09 18:12 UTC_

## [NORMAL] 2026-04-09 18:16 UTC
may want to put quantity on the list view

---
_Device: Screen: home | Viewport: 1710x904 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 | Dark: light | Items: 14_
_Submitted: 2026-04-09 18:16 UTC_

## [NORMAL] 2026-04-09 18:17 UTC
it is putting a lot of items in other category

---
_Device: Screen: home | Viewport: 1710x904 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 | Dark: light | Items: 15_
_Submitted: 2026-04-09 18:17 UTC_

## [NORMAL] 2026-04-09 18:21 UTC
when i make an update to the list, please send link via text to a group chat of all folks with whom i've shared the list.  so we have a group shopping chat group and easy to find the list link when someone goes shopping

---
_Device: Screen: home | Viewport: 1710x904 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/146.0.0.0 Safari/537.36 | Dark: light | Items: 21_
_Submitted: 2026-04-09 18:21 UTC_

---

## Harvested from Unicorn CEO Panel — 2026-03-09 (archived in `archive/CEO-PANEL-UNICORN-2026-03-09.md`)

### [STRATEGIC] Patrick Collison — Reliability & graceful degradation
"Every endpoint is a promise." Specific asks:
- Load-test household coordination under 50 concurrent users editing the same list (SQLite WAL limits under contention).
- Claude dependency fallback: define graceful-degrade path when Anthropic has a bad hour. Users should see degraded features, not errors.
- Chaos test: kill Claude API, kill DB, kill SMS — does the core list flow still work?
- Define the 99.9% availability story before scaling past 10K households.

**Product team decision:** _pending_. Ranking against data flywheel and household coordination P0s. Chaos testing is a Gate 3 enabler (compounding data is worthless if the core flywheel drops signal during outages).

---

### [STRATEGIC] Whitney Wolfe Herd — Household decision-making model
Households aren't egalitarian. UI currently treats "everyone can add items" as equal, but:
- Who decides brand when Alex adds "fancy pasta" and Jamie adds "budget pasta"?
- Who sees household feedback/complaints?
- Should households pick an operating model: "I decide brands" vs "Everyone votes" vs "Penny surprises us"?

**Product team decision:** _pending_. Intersects with dietary-attribute conflict resolution (what if members have conflicting preferences?) and swap acceptance semantics (per-user is current rule — but brand choice on a shared list is ambiguous). Worth a short focus-group ping with James (household coordinator persona) before spec work.

---
