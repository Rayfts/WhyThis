# Contributing to WhyThis

Thanks for helping improve WhyThis. Useful contributions include Git-history retrieval, evidence modeling, indexing, GitHub enrichment, TUI/API work, harness adapters, performance improvements, fixtures, and documentation.

The project's central rule is simple: **never answer “why” without showing the evidence.**

## Development setup

WhyThis requires Go 1.27+ and the native `git` executable.

```bash
make fmt
make vet
make test
make race
make bench
```

For focused development, normal Go commands are also fine:

```bash
go test ./...
go vet ./...
go build ./cmd/whythis
```

## Design rules

1. Git/GitHub-derived observations are **FACTS**; agent interpretation is **INFERENCE**.
2. Unknown history stays **UNKNOWN**. Do not turn absence of evidence into a confident explanation.
3. Prefer native Git behavior for blame, rename tracking, ancestry, pickaxe, and line history rather than reimplementing semantics loosely.
4. Historical-risk scores must remain deterministic, explainable, and independent of LLM output.
5. New evidence nodes/edges must preserve provenance and a stable locator/revision where possible.
6. Harness adapters may synthesize evidence but must not become the source of historical facts.
7. Do not invent third-party flags or capabilities; cite upstream source/docs in adapter changes.
8. Add fixtures or regression tests for archaeology edge cases such as renames, reverts, deleted files, merges, shallow history, and rebases.

## Pull requests

Keep PRs focused and explain the evidence path affected. Include tests for behavior changes, benchmark notes for expensive history/indexing changes, and documentation updates when CLI/API output or evidence semantics change.

Security-sensitive changes should follow `SECURITY.md`. By contributing, you agree to follow `CODE_OF_CONDUCT.md` and the Apache-2.0 license terms.