# Evidence model

WhyThis deliberately separates observations from interpretations.

## Claims

### FACT

A fact is directly supported by collected Git/GitHub material. Examples:

- blame attributes a line to commit `abc123`
- a commit message literally references `#42`
- a `git show --name-status --find-renames` result records a rename
- a commit body contains Git's explicit `This reverts commit ...` trailer
- GitHub's API reports a particular PR title/state

### INFERENCE

An inference is an interpretation of facts. Examples:

- a retry guard was *likely* introduced in response to a timeout regression
- a rename and nearby test addition *suggest* a refactor was intended to preserve behavior

Harness output is always stored/displayed as inference, never promoted to fact.

### UNKNOWN

Unknown records the boundary of available history. If an engineering decision was never documented, WhyThis says so rather than filling the gap with a plausible story.

## Provenance

Every evidence node/edge carries:

- `source` — collector family (`git`, `git-blame`, `github-rest`, etc.)
- `locator` — command/resource that produced the observation
- `repository`
- `revision`
- `collected_at`

This allows downstream clients to show exactly where each assertion came from.

## Graph vocabulary

Implemented node kinds include repository, file, line range, symbol, commit, pull request, issue, test and blame segment, with the wider model reserving diff, patch, review comment, issue comment, dependency, release, author, documentation and CI-failure nodes.

Implemented/available edge vocabulary includes `introduced-by`, `changed-by`, `reverted-by`, `renamed-from`, `references`, `tested-by`, `followed-by`, `depends-on`, `associated-with` and the broader model's removal/restoration/move/fix/discussion/supersession relations.

## Risk indicators

Risk is deterministic and explainable. There is intentionally no LLM-generated score.

Current thresholds:

| Indicator | Trigger | Meaning |
|---|---:|---|
| repeated-fixes | 3 fix/bug/regression-like commit subjects | inspect prior defects before changes |
| reverts | 2 revert-form subjects | prior implementations were rolled back at least twice |
| high-churn | 500 added/deleted lines | substantial historical change pressure |
| ownership-churn | 10 distinct author emails | ownership context is distributed |

These are triage signals, not probabilities and not proof of current defect risk.


## Ownership evidence

WhyThis distinguishes declared code ownership from historical authorship. Git commit authors are collected from repository history. Declared ownership is evaluated from GitHub-standard CODEOWNERS locations in precedence order: `.github/CODEOWNERS`, `CODEOWNERS`, then `docs/CODEOWNERS`; within a ruleset, the last matching rule wins. The matched source, rule, line and owners are recorded in the deterministic historical-risk report.

A CODEOWNERS assignment does not prove why code exists and is never treated as intent. It only records the repository's declared responsibility for that path at the analyzed revision. Repositories without CODEOWNERS continue normally.
