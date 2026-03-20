#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)
cd "$repo_root"

fail=0

# Rule: handlers/controllers should not import repository directly.
if rg -n '"github.com/Wei-Shaw/sub2api/internal/repository"' backend/internal/handler >/tmp/sub2api-arch-handler-repo.txt 2>/dev/null; then
  echo "FAIL: handler imports repository directly"
  cat /tmp/sub2api-arch-handler-repo.txt
  fail=1
fi

# Rule: service code should not import Gin directly.
if rg -n '"github.com/gin-gonic/gin"' backend/internal/service >/tmp/sub2api-arch-service-gin.txt 2>/dev/null; then
  echo "FAIL: service imports gin directly"
  cat /tmp/sub2api-arch-service-gin.txt
  fail=1
fi

if [[ $fail -ne 0 ]]; then
  exit 1
fi

echo "PASS: architecture checks"
