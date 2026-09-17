# WhyThis

**WhyThis reconstructs the evidence-backed history behind code.**

Modern tools are good at explaining what code does. WhyThis is aimed at the harder maintenance question: **why does this code exist?** It walks Git history, blame, renames, commits, reverts, tests and—when configured—GitHub pull requests/issues, then preserves that material in a provenance-aware evidence graph. Optional coding-agent adapters can interpret the evidence, but they are never allowed to replace it.

> Core rule: **never answer “why” without showing the evidence.**

## Status

WhyThis is an early production-oriented implementation on the road to `v0.1.0`. The deterministic core, incremental SQLite index, local HTTP API, risk signals, GitHub PR enrichment, and ten harness capability adapters are implemented. Symbol tracking is currently text/pickaxe based rather than AST-identity based. Roo Code support is intentionally limited to legacy extension detection and evidence handoff because the upstream project states that the Roo Code extension was shut down on May 15, 2026.

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
whythis pr 123
whythis ask "What broke before this retry code was added?"
whythis index
whythis doctor
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

For a line/range, WhyThis uses native Git blame and history machinery to identify introducing/current attribution, subsequent file edits, explicit revert trailers, rename detection, issue/PR references in commit messages, and test-like files changed alongside relevant commits. File history uses `git log --follow`; symbol archaeology uses pickaxe history; PR lookup can add reviews/comments through the GitHub REST API. The resulting nodes and edges retain provenance fields describing the collector, locator and revision.

The index is incremental. It records the indexed head, verifies ancestry with `git merge-base --is-ancestor`, indexes only new commits on a normal fast-forward history, and rebuilds when the previous indexed head is no longer an ancestor (for example after a rebase).

## Historical risk

`whythis risk <file>` reports deterministic signals rather than an opaque AI score. The current default indicators are repeated fix-like commits, repeated reverts, line churn and author-count/ownership churn. Thresholds and their exact meaning are documented in [`docs/evidence-model.md`](docs/evidence-model.md). A lack of triggered indicators is **not** a safety guarantee.

## GitHub enrichment

Local Git works without GitHub. To enrich PR/issue context, set one of:

```bash
export WHYTHIS_GITHUB_TOKEN=...
# or GITHUB_TOKEN / GH_TOKEN
```

GitHub responses are cached in the same SQLite database with ETags and bounded TTLs. Rate-limit exhaustion is returned as an explicit error rather than silently dropping evidence.

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

See [`docs/harnesses.md`](docs/harnesses.md) for the upstream source paths used to justify each mechanism and for limitations. Run `whythis harnesses` or `whythis capabilities <id>` to inspect what is actually available on the current machine.

## Local API

```bash
whythis serve 127.0.0.1:7788
```

Endpoints:

- `GET /healthz`
- `GET /v1/archaeology?target=src/foo.go:42-60`
- `GET /v1/history?path=src/foo.go`
- `GET /v1/risk?path=src/foo.go`
- `GET /v1/harnesses`

The server binds to loopback by default and sets conservative HTTP timeouts. Authentication is not yet implemented; do not expose it directly to untrusted networks.

## Architecture

See:

- [`docs/architecture.md`](docs/architecture.md)
- [`docs/evidence-model.md`](docs/evidence-model.md)
- [`docs/indexing.md`](docs/indexing.md)
- [`docs/harnesses.md`](docs/harnesses.md)
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

`modernc.org/sqlite` is used for cross-platform, cgo-free SQLite persistence. CI runs formatting, vet, tests, race detection where practical, vulnerability scanning, linting, benchmarks, and cross-platform builds.

## Security and evidence integrity

Do not place secrets in issue bodies, commit messages, prompts, or exported evidence. Harnesses are run in temporary working directories for synthesis so they do not need write access to the analyzed repository. WhyThis treats harness output as inference, not evidence. See [`SECURITY.md`](SECURITY.md) for reporting vulnerabilities.

## License

Apache-2.0. See [`LICENSE`](LICENSE).
