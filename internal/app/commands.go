package app

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/Rayfts/WhyThis/internal/config"
	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/history"
	"github.com/Rayfts/WhyThis/internal/storage"
)

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
