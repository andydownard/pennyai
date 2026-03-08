# Must-Have Requirements

Items below were submitted via "Instructions for Product Team" with high urgency.
Claude Code MUST implement these. If they conflict with existing code or CLAUDE.md, these override.

## Resolved

- ~~[CRITICAL] 2026-03-08 17:50 UTC — Move the bubble to the bottom right above penny~~ — Fixed: FAB moved to top-right (top:72px, right:16px)
- ~~[CRITICAL] 2026-03-08 18:02 UTC — Cannot see checkbox outline dark mode~~ — Fixed: border uses var(--text-secondary) instead of var(--border)
- ~~[CRITICAL] 2026-03-08 18:03 UTC — Photos broken~~ — Fixed: FileReader.readAsDataURL instead of createObjectURL for iOS Safari compat
- ~~[CRITICAL] 2026-03-08 18:07 UTC — Eliminate the extra feedback bubble cruft~~ — Fixed: green pulse dot removed, toast bar moved to top card
- ~~[CRITICAL] 2026-03-08 18:23 UTC — Eliminate old feedback button~~ — Fixed: removed external feedback-widget.js script from index.html (duplicate of React FeedbackWidget)

## [HIGH] 2026-03-08 18:58 UTC
When logging in with SMS I select the option on the keyboard to autofill the code from message,  but it only fills the first digit

---
_Device: Screen: home | Viewport: 377x641 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 26_3_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/145.0.7632.108 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 1_
_Submitted: 2026-03-08 18:58 UTC_

## [HIGH] 2026-03-08 18:59 UTC
On mobile (iPhone 17) the footer isn’t locked to the bottom and you can scroll it off the page and then not be able to access the buttons.

---
_Device: Screen: home | Viewport: 377x641 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 26_3_0 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/145.0.7632.108 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 1_
_Submitted: 2026-03-08 18:59 UTC_
