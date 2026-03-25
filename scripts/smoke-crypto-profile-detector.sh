#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "$0")/.." && pwd)

for arg in "$@"; do
  if [[ "$arg" == "--help" || "$arg" == "-h" ]]; then
    cd "$repo_root/backend"
    export GOCACHE="${GOCACHE:-/tmp/sub2api-go-build}"
    export GOMODCACHE="${GOMODCACHE:-/tmp/sub2api-go-mod}"
    export GOTMPDIR="${GOTMPDIR:-/tmp}"
    go run ./cmd/crypto_profile_smoke --help
    exit 0
  fi
done

if [[ -z "${OPENROUTER_API_KEY:-}" ]]; then
  echo "OPENROUTER_API_KEY is required"
  exit 1
fi

cd "$repo_root/backend"

export GOCACHE="${GOCACHE:-/tmp/sub2api-go-build}"
export GOMODCACHE="${GOMODCACHE:-/tmp/sub2api-go-mod}"
export GOTMPDIR="${GOTMPDIR:-/tmp}"

go run ./cmd/crypto_profile_smoke --dataset testdata/crypto_profile_dataset.json "$@"
