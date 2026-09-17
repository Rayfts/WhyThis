# GitHub Actions usage

WhyThis intentionally does not comment on every pull request by default. Historical context is useful only when it is relevant, and noisy bots train maintainers to ignore evidence.

The repository includes a manual `workflow_dispatch` example (`.github/workflows/whythis-pr.yml`). It builds the current source and emits a JSON artifact for a requested PR. Teams can later decide whether to post selected findings as checks/comments after defining their own noise threshold.

The workflow uses the built-in `GITHUB_TOKEN` only for GitHub enrichment. Deterministic local Git analysis does not require a token.
