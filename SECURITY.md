# Security Policy

WhyThis reads repository history, optionally queries GitHub, and can pass collected evidence to external coding-agent harnesses. Treat repositories, commit messages, issue text, review text, and harness output as untrusted input.

## Reporting a vulnerability

Do not publish exploit details, credentials, private repository contents, or token-exposure bugs in a public issue.

If GitHub private vulnerability reporting is enabled, use **Security → Report a vulnerability**. Otherwise, open a minimal non-sensitive issue requesting a private reporting channel.

Include the affected commit/version, operating system, minimal reproduction, impact, and whether the issue can expose repository data, GitHub credentials, local files, or execute unintended commands.

## Security boundaries

- Harness output is inference, never historical evidence.
- Harness synthesis should use temporary working directories and should not require write access to the analyzed repository.
- GitHub tokens should be narrowly scoped and are used only when enrichment is requested/configured.
- The local HTTP API binds to loopback by default. Authentication is not currently provided; do not expose it directly to untrusted networks.
- Do not place secrets in prompts, commit messages, issue bodies, exported evidence, screenshots, or fixtures.
- Repository text rendered in the TUI/API must be treated as data, not instructions.

A security fix that changes evidence interpretation should include tests proving that provenance and FACT / INFERENCE / UNKNOWN separation are preserved.