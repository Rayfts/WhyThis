# Coding-agent harness integrations

Harnesses interpret already-collected evidence. They are not evidence collectors and cannot promote their output to FACT.

The table below records the upstream repository/source inspected during the initial implementation. The point is to make integration claims auditable rather than guessed. Pi repository provenance was refreshed on **2026-09-18** after its upstream ownership move.

| ID | Official/current upstream inspected | Verified mechanism used | Notes |
|---|---|---|---|
| `codex` | `openai/codex` — `codex-rs/exec/src/cli.rs`, `exec_events.rs` | `codex exec --json --ephemeral --skip-git-repo-check -` | CLI source documents non-interactive exec, JSONL, ephemeral sessions and stdin prompt `-`. |
| `claude-code` | `anthropics/claude-code` — release feed and `plugins/plugin-dev` hook docs | print mode with `--output-format stream-json` | Upstream also exposes hooks including PreToolUse/PostToolUse/Stop/Session lifecycle events. WhyThis uses one-shot synthesis, not hooks. |
| `opencode` | `anomalyco/opencode` — `packages/web/src/content/docs/cli.mdx` | `opencode run --format json` | The former `opencode-ai/opencode` repository is archived; the active repository inspected is `anomalyco/opencode`. Docs also expose a headless `serve` backend. |
| `pi` | `mitsuhiko/pi-mono` — `packages/coding-agent/docs/rpc.md` | `pi --mode rpc --no-session` | Strict JSONL RPC over stdin/stdout; WhyThis sends a `prompt` command and consumes events until `agent_end`. The current package is `@mariozechner/pi-coding-agent`; the upstream project has moved owners over time, so capability evidence points at the current canonical repository. |
| `gemini` | `google-gemini/gemini-cli` — `docs/reference/configuration.md` | `gemini -p ... --output-format stream-json` | Upstream documents `json` and streaming `stream-json` non-interactive output. |
| `aider` | `Aider-AI/aider` — `aider/args.py` and config docs | `aider --message-file <file> --yes-always` | `--message-file` is documented as send/process/exit. WhyThis runs synthesis in a temporary directory to isolate it from the analyzed repo. |
| `goose` | `aaif-goose/goose` — `crates/goose-cli/src/cli.rs` and headless docs | `goose run --text ... --output-format stream-json --no-session` | CLI source defines text input, JSON/stream-JSON output and no-session automation mode. |
| `cline` | `cline/cline` — `apps/cli/README.md`, `apps/cli/src/main.ts` | `cline --json <prompt>` | Current Cline CLI explicitly documents one-shot/headless operation and NDJSON output. |
| `roo-code` | `RooCodeInc/Roo-Code` — `README.md` | installed-extension detection and prompt handoff | Upstream README states the extension was shut down on May 15, 2026. WhyThis does **not** invent a maintained headless CLI. |
| `continue` | `continuedev/continue` — `extensions/cli/src/index.ts`, `docs/cli/headless-mode.mdx` | `cn -p <prompt> --format json` | Upstream CLI source describes `-p/--print` as non-interactive and exposes JSON headless output. |

## Isolation

For subprocess-based synthesis WhyThis creates a temporary working directory and serializes all deterministic evidence into the prompt. This reduces the need for the harness to touch the analyzed repository. The prompt explicitly forbids tool use and file edits.

## Capability detection

`whythis harnesses` performs executable/extension detection. `whythis capabilities <id>` returns machine-readable details including integration mode, structured-output support and the upstream evidence sources used by the adapter.

Availability means the integration surface was detected locally; it does not mean provider credentials are configured.

## External adapters

The built-ins are not a closed list. `pkg/harness` exposes the public `Adapter`, `Capabilities`, `AnalysisRequest` and `AnalysisResult` types so external Go modules can integrate additional harnesses without importing internal packages. See [`adapter-development.md`](adapter-development.md).
