#!/bin/bash
# Check test-to-code ratio for Go backend
# Required ratio: 1.3 (1.3 lines of test per 1 line of production code)
# Usage: ./scripts/check-test-ratio.sh [--ci]
# In CI mode (--ci), exits with non-zero if ratio is below threshold.

set -euo pipefail

THRESHOLD="1.3"
CI_MODE=false
if [[ "${1:-}" == "--ci" ]]; then
  CI_MODE=true
fi

# Count production Go lines (exclude test files, vendor, generated)
PROD_LINES=$(find . -name '*.go' \
  ! -name '*_test.go' \
  ! -path './vendor/*' \
  ! -path './.git/*' \
  -exec cat {} + | wc -l | tr -d ' ')

# Count test Go lines
TEST_LINES=$(find . -name '*_test.go' \
  ! -path './vendor/*' \
  ! -path './.git/*' \
  -exec cat {} + | wc -l | tr -d ' ')

# Calculate ratio
if [ "$PROD_LINES" -eq 0 ]; then
  echo "No production code found."
  exit 0
fi

RATIO=$(echo "scale=2; $TEST_LINES / $PROD_LINES" | bc)

echo "============================================"
echo "  Test-to-Code Ratio Check"
echo "============================================"
echo "  Production lines (*.go):      $PROD_LINES"
echo "  Test lines (*_test.go):       $TEST_LINES"
echo "  Ratio:                        ${RATIO}x"
echo "  Required:                     >= ${THRESHOLD}x"
echo "============================================"

# Compare ratio to threshold
PASS=$(echo "$RATIO >= $THRESHOLD" | bc -l)

if [ "$PASS" -eq 1 ]; then
  echo "  ✅ PASS — ratio meets requirement"
  echo "============================================"
  exit 0
else
  echo "  ❌ FAIL — ratio below ${THRESHOLD}x"
  NEEDED=$(echo "scale=0; ($PROD_LINES * $THRESHOLD) - $TEST_LINES" | bc | cut -d. -f1)
  echo "  Need ~${NEEDED} more test lines to meet threshold"
  echo "============================================"
  if $CI_MODE; then
    exit 1
  else
    exit 0
  fi
fi
