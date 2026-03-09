# PennyAI — CEO Panel Strategic Proposals

**Date:** 2026-03-09
**Panel:** Brian Chesky, Whitney Wolfe Herd, Stewart Butterfield, Katrina Lake, Patrick Collison, Tobi Lütke, Sarah Friar

---

## IMMEDIATE ACTIONS (This Week)

### 1. Fix Brand Drifts

**Issue:** Design tokens inconsistent across surfaces.
- App has non-brand green (#2E7D32) instead of #35644F
- App has Roboto font (should be Avenir/Avenir Next only)

**Action:**
- [ ] Audit all color usage in app; replace #2E7D32 with #35644F
- [ ] Remove Roboto font references; confirm only Avenir/Avenir Next present
- [ ] QA test on iOS and Android; confirm brand green renders correctly
- **Owner:** Product/Design
- **Timeline:** 2 days
- **Rationale:** Brand consistency enables scaling. Mixed signals dilute the brand story.

---

### 2. Rewrite "First 5 Minutes" Experience

**Issue:** Stewart Butterfield's observation: New users without household history see friction, not magic.

**Current Flow:**
1. User texts Penny an item
2. Penny responds
3. Penny asks for clarification (quantity? brand?)
4. Friction → user abandons

**Proposed Flow:**
1. User texts Penny an item
2. Penny *assumes* reasonable defaults (milk → 1 gallon, popular brand from similar households OR generic)
3. Penny adds to list with options: "If you prefer [Brand X], tap here"
4. User feels "this just works" → invites household immediately

**Implementation:**
- [ ] Use Claude to infer quantity + popular brand from item context
- [ ] Fall back to generic + popular brand (Kroger brand, Organic Valley milk, etc.)
- [ ] Add "brand correction" buttons, not request loops
- [ ] A/B test: current flow vs. new flow. Measure: % new users activate + invite.

**Owner:** Product + AI Engineering
**Timeline:** 5 days
**Rationale:** This directly addresses Stewart's "first-5-minutes" critique. New user activation is the growth foundation.

---

### 3. Define Penny's 11-Star Moment in Writing

**Issue:** Brian Chesky's challenge: Penny solves a real problem (grocery chaos), but doesn't yet have a *magical* moment.

**Current Drafts:**
- "Fewer trips, less waste, organized" (pragmatic, not magical)
- "SMS = no app download" (table stakes now)
- "Family stays in sync" (expected, not wow)

**11-Star Moment Candidates:**
1. **Joy of Coordination:** "You never ask 'Did you get milk?' again. You know."
2. **Relief through AI Understanding:** "Penny knows taco night at Sarah's means you need 2x tomatoes, cilantro, and lime. No items forgotten."
3. **Family Tradition:** "Kids learn meal planning alongside parents. Penny is the invisible teacher."
4. **Budget Magic:** "Penny saves our family $50/trip by suggesting store brands we'd never buy. We trust her."

**Action:**
- [ ] Run a focus group (5-7 power users) asking: "What would you tell a friend about Penny?"
- [ ] Pick ONE 11-star narrative
- [ ] Brief entire team on this narrative; every feature should ladder up to it
- [ ] Update pitch deck, website copy, roadmap to reflect this narrative

**Owner:** CEO (Andy) + Product
**Timeline:** 1 week (focus group + narrative lock)
**Rationale:** When everyone rallies around one truth, shipping becomes faster and clearer.

---

## ROADMAP UPDATES (Next 30 Days)

### 4. Pause New Feature Ships; Focus on Core Flywheel

**Current Pipeline:**
- Dark mode (shipped) ✓
- Feedback widget (beta, ops unstable) — **HOLD**
- Trip planner (in progress) — **DEPRIORITIZE**
- Smart sorting (spec'd) — **DEPRIORITIZE**

**Proposed Pause:**
- [ ] **HOLD** all new feature ships until first-5-minutes is optimized
- [ ] **HOLD** feedback widget until ops is rock-solid (1+ week)
- [ ] **FOCUS** on: Data flywheel story, activation metrics, retention curves

**Timeline:** 30-day runway
**Rationale:** Katrina Lake's insight: You're building for compound growth, not feature accumulation. Every ship should make the flywheel tighter, not wider.

---

### 5. Document the Data Flywheel (12-Month Roadmap)

**Issue:** Katrina Lake's observation: You have the infrastructure but haven't told the compounding story.

**Proposed Narrative:**
- **Month 0–3:** MVP: List + household sharing. Data: basic item history.
- **Month 3–6:** Per-member brand memory kicks in. Data: brand preferences by member.
- **Month 6–9:** Household-level patterns emerge (taco nights = predictable). Data: meal planning signals.
- **Month 9–12:** Cross-household trends visible (all households with kids add similar items). Data: demographic + behavior bundles.

**At Year 1:** Penny can suggest items not yet on the list. "You usually add organic milk — want some?" — 40% acceptance rate.

**Implementation:**
- [ ] Chart the data collection → model improvement → retention lift at each stage
- [ ] Define KPIs per stage (activation, brand accuracy, retention)
- [ ] Add this narrative to ROADMAP.md under "12-Month Data Compounding Plan"

**Owner:** Andy + Head of Product
**Timeline:** 1 week (write), ongoing (track)
**Rationale:** Shows board, team, and users that Penny gets *smarter*, not just *broader*.

---

## BROADER STRATEGIC DECISIONS (For Andy)

### 6. Clarify Rally Integration Story

**Issue:** Tobi Lütke's question: Is Rally separate or part of Penny's platform?

**Decision Matrix:**

| Scenario | Implication | Action |
|----------|-------------|--------|
| **Rally is separate** | Two flywheels, two businesses. Clear. | Update CLAUDE.md. Stop talking about Rally in Penny materials. |
| **Rally feeds Penny** | Helpers use Penny. Create feedback loop. | Roadmap item: "Helpers who grocery shop together" (social flywheel). |
| **Penny feeds Rally** | Task bounties for households (earn by coordinating). | Roadmap item: "Household challenges" (gamification). |
| **Both: bidirectional** | Penny households → Rally tasks; Rally helpers → Penny households. | Complex. Requires strategy. Recommend *not* this initially. |

**Recommendation:** **Rally is separate venture.** Focus Penny on households. Let Rally stand alone. Revisit in Year 2.

**Action:**
- [ ] Andy decides
- [ ] Update CLAUDE.md and ROADMAP.md to clarify separation or integration
- [ ] Brief Tim (Rally coach) on decision

**Timeline:** This week
**Rationale:** Strategy clarity unblocks both teams.

---

### 7. Reposition Website to "Why Penny" Instead of "What Penny Does"

**Issue:** Current website lists features (saved lists, dark mode, shared lists, trip planner). Tobi's feedback: Choose ONE hero message.

**Proposed Website Reframe:**

| Current | New | Rationale |
|---------|-----|-----------|
| "The smartest shared shopping list" | "Stop asking 'Did you get milk?'" | Real problem + solution |
| (5 features listed) | (3 pillars: Natural language, Smart sorting, Family sync) | Clarity over completeness |
| (Vague value props) | (11-star moment: Relief through coordination) | Emotional, not functional |

**Implementation:**
- [ ] Update Home.tsx headline, hero section, features grid
- [ ] Cut trip planner from website until it's core story
- [ ] Expand household sharing section (not trip planning)
- [ ] Ensure every section answers: "Why would a busy family use this?"

**Owner:** Product Marketing + Frontend
**Timeline:** 3 days
**Rationale:** Focus drives conversion.

---

## DECISIONS WAITING ON ANDY

1. **11-Star Moment:** Which candidate resonates? (Joy of coordination? Relief through AI? Family tradition? Budget magic?)

2. **First-5-Minutes Fix:** Approved? (Implies: No new features until this is 7/10 activation.)

3. **Rally Integration:** Separate or connected?

4. **Data Flywheel Timeline:** Lock the narrative? (Enables team alignment on priorities.)

5. **Website Reposition:** Hero message? (Enables marketing to move forward.)

---

## Success Metrics (Q2 2026)

### Activation
- **New user → first list within 2 minutes:** Target 80% (currently unknown)
- **New user → household invite within session:** Target 40%

### Retention
- **D7 retention:** Target 45%
- **D30 retention:** Target 25%

### Data Flywheel
- **% users with 3+ household members:** Track monthly
- **Brand accuracy (% items user doesn't correct):** Target 75% by month 6

### Referral
- **k-factor (viral coefficient):** Track. Target > 1 by Q3.

---

## Summary

**Stop shipping wide. Start shipping deep.** Focus on:
1. First-5-minutes magic (Stewart)
2. 11-star moment clarity (Brian)
3. Data compounding story (Katrina)

**This unblocks:**
- Marketing (clear hero message)
- Product (clear roadmap)
- Team (clear rallying point)
- Investors (clear moat story: data flywheel, not feature list)

**Timeline:** Decisions this week, implementation over next 30 days.

---

## User Panel Feedback on Implementation

The 7-person customer panel (Linda, Devon, Marcus, Tamika, James, Priya, Nadia) reviewed the Three Gates framework and First 5 Minutes onboarding rules. Here is their feedback:

### Nadia (62, Retired Teacher, Not Tech-Savvy)
**"Will the defaults confuse me? I like being asked things. You should ask, not assume."** Concern: Assumptions feel like the app is doing things without permission. However, acknowledges that options ("I picked Organic Valley — tap to change") feel helpful, not pushy.

### Marcus (41, Privacy Skeptic)
**"Does 'assuming brands' mean you're tracking me?"** Concern: Using "popularity data from similar households" sounds like privacy invasion. Clarification needed: Are brand defaults from global trends (all app users) vs. personal tracking? Prefers explicit statement: "I picked milk from our community's top choices."

### Tamika (29, Single Mom)
**"Love the speed, but what if the defaults are wrong brands for my kids?"** Concern: Her family has specific brands (dairy-free, nut-free for allergies). One-tap correction is reassuring. Wants visual confirmation that brand is changed (toast notification: "✓ Changed to Organic Valley").

### James (45, Household Coordinator)
**"Household invite after 3rd item — is that too aggressive?"** Concern: Feels like the app is pushing features. Prefers softer language ("Want to invite your wife?" vs. "Add household members now"). Accepts timing after 3rd item as reasonable (feels earned).

### Linda (58, Budget Optimizer)
**"Where's the price in all this?"** Concern: First 5 minutes focuses on item defaults, not savings. Brand suggestions should show price impact ("Organic Valley is $0.30 more — worth it?"). Gates framework is sound but needs price as a component.

### Priya (31, Health-Conscious)
**"Check the attributes, not just brand."** Concern: "Popular brand" might miss dietary needs (organic, non-GMO, allergen-free). Suggest: If user previously marked items as organic/allergen-free, Penny respects those attributes in defaults, not just popularity.

### Devon (34, Tech-Savvy)
**"This is the right framework. Ship it."** Accepts all three gates. Suggestion: Document the data flywheel clearly so power users understand why Penny gets better over time. Wants visibility into "what Penny learned about me."

---

## CEO Resolution

**Andy's call on implementation:**

1. **Brand defaults ON for new users, with explicit signal** — Penny says "I picked [brand] — tap to change" in a card below the item. Addresses Nadia's "no assumptions" concern by making the assumption visible and easy to override.

2. **Brands sourced from global popularity + privacy-respecting** — Use aggregate app-wide brand preferences (not personal household tracking) for defaults. Label clearly: "I picked [brand] based on popular choices in our app." Addresses Marcus's privacy concern.

3. **Brand change is 1-tap, with confirmation toast** — When user taps to change, show quick brand selector (top 5 alternatives) + confirmation toast ("✓ Changed to Organic Valley"). Addresses Tamika's need for visual feedback and quick correction.

4. **Household invite after 3rd item, with soft language** — Timing stays at item #3 (feels earned, not pushy). Copy changes from "Add household members now" to "Want to invite your family to this list?" Addresses James's concern about pushiness.

5. **Brand defaults respect dietary attributes** — If user has previously selected organic/allergen-free items, Penny's brand defaults prioritize matching those attributes over pure popularity. Addresses Priya's health-conscious concern.

6. **Price signal added to brand suggestions (future)** — First 5 minutes focuses on speed (no delays). After onboarding, Penny shows price impact on brand switches. This is P1 (ship after initial activation metrics validate). Addresses Linda's budget concern.

7. **Data transparency for power users** — Devon asked for visibility into "what Penny learned." Add a debug panel in Profile: "Penny knows: You prefer organic [72% of items]. You shop taco night Fridays [pattern detected]. Your household size is 3." This is P2 (after core flywheel ships).

**Implementation sequence:**
- **Week 1:** Brand defaults + 1-tap correction (P0)
- **Week 2:** Household invite after 3rd item with soft language (P0)
- **Week 3:** Dietary attribute respect in defaults (P0.5)
- **Week 4+:** Price signals, data transparency (P1)

