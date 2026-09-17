package index

import (
	"bufio"
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/storage"
)

type Result struct {
	FromCommit  string        `json:"from_commit,omitempty"`
	ToCommit    string        `json:"to_commit"`
	Commits     int           `json:"commits_indexed"`
	FullRebuild bool          `json:"full_rebuild"`
	Duration    time.Duration `json:"duration"`
}

type Indexer struct {
	Git   *gitx.Runner
	Store *storage.Store
}

func (i *Indexer) Run(ctx context.Context) (Result, error) {
	started := time.Now()
	head, err := i.Git.Head(ctx)
	if err != nil {
		return Result{}, err
	}
	last, err := i.Store.GetMeta(ctx, "index.head")
	if err != nil {
		return Result{}, err
	}
	full := false
	if last != "" {
		ancestor, err := i.Git.IsAncestor(ctx, last, head)
		if err != nil || !ancestor {
			if err := i.Store.ResetIndex(ctx); err != nil {
				return Result{}, err
			}
			last = ""
			full = true
		}
	} else {
		full = true
	}
	commits, err := i.Git.CommitsSince(ctx, last)
	if err != nil {
		return Result{}, err
	}
	for _, sha := range commits {
		if err := i.indexCommit(ctx, sha); err != nil {
			return Result{}, fmt.Errorf("index %s: %w", sha, err)
		}
	}
	if err := i.Store.SetMeta(ctx, "index.head", head); err != nil {
		return Result{}, err
	}
	return Result{FromCommit: last, ToCommit: head, Commits: len(commits), FullRebuild: full, Duration: time.Since(started)}, nil
}

func (i *Indexer) indexCommit(ctx context.Context, sha string) error {
	meta, err := i.Git.Output(ctx, "show", "-s", "--date=iso-strict", "--format=%H%x1f%P%x1f%aN%x1f%aE%x1f%aI%x1f%s%x1f%b", sha)
	if err != nil {
		return err
	}
	f := strings.Split(strings.TrimSpace(meta), "\x1f")
	if len(f) < 7 {
		return fmt.Errorf("unexpected commit metadata")
	}
	dt, _ := time.Parse(time.RFC3339, f[4])
	row := storage.CommitRow{SHA: f[0], Parents: f[1], Author: f[2], Email: f[3], Date: dt, Subject: f[5], Body: f[6]}
	numstat, err := i.Git.Output(ctx, "show", "--numstat", "--format=", "--find-renames", sha)
	if err != nil {
		return err
	}
	status, _ := i.Git.Output(ctx, "show", "--name-status", "--format=", "--find-renames", sha)
	statusByPath := map[string][2]string{}
	for _, line := range strings.Split(strings.TrimSpace(status), "\n") {
		cols := strings.Split(line, "\t")
		if len(cols) < 2 {
			continue
		}
		old, path := "", cols[len(cols)-1]
		if len(cols) == 3 {
			old = cols[1]
		}
		statusByPath[path] = [2]string{cols[0], old}
	}
	var changes []storage.FileChangeRow
	s := bufio.NewScanner(strings.NewReader(numstat))
	for s.Scan() {
		cols := strings.Split(s.Text(), "\t")
		if len(cols) < 3 {
			continue
		}
		add, _ := strconv.Atoi(strings.ReplaceAll(cols[0], "-", "0"))
		del, _ := strconv.Atoi(strings.ReplaceAll(cols[1], "-", "0"))
		path := cols[len(cols)-1]
		st := statusByPath[path]
		changes = append(changes, storage.FileChangeRow{CommitSHA: sha, Status: st[0], OldPath: st[1], Path: path, Additions: add, Deletions: del})
	}
	return i.Store.UpsertCommit(ctx, row, changes)
}
