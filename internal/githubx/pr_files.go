package githubx

import (
	"context"
	"fmt"
	"net/url"
)

const githubPageSize = 100

type PRFile struct {
	SHA              string `json:"sha"`
	Filename         string `json:"filename"`
	Status           string `json:"status"`
	Additions        int    `json:"additions"`
	Deletions        int    `json:"deletions"`
	Changes          int    `json:"changes"`
	BlobURL          string `json:"blob_url"`
	RawURL           string `json:"raw_url"`
	ContentsURL      string `json:"contents_url"`
	PreviousFilename string `json:"previous_filename,omitempty"`
}

type PRCommit struct {
	SHA     string `json:"sha"`
	HTMLURL string `json:"html_url"`
	Commit  struct {
		Message string `json:"message"`
	} `json:"commit"`
}

// PRFiles returns all changed files visible through GitHub's paginated REST
// endpoint. Each page is independently cached by Client.get.
func (c *Client) PRFiles(ctx context.Context, repo Repo, number int) ([]PRFile, error) {
	return getAllPages[PRFile](ctx, c, func(page int) string {
		return fmt.Sprintf("/repos/%s/%s/pulls/%d/files?per_page=%d&page=%d", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number, githubPageSize, page)
	})
}

// PRCommits returns all commits currently associated with a pull request.
func (c *Client) PRCommits(ctx context.Context, repo Repo, number int) ([]PRCommit, error) {
	return getAllPages[PRCommit](ctx, c, func(page int) string {
		return fmt.Sprintf("/repos/%s/%s/pulls/%d/commits?per_page=%d&page=%d", url.PathEscape(repo.Owner), url.PathEscape(repo.Name), number, githubPageSize, page)
	})
}

func getAllPages[T any](ctx context.Context, c *Client, path func(page int) string) ([]T, error) {
	var all []T
	for page := 1; ; page++ {
		var batch []T
		if err := c.get(ctx, path(page), &batch); err != nil {
			return nil, err
		}
		all = append(all, batch...)
		if len(batch) < githubPageSize {
			return all, nil
		}
	}
}
