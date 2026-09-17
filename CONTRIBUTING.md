# Contributing

Thanks for helping improve WhyThis.

## Development

Use Go 1.27 or newer and Git. Then run:

```bash
go mod download
make fmt
make vet
make test
make race
```

Before opening a pull request, add or update tests for archaeology behavior. Prefer synthetic repositories whose history is intentionally constructed and whose expected evidence can be asserted exactly.

## Evidence integrity rules

- Keep Go source files at or below 300 lines; split by responsibility when they grow beyond that boundary.
- Do not add a FACT unless a deterministic collector can point to its provenance.
- Do not parse an LLM response into deterministic evidence.
- Keep Git process execution inside `internal/gitx`.
- If a harness integration changes, cite current upstream source/docs in `docs/harnesses.md` and add capability tests.
- Do not silently ignore shallow history, GitHub rate limits, parse failures or missing credentials.

## Changes

Keep commits focused. Public behavior and new evidence relations should be documented. Security-sensitive changes should follow `SECURITY.md`.
