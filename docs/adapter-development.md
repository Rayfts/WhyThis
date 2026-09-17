# Harness adapter development

WhyThis is harness-neutral. Third-party Go modules can implement `github.com/Rayfts/WhyThis/pkg/harness.Adapter` without importing any `internal/` package.

```go
type Adapter interface {
    ID() string
    Detect(context.Context) (Capabilities, error)
    Analyze(context.Context, AnalysisRequest) (AnalysisResult, error)
}
```

`Detect` must report only capabilities that can actually be observed for the installed harness. `Analyze` receives a prompt containing already-collected WhyThis evidence. An adapter may interpret or summarize that evidence, but it must not manufacture provenance, promote its own text to FACT, or determine deterministic Git/GitHub facts that the collector can establish itself.

Built-in adapters are registered by `internal/harness.NewRegistry`. Embedders can create a registry and call `Register` with additional adapters. IDs must be non-empty and unique.

## Adapter checklist

1. Link the upstream repository/source that documents the integration mechanism.
2. Prefer a documented non-interactive protocol, structured stream, RPC mode, or explicit handoff contract.
3. Keep the analyzed repository read-only to the harness when practical; WhyThis built-ins execute synthesis in temporary directories.
4. Report unsupported telemetry/capabilities rather than guessing.
5. Add detection and invocation tests using sanitized fixtures or fake executables.
6. Keep model-generated conclusions as INFERENCE.

See `docs/harnesses.md` for the evidence-backed built-in integrations.
