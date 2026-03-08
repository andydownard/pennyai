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
- ~~[CRITICAL] 2026-03-08 21:01 UTC — Login code autofill only fills first digit~~ — Fixed: removed maxLength={1} that clipped iOS autofill, added native input listener fallback
- ~~[CRITICAL] 2026-03-08 21:42 UTC — Snap receipt shown when no swaps~~ — Fixed: gated behind hasSwapsToClaim prop
- ~~[CRITICAL] 2026-03-08 21:42 UTC — Keep keyboard up when selecting urgency~~ — Fixed: replaced <select> dropdown with tappable chips
- ~~[CRITICAL] 2026-03-08 21:43 UTC — Reconsider urgency to low/normal/urgent~~ — Fixed: chip options are Low, Normal, Urgent
- ~~[CRITICAL] 2026-03-08 21:45 UTC — Feedback bubble for approved numbers only, on every page~~ — Fixed: isFeedbackUser checks FeedbackPhones, widget renders on all authenticated screens
- ~~[CRITICAL] 2026-03-08 21:50 UTC — Include shopping experience, not just list creation~~ — Fixed: feedback widget now shows on all screens including during shopping trips

- ~~[CRITICAL] 2026-03-08 21:01 UTC — Show more items, less scrolling~~ — Fixed: compact item rows (logo 52→36px, name 17→15px, padding reduced), compact department headers, compact Penny bar/tabs
- ~~[CRITICAL] 2026-03-08 21:01 UTC — Items in store-aisle order~~ — Already implemented: items grouped by DEPARTMENTS array in store-walk order (Produce → Bakery → ... → Other)
- ~~[CRITICAL] 2026-03-08 21:01 UTC — Auto-save list~~ — Fixed: checked items auto-saved as "Trip [date]" when finishing a trip
- ~~[CRITICAL] 2026-03-08 21:01 UTC — Camera as top-level button~~ — Fixed: separate camera + photo library buttons in input bar

## [CRITICAL] 2026-03-08 21:01 UTC — Remaining items

Seems like there should be a master list that’s automatically compiled as you go

Maybe one master list per store: WF, Costco etc. and you can easily switch between them

When in camera mode, maybe be able to stay there to add multiple items (submit one at a time) one after the other without always tapping +.  Ie the item confirmation could appear briefly over top of the camera mode and then disappear.

---
_Device: Screen: home | Viewport: 394x699 | UA: Mozilla/5.0 (iPhone; CPU iPhone OS 18_7 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.3 Mobile/15E148 Safari/604.1 | Dark: dark | Items: 0_
_Submitted: 2026-03-08 21:01 UTC_
