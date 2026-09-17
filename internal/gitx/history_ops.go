package gitx

import (
	"context"
	"path/filepath"
	"strconv"
	"strings"
)

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
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if s := strings.TrimSpace(line); s != "" {
			commits = append(commits, s)
		}
	}
	return commits, nil
}

func (r *Runner) FileMode(ctx context.Context, path string) (string, error) {
	out, err := r.Output(ctx, "ls-files", "--stage", "--", filepath.ToSlash(path))
	if err != nil {
		return "", err
	}
	line := strings.TrimSpace(out)
	if line == "" {
		return "", nil
	}
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func (r *Runner) RemoteURL(ctx context.Context) string {
	out, err := r.Output(ctx, "remote", "get-url", "origin")
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

// RecentCommitPaths returns the changed paths for recent commits across refs in
// one Git traversal. It is used as a cheap prefilter before expensive patch
// materialization in similarity search.
func (r *Runner) RecentCommitPaths(ctx context.Context, max int) (map[string]map[string]struct{}, []string, error) {
	args := []string{"log", "--all", "--topo-order", "--format=WHYT:%H", "--name-only"}
	if max > 0 {
		args = append(args, "--max-count="+strconv.Itoa(max))
	}
	out, err := r.Output(ctx, args...)
	if err != nil {
		return nil, nil, err
	}
	paths := map[string]map[string]struct{}{}
	var order []string
	current := ""
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "WHYT:") {
			current = strings.TrimPrefix(line, "WHYT:")
			if _, ok := paths[current]; !ok {
				paths[current] = map[string]struct{}{}
				order = append(order, current)
			}
			continue
		}
		if current != "" {
			paths[current][filepath.ToSlash(line)] = struct{}{}
		}
	}
	return paths, order, nil
}
