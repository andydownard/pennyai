# Groflo — Content Guide

## Voice

Penny's voice — friendly, efficient, like a helpful friend who loves grocery shopping. Short sentences. Never preachy about savings. Never judgmental about food choices. Warm, practical, encouraging. Strunk & White. 8th-grade reading level. ESL-friendly: no idioms, no slang.

See [PENNY_PROMPT.md](./PENNY_PROMPT.md) for Penny's full personality: typewriter effect, chime-ins, idle reminders, personality quotes, and conversational warmth.

## Button Labels

| Say | Never say |
|---|---|
| Add Item | Create, New, Input |
| Claim | Take, Grab, I'll Get This |
| Share List | Invite, Send, Distribute |
| Save List | Create Template, Favorite, Pin |
| Check Off | Complete, Done, Mark |
| Swap | Replace, Switch, Substitute |
| Save | Update, Store, Apply |
| Cancel | Go Back, Dismiss, Close |

## Error Messages

Every error states what happened + what to do. No error codes, stack traces, JSON, or blame. Never make the user feel dumb.

- "Penny can't reach the internet right now. Your list is saved locally."
- "That item is already on the list."
- "Couldn't find that store. Try a different address or search online."
- "Something went wrong. Your list is safe."
- "Too many requests. Give Penny a moment."
- "That household invite code expired. Ask for a new one."

## Empty States

Distinct message per context. Inviting, not empty. Encourage action.

- **Shopping list (empty):** "Your list is empty. Tell Penny what you need!"
- **Saved lists (none):** "No saved lists yet. Save your current list to reuse it."
- **Household (solo):** "Shopping solo? Invite your family to share this list."
- **Shopping trips (none):** "No trips yet. Check off items to start tracking."
- **Completed items (none):** "Nothing checked off yet."

## Notifications

SMS and in-app. Include who did what and when, so recipient knows immediately.

- "{partner_name} claimed {item}"
- "Penny thinks you're running low on {item}"
- "{partner_name} added {count} items"
- "Your saved list '{list_name}' is ready to load"
- "Trip total: ${amount} at {store_name}"

## Words to Avoid

| Jargon | Say instead |
|---|---|
| cart | list |
| receipt | trip |
| coupon, promotion | deal, swap |
| grocery store | store |
| user, account, member | you, household |
| subscription, plan | plan |
| barcode scan | scan |
| item SKU | item |
| household administrator | you, creator |
| sync | update, share |
| notification, alert | nudge, reminder |
