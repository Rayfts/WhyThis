# Incremental indexing

WhyThis keeps local cache/index state in SQLite (`.whyth/whythis.db` by default). The repository remains authoritative; the database can be deleted and rebuilt at any time.

## Update algorithm

1. Resolve current `HEAD`.
2. Read the previously indexed head from SQLite.
3. If there is no previous head, traverse all commits reachable from `HEAD` oldest-first.
4. If the previous head is an ancestor of current `HEAD`, traverse only `previous..HEAD`.
5. If ancestry fails (for example after rebase/history replacement), rebuild the commit/file-change index from reachable history.
6. Persist the new head only after the batch completes.

This preserves correctness across ordinary fast-forward work while avoiding full rescans.

## What is indexed

The current index stores commit metadata and changed paths with supporting SQLite indexes for commit/path lookup. Full evidence graphs and GitHub HTTP cache entries are also persisted through the same store when commands produce them. Language-aware Go symbol anchoring is computed against the current working tree and Git lineage rather than treated as a stale permanent symbol table.

## Large repositories

Similarity search does not materialize every historical patch. It performs one bounded Git traversal over up to 2,000 recent commits to collect changed paths, ranks candidates by path overlap, then materializes only a bounded shortlist for stable patch-ID and changed-line comparison. `fixtures/build-large.sh` produces a deterministic 1,000+ commit monorepo-shaped fixture for profiling.

## Shallow clones and rewritten history

`whythis doctor`, evidence reports and historical-risk confidence explicitly surface shallow repository state. WhyThis only claims history present locally. It never implies that missing ancestors, old renames, reverts or releases were inspected.

Merge commits retain all parents. Squash/rebase archaeology reflects the DAG that actually exists after rewriting; WhyThis does not invent discarded pre-squash/pre-rebase commits.

## Generated, vendor and submodule paths

Generated/vendor paths remain queryable because they can still be historically relevant, but deterministic path classification identifies them so reports do not imply repository-local history is authoritative for generated/upstream code. Git submodule entries are detected via mode `160000`: WhyThis can explain pointer changes in the parent repository, while nested commit history belongs to the submodule repository itself.
