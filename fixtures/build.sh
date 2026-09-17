#!/usr/bin/env sh
set -eu
root=${1:-./tmp-history}
rm -rf "$root"
mkdir -p "$root"
cd "$root"
git init -q
git config user.name "WhyThis Fixture"
git config user.email fixture@example.com
cat > retry.go <<'GO'
package fixture
func Retry() int { return 1 }
GO
git add . && git commit -q -m "feat: add retry loop refs #10"
cat > retry.go <<'GO'
package fixture
func Retry() int { return 3 } // regression: retries too aggressively
GO
git add . && git commit -q -m "bug: reproduce retry storm refs #11"
cat > retry.go <<'GO'
package fixture
func Retry() int { return 2 }
GO
cat > retry_test.go <<'GO'
package fixture
func ExampleRetry() { _ = Retry() }
GO
git add . && git commit -q -m "fix: bound retry attempts fixes #11"
git revert -q --no-edit HEAD
git mv retry.go backoff.go
git commit -q -m "refactor: rename retry implementation"
cat > backoff.go <<'GO'
package fixture
func Retry() int { return 2 } // reimplementation after revert
GO
git add . && git commit -q -m "feat: reimplement bounded retry refs #14"
printf '%s\n' "$root"
