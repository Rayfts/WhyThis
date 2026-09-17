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
	out, err := r.Output(ctx, "rev-parse", "HEAD")
	return strings.TrimSpace(out), err
}

func (r *Runner) ResolveRevision(ctx context.Context, rev string) (string, error) {
	out, err := r.Output(ctx, "rev-list", "-n", "1", rev)
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

// LineLogSHAs asks Git to trace the evolution of a concrete line range. Git's
// -L machinery follows the selected hunk through edits within the current
// path. It does not follow file renames, so callers should combine it with
// blame/file-history evidence when rename lineage matters.
func (r *Runner) LineLogSHAs(ctx context.Context, path string, start, end int) ([]string, error) {
	if start <= 0 {
		start = 1
	}
	if end < start {
		end = start
	}
	spec := strconv.Itoa(start) + "," + strconv.Itoa(end) + ":" + filepath.ToSlash(path)
	out, err := r.Output(ctx, "log", "--no-color", "--format=WHYT:%H", "-L", spec)
	if err != nil {
		return nil, err
	}
	var shas []string
	seen := map[string]struct{}{}
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "WHYT:") {
			continue
		}
		sha := strings.TrimSpace(strings.TrimPrefix(line, "WHYT:"))
		if sha == "" {
			continue
		}
		if _, ok := seen[sha]; ok {
			continue
		}
		seen[sha] = struct{}{}
		shas = append(shas, sha)
	}
	return shas, nil
}
