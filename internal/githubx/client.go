package githubx

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Cache interface {
	Cached(context.Context, string) ([]byte, string, bool, error)
	PutCache(context.Context, string, string, []byte, time.Duration) error
}

type Client struct {
	Token   string
	BaseURL string
	HTTP    *http.Client
	Cache   Cache
	TTL     time.Duration
}

type Repo struct{ Owner, Name string }

type User struct {
	Login string `json:"login"`
}
type Issue struct {
	Number    int        `json:"number"`
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	State     string     `json:"state"`
	HTMLURL   string     `json:"html_url"`
	User      User       `json:"user"`
	CreatedAt time.Time  `json:"created_at"`
	ClosedAt  *time.Time `json:"closed_at"`
}
type PullRequest struct {
	Number         int        `json:"number"`
	Title          string     `json:"title"`
	Body           string     `json:"body"`
	State          string     `json:"state"`
	HTMLURL        string     `json:"html_url"`
	User           User       `json:"user"`
	MergeCommitSHA string     `json:"merge_commit_sha"`
	CreatedAt      time.Time  `json:"created_at"`
	MergedAt       *time.Time `json:"merged_at"`
}
type Comment struct {
	ID        int64     `json:"id"`
	Body      string    `json:"body"`
	HTMLURL   string    `json:"html_url"`
	User      User      `json:"user"`
	CreatedAt time.Time `json:"created_at"`
}
type Review struct {
	ID          int64     `json:"id"`
	Body        string    `json:"body"`
	State       string    `json:"state"`
	HTMLURL     string    `json:"html_url"`
	User        User      `json:"user"`
	SubmittedAt time.Time `json:"submitted_at"`
}
type PRContext struct {
	PullRequest    PullRequest `json:"pull_request"`
	IssueComments  []Comment   `json:"issue_comments"`
	ReviewComments []Comment   `json:"review_comments"`
	Reviews        []Review    `json:"reviews"`
}

func New(token, base string, cache Cache) *Client {
	if base == "" {
		base = "https://api.github.com"
	}
	return &Client{Token: token, BaseURL: strings.TrimRight(base, "/"), HTTP: &http.Client{Timeout: 20 * time.Second}, Cache: cache, TTL: 15 * time.Minute}
}

var scpRemote = regexp.MustCompile(`^[^@]+@github\.com:([^/]+)/(.+?)(?:\.git)?$`)

func ParseRemote(raw string) (Repo, error) {
	raw = strings.TrimSpace(raw)
	if m := scpRemote.FindStringSubmatch(raw); len(m) == 3 {
		return Repo{Owner: m[1], Name: strings.TrimSuffix(m[2], ".git")}, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return Repo{}, err
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) < 2 {
		return Repo{}, fmt.Errorf("unsupported GitHub remote %q", raw)
	}
	return Repo{Owner: parts[0], Name: strings.TrimSuffix(parts[1], ".git")}, nil
}

func (c *Client) PR(ctx context.Context, repo Repo, number int) (PRContext, error) {
	var out PRContext
	if err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/pulls/%d", repo.Owner, repo.Name, number), &out.PullRequest); err != nil {
		return out, err
	}
	_ = c.get(ctx, fmt.Sprintf("/repos/%s/%s/issues/%d/comments?per_page=100", repo.Owner, repo.Name, number), &out.IssueComments)
	_ = c.get(ctx, fmt.Sprintf("/repos/%s/%s/pulls/%d/comments?per_page=100", repo.Owner, repo.Name, number), &out.ReviewComments)
	_ = c.get(ctx, fmt.Sprintf("/repos/%s/%s/pulls/%d/reviews?per_page=100", repo.Owner, repo.Name, number), &out.Reviews)
	return out, nil
}
func (c *Client) Issue(ctx context.Context, repo Repo, number int) (Issue, []Comment, error) {
	var issue Issue
	var comments []Comment
	if err := c.get(ctx, fmt.Sprintf("/repos/%s/%s/issues/%d", repo.Owner, repo.Name, number), &issue); err != nil {
		return issue, nil, err
	}
	_ = c.get(ctx, fmt.Sprintf("/repos/%s/%s/issues/%d/comments?per_page=100", repo.Owner, repo.Name, number), &comments)
	return issue, comments, nil
}

func (c *Client) get(ctx context.Context, path string, dst any) error {
	key := "github:" + path
	var cached []byte
	var etag string
	if c.Cache != nil {
		body, e, ok, err := c.Cache.Cached(ctx, key)
		if err == nil {
			cached, etag = body, e
			if ok {
				return json.Unmarshal(body, dst)
			}
		}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "WhyThis/0.1")
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotModified && len(cached) > 0 {
		return json.Unmarshal(cached, dst)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0" {
			reset, _ := strconv.ParseInt(resp.Header.Get("X-RateLimit-Reset"), 10, 64)
			return fmt.Errorf("GitHub API rate limit exhausted until %s", time.Unix(reset, 0).Format(time.RFC3339))
		}
		return fmt.Errorf("GitHub API %s: status %d: %s", path, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if c.Cache != nil {
		_ = c.Cache.PutCache(ctx, key, resp.Header.Get("ETag"), body, c.TTL)
	}
	if err := json.Unmarshal(body, dst); err != nil {
		return errors.Join(err, fmt.Errorf("decode %s", path))
	}
	return nil
}
