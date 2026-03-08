#!/bin/sh
# Start the Thriftly server, optionally with Litestream continuous replication.
# If LITESTREAM_REPLICA_BUCKET is set, Litestream wraps the server process and
# continuously replicates the SQLite WAL to the configured S3/R2 bucket.
# Otherwise, the server starts directly (no backup overhead).

if [ -n "$LITESTREAM_REPLICA_BUCKET" ]; then
  echo "[run] starting with Litestream replication to $LITESTREAM_REPLICA_BUCKET"
  exec litestream replicate -exec "./thriftly" -config /etc/litestream.yml
else
  echo "[run] starting without Litestream (no LITESTREAM_REPLICA_BUCKET set)"
  exec ./thriftly
fi
