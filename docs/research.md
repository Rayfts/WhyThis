# Initial archaeology/competitor research

The initial architecture was chosen after inspecting established Git-history/code-evolution projects and searching GitHub for tools attempting software archaeology.

## Relevant projects

- [`erikbern/git-of-theseus`](https://github.com/erikbern/git-of-theseus) visualizes how code survives/replaces itself through repository history. Its strength is code-age/evolution visualization rather than reconstructing evidence-backed engineering decisions.
- [`adamtornhill/code-maat`](https://github.com/adamtornhill/code-maat) mines version-control history for behavioral code metrics such as change coupling and churn. It strongly informed WhyThis's decision to keep risk indicators deterministic.
- Native Git remains the most important dependency: `blame`, `log -L`, `--follow`, rename detection, `-S`/`-G` pickaxe, `patch-id`, `range-diff`, `merge-base` and ancestry traversal encode years of edge-case behavior that should not be casually reimplemented.

The GitHub search did not reveal an established project with the exact WhyThis contract: select a line/symbol/PR, reconstruct a provenance graph across Git and GitHub discussion, then force semantic synthesis to distinguish FACT / INFERENCE / UNKNOWN. That gap is the project's intended niche, not a claim that no adjacent commercial or research tool exists.

## Architectural consequences

1. Native Git is wrapped, not replaced.
2. Historical metrics/risk remain deterministic.
3. GitHub discussion is evidence only when fetched and preserved with provenance.
4. AI is a semantic interpreter behind a `--no-ai`-complete deterministic path.
5. Symbol identity is not overclaimed. The initial design used pickaxe/text history as the portable fallback; the current implementation also uses Go AST declarations and `git log -L` for Go symbols, while non-Go and deleted-symbol paths retain the textual fallback.
