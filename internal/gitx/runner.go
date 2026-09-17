package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var ErrNotRepository = errors.New("not a git repository")

type Runner struct {
	Dir string
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

func New(dir string) (*Runner, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	r := &Runner{Dir: abs}
	root, err := r.Output(context.Background(), "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrNotRepository, err)
	}
	r.Dir = strings.TrimSpace(root)
	return r, nil
}

func (r *Runner) Run(ctx context.Context, args ...string) (Result, error) {
	if len(args) == 0 {
		return Result{}, errors.New("git command requires arguments")
	}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = r.Dir
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	started := time.Now()
	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(started)}
	if cmd.ProcessState != nil {
		res.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		return res, fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(res.Stderr))
	}
	return res, nil
}

func (r *Runner) Output(ctx context.Context, args ...string) (string, error) {
	res, err := r.Run(ctx, args...)
	return res.Stdout, err
}

func (r *Runner) Head(ctx context.Context) (string, error) {
	out, err := r.Output(context.Background(), "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

func (r *Runner) IsShallow(ctx context.Context) (bool, error) {
	out, err := r.Output(ctx, "rev-parse", "--is-shallow-repository")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) == "true", nil
}

func (r *Runner) IsAncestor(ctx context.Context, ancestor, descendant string) (bool, error) {
	res, err := r.Run(ctx, "merge-base", "--is-ancestor", ancestor, descendant)
	if err == nil {
		return true, nil
	}
	if res.ExitCode == 1 {
		return false, nil
	}
	return false, err
}

func (r *Runner) Blame(ctx context.Context, path string, start, end int) (string, error) {
	if start <= 0 {
		start = 1
	}
	if end < start {
		end = start
	}
	return r.Output(ctx, "blame", "--line-porcelain", "-L", strconv.Itoa(start)+","+strconv.Itoa(end), "--", filepath.ToSlash(path))
}

func (r *Runner) FileLog(ctx context.Context, path string) (string, error) {
	return r.Output(ctx, "log", "--follow", "--date=iso-strict", "--format=%H%x1f%aN%x1f%aE%x1f%aI%x1f%s%x1f%b%x1e", "--name-status", "--find-renames", "--", filepath.ToSlash(path))
}

func (r *Runner) Commit(ctx context.Context, sha string) (string, error) {
	return r.Output(ctx, "show", "--no-ext-diff", "--date=iso-strict", "--format=%H%x1f%P%x1f%aN%x1f%aE%x1f%aI%x1f%s%x1f%b%x1e", "--name-status", "--find-renames", sha)
}

func (r *Runner) Patch(ctx context.Context, sha string) (string, error) {
	return r.Output(ctx, "show", "--no-ext-diff", "--format=", "--find-renames", sha)
}

// PatchID returns Git's stable patch identity for a commit. Patch identity is
// content-based and intentionally ignores commit metadata. An empty patch
// (for example, some merges) has no patch ID.
func (r *Runner) PatchID(ctx context.Context, sha string) (string, error) {
	patch, err := r.Patch(ctx, sha)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(patch) == "" {
		return "", nil
	}
	cmd := exec.CommandContext(ctx, "git", "patch-id", "--stable")
	cmd.Dir = r.Dir
	cmd.Env = append(cmd.Environ(), "LC_ALL=C", "GIT_OPTIONAL_LOCKS=0")
	cmd.Stdin = strings.NewReader(patch)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git patch-id --stable: %w: %s", err, strings.TrimSpace(stderr.String()))
	}
	fields := strings.Fields(stdout.String())
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func (r *Runner) RevListAll(ctx context.Context, max int) ([]string, error) {
	args := []string{"rev-list", "--all", "--topo-order"}
	if max > 0 {
		args = append(args, "--max-count="+strconv.Itoa(max))
	}
	out, err := r.Output(ctx, args...)
	if err != nil {
		return nil, err
	}
	var shas []string
	for _, line := range strings.Split(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			shas = append(shas, line)
		}
	}
	return shas, nil
}

// RangeDiff exposes native Git range-diff for callers that need to compare
// two commit series without reimplementing Git's patch-series matching.
func (r *Runner) RangeDiff(ctx context.Context, oldRange, newRange string) (string, error) {
	if strings.TrimSpace(oldRange) == "" || strings.TrimSpace(newRange) == "" {
		return "", errors.New("range-diff requires two non-empty ranges")
	}
	return r.Output(ctx, "range-diff", "--no-color", oldRange, newRange)
}

func (r *Runner) SymbolLog(ctx context.Context, symbol string) (string, error) {
	return r.Output(ctx, "log", "--all", "--date=iso-strict", "--format=%H%x1f%aN%x1f%aE%x1f%aI%x1f%s%x1f%b%x1e", "-S", symbol, "--pickaxe-regex", "--find-renames")
}

func (r *Runner) GrepHistory(ctx context.Context, regex string) (string, error) {
	return r.Output(ctx, "log", "--all", "--date=iso-strict", "--format=%H%x1f%aN%x1f%aE%x1f%aI%x1f%s%x1f%b%x1e", "-G", regex, "--find-renames")
}

func (r *Runner) CommitsSince(ctx context.Context, since string) ([]string, error) {
	args := []string{"rev-list", "--reverse", "HEAD"}
	if since != "" {
		args = []string{"rev-list", "--reverse", since + "..HEAD"}
	}
	out, err := r.Output(ctx, args...)
	if err != nil {
		return nil, err
	}
	var commits []string
	for _, line := rane strings.Split(strings.TrimSpace(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			commits = append(commits, s)
		}
	}
	return commits, nil
}

func (r *Runner) RemoteURL(ctx context.Context) string {
	out, err := r.Output(ctx, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}
