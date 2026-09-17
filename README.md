# WhyThis

**WhyThis reconstructs the evidence-backed history behind code.**

Modern tools are good at explaining what code does. WhyThis is aimed at the harder maintenance question: **why does this code exist?** It walks Git history, blame, renames, commits, reverts, tests and—when configured—GitHub pull requests/issues, then preserves that material in a provenance-aware evidence graph. Historical-risk reports also surface matching CODEOWNERS declarations when present. Optional coding-agent adapters can interpret the evidence, but they are never allowed to replace it.

> Core rule: **never answer “why” without showing the evidence.**

## Status

WhyThis implements the planned `v0.1` feature set as a local-first Go tool: deterministic Git archaeology, Go AST-backed symbol anchoring with textual fallback, incremental SQLite indexing, a local HTTP API, an interactive TUI, explainable historical-risk scoring, deterministic sensitive-code classification, CODEOWNERS evidence, paginated GitHub PR/issue/review enrichment, historical CI/release evidence, deterministic similar-change search, and ten built-in harness adapters plus a public adapter contract. Roo Code support is intentionally limited to legacy extension detection and evidence handoff because the upstream project states that the Roo Code extension was shut down on May 15, 2026.

## Install

Requires **Go 1.27+** and the native `git` executable.

```bash
go install github.com/Rayfts/WhyThis/cmd/whythis@latest
```

Or build from source:

```bash
git clone https://github.com/Rayfts/WhyThis.git
cd WhyThis
go build -o whythis ./cmd/whythis
```

## Examples

```bash
whythis src/auth/session.go:142
whythis file src/auth/session.go:100-140
whythis symbol SessionManager.Refresh
whythis history src/auth/session.go
whythis risk src/auth/session.go
whythis commit <sha>
whythis similar <sha> 10
whythis pr 123
whythis ask "What broke before this retry code was added?"
whythis index
whythis doctor
whythis tui
```

Machine-readable, deterministic operation:

```bash
whythis --json --no-ai src/auth/session.go:142
```

Optional semantic synthesis:

```bash
whythis --harness codex src/auth/session.go:142
whythis --harness claude-code symbol SessionManager.Refresh
```

The synthesis is emitted under **INFERENCES**. Git/GitHub-derived **FACTS** stay separate. Questions that history cannot establish are emitted under **UNKNOWN**.

## What the deterministic core does

For a line/range, WhyThis uses native Git blame and history machinery to identify introducing/current attribution, subsequent file edits, explicit revert trailers, rename detection, issue/PR references in commit messages, and test-like files changed alongside relevant commits. File history uses `git log --follow`. Go symbols are anchored to real declarations with the Go AST and traced through `git log -L` plus surrounding file/rename history; textual pickaxe remains a fallback for deleted symbols and non-Go languages. With a GitHub token, historical failed checks/statuses and release tags are attached to relevant commits as first-class evidence. PR lookup adds fully paginated reviews/comments/files/commits. `whythis similar <sha>` first scans changed paths for up to 2,000 recent commits in a single Git traversal, then materializes only a bounded shortlist for stable patch-ID and changed-line comparison. The measured overlap is evidence, while any claim that two patches share intent remains inference. The resulting nodes and edges retain provenance fields describing the collector, locator and revision.

The index is incremental. It records the indexed head, verifies ancestry with `git merge-base --is-ancestor`, indexes only new commits on a normal fast-forward history, and rebuilds when the previous indexed head is no longer an ancestor (for example after a rebase).

## Historical risk

`whythis risk <file>` reports deterministic signals plus an explainable **0–100 historical-risk heuristic** and evidence confidence (`low`, `medium`, `high`). The score is computed only from bounded repository-history signals: fix-like commit density, reverts, churn, and authorship breadth. Sensitive-code, generated/vendor, submodule and CODEOWNERS classifications are reported separately and do not silently inflate the score. Confidence is reduced for shallow or very sparse history. CODEOWNERS source/rule/owners are reported separately as responsibility evidence and do not add score by themselves. The formula and thresholds are documented in [`docs/evidence-model.md`](docs/evidence-model.md). The score is not a defect probability, and a low score is **not** a safety guarantee.

## GitHub enrichment

Local Git works without GitHub. To enrich PR/issue context, set one of:

```bash
export WHYTHIS_GITHUB_TOKEN=...
# or GITHUB_TOKEN / GH_TOKEN
```

