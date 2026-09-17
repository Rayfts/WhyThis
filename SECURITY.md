# Security policy

Please report security vulnerabilities privately through GitHub's security advisory flow for this repository when available. Do not open a public issue containing exploit details, credentials or sensitive repository history.

## Scope

Security-relevant areas include command construction, path handling, GitHub token handling, HTTP server exposure, SQLite/cache integrity, harness process isolation, prompt/evidence leakage and unsafe parsing of untrusted repository metadata.

WhyThis does not require a GitHub token for local Git analysis. Tokens are read from environment variables and must never be written into evidence, logs or cache keys. Harness synthesis may transmit the evidence included in a prompt to the configured model/provider; users are responsible for ensuring that repository history is appropriate to send to that provider.

The local HTTP API binds to loopback by default and currently has no authentication. Do not expose it directly to an untrusted network.
