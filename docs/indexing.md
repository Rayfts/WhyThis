# Incremental indexing

WhyThis stores index state in `.whyth/whythis.db` by default.

## Normal update

1. Read `index.head` from SQLite.
2. Resolve current `HEAD`.
3. Verify the stored commit is still an ancestor with `git merge-base --is-ancestor`.
4. Traverse only `index.head..HEAD` with `git rev-list --reverse`.
5. Store commit metadata and per-file numstat/name-status rows transactionally.
6. Advance `index.head` only after all new commits are processed.

## Rewrites

If the prior indexed head is no longer an ancestor—common after a rebase, force-push or history replacement—the commit/file-change index is rebuilt. This favors correctness over trying to reconcile arbitrary rewritten ancestry.

## Shallow clones

`whythis doctor` reports shallow repository state. WhyThis still works, but evidence can only describe history present locally. It never implies that missing ancestors were inspected.

## Large repositories

The index avoids reprocessing all history on normal updates and maintains an index on file-change paths. Future work includes batched commit ingestion, configurable generated/vendor exclusions, symbol tables and background GitHub enrichment. Concurrency will be added only where it does not alter Git ordering/provenance semantics.
