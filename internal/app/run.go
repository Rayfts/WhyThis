package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/analysis"
	"github.com/Rayfts/WhyThis/internal/config"
	"github.com/Rayfts/WhyThis/internal/githubx"
	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/history"
	"github.com/Rayfts/WhyThis/internal/index"
	"github.com/Rayfts/WhyThis/internal/server"
	"github.com/Rayfts/WhyThis/internal/storage"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

const version = "0.1.0-dev"

type commonFlags struct {
	repo    string
	json    bool
	noAI    bool
	harness string
	db      string
}

func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	cf, rest, err := parseCommon(args)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "whythis:", err)
		return 2
	}
	if len(rest) == 0 {
		usage(stdout)
		return 0
	}
	if rest[0] == "version" || rest[0] == "--version" {
		_, _ = fmt.Fprintln(stdout, version)
		return 0
	}
	cfg := config.Load(cf.repo)
	if cf.db != "" {
		cfg.DBPath = cf.db
	}
	cfg.NoAI = cf.noAI
	if cf.harness != "" {
		cfg.Harness = cf.harness
	}
	if rest[0] == "harnesses" {
		return cmdHarnesses(ctx, harness.NewRegistry(), cf.json, stdout, stderr)
	}
	if rest[0] == "capabilities" {
		if len(rest) < 2 {
			_, _ = fmt.Fprintln(stderr, "capabilities requires a harness id")
			return 2
		}
		return cmdCapabilities(ctx, harness.NewRegistry(), rest[1], stdout, stderr)
	}
	g, err := gitx.New(cfg.RepoPath)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "whythis:", err)
		return 1
	}
	svc := NewService(g)
	var st *storage.Store
	openStore := func() (*storage.Store, error) {
		if st != nil {
			return st, nil
		}
		s, e := storage.Open(cfg.DBPath)
		if e == nil {
			st = s
		}
		return s, e
	}
	defer func() {
		if st != nil {
			_ = st.Close()
		}
	}()

	cmd := rest[0]
	tail := rest[1:]
	var report evidence.Report
	switch cmd {
	case "help":
		usage(stdout)
		return 0
	case "file":
		if len(tail) < 1 {
			return argErr(stderr, "file requires path:line[-line]")
		}
		report, err = svc.Line(ctx, tail[0])
	case "symbol":
		if len(tail) < 1 {
			return argErr(stderr, "symbol requires a symbol name")
		}
		report, err = svc.History.Symbol(ctx, tail[0])
	case "blame":
		if len(tail) < 1 {
			return argErr(stderr, "blame requires a path[:line-range]")
		}
		return cmdBlame(ctx, g, tail[0], cf.json, stdout, stderr)
	case "history":
		if len(tail) < 1 {
			return argErr(stderr, "history requires a path")
		}
		report, err = svc.FileHistory(ctx, tail[0])
	case "risk":
		if len(tail) < 1 {
			return argErr(stderr, "risk requires a path")
		}
		r, e := svc.Risk(ctx, tail[0])
		if e != nil {
			err = e
		} else {
			return writeAny(r, cf.json, stdout)
		}
	case "commit":
		if len(tail) < 1 {
			return argErr(stderr, "commit requires a sha")
		}
		report, err = svc.History.Commit(ctx, tail[0])
	case "similar":
		if len(tail) < 1 {
			return argErr(stderr, "similar requires a commit sha")
		}
		limit := 10
		if len(tail) > 1 {
			n, e := strconv.Atoi(tail[1])
			if e != nil || n <= 0 {
				return argErr(stderr, "similar limit must be a positive integer")
			}
			limit = n
		}
		report, err = svc.Similar(ctx, tail[0], limit)
	case "ask":
		if len(tail) < 1 {
			return argErr(stderr, "ask requires a question")
		}
		q := strings.Join(tail, " ")
		report, err = svc.History.SearchQuestion(ctx, q)
	case "index":
		s, e := openStore()
		if e != nil {
			err = e
			break
		}
		r, e := (&index.Indexer{Git: g, Store: s}).Run(ctx)
		if e != nil {
			err = e
		} else {
			return writeAny(r, true, stdout)
		}
	case "pr":
		if len(tail) < 1 {
			return argErr(stderr, "pr requires a number")
		}
		n, e := strconv.Atoi(tail[0])
		if e != nil {
			return argErr(stderr, "invalid PR number")
		}
		s, _ := openStore()
		report, err = prReportEnhanced(ctx, g, cfg, n, s)
	case "doctor":
		return cmdDoctor(ctx, g, cfg, svc, openStore, stdout)
	case "serve":
		addr := "127.0.0.1:7788"
		if len(tail) > 0 {
			addr = tail[0]
		}
		log := slog.New(slog.NewTextHandler(stderr, nil))
		err = (&server.Server{Addr: addr, Service: svc, Logger: log}).ListenAndServe(ctx)
		if err == nil {
			return 0
		}
	default:
		// Shorthand: whythis path/to/file.go:123
		if strings.Contains(cmd, ":") {
			report, err = svc.Line(ctx, cmd)
		} else {
			usage(stderr)
			return 2
		}
	}
	if err != nil {
		_, _ = fmt.Fprintln(stderr, "whythis:", err)
		return 1
	}
	if s, e := openStore(); e == nil {
		_ = s.PutGraph(ctx, report.Graph)
	}
	if !cf.noAI && cfg.Harness != "" {
		report, err = (&analysis.Orchestrator{Registry: svc.HarnessRegistry}).Synthesize(ctx, cfg.Harness, report)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "whythis: harness synthesis skipped: %v\n", err)
		}
	}
	return writeReport(report, cf.json, stdout)
}

