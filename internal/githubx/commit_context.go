package githubx

import (
	"context"
	"fmt"
	"net/url"
	"time"
)

type CheckRun struct {
	ID          int64      `json:"id"`
	Name        string     `json:"name"`
	Status      string     `json:"status"`
	Conclusion  string     `json:"conclusion"`
	HTMLURL     string     `json:"html_url"`
	DetailsURL  string     `json:"details_url"`
	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	App         struct {
		Name string `json:"name"`
	} `json:"app"`
}

type CommitStatus struct {
	ID          int64     `json:"id"`
	State       string    `json:"state"`
	Context     string    `json:"context"`
	Description string    `json:"description"`
	TargetURL   string    `json:"target_url"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Creator     User      `json:"creator"`
}

type Release struct {
	ID              int64      `json:"id"`
	TagName         string     `json:"tag_name"`
	TargetCommitish string     `json:"target_commitish"`
	Name            string     `json:"name"`
	Body            string     `json:"body"`
	HTMLURL         string     `json:"html_url"`
	Draft           bool       `json:"draft"`
	Prerelease      bool       `json:"prerelease"`
	PublishedAt     *time.Time `json:"published_at"`
}

func (c *Client) CommitChecks(ctx context.Context, repo Repo, sha string) ([]CheckRun, error) {
	owner, name, rev := url.PathEscape(repo.Owner), url.PathEscape(repo.Name), url.PathEscape(sha)
	var out []CheckRun
	for page := 1; ; page++ {
		var payload struct {
			CheckRuns []CheckRun `json:"check_runs"`
		}
		path := fmt.Sprintf("/repos/%s/%s/commits/%s/check-runs?per_page=%d&page=%d", owner, name, rev, githubPageSize, page)
		if err := c.get(ctx, path, &payload); err != nil {
			return nil, err
		}
		out = append(out, payload.CheckRuns...)
		if len(payload.CheckRuns) < githubPageSize {
			return out, nil
		}
	}
}

func (c *Client) CommitStatuses(ctx context.Context, repo Repo, sha string) ([]CommitStatus, error) {
	owner, name, rev := url.PathEscape(repo.Owner), url.PathEscape(repo.Name), url.PathEscape(sha)
	return getAllPages[CommitStatus](ctx, c, func(page int) string {
		return fmt.Sprintf("/repos/%s/%s/commits/%s/statuses?per_page=%d&page=%d", owner, name, rev, githubPageSize, page)
	})
}

func (c *Client) Releases(ctx context.Context, repo Repo) ([]Release, error) {
	owner, name := url.PathEscape(repo.Owner), url.PathEscape(repo.Name)
	return getAllPages[Release](ctx, c, func(page int) string {
		return fmt.Sprintf("/repos/%s/%s/releases?per_page=%d&page=%d", owner, name, githubPageSize, page)
	})
}
