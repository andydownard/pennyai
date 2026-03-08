# Must-Have Requirements

Items below were submitted via "Instructions for Product Team" with high urgency.
Claude Code MUST implement these. If they conflict with existing code or CLAUDE.md, these override.

## [CRITICAL] 2026-03-08 18:23 UTC
Eliminate the button that opened this, and the cruft code. I tapped old button to open this not the lightbulb one
_Submitted: 2026-03-08 18:23 UTC_

## Resolved

- ~~[CRITICAL] 2026-03-08 17:50 UTC — Move the bubble to the bottom right above penny~~ — Fixed: FAB moved to top-right (top:72px, right:16px)
- ~~[CRITICAL] 2026-03-08 18:02 UTC — Cannot see checkbox outline dark mode~~ — Fixed: border uses var(--text-secondary) instead of var(--border)
- ~~[CRITICAL] 2026-03-08 18:03 UTC — Photos broken~~ — Fixed: FileReader.readAsDataURL instead of createObjectURL for iOS Safari compat
- ~~[CRITICAL] 2026-03-08 18:07 UTC — Eliminate the extra feedback bubble cruft~~ — Fixed: green pulse dot removed, toast bar moved to top card
