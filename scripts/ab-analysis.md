# PennyAI A/B Test Analysis — Daily Cron

> **Schedule:** Daily at 9:00 AM local time (`0 9 * * *`)
> **To register:** Use Claude desktop → `/schedule` → paste this prompt

## Prompt

You are analyzing A/B test results for PennyAI (Thriftly grocery app). Run this analysis daily.

### Steps

1. Read CURRENT_SPRINT.md in the PennyAI project folder. Check the "Active A/B Tests" table.

2. For each active test:
   - Query the `ab_tests` and `ab_events` tables in the PennyAI SQLite database.
   - Or call: `GET /api/admin/ab-tests?status=active` and `GET /api/admin/ab-tests/{name}/results`.
   - Calculate: sample size per variant, key metric mean, lift %, whether min_sample and measurement_days are met.

3. For tests with sufficient data:
   - **>10% lift:** Recommend shipping winner to 100%. Move to "Completed A/B Tests" in CURRENT_SPRINT.md.
   - **No significant lift:** Recommend extending 7 days or testing a bolder variant.
   - **Control wins >10%:** Recommend reverting. Document the learning.

4. For P0 features (swap acceptance, onboarding, earnings), check if next iteration round should start (up to 3 rounds per the A/B Testing Process in parent CLAUDE.md).

5. Flag tests running past measurement_days + 3 without resolution.

6. Update CURRENT_SPRINT.md: move resolved tests, update metrics, log learnings.

### Key Metrics
- Swap acceptance rate (target: >=8% week 4, >=12% week 8)
- Swaps offered/day (target: 50 week 4)
- Paying brands (target: 3 week 4)
