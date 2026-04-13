# Groflo — Panel Governance

## Customer Panel

For any user-facing feature, simulate this panel BEFORE implementation. See Panel Trigger Rules below.

| Panelist | Archetype | Decision Framework | Dealbreaker |
|----------|-----------|-------------------|-------------|
| **Linda** (58) | Budget optimizer, coupon clipper | "Will this actually save me money I can verify?" | Hidden costs, unclear savings, unproven claims |
| **Devon** (34) | Tech-savvy early adopter, shares finds | "Would I screenshot this and send to my group chat?" | Cluttered UI, no data transparency, nothing to brag about |
| **Marcus** (41) | Skeptical about app tracking | "What data are you collecting and why?" | Dark patterns, tracking without consent, vague privacy language |
| **Tamika** (29) | Single mom, two kids, juggles work + school | "Can I do this one-handed while holding a toddler?" | Extra steps, clarifying questions, anything requiring focus |
| **James** (45) | Household coordinator, shares list with wife | "Will my wife and I stop buying duplicate milk?" | Confusing coordination, unclear who claimed what |
| **Priya** (31) | Health-conscious, reads every label | "Does Penny respect my dietary needs without me repeating them?" | Allergen-blind suggestions, no ingredient info |
| **Nadia** (62) | Retired teacher, not tech-savvy, lives alone | "Can I figure this out without asking my grandson?" | Small text, jargon, anything requiring instructions |

## Panel Protocol (4 Rounds)

1. **First Reactions** — Gut reaction per panelist (30 seconds each)
2. **Deep Dive** — 2-3 specific questions per panelist from their framework
3. **Dealbreakers** — Each panelist states #1 condition for using / not using
4. **Synthesis** — PM summarizes consensus, flags disagreements, produces requirements with panelist attribution

## Panel Trigger Rules — MANDATORY

Convene Customer Panel BEFORE implementation when ANY of these are true:

- Adding or changing text the user reads (error messages, empty states, Penny dialogue)
- Adding or removing a tap/click from any user flow
- Changing what information is shown or hidden
- Adding a new screen, modal, or bottom sheet

Convene CEO Panel BEFORE implementation when ANY of these are true:

- Feature takes >3 days to build
- Feature affects revenue path (swaps, brands, payments)
- Feature could be cut without user complaint (test for necessity)
- Scope is growing beyond original sprint spec

Do NOT convene panels for: refactors with no user-facing change, test improvements, dependency updates, bug fixes where correct behavior is already specified.

## Product Team (7 Roles)

| Role | Focus | Convene when |
|------|-------|-------------|
| **PM** | User value, prioritization, scope | Every feature |
| **Architect** | System design, data flow, scalability | New tables, services, integrations |
| **UX Designer** | Edge cases, touch targets, animation, accessibility | Any user-facing change |
| **QA** | Test coverage, edge cases, failure modes | Every feature |
| **DevOps** | Deploy safety, monitoring, rollback | New infra, env vars, external services |
| **BI Analyst** | Metrics, tracking, dashboards | Features with success metrics |
| **Growth** | Virality, retention, activation | User-facing features, onboarding, sharing |

Always convene before: changes to revenue paths, auth flows, Penny's personality, database schema, external API integrations. Always have UX Designer review BEFORE implementation begins for any user-facing change. PennyAI-specific: always convene before changes to brand system (logo caching, brand preferences) or household sharing flows.
