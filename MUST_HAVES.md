# Must-Have Requirements

Items below were submitted via "Instructions for Product Team" with high urgency.
Claude Code MUST implement these. If they conflict with existing code or CLAUDE.md, these override.

## Resolved

- ~~[CRITICAL] 2026-03-08 17:50 UTC — Move the bubble to the bottom right above penny~~ — Fixed: FAB moved to top-right (top:72px, right:16px)
- ~~[CRITICAL] 2026-03-08 18:02 UTC — Cannot see checkbox outline dark mode~~ — Fixed: border uses var(--text-secondary) instead of var(--border)
- ~~[CRITICAL] 2026-03-08 18:03 UTC — Photos broken~~ — Fixed: FileReader.readAsDataURL instead of createObjectURL for iOS Safari compat
- ~~[CRITICAL] 2026-03-08 18:07 UTC — Eliminate the extra feedback bubble cruft~~ — Fixed: green pulse dot removed, toast bar moved to top card
- ~~[CRITICAL] 2026-03-08 18:23 UTC — Eliminate old feedback button~~ — Fixed: removed external feedback-widget.js script from index.html (duplicate of React FeedbackWidget)
- ~~[HIGH] 2026-03-08 18:58 UTC — SMS autofill only fills first digit~~ — Fixed: detect multi-digit iOS autofill paste and distribute across all 6 code inputs
- ~~[HIGH] 2026-03-08 18:59 UTC — Footer scrolls off page on iPhone~~ — Fixed: body position:fixed + overflow:hidden + overscroll-behavior:none prevents iOS elastic bounce

## [CRITICAL] 2026-03-08 21:01 UTC
Bug: login code that appears on top of keyboard doesn’t copy fully into the boxes (only the first number does)

Need to show more items on screen so there’s less scrolling

Show items in same order as picking in the store

Saving the list should be automatic

Seems like there should be a master list that’s automatically compiled as you go

Maybe one master list per store: WF, Costco etc. and you can easily switch between them

Camera itself should be top level button; maybe Photo library too

When in camera mode, maybe be able to stay there to add multiple items (submit one at a time) one after the other without always tapping +.  Ie the item confirmation could appear briefly over top of the camera mode and then disappear.

---
_Device: Screen: home | Viewport: 394x699 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 0_
_Submitted: 2026-03-08 21:01 UTC_

## [CRITICAL] 2026-03-08 21:42 UTC
Snap your receipt should not be displayed when there’s no swaps to claim. Need empty state. Keep keyboard up when I select urgency here. Make sure photo capture works for receipt

---
_Device: Screen: home | Viewport: 394x374 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 4_
_Submitted: 2026-03-08 21:42 UTC_

## [CRITICAL] 2026-03-08 21:43 UTC
Reconsider urgency to low, normal, urgent

---
_Device: Screen: home | Viewport: 394x374 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 4_
_Submitted: 2026-03-08 21:43 UTC_

## [CRITICAL] 2026-03-08 21:45 UTC
Idea bubble is for approved numbers only. Must have. And it appears on every page

---
_Device: Screen: home | Viewport: 394x374 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 4_
_Submitted: 2026-03-08 21:45 UTC_

## [CRITICAL] 2026-03-08 21:50 UTC
Also it should include the experience while shopping, not just creating the list

---
_Device: Screen: home | Viewport: 394x699 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 4_
_Submitted: 2026-03-08 21:50 UTC_
