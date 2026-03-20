#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

plan_file="${1:-plans/2026-03-20-kimi-code-support.md}"

if [[ ! -f "$plan_file" ]]; then
  echo "FAIL: plan file not found: $plan_file"
  exit 1
fi

acs=$(grep -o 'AC[0-9]\+' "$plan_file" | sort -u)
if [[ -z "$acs" ]]; then
  echo "FAIL: no AC markers found in $plan_file"
  exit 1
fi

search_files=$(find docs plans scripts backend/internal backend/cmd \
  \( -name '*.md' -o -name '*.sh' -o -name '*.go' -o -name '*.txt' \) -type f 2>/dev/null)

missing=0
for ac in $acs; do
  if ! grep -nH -E "${ac}([^0-9]|$)" $search_files >/dev/null 2>&1; then
    echo "FAIL: missing coverage for $ac"
    missing=1
  fi
done

if [[ $missing -ne 0 ]]; then
  exit 1
fi

echo "PASS: AC coverage check for $plan_file"
