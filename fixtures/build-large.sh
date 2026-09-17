#!/usr/bin/env bash
set -euo pipefail
COUNT="${1:-1000}"
ROOT="${2:-$(pwd)/fixtures/tmp-large-monorepo}"
rm -rf "$ROOT"
mkdir -p "$ROOT"
cd "$ROOT"
git init -q
git config user.name "WhyThis Large Fixture"
git config user.email "fixture@example.com"
for ((i=1;i<=COUNT;i++)); do
  pkg=$((i % 200))
  mkdir -p "packages/p${pkg}"
  printf 'package p%d\n// revision %d\nfunc Value() int { return %d }\n' "$pkg" "$i" "$i" > "packages/p${pkg}/value.go"
  git add .
  GIT_AUTHOR_DATE="2026-01-01T00:00:00Z" GIT_COMMITTER_DATE="2026-01-01T00:00:00Z" git commit -q -m "fixture: change package ${pkg} at ${i}"
done
printf 'built %s commits at %s\n' "$COUNT" "$ROOT"
