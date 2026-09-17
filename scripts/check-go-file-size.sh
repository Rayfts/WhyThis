#!/usr/bin/env sh
set -eu

max_lines=${WHYTHIS_MAX_GO_FILE_LINES:-300}
failed=0

find cmd internal pkg -type f -name '*.go' -print | sort | while IFS= read -r file; do
  lines=$(wc -l < "$file" | tr -d ' ')
  if [ "$lines" -gt "$max_lines" ]; then
    printf '%s has %s lines (limit %s)\n' "$file" "$lines" "$max_lines" >&2
    failed=1
  fi
done

# POSIX while loops fed by a pipe run in a subshell, so perform a second
# aggregation to make the failure status observable by this script.
over=$(find cmd internal pkg -type f -name '*.go' -exec wc -l {} \; | awk -v max="$max_lines" '$1 > max {n++} END {print n+0}')
if [ "$over" -gt 0 ]; then
  exit 1
fi

printf 'Go source size check passed: every file is <= %s lines.\n' "$max_lines"
