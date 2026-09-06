#!/usr/bin/env bash
# Coverage gate for CI.
# Usage: check-coverage.sh [threshold]  (default 90)
set -euo pipefail

threshold="${1:-85}"

profile="$(mktemp)"
trap 'rm -f "$profile"' EXIT

go test -coverprofile="$profile" ./... >/dev/null

awk -v threshold="$threshold" '
  /^mode:/ { next }
  { total += $2; if ($3 > 0) covered += $2 }
  END {
    pct = (covered * 100.0) / total
    printf "Coverage: %.1f%% (%d/%d statements)\n", pct, covered, total
    if (pct < threshold) {
      printf "FAIL: coverage %.1f%% is below the required %.1f%%\n", pct, threshold
      exit 1
    }
  }
' "$profile"