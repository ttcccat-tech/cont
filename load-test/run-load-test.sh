#!/bin/bash
# ──────────────────────────────────────────────────────────────────────────────
# Cont Load Test Runner
# Usage: ./run-load-test.sh [TARGET_URL] [DURATION] [VUS]
#   TARGET_URL  — upstream to hit  (default: http://192.168.1.202:3010)
#   DURATION    — test duration     (default: 60s)
#   VUS         — concurrent VUs    (default: 500)
# ──────────────────────────────────────────────────────────────────────────────

set -euo pipefail

TARGET_URL="${1:-http://192.168.1.202:3010}"
DURATION="${2:-60s}"
VUS="${3:-500}"
OUT_DIR="$(cd "$(dirname "$0")" && pwd)/results"
mkdir -p "$OUT_DIR"
TIMESTAMP=$(date +%Y%m%d_%H%M%S)
OUT_FILE="$OUT_DIR/${TIMESTAMP}_${VUS}vus_${DURATION}.json"
SUMMARY_FILE="$OUT_DIR/${TIMESTAMP}_${VUS}vus_${DURATION}_summary.txt"

echo "═══════════════════════════════════════════════════"
echo "  Cont Load Test"
echo "  Target : $TARGET_URL"
echo "  Duration: $DURATION"
echo "  VUs    : $VUS"
echo "  Output : $OUT_FILE"
echo "═══════════════════════════════════════════════════"

k6 run load-test.js \
  --env TARGET_URL="$TARGET_URL" \
  --env DURATION="$DURATION" \
  --env VUS="$VUS" \
  --out json="$OUT_FILE" \
  --summary-export="$SUMMARY_FILE"

echo ""
echo "═══════════════════════════════════════════════════"
echo "  Results saved to:"
echo "  $OUT_FILE"
echo "  $SUMMARY_FILE"
echo "═══════════════════════════════════════════════════"
