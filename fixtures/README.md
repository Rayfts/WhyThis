# Synthetic history fixtures

`build.sh` creates a small deterministic repository containing a bug/fix, regression-oriented test, rename, revert, and reimplementation-style history for correctness tests.

`build-large.sh [commit-count] [output-dir]` creates a deterministic monorepo-shaped history (default 1,000 commits across 200 package paths) for manual scale testing and benchmark profiling. The generated repositories live under ignored `fixtures/tmp-*` paths and are never committed.
