package harness

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func builtins() []Harness {
	return []Harness{
		commandHarness{id: "codex", binary: "codex", integration: "process: codex exec --json --ephemeral", structured: true, sources: []string{"openai/codex codex-rs/exec/src/cli.rs"}, stdin: true, args: func(_ string, _ string) []string {
			return []string{"exec", "--json", "--ephemeral", "--skip-git-repo-check", "-"}
		}},
		commandHarness{id: "claude-code", binary: "claude", integration: "process: claude print mode + stream-json", structured: true, sources: []string{"anthropics/claude-code feed.xml; plugins/plugin-dev hook docs"}, args: func(_ string, p string) []string { return []string{"-p", p, "--output-format", "stream-json"} }},
		commandHarness{id: "opencode", binary: "opencode", integration: "process: opencode run --format json", structured: true, sources: []string{"anomalyco/opencode packages/web/src/content/docs/cli.mdx"}, args: func(_ string, p string) []string { return []string{"run", "--format", "json", p} }},
		piHarness{},
		commandHarness{id: "gemini", binary: "gemini", integration: "process: gemini -p + stream-json", structured: true, sources: []string{"google-gemini/gemini-cli docs/reference/configuration.md"}, args: func(_ string, p string) []string { return []string{"-p", p, "--output-format", "stream-json"} }},
		commandHarness{id: "aider", binary: "aider", integration: "process: --message-file (one-shot)", structured: false, sources: []string{"Aider-AI/aider aider/args.py"}, args: func(f string, _ string) []string { return []string{"--message-file", f, "--yes-always"} }},
		commandHarness{id: "goose", binary: "goose", integration: "process: goose run --text + stream-json", structured: true, sources: []string{"aaif-goose/goose crates/goose-cli/src/cli.rs"}, args: func(_ string, p string) []string {
			return []string{"run", "--text", p, "--output-format", "stream-json", "--no-session"}
		}},
		commandHarness{id: "cline", binary: "cline", integration: "process: cline --json one-shot", structured: true, sources: []string{"cline/cline apps/cli/README.md"}, args: func(_ string, p string) []string { return []string{"--json", p} }},
		rooHarness{},
		commandHarness{id: "continue", binary: "cn", integration: "process: cn -p --format json", structured: true, sources: []string{"continuedev/continue extensions/cli/src/index.ts; docs/cli/headless-mode.mdx"}, args: func(_ string, p string) []string { return []string{"-p", p, "--format", "json"} }},
	}
}

type piHarness struct{}

func (piHarness) ID() string { return "pi" }
func (piHarness) Detect(ctx context.Context) (Capabilities, error) {
	p, err := exec.LookPath("pi")
	return Capabilities{ID: "pi", Available: err == nil, Executable: p, Integration: "JSONL RPC: pi --mode rpc --no-session", StructuredOutput: true, SessionProtocol: true, EvidenceSources: []string{"earendil-works/pi packages/coding-agent/docs/rpc.md"}}, nil
}
func (piHarness) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResult, error) {
	p, err := exec.LookPath("pi")
	if err != nil {
		return AnalysisResult{}, err
	}
	dir, err := os.MkdirTemp("", "whythis-pi-*")
	if err != nil {
		return AnalysisResult{}, err
	}
	defer os.RemoveAll(dir)
	cmd := exec.CommandContext(ctx, p, "--mode", "rpc", "--no-session")
	cmd.Dir = dir
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return AnalysisResult{}, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return AnalysisResult{}, err
	}
	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return AnalysisResult{}, err
	}
	payload, _ := json.Marshal(map[string]any{"id": "whythis-1", "type": "prompt", "message": req.Prompt})
	if _, err := fmt.Fprintf(stdin, "%s\n", payload); err != nil {
		return AnalysisResult{}, err
	}
	dec := json.NewDecoder(stdout)
	var raw strings.Builder
	var text []string
	for {
		var ev map[string]any
		if err := dec.Decode(&ev); err != nil {
			break
		}
		b, _ := json.Marshal(ev)
		raw.Write(b)
		raw.WriteByte('\n')
		collectStrings(ev, &text, 0)
		if ev["type"] == "agent_end" {
			break
		}
	}
	_ = stdin.Close()
	err = cmd.Wait()
	if err != nil {
		return AnalysisResult{Harness: "pi", Raw: raw.String() + stderr.String(), Format: "jsonl-rpc"}, err
	}
	return AnalysisResult{Harness: "pi", Text: strings.Join(text, "\n"), Raw: raw.String(), Format: "jsonl-rpc"}, nil
}

type rooHarness struct{}

func (rooHarness) ID() string { return "roo-code" }
func (rooHarness) Detect(ctx context.Context) (Capabilities, error) {
	c := Capabilities{ID: "roo-code", Integration: "legacy VS Code extension detection + evidence handoff", StructuredOutput: false, SessionProtocol: false, EvidenceSources: []string{"RooCodeInc/Roo-Code README.md (extension shutdown notice)"}, Notes: []string{"Upstream states the Roo Code extension was shut down on May 15, 2026; no maintained headless CLI is claimed."}}
	code, err := exec.LookPath("code")
	if err != nil {
		return c, nil
	}
	out, e := exec.CommandContext(ctx, code, "--list-extensions").Output()
	if e == nil && strings.Contains(strings.ToLower(string(out)), "rooveterinaryinc.roo-cline") {
		c.Available = true
		c.Executable = code
	}
	return c, nil
}
func (rooHarness) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResult, error) {
	dir := filepath.Join(".whyth", "handoff")
	_ = os.MkdirAll(dir, 0o755)
	path := filepath.Join(dir, "roo-code.md")
	if err := os.WriteFile(path, []byte(req.Prompt), 0o600); err != nil {
		return AnalysisResult{}, err
	}
	return AnalysisResult{Harness: "roo-code", Text: "Evidence prompt written to " + path, Format: "handoff"}, ErrInteractiveOnly
}
