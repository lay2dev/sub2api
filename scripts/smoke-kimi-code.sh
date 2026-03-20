#!/usr/bin/env bash
set -euo pipefail

cat <<'EOF'
Kimi Code smoke checklist

AC1: verify configurable base_url/auth/default_headers can target https://api.kimi.com/coding/v1
AC2: verify upstream requests include User-Agent: claude-code/1.0
AC3: verify /models can return kimi-for-coding
AC4: verify non-stream /chat/completions succeeds
AC5: verify stream /chat/completions returns SSE and [DONE]
AC6: verify implementation remains OpenAI-compatible first-pass, without full PlatformKimi

This script is intentionally lightweight until code integration lands.
When implementation is complete, replace this checklist with executable smoke requests.
EOF
