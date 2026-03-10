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

## Remaining

### [NORMAL] 2026-03-08 21:50 UTC
I wonder what would happen if you said, "I'd like the experience to be more automatic, more delightful, and require fewer taps."

---
_Device: Screen: home | Viewport: 394x374 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 4_
_Submitted: 2026-03-08 21:50 UTC_

### [NORMAL] 2026-03-08 21:56 UTC
You should show actual images of the product (either the full item or a close-up) for the icon. Generic images or icons are not helpful.

---
_Device: Screen: home | Viewport: 403x395 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 26_3_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/145.0.7632.108 Mobile/15E148 Safari/604.1 | Dark: light | Items: 10_
_Submitted: 2026-03-08 21:56 UTC_

### [NORMAL] 2026-03-08 21:58 UTC
I should be able to drag whole sections so they match the order I will buy the items in my store

---
_Device: Screen: home | Viewport: 403x395 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 26_3_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/145.0.7632.108 Mobile/15E148 Safari/604.1 | Dark: light | Items: 10_
_Submitted: 2026-03-08 21:58 UTC_

## [NORMAL] 2026-03-09 12:02 UTC
have penny reach out to brands and let them self serve enter deals to get me to buy their brand

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 17_
_Submitted: 2026-03-09 12:02 UTC_

## [NORMAL] 2026-03-09 12:02 UTC
load groflo deals if they have an mcp

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 17_
_Submitted: 2026-03-09 12:02 UTC_

## [NORMAL] 2026-03-09 12:03 UTC
pull correct logo from the kroger mcp.  https://www.dropbox.com/scl/fi/ajkxjmmzmnjl5mhni8ggi/Screenshot-2026-03-09-at-7.02.31-AM.png?rlkey=axbqd0w28xmjajz1z5czb7mp4&dl=0

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 17_
_Submitted: 2026-03-09 12:03 UTC_


## [NORMAL] 2026-03-09 12:14 UTC
add scout to the share page as 4th option to help get folks to get us to our ICPs (consumers and brand managers)

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 18_
_Submitted: 2026-03-09 12:14 UTC_

## [NORMAL] 2026-03-10 12:46 UTC
have the feature/bug fix button on all pages and tabs

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 18_
_Submitted: 2026-03-10 12:46 UTC_

## [NORMAL] 2026-03-10 12:46 UTC
Wendy couldn’t remember where the list was.   Please do a group text with a link to the list when it is created, and new folks are invited, and once per day when a new item is added to a list

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 18_
_Submitted: 2026-03-10 12:46 UTC_

## [NORMAL] 2026-03-10 12:47 UTC
I get NPS popup  every time i come to the site.  ask it once a week

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 18_
_Submitted: 2026-03-10 12:47 UTC_

## [NORMAL] 2026-03-10 12:49 UTC
please reach out to brand managers for the items in my lists to my brands and competitve brands for each item on my list and get me Penny Swap deals for cheaper and healthier options

---
_Device: Screen: home | Viewport: 1710x907 | UA: Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.36 | Dark: dark | Items: 18_
_Submitted: 2026-03-10 12:49 UTC_
