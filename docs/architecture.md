# Architecture

WhyThis is split into three trust layers.

## 1. Deterministic collectors

`internal/gitx` is the only package allowed to launch Git. It wraps native commands rather than scattering `exec.Command("git", ...)` across the application. History analysis composes `git blame`, `git log --follow`, `git show`, pickaxe, rename detection, ancestry and related machinery into typed evidence.

`internal/githubx` adds optional remote context through the GitHub REST API. It is never required for local analysis. Responses are bounded, cached, ETag-aware and rate-limit errors are explicit.

## 2. Evidence/index layer

`pkg/evidence` defines stable public node, edge, provenance and claim types. `internal/graph` deduplicates graph objects. `internal/storage` persists commit/file-change metadata, graph elements and GitHub cache data in SQLite. `internal/index` performs ancestry-aware incremental indexing.

An evidence node is not merely display text. It carries a source, locator, repository, revision and collection timestamp. That provenance is the boundary between observed history and interpretation.

## 3. Interpretation layer

`internal/harness` exposes a common Go interface over verified harness integration mechanisms. `internal/analysis` sends a serialized deterministic report to a selected harness with strict instructions not to invent evidence. Returned text is appended only as an `inference` claim.

No harness can add a fact to the deterministic graph.

## Data flow

```text
user target
   |
   v
native Git -----> deterministic evidence graph -----> renderer / JSON / HTTP
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

## Native Git rationale

Git's historical behavior is full of edge cases: combined merges, rename similarity, path following, shallow history, patch equivalence and pickaxe semantics. WhyThis prefers calling the installed Git implementation for those operations. The Go code owns orchestration, parsing, graph construction, persistence, APIs and concurrency boundaries.

## Current limitations

- Symbol archaeology starts with textual pickaxe identity. Tree-sitter/language-server identity tracking is planned but not claimed as implemented.
- GitHub enrichment currently targets PR/issue/review/comment context; releases and CI-failure ingestion are modelled but not yet collected by default.
- Similar-patch retrieval and patch-id/range-diff ranking are planned extensions to the existing Git wrapper.
- Roo Code is legacy/handoff-only because its upstream extension is shut down.
- The local HTTP server has no authentication and is loopback-only by default.
