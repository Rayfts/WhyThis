# Architecture

WhyThis is split into three trust layers plus presentation surfaces.

## 1. Deterministic collectors

`internal/gitx` is the only package allowed to launch Git. It wraps native commands rather than scattering Git subprocesses across the application. History analysis composes blame, `log --follow`, `log -L`, `show`, stable patch IDs, rename detection, ancestry and pickaxe behavior into typed evidence.

`internal/symbol` adds language-aware anchors. Go declarations are parsed with the standard Go AST to produce concrete path/line identities for functions, methods, types, variables and constants. The history layer then asks native Git to trace those declaration ranges. Textual pickaxe remains a fallback for non-Go languages, deleted declarations and names that only exist historically.

`internal/githubx` adds optional remote context through the GitHub REST API: PRs, issues, reviews/comments, files, PR commits, historical check runs/statuses and releases. Pagination is explicit. Automatic remote enrichment requires a configured GitHub token so local archaeology never becomes network-dependent.

## 2. Evidence/index layer

`pkg/evidence` defines public node, edge, provenance and claim types. `internal/graph` deduplicates graph objects and validates edge closure. `internal/storage` persists commit/file-change metadata, evidence graphs and GitHub cache data in SQLite. `internal/index` performs ancestry-aware incremental indexing.

Every deterministic node/edge retains collector, locator, repository/revision where applicable, and collection timestamp. FACT claims point back to those evidence IDs.

Historical risk is deterministic. `internal/risk` computes bounded signals and an explainable 0–100 heuristic from fix-like history, reverts, churn and authorship breadth. Confidence reflects history completeness/volume; no model is involved in scoring.

## 3. Interpretation layer

`pkg/harness` is the public external adapter contract. `internal/harness` registers the ten built-in, source-researched integrations. `internal/analysis` serializes the deterministic report to a selected harness and appends returned text only as INFERENCE.

No harness can add FACT claims to the deterministic graph.

## Presentation

The CLI, `internal/tui` interactive terminal workspace and `internal/server` HTTP API all use the same application service layer. The TUI renders source, timeline, blame, commit history, linked GitHub/CI/release evidence, graph edges and FACT / INFERENCE / UNKNOWN panels.

## Data flow

```text
user target
   |
   v
native Git -----> deterministic evidence graph -----> CLI / TUI / JSON / HTTP
   |                         |
   |                         +------> SQLite index/cache
   |
GitHub REST (optional) ------+
                              |
                              +------> harness synthesis (optional)
                                           |
                                           v
                                    INFERENCE only
```

## Scale behavior

Similarity search performs one cheap changed-path traversal over up to 2,000 recent commits, ranks candidates by path overlap and recency, and materializes patches only for a bounded shortlist before stable patch-ID / changed-line comparison. Incremental indexing processes only commits after the previously indexed ancestor and rebuilds after rewritten history.

## History boundaries

- Shallow clones are detected and surfaced by `doctor`; risk confidence drops when history is incomplete.
- Renames use native Git rename/follow behavior; deleted paths remain queryable through file history even when they no longer exist at HEAD.
- Merge, squash and rebase archaeology reflects the history that actually exists in the local DAG. WhyThis does not invent pre-squash or pre-rebase commits that are no longer present.
- Go AST anchors strengthen symbol identity at HEAD, but a semantic rename that changes identifier and surrounding declaration still requires Git lineage or recorded discussion to prove continuity.
- Roo Code remains legacy/handoff-only because its upstream extension is shut down.
- The HTTP server has no authentication and binds to loopback by default.

## Sensitive-code detection and repository edge cases

`internal/sensitive` performs deterministic path/content-keyword classification without exposing matched secret values. `risk` surfaces sensitivity, generated/vendor state, shallow-history confidence and Git submodule mode separately from the historical-risk score. Generated/vendor code stays queryable, and submodule pointer history is analyzed in the parent repository without pretending to contain the nested repository's commit history.
