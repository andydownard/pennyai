# Sprint 2: Discoverability & Clarity

**Duration:** 2 weeks
**Theme:** Make existing features findable, comprehensible, and trustworthy. Remove confusion, surface value.

---

## P0 -- Must Ship

| # | Item | Files | Effort | Impact |
|---|------|-------|--------|--------|
| 1 | **Saved Lists UI entry point** -- Add "My Lists" to NavMenu. Tap to load, long-press to rename/delete. Keep Penny chat as secondary path. | `NavMenu.tsx`, new `SavedListsScreen.tsx`, `App.tsx` | M (3-4d) | HIGH |
| 2 | **Progressive disclosure: hide swaps/earnings for new users** -- Hide "$0.00 swaps" header badge until user has earned >$0. Show "What are swaps?" explainer on first swap offer. | `App.tsx`, `SwapCard.tsx` | S (1-2d) | HIGH |
| 3 | **Fix B25: New user confusion** -- Improve first-session empty state with warm Penny-driven welcome + clear CTAs. | `App.tsx`, `PennyBar.tsx`, `useChatState.ts` | S-M (2-3d) | HIGH |
| 4 | **Error feedback on API failures** -- Centralized error handler showing toasts on non-401 failures. Audit list CRUD, chat send, saved list operations. Rate-limit toasts (1 per 3s per error type). | `api.ts`, `App.tsx`, `useChatState.ts`, `ProfileScreen.tsx` | M (3-4d) | HIGH |

## P1 -- Should Ship

| # | Item | Files | Effort | Impact |
|---|------|-------|--------|--------|
| 5 | **Rich empty states with CTAs** -- PennyAvatar + explanation + action button for Receipts, Saved Lists, Trips. Follow EarningsScreen pattern. | `ReceiptScreen.tsx`, `SavedListsScreen.tsx`, `TripPlannerSheet.tsx` | S (2d) | MEDIUM |
| 6 | **Household invite copy refinement** -- Update to match brand bible ("Want your family on this list too?"). Verify localStorage dismissal persistence. | `App.tsx`, `JoinPrompt.tsx` | S (1-2d) | MEDIUM |
| 7 | **Offline toggle queueing** -- Queue list check/uncheck operations when offline, replay on reconnect. Show "Changes will sync when you're back online." Do NOT queue add/delete/claim. | `useAuthState.ts`, `useListState.ts`, `PennyBar.tsx` | M (3-4d) | MEDIUM |
| 8 | **Brand messaging alignment pass** -- Audit all user-facing copy against brand bible. Fix mismatches in auth screens, PennyBar, ShareScreen, EarningsScreen. | `NavMenu.tsx`, auth screens, `ShareScreen.tsx`, `HelpScreen.tsx` | S (1-2d) | MEDIUM |
| 9 | **Penny conversation edge cases** -- Partial success handling ("Added 3 of 5 items"), retry button on failure, better guidance on ambiguous input. | `penny.go`, `useChatState.ts`, `PennyBar.tsx` | M (3-4d) | MEDIUM |

## P2 -- Nice to Have

| # | Item | Files | Effort | Impact |
|---|------|-------|--------|--------|
| 10 | **Accessibility: focus-restore on modal close** -- Return focus to trigger element when modal/sheet closes. Same pattern across 7 components. | `ConfirmModal.tsx`, `BottomSheet.tsx`, `ItemEditSheet.tsx`, etc. | S (1-2d) | LOW |
| 11 | **Hide trip planner for new users** -- Gate visibility on user having completed at least one shopping session. | `App.tsx` | XS (0.5d) | LOW |
| 12 | **Screen reader label audit** -- VoiceOver walk-through of Sprint 1 features (auto-submit ring, swipe-to-check, store filter pills). | `App.tsx`, `ItemRow.tsx`, `PennyBar.tsx` | S (1-2d) | LOW |

## Developer Grouping (Parallel Tracks)

| Track | Items | Skills |
|-------|-------|--------|
| **A: Components** | P0#1, P1#5, P2#10 | New screens, empty states, focus management |
| **B: Infrastructure** | P0#4, P1#7 | Error handling, offline queueing, api.ts |
| **C: Copy & Disclosure** | P0#2, P0#3, P1#6, P1#8 | Conditional rendering, copy updates |
| **D: Penny AI** | P1#9 | Claude prompt engineering, partial-success handling |

## Sprint 3 Preview

- Clerk auth migration (replace 500 LOC custom auth)
- Penny conversation memory (persistent context across sessions)
- SSE reconnection with exponential backoff
- Receipt scanning UX polish (multi-photo, progress indicator)
- Native app packaging evaluation (Capacitor)
- Penny avatar expressions in more contexts

## Risk Notes

- **P0#1** conflicts with CLAUDE.md rule ("saved lists only accessible via Penny chat"). Adding to NavMenu (not Profile) is a workaround. **Needs founder approval.**
- **P0#4** must follow Notification Safety hard rule -- no toast storms from retry loops.
- **P1#7** must use idempotent mutations only (toggle is safe; add/delete/claim are NOT).
