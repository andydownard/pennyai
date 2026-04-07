#!/bin/sh
# Start the Groflo server, optionally with Litestream continuous replication.
# If LITESTREAM_REPLICA_BUCKET is set, Litestream wraps the server process and
# continuously replicates the SQLite WAL to the configured S3/R2 bucket.
# Otherwise, the server starts directly (no backup overhead).

if [ -n "$LITESTREAM_REPLICA_BUCKET" ]; then
  DB_FILE="${DB_PATH:-thriftly.db}"
  if [ ! -f "$DB_FILE" ]; then
    echo "[run] no local database found, restoring from $LITESTREAM_REPLICA_BUCKET"
    litestream restore -config /etc/litestream.yml "$DB_FILE"
    echo "[run] restore complete"
  else
    echo "[run] existing database found at $DB_FILE, skipping restore"
  fi
  echo "[run] starting with Litestream replication to $LITESTREAM_REPLICA_BUCKET"
  exec litestream replicate -exec "./groflo" -config /etc/litestream.yml
else
  echo "[run] starting without Litestream (no LITESTREAM_REPLICA_BUCKET set)"
  exec ./groflo
fi
