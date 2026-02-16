#!/usr/bin/env bash
# Checks that all Go code uses time.Now().UTC() instead of bare time.Now().
# Exits with 1 if any violations are found.
#
# Allowed exceptions:
#   - time.Now().UTC()      (correct usage)
#   - time.Now().Unix()     (Unix timestamps are always UTC)
#   - time.Now().UnixMilli/UnixMicro/UnixNano (same reason)
#   - _test.go files        (optional: pass --include-tests to check them too)

set -euo pipefail

INCLUDE_TESTS=false
CHECK_STAGED_ONLY=false

for arg in "$@"; do
  case "$arg" in
    --include-tests) INCLUDE_TESTS=true ;;
    --staged) CHECK_STAGED_ONLY=true ;;
  esac
done

# Determine which files to check
if [ "$CHECK_STAGED_ONLY" = true ]; then
  FILES=$(git diff --cached --name-only --diff-filter=ACM -- '*.go' || true)
else
  FILES=$(find . -name '*.go' -not -path './vendor/*' -not -path './.git/*')
fi

if [ -z "$FILES" ]; then
  exit 0
fi

# Filter out test files unless --include-tests is passed
if [ "$INCLUDE_TESTS" = false ]; then
  FILES=$(echo "$FILES" | grep -v '_test\.go$' || true)
fi

if [ -z "$FILES" ]; then
  exit 0
fi

# Find time.Now() calls that are NOT followed by .UTC() or .Unix*()
# Pattern: time.Now() not followed by .UTC() or .Unix
VIOLATIONS=""
while IFS= read -r file; do
  # Use grep to find lines with time.Now() then filter out allowed patterns
  MATCHES=$(grep -n 'time\.Now()' "$file" 2>/dev/null | \
    grep -v 'time\.Now()\.UTC()' | \
    grep -v 'time\.Now()\.Unix' | \
    grep -v '// utc-lint:ignore' || true)
  if [ -n "$MATCHES" ]; then
    VIOLATIONS="${VIOLATIONS}${file}:
${MATCHES}
"
  fi
done <<< "$FILES"

if [ -n "$VIOLATIONS" ]; then
  echo "ERROR: Found time.Now() without .UTC() — use time.Now().UTC() instead."
  echo ""
  echo "Violations:"
  echo "$VIOLATIONS"
  echo ""
  echo "To fix: replace time.Now() with time.Now().UTC()"
  echo "To suppress a specific line, add: // utc-lint:ignore"
  exit 1
fi

exit 0
