# Groflo — Panel Governance

**Foundational Principle:** Build a grocery app Tamika can use one-handed while holding a toddler. Non-negotiable. Every change must enable the busiest, most-constrained user first — the rest follow naturally.

**Rule:** All user-facing work requires panel review. Panel composition and governance live in this document; changes require Customer Panel + Product Team consensus.

## Process

1. **Intake** — What, why, files affected, scope (30 sec)
2. **Route** — Customer Panel + Product Team + relevant domain experts
3. **Review** — Each panel: GO / GO WITH CONDITIONS / HOLD / BLOCK
4. **Consensus** — All GO → execute. CONDITIONS → add to task. HOLD → resolve within 24 hours. BLOCK → stop, redesign, re-present.
5. **Document** — Date, verdict, conditions, dissents in PR description
6. **Execute** — Work proceeds with conditions met

## Speed Tiers

| Scope | Example | Panels | Time |
|---|---|---|---|
| Micro | Typo, label change | Customer Panel (Tamika test) | 30 sec (auto-GO unless one objects) |
| Small | Bug fix, clarifying copy | + Product Team (1 role) | 2 min |
| Medium | New household feature, new Penny behavior | + Product Team (3 roles) | 10 min |
| Large | Revenue path change, auth flow change, brand system change | All panels + domain experts | 30 min |

## Anti-Rubber-Stamp Rules

- Every panelist MUST name the specific persona tested and describe what they'd actually see and feel.
- Every panelist MUST cite specific files or screens reviewed (e.g., `App.tsx` lines 120-145, the household invite flow).
- Every panelist MUST note at least one risk, even for GO (e.g., "Risk: Text too small at 375px width on iPhone SE").
- Conditions must be testable ("Add test: Tamika uses one hand, taps target correctly" — not "Should be easy").
- The Customer Panel is ALWAYS consulted on any user-facing change. They spot the failure modes the team cannot.
- Penny's personality or voice changes require Customer Panel review. Tone matters.

---

## Panel 1: Customer Panel (7 personas — ALWAYS CONSULTED)

Real households who would actually use Groflo. This is the heart of governance.

| Persona | Profile | Core Question | Dealbreaker |
|---|---|---|---|
| **Linda** | 58, budget optimizer, clips coupons, trusts price | "Will this actually save me money I can verify?" | Hidden costs, vague savings claims, unproven rebates. |
| **Devon** | 34, tech-savvy early adopter, shares finds | "Would I screenshot this and send to my group chat?" | Cluttered UI, no transparency, nothing worth bragging about. |
| **Marcus** | 41, skeptical about data collection | "What data are you collecting, and why?" | Dark patterns, tracking without consent, vague privacy terms. |
| **Tamika** | 29, single mom with toddler, one hand free | "Can I do this one-handed while holding a toddler?" | Extra steps, clarifying questions, anything requiring focus. |
| **James** | 45, household coordinator, shops with wife | "Will my wife and I stop buying duplicate milk?" | Confusing assignment, unclear who claimed what, more work. |
| **Priya** | 31, health-conscious, reads labels | "Does Penny respect my dietary needs without me repeating them?" | Allergen-blind suggestions, missing ingredient info, wrong swaps. |
| **Nadia** | 62, retired teacher, not tech-savvy | "Can I figure this out without asking my grandson?" | Small text, jargon, anything requiring instructions or videos. |

**Standing Questions (every review):**

1. Can Tamika complete this flow one-handed with a toddler in the other arm?
2. Does Linda see her savings materialize in the next three trips, or is it theoretical?
3. Would Devon screenshot this and post it?
4. Does James's household actually stop buying duplicates, or does this add work?
5. Does Priya get allergen-aware suggestions, or does Penny forget her preferences?
6. Can Nadia figure this out without a how-to video?

**Convene when:** ANY user-facing change — text, tap targets, Penny dialogue, empty states, form fields, onboarding, household flow. NO EXCEPTIONS.

**Dissent protocol:** If Linda (budget) and Priya (health) disagree, Priya wins. Optimize for health/safety first, savings second. If Tamika (one hand) and anyone else disagree, Tamika wins. Constrained users first.

---

## Panel 2: Product Team (7 roles)

| Role | Focus | Decision Authority | Convene when |
|------|-------|-------------------|-------------|
| **PM** | User value, prioritization, scope | Roadmap decisions, feature necessity | Every feature |
| **Architect** | System design, data flow, scalability | Tech design, multi-user coordination | New tables, integrations, household flow changes |
| **UX Designer** | Touch targets, animations, accessibility | Interaction patterns, edge cases | Any user-facing change |
| **QA** | Test coverage, edge cases, failure modes | Test sufficiency | Every feature |
| **DevOps** | Deploy safety, monitoring, alerts | Infrastructure safety | New env vars, external services, schema changes |
| **BI Analyst** | Metrics, retention, user behavior | Success metric validity | Features with tracked outcomes |
| **Growth** | Retention, virality, activation | Feature desirability | Onboarding, household sharing, referral |

**Mandatory convenes:** Changes to revenue paths (brand deals, swaps, referral), auth flows, Penny's personality or voice, database schema, external API integrations, brand system (logo caching, brand preferences), household sharing flows.

**UX Designer rule:** ALWAYS reviews user-facing changes BEFORE implementation begins. Review is not optional.

---

## Panel Protocol (4 Rounds)

1. **First Reactions** — Gut reaction per panelist (30 seconds each): "Does Tamika pass the one-hand test?"
2. **Deep Dive** — 2-3 specific questions per panelist from their framework
3. **Dealbreakers** — Each panelist states #1 condition for using / not using
4. **Synthesis** — PM summarizes consensus, flags disagreements, produces requirements with panelist attribution

---

## Panel Trigger Rules — MANDATORY

**Convene Customer Panel BEFORE implementation when ANY of these are true:**
- Adding or changing text the user reads (error messages, empty states, Penny dialogue, brand swap descriptions)
- Adding or removing a tap/click from any user flow
- Changing what information is shown or hidden
- Adding a new screen, modal, or bottom sheet
- Changing Penny's voice or personality
- Household sharing flow changes (invite, join, leave, member list)

**Convene Architect + UX Designer BEFORE implementation when ANY of these are true:**
- New database table or schema change
- New external service integration (API, SMS, email)
- Brand system changes (logo caching, brand preferences)
- Household coordination logic

**Convene Product Team BEFORE implementation when ANY of these are true:**
- Feature takes >3 days to build
- Feature affects revenue path (brand deals, swaps, referral, payments)
- Feature could be cut without user complaint (test for necessity)
- Scope is growing beyond sprint spec

**Convene Growth + BI Analyst BEFORE implementation when:**
- Feature tracks new user behavior or changes onboarding
- Feature affects D1/D7/D30 retention or household conversion
- Feature is a viral or network-effect driver (sharing, invites)

**Do NOT convene panels for:** Internal refactors with no user-facing change, test improvements, dependency updates, bug fixes where correct behavior is already specified in ROADMAP.md.

---

## CST Compliance Checklist (Required for Every Change)

A "No" on any box triggers HOLD until resolved.

| Principle | Test | Persona |
|---|---|---|
| **Human Dignity** | Does this respect household decisions without judgment (dietary, religious, budget)? | Priya (dietary needs) |
| **Common Good** | Does this strengthen households, not replace human coordination? | James (coordination) |
| **Subsidiarity** | Does this empower the household to coordinate without depending on an app? | Tamika (one-handed test) |
| **Solidarity** | Does this bridge cultural, linguistic, and dietary gaps? | All personas together |
| **Preferential Option for the Poor** | Can a single mom with a toddler, one free hand, and a prepaid phone use it? | Tamika (constraint test) |

---

## Anti-Rubber-Stamp Best Practices

1. **Don't fake it.** A persona response must describe what that person would actually see, feel, and do. "GO" without a persona is not a review.
2. **Conditions over BLOCKS where possible.** Most BLOCKS can become CONDITIONS with a clear, testable fix. Conditions go straight onto the task as test requirements.
3. **Disagreements get logged.** If Linda says GO and Priya says HOLD, both verdicts go in the log. PM breaks the tie. The dissent stays in the log.
4. **Panels can be wrong.** When a real Tamika or Linda tells us our panel was wrong, we update the persona. Sync with PANEL_GOVERNANCE.md annually.
5. **The smaller the change, the more important the personas.** A typo in an error message can break trust. A one-pixel shift can make a target unusable. Don't skip the panel for "tiny" things.
6. **When in doubt, constrained users win.** If Tamika (one-handed) and anyone else disagree, Tamika wins. If Linda (budget-conscious) and anyone else disagree on pricing, Linda wins. Optimize for the most-constrained user first.
7. **Document the verdict next to the code.** Panel reviews live in the PR description. PRs without panel reviews don't merge. Period.

---

## Feedback Lifecycle

Idea → Triage (P0–P3) → Panel Review (verdict + conditions) → Implement → Test → Ship → Close.

| Priority | SLA | Example |
|---|---|---|
| P0 (household coordination broken, payment failing) | 1 hr, all hands | Household member can't join. Brand deals not crediting. |
| P1 (user blocked from common flow) | Next sprint | Tamika can't add item one-handed. Penny forgets dietary pref. |
| P2 (UX friction, polish) | Next sprint | Small text. Slow modal. Confusing empty state. |
| P3 (nice-to-have, idea) | ROADMAP.md, triage in 30 days | Dark mode. New Penny quote. Additional language. |

---

## When This Document Changes

Whenever a real user surfaces a new use case, constraint, or persona that kills an old one, edit this file and commit. The panel governance is the doc; the doc is the panel governance. Sync changes with Customer Panel monthly during onboarding refreshes.