GitHub responses are cached in the same SQLite database with ETags and bounded TTLs. Automatic CI/release enrichment is enabled when a GitHub token is configured; ordinary Git archaeology remains fully local/offline. Explicit `whythis pr` queries can still use anonymous GitHub access for public repositories. Rate-limit exhaustion is explicit rather than silently dropping evidence.

## Harnesses

WhyThis currently exposes adapters/capability detection for:

| Harness | Verified integration used by WhyThis |
|---|---|
| OpenAI Codex | `codex exec --json --ephemeral` JSONL |
| Claude Code | print mode with `--output-format stream-json` |
| OpenCode | `opencode run --format json` |
| Pi | `pi --mode rpc --no-session` strict JSONL RPC |
| Gemini CLI | `gemini -p ... --output-format stream-json` |
| Aider | one-shot `--message-file` |
| Goose | `goose run --text ... --output-format stream-json --no-session` |
| Cline | one-shot `cline --json ...` NDJSON |
| Roo Code | legacy VS Code extension detection + evidence handoff only |
| Continue | `cn -p ... --format json` |

See [`docs/harnesses.md`](docs/harnesses.md) for the upstream source paths used to justify each mechanism and for limitations. Run `whythis harnesses` or `whythis capabilities <id>` to inspect what is actually available on the current machine. External Go integrations can implement the public [`pkg/harness.Adapter`](pkg/harness/harness.go) contract; see [`docs/adapter-development.md`](docs/adapter-development.md).


## Interactive TUI

Run `whythis tui` for a local terminal workspace. It exposes line, symbol, file, commit, PR, similar-change, risk, natural-language and harness views. Evidence reports render dedicated source, FACT / INFERENCE / UNKNOWN, timeline, blame, linked PR/issue/review/CI/release, and graph-edge panels. The TUI calls the same service layer as the CLI and HTTP API; it does not introduce a separate evidence path.

## Local API

```bash
whythis serve 127.0.0.1:7788
```

Endpoints:

- `GET /healthz`
- `GET /v1/archaeology?target=src/foo.go:42-60`
- `GET /v1/history?path=src/foo.go`
- `GET /v1/symbol?name=SessionManager.Refresh`
- `GET /v1/commit?sha=<sha>`
- `GET /v1/similar?sha=<sha>&limit=10`
- `GET /v1/ask?q=What+broke+before+this+retry+code+was+added%3F`
- `GET /v1/pr?number=123`
- `GET /v1/risk?path=src/foo.go`
- `GET /v1/harnesses`
- `GET /v1/capabilities?id=codex`

The server binds to loopback by default and sets conservative HTTP timeouts. Authentication is not yet implemented; do not expose it directly to untrusted networks.

## Architecture

See:

- [`docs/architecture.md`](docs/architecture.md)
- [`docs/evidence-model.md`](docs/evidence-model.md)
- [`docs/indexing.md`](docs/indexing.md)
- [`docs/harnesses.md`](docs/harnesses.md)
- [`docs/adapter-development.md`](docs/adapter-development.md)
- [`docs/research.md`](docs/research.md)

The project intentionally shells out to the native Git executable through a single `internal/gitx` package instead of reimplementing mature behavior such as blame, rename detection, ancestry, follow, and pickaxe semantics in Go.

## Development

```bash
make fmt
make vet
make test
make race
make bench
```

`modernc.org/sqlite` is used for cross-platform, cgo-free SQLite persistence. CI runs formatting, module-tidiness checks, vet, tests, race detection, vulnerability scanning, strict linting, benchmark smoke tests, and Linux/macOS/Windows builds. Benchmarks cover Git traversal, file-history queries, large-monorepo similarity prefiltering, incremental indexing, risk evaluation, and SQLite metadata I/O. `fixtures/build-large.sh` can generate a deterministic 1,000+ commit monorepo for profiling. Edge-case coverage includes shallow clones, deleted files, rename lineage, merge parents, explicit multi-link revert chains, generated/vendor classification and submodule pointer detection.

## Security and evidence integrity

Do not place secrets in issue bodies, commit messages, prompts, or exported evidence. Harnesses are run in temporary working directories for synthesis so they do not need write access to the analyzed repository. WhyThis treats harness output as inference, not evidence. See [`SECURITY.md`](SECURITY.md) for reporting vulnerabilities.

## License

Apache-2.0. See [`LICENSE`](LICENSE).
