package gitx

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

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
	return r.Output(ctx, "log", "--all", "--date=iso-strict", "--format=%H%x1f%aN%x1f%aE%x1f%aI%x1f%s%x1f%b%x1e", "-S", symbol, "--find-renames")
}

func (r *Runner) SymbolLogSHAs(ctx context.Context, symbol string) ([]string, error) {
	out, err := r.Output(ctx, "log", "--all", "--format=%H", "-S", symbol, "--find-renames")
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
