#!/bin/bash
# migrate_sqlite_to_postgres.sh
#
# Migrates pennyai data from SQLite (thriftly.db) to the shared PostgreSQL
# database in timpager's Railway project.
#
# Prerequisites:
#   - sqlite3 CLI installed
#   - psql CLI installed
#   - Local copy of thriftly.db (download from Railway via Litestream restore)
#   - DATABASE_URL pointing to timpager's Postgres
#
# Usage:
#   export DATABASE_URL="postgresql://postgres:PASSWORD@HOST:5432/railway"
#   ./scripts/migrate_sqlite_to_postgres.sh /path/to/thriftly.db

set -euo pipefail

SQLITE_DB="${1:?Usage: $0 /path/to/thriftly.db}"
PG_URL="${DATABASE_URL:?Set DATABASE_URL to timpager Postgres}"

echo "=== Pennyai SQLite → PostgreSQL Migration ==="
echo "Source: $SQLITE_DB"
echo "Target: ${PG_URL%%@*}@***"
echo ""

# Verify SQLite DB exists and is readable
if ! sqlite3 "$SQLITE_DB" "SELECT COUNT(*) FROM users;" > /dev/null 2>&1; then
    echo "ERROR: Cannot read $SQLITE_DB or users table missing"
    exit 1
fi

# Show row counts before migration
echo "=== Source row counts ==="
for table in users list_items saved_lists saved_list_items households household_members \
    invites referrals nps_responses auth_methods brand_choices brand_preferences \
    brand_deals swap_events swap_rebates receipts receipt_line_items swap_claims \
    brand_users brand_sessions brand_balance brand_transactions \
    profile_photos shopping_trips bug_reports feedback_requests \
    user_signals penny_learnings user_dietary_profiles penny_context_cache \
    deal_performance_daily category_swap_insights business_metrics_daily \
    churn_signals user_cohorts marketplace_health categories stores \
    user_swap_predictions category_pricing_tiers shared_links \
    savings_milestones retailer_abbreviations item_modifications; do
    count=$(sqlite3 "$SQLITE_DB" "SELECT COUNT(*) FROM $table;" 2>/dev/null || echo "N/A")
    printf "  %-35s %s\n" "$table" "$count"
done

echo ""
echo "=== Step 1: Ensure Postgres schema exists ==="
echo "Run the pennyai server once to create tables (initDB), or run:"
echo "  go run . &  # starts server, creates tables, then ctrl-c"
echo ""
read -p "Press Enter when Postgres schema is ready, or Ctrl-C to abort..."

echo ""
echo "=== Step 2: Migrate users ==="
echo "Users need special handling — timpager shared table uses TEXT UUIDs."
echo "We'll generate UUIDs for existing integer IDs and maintain a mapping."
echo ""

# Create temporary mapping file
MAPPING_FILE="/tmp/pennyai_user_id_mapping.csv"
echo "old_id,new_uuid" > "$MAPPING_FILE"

# Export users with new UUIDs
sqlite3 -csv "$SQLITE_DB" "
    SELECT id, COALESCE(phone_number,''), COALESCE(name,''), COALESCE(email,''),
           COALESCE(household_id,''), COALESCE(referral_code,''),
           COALESCE(referral_points,0), COALESCE(login_count,0),
           COALESCE(address,''), COALESCE(penny_brand_mode,'ask'),
           COALESCE(deal_accept_count,0), COALESCE(deal_shown_count,0),
           COALESCE(penny_slider,15), COALESCE(penny_autotune,1),
           COALESCE(penny_deals,0), COALESCE(swap_credits_cents,0),
           created_at
    FROM users
" | while IFS=',' read -r old_id phone name email hh_id ref_code ref_points login_count \
    address brand_mode deal_accept deal_shown slider autotune deals credits created_at; do

    # Generate deterministic UUID from old integer ID
    new_uuid=$(python3 -c "import uuid; print(uuid.uuid5(uuid.NAMESPACE_DNS, 'pennyai-user-$old_id'))")
    echo "$old_id,$new_uuid" >> "$MAPPING_FILE"

    # Check if user already exists in shared table (by phone)
    if [ -n "$phone" ] && [ "$phone" != "" ]; then
        existing=$(psql "$PG_URL" -tAc "SELECT id FROM users WHERE phone = '$phone'" 2>/dev/null || echo "")
        if [ -n "$existing" ]; then
            # User exists in timpager — use their ID, just add penny columns
            echo "  User $old_id (phone $phone) → existing timpager user $existing"
            # Update mapping
            sed -i '' "s/$old_id,$new_uuid/$old_id,$existing/" "$MAPPING_FILE"
            new_uuid="$existing"
        fi
    fi

    # Insert into shared users table (skip if exists)
    psql "$PG_URL" -c "
        INSERT INTO users (id, phone, display_name, email, created_at)
        VALUES ('$new_uuid', NULLIF('$phone',''), '$name', NULLIF('$email',''),
                COALESCE('$created_at'::timestamptz, NOW()))
        ON CONFLICT (id) DO NOTHING;
    " 2>/dev/null

    # Update penny-specific columns
    psql "$PG_URL" -c "
        UPDATE users SET
            household_id = NULLIF($hh_id, ''),
            referral_code = NULLIF('$ref_code',''),
            referral_points = $ref_points,
            login_count = $login_count,
            address = '$address',
            penny_brand_mode = '$brand_mode',
            deal_accept_count = $deal_accept,
            deal_shown_count = $deal_shown,
            penny_slider = $slider,
            penny_autotune = $autotune,
            penny_deals = $deals,
            swap_credits_cents = $credits
        WHERE id = '$new_uuid';
    " 2>/dev/null

