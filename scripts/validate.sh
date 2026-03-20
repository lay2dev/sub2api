#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

bash scripts/check-architecture.sh
bash scripts/check-ac-coverage.sh plans/2026-03-20-kimi-code-support.md
bash scripts/smoke-kimi-code.sh

echo "PASS: validate.sh"
