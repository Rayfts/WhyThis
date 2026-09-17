package app

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/Rayfts/WhyThis/internal/analysis"
	"github.com/Rayfts/WhyThis/internal/config"
	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/index"
	"github.com/Rayfts/WhyThis/internal/server"
	"github.com/Rayfts/WhyThis/internal/storage"
	"github.com/Rayfts/WhyThis/internal/tui"
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
	svc.ConfigureGitHub(cfg.GitHubToken, cfg.GitHubAPI, nil)
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
		report, err = svc.Symbol(ctx, tail[0])
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
		report, err = svc.Commit(ctx, tail[0])
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
		report, err = svc.Ask(ctx, q)
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
		svc.ConfigureGitHub(cfg.GitHubToken, cfg.GitHubAPI, s)
		report, err = svc.PR(ctx, n)
	case "doctor":
		return cmdDoctor(ctx, g, cfg, svc, openStore, stdout)
	case "tui":
		if s, e := openStore(); e == nil {
			svc.ConfigureGitHub(cfg.GitHubToken, cfg.GitHubAPI, s)
		}
		if e := tui.Run(ctx, svc, g.Dir, os.Stdin, stdout); e != nil {
			_, _ = fmt.Fprintln(stderr, "whythis:", e)
			return 1
		}
		return 0
	case "serve":
		addr := "127.0.0.1:7788"
		if len(tail) > 0 {
			addr = tail[0]
		}
		log := slog.New(slog.NewTextHandler(stderr, nil))
		if s, e := openStore(); e == nil {
			svc.ConfigureGitHub(cfg.GitHubToken, cfg.GitHubAPI, s)
		}
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