done

echo "  User mapping saved to $MAPPING_FILE"
echo "  $(wc -l < "$MAPPING_FILE") users mapped"

echo ""
echo "=== Step 3: Migrate remaining tables ==="
echo "Using user ID mapping to convert integer IDs to UUIDs..."

# Helper: migrate a table, replacing user_id integers with UUIDs
migrate_table() {
    local table="$1"
    local user_id_col="${2:-user_id}"

    local count=$(sqlite3 "$SQLITE_DB" "SELECT COUNT(*) FROM $table;" 2>/dev/null || echo "0")
    if [ "$count" = "0" ] || [ "$count" = "N/A" ]; then
        echo "  $table: skipped (empty or missing)"
        return
    fi

    echo "  $table: $count rows..."

    # Export as CSV, transform user IDs, import via psql COPY
    sqlite3 -csv -header "$SQLITE_DB" "SELECT * FROM $table;" > "/tmp/pennyai_${table}.csv"

    # Use Python to remap user IDs
    python3 << PYEOF
import csv, sys

mapping = {}
with open('$MAPPING_FILE') as f:
    reader = csv.DictReader(f)
    for row in reader:
        mapping[row['old_id']] = row['new_uuid']

with open('/tmp/pennyai_${table}.csv') as fin, open('/tmp/pennyai_${table}_mapped.csv', 'w', newline='') as fout:
    reader = csv.DictReader(fin)
    writer = csv.DictWriter(fout, fieldnames=reader.fieldnames)
    writer.writeheader()
    for row in reader:
        if '$user_id_col' in row and row['$user_id_col'] in mapping:
            row['$user_id_col'] = mapping[row['$user_id_col']]
        writer.writerow(row)
PYEOF

    # Import via psql (skip id column for SERIAL tables, let Postgres auto-generate)
    psql "$PG_URL" -c "\COPY $table FROM '/tmp/pennyai_${table}_mapped.csv' CSV HEADER" 2>/dev/null || \
        echo "    WARNING: COPY failed for $table (may need manual review)"
}

# Migrate tables in dependency order
migrate_table "households"
migrate_table "household_members" "user_id"
migrate_table "list_items" "user_id"
migrate_table "saved_lists" "user_id"
migrate_table "saved_list_items"
migrate_table "invites"
migrate_table "item_modifications" "user_id"
migrate_table "referrals"
migrate_table "nps_responses" "user_id"
migrate_table "auth_methods" "user_id"
migrate_table "brand_choices" "user_id"
migrate_table "brand_preferences" "user_id"
migrate_table "brand_deals"
migrate_table "swap_events" "user_id"
migrate_table "swap_rebates" "user_id"
migrate_table "swap_suppressions" "user_id"
migrate_table "receipts" "user_id"
migrate_table "receipt_line_items"
migrate_table "swap_claims" "user_id"
migrate_table "profile_photos" "user_id"
migrate_table "shopping_trips" "user_id"
migrate_table "bug_reports" "user_id"
migrate_table "feedback_requests" "user_id"
migrate_table "user_signals" "user_id"
migrate_table "penny_learnings"
migrate_table "user_dietary_profiles" "user_id"
migrate_table "penny_context_cache" "user_id"
migrate_table "shared_links" "user_id"
migrate_table "savings_milestones" "user_id"
migrate_table "brand_users"
migrate_table "brand_sessions"
migrate_table "brand_balance"
migrate_table "brand_transactions"
migrate_table "stores"
migrate_table "price_cache"
migrate_table "categories"
migrate_table "category_pricing_tiers"
migrate_table "retailer_abbreviations"

echo ""
echo "=== Step 4: Reset sequences ==="
echo "Postgres SERIAL sequences need to be bumped past the max imported ID..."

for table in list_items saved_lists saved_list_items households household_members \
    invites referrals nps_responses auth_methods brand_choices brand_preferences \
    brand_deals swap_events swap_rebates receipts receipt_line_items swap_claims \
    shopping_trips bug_reports feedback_requests user_signals penny_learnings \
    brand_users brand_sessions brand_transactions stores categories \
    savings_milestones retailer_abbreviations item_modifications; do
    psql "$PG_URL" -c "
        SELECT setval(pg_get_serial_sequence('$table', 'id'),
               COALESCE((SELECT MAX(id) FROM $table), 0) + 1, false);
    " 2>/dev/null || true
done

echo ""
echo "=== Step 5: Verify ==="
echo "Target row counts:"
for table in users list_items saved_lists households brand_deals swap_events \
    auth_methods brand_users receipts; do
    count=$(psql "$PG_URL" -tAc "SELECT COUNT(*) FROM $table;" 2>/dev/null || echo "ERROR")
    printf "  %-35s %s\n" "$table" "$count"
done

echo ""
echo "=== Migration complete ==="
echo "User ID mapping: $MAPPING_FILE"
echo ""
echo "Next steps:"
echo "  1. Verify data in Postgres (spot-check a few users, lists, swaps)"
echo "  2. Update pennyai Railway service to use DATABASE_URL"
echo "  3. Deploy pennyai to timpager's Railway canvas"
echo "  4. Verify healthcheck passes"
echo "  5. Cut DNS from old service to new"
echo "  6. Keep old SQLite + Litestream running for 7 days as rollback"
