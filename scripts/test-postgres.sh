#!/bin/bash
# Runs the whole Go suite against a real PostgreSQL server, in parallel chunks
# so one panicking test cannot hide the rest.
#
#   scripts/test-postgres.sh [outdir]
#
# Needs a local server where the current user may CREATE DATABASE. Override the
# admin URL with TEST_DATABASE_URL. Results land in <outdir>/all.log and
# <outdir>/fails.txt; exit status is non-zero if any test failed.
set -u
REPO="$(cd "$(dirname "$0")/.." && pwd)"
OUT="${1:-$(mktemp -d)}"
export TEST_DATABASE_URL="${TEST_DATABASE_URL:-postgres://localhost:5432/postgres?sslmode=disable}"
rm -rf "$OUT"; mkdir -p "$OUT"
cd "$REPO" && go test -c -o "$OUT/suite.test" . || exit 2
"$OUT/suite.test" -test.list '.*' | grep '^Test' > "$OUT/all.txt"
split -l 60 "$OUT/all.txt" "$OUT/chunk_"
run_chunk() {
  f=$1; re="^($(paste -sd'|' "$f"))\$"
  "$OUT/suite.test" -test.count=1 -test.timeout 10m -test.run "$re" > "$f.log" 2>&1
}
export -f run_chunk; export OUT REPO
ls "$OUT"/chunk_?? | xargs -P 6 -I{} bash -c 'cd "$REPO" && run_chunk {}'
cat "$OUT"/chunk_??.log > "$OUT/all.log"
grep -h '^--- FAIL' "$OUT/all.log" | sed -E 's/ \([0-9.]+s\)//; s/--- FAIL: //' | sort > "$OUT/fails.txt"
FAILS=$(wc -l < "$OUT/fails.txt" | tr -d ' ')
PANICS=$(grep -l '^panic' "$OUT"/chunk_??.log 2>/dev/null | wc -l | tr -d ' ')
echo "tests: $(wc -l < "$OUT/all.txt" | tr -d ' ')  failing: $FAILS  panicked chunks: $PANICS  (logs: $OUT)"
[ "$FAILS" = "0" ] && [ "$PANICS" = "0" ]