func parseCommon(args []string) (commonFlags, []string, error) {
	var c commonFlags
	c.repo = "."
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch a {
		case "--json":
			c.json = true
		case "--no-ai":
			c.noAI = true
		case "--repo":
			i++
			if i >= len(args) {
				return c, nil, errors.New("--repo requires a path")
			}
			c.repo = args[i]
		case "--db":
			i++
			if i >= len(args) {
				return c, nil, errors.New("--db requires a path")
			}
			c.db = args[i]
		case "--harness":
			i++
			if i >= len(args) {
				return c, nil, errors.New("--harness requires an id")
			}
			c.harness = args[i]
		case "--help", "-h":
			return c, []string{"help"}, nil
		default:
			rest = append(rest, a)
		}
	}
	return c, rest, nil
}
func argErr(w io.Writer, msg string) int { _, _ = fmt.Fprintln(w, "whythis:", msg); return 2 }
func writeAny(v any, asJSON bool, w io.Writer) int {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v)
		return 0
	}
	b, _ := json.MarshalIndent(v, "", "  ")
	_, _ = fmt.Fprintln(w, string(b))
	return 0
}
func writeReport(r evidence.Report, asJSON bool, w io.Writer) int {
	if asJSON {
		return writeAny(r, true, w)
	}
	_, _ = fmt.Fprintf(w, "WhyThis — %s\nrevision: %s\n\n", r.Target, short(r.Revision))
	printClaims(w, "FACTS", r.Facts)
	printClaims(w, "INFERENCES", r.Inferences)
	printClaims(w, "UNKNOWN", r.Unknowns)
	if len(r.Timeline) > 0 {
		_, _ = fmt.Fprintln(w, "TIMELINE")
		for _, n := range r.Timeline {
			date := ""
			if n.Attributes != nil {
				date = fmt.Sprint(n.Attributes["date"])
			}
			_, _ = fmt.Fprintf(w, "  %s  %-12s  %s\n", prefixDate(date), strings.TrimPrefix(n.ID, "commit:")[:min(12, len(strings.TrimPrefix(n.ID, "commit:")))], n.Label)
		}
		_, _ = fmt.Fprintln(w)
	}
	_, _ = fmt.Fprintf(w, "evidence: %d nodes, %d edges\n", len(r.Graph.Nodes), len(r.Graph.Edges))
	return 0
}
func printClaims(w io.Writer, title string, claims []evidence.Claim) {
	_, _ = fmt.Fprintln(w, title)
	if len(claims) == 0 {
		_, _ = fmt.Fprintln(w, "  (none)")
		_, _ = fmt.Fprintln(w)
		return
	}
	for _, c := range claims {
		_, _ = fmt.Fprintln(w, " -", c.Text)
		if len(c.EvidenceID) > 0 {
			_, _ = fmt.Fprintln(w, "   evidence:", strings.Join(c.EvidenceID, ", "))
		}
	}
	_, _ = fmt.Fprintln(w)
}
func prefixDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return "          "
}
func short(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func cmdBlame(ctx context.Context, g *gitx.Runner, target string, asJSON bool, stdout, stderr io.Writer) int {
	t, err := history.ParseTarget(target)
	if err != nil {
		return argErr(stderr, err.Error())
	}
	raw, err := g.Blame(ctx, t.Path, t.Start, t.End)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	segments := gitx.ParseBlamePorcelain(raw)
	return writeAny(segments, asJSON, stdout)
}
func cmdHarnesses(ctx context.Context, r *harness.Registry, asJSON bool, stdout, stderr io.Writer) int {
	caps := r.List(ctx)
	if asJSON {
		return writeAny(caps, true, stdout)
	}
	for _, c := range caps {
		state := "missing"
		if c.Available {
			state = "available"
		}
		_, _ = fmt.Fprintf(stdout, "%-13s %-10s %s\n", c.ID, state, c.Integration)
	}
	return 0
}
func cmdCapabilities(ctx context.Context, r *harness.Registry, id string, stdout, stderr io.Writer) int {
	h, ok := r.Get(id)
	if !ok {
		return argErr(stderr, "unknown harness "+id)
	}
	c, err := h.Detect(ctx)
	if err != nil {
		_, _ = fmt.Fprintln(stderr, err)
		return 1
	}
	return writeAny(c, true, stdout)
}

func cmdDoctor(ctx context.Context, g *gitx.Runner, cfg config.Config, svc *Service, openStore func() (*storage.Store, error), w io.Writer) int {
	type check struct {
		Name   string `json:"name"`
		OK     bool   `json:"ok"`
		Detail string `json:"detail"`
	}
	checks := []check{{"git", true, gitVersion()}, {"repository", true, g.Dir}}
	shallow, err := g.IsShallow(ctx)
	checks = append(checks, check{"history-depth", err == nil && !shallow, mapBool(shallow, "shallow clone: some archaeology may be incomplete", "full history available")})
	if _, err := openStore(); err != nil {
		checks = append(checks, check{"sqlite", false, err.Error()})
	} else {
		checks = append(checks, check{"sqlite", true, cfg.DBPath})
	}
	checks = append(checks, check{"github-token", cfg.GitHubToken != "", mapBool(cfg.GitHubToken != "", "configured", "not configured; local Git still works")})
	avail := 0
	for _, c := range svc.Harnesses(ctx) {
		if c.Available {
			avail++
		}
	}
	checks = append(checks, check{"harnesses", avail > 0, fmt.Sprintf("%d/10 detected", avail)})
	return writeAny(checks, true, w)
}
func gitVersion() string {
	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(string(out))
}
func mapBool(v bool, a, b string) string {
	if v {
		return a
	}
	return b
}

func prReport(ctx context.Context, g *gitx.Runner, cfg config.Config, n int, cache githubx.Cache) (evidence.Report, error) {
	remote := g.RemoteURL(ctx)
	repo, err := githubx.ParseRemote(remote)
	if err != nil {
		return evidence.Report{}, fmt.Errorf("GitHub remote: %w", err)
	}
	client := githubx.New(cfg.GitHubToken, cfg.GitHubAPI, cache)
	pr, err := client.PR(ctx, repo, n)
	if err != nil {
		return evidence.Report{}, err
	}
	head, _ := g.Head(ctx)
	now := time.Now().UTC()
	id := fmt.Sprintf("pr:%d", n)
	node := evidence.Node{ID: id, Kind: evidence.KindPullRequest, Label: pr.PullRequest.Title, Attributes: map[string]any{"number": n, "state": pr.PullRequest.State, "url": pr.PullRequest.HTMLURL, "author": pr.PullRequest.User.Login, "merge_commit_sha": pr.PullRequest.MergeCommitSHA, "body": pr.PullRequest.Body, "reviews": pr.Reviews, "issue_comments": pr.IssueComments, "review_comments": pr.ReviewComments}, Provenance: evidence.Provenance{Source: "github-rest", Locator: pr.PullRequest.HTMLURL, Repository: repo.Owner + "/" + repo.Name, Revision: head, CollectedAt: now}}
	facts := []evidence.Claim{{Class: evidence.Fact, Text: fmt.Sprintf("GitHub PR #%d is %s: %s", n, pr.PullRequest.State, pr.PullRequest.Title), EvidenceID: []string{id}}}
	if pr.PullRequest.MergeCommitSHA != "" {
		facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: "GitHub reports merge commit " + pr.PullRequest.MergeCommitSHA + " for this PR.", EvidenceID: []string{id}})
	}
	return evidence.Report{Target: fmt.Sprintf("PR #%d", n), Generated: now, Repository: g.Dir, Revision: head, Graph: evidence.Graph{Nodes: []evidence.Node{node}}, Facts: facts, Unknowns: []evidence.Claim{{Class: evidence.Unknown, Text: "A PR discussion can document rationale, but comments are not assumed to be correct unless corroborated by repository evidence."}}}, nil
}

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `WhyThis reconstructs why code exists from deterministic Git/GitHub evidence.

Usage:
  whythis [global flags] path/to/file.go:100-140
  whythis file path/to/file.go:100-140
  whythis symbol Foo.Bar
  whythis blame path/to/file.go[:line-range]
  whythis history path/to/file.go
  whythis risk path/to/file.go
  whythis commit <sha>
  whythis similar <sha> [limit]
  whythis pr <number>
  whythis ask "Why is this retry loop here?"
  whythis index
  whythis serve [127.0.0.1:7788]
  whythis doctor
  whythis harnesses
  whythis capabilities <id>

Global flags:
  --repo <path>       repository root (default .)
  --db <path>         SQLite index path (default .whyth/whythis.db)
  --json              machine-readable output
  --no-ai             deterministic evidence only
  --harness <id>      optional semantic synthesis adapter

Harness IDs: codex, claude-code, opencode, pi, gemini, aider, goose, cline, roo-code, continue
`)
}
