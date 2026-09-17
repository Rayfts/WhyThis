package app

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/githubx"
	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/graph"
	"github.com/Rayfts/WhyThis/internal/history"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

const maxPRHistoryFiles = 20

var prIssueReference = regexp.MustCompile(`(?i)(?:#|GH-)([0-9]+)`) // repository-local references

func prReportEnhanced(ctx context.Context, g *gitx.Runner, token, api string, number int, cache githubx.Cache) (evidence.Report, error) {
	remote := g.RemoteURL(ctx)
	repo, err := githubx.ParseRemote(remote)
	if err != nil {
		return evidence.Report{}, fmt.Errorf("GitHub remote: %w", err)
	}
	client := githubx.New(token, api, cache)
	pr, err := client.PR(ctx, repo, number)
	if err != nil {
		return evidence.Report{}, err
	}
	files, err := client.PRFiles(ctx, repo, number)
	if err != nil {
		return evidence.Report{}, fmt.Errorf("PR files: %w", err)
	}
	commits, err := client.PRCommits(ctx, repo, number)
	if err != nil {
		return evidence.Report{}, fmt.Errorf("PR commits: %w", err)
	}

	head, _ := g.Head(ctx)
	now := time.Now().UTC()
	prID := fmt.Sprintf("pr:%d", number)
	builder := graph.New()
	builder.AddNode(evidence.Node{
		ID: prID, Kind: evidence.KindPullRequest, Label: pr.PullRequest.Title,
		Attributes: map[string]any{
			"number": number, "state": pr.PullRequest.State, "url": pr.PullRequest.HTMLURL,
			"author": pr.PullRequest.User.Login, "merge_commit_sha": pr.PullRequest.MergeCommitSHA,
			"body": pr.PullRequest.Body, "changed_files": len(files), "commit_count": len(commits),
		},
		Provenance: githubProvenance(repo, pr.PullRequest.HTMLURL, head, now),
	})
	facts := []evidence.Claim{{Class: evidence.Fact, Text: fmt.Sprintf("GitHub PR #%d is %s: %s", number, pr.PullRequest.State, pr.PullRequest.Title), EvidenceID: []string{prID}}}
	if pr.PullRequest.MergeCommitSHA != "" {
		facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: "GitHub reports merge commit " + pr.PullRequest.MergeCommitSHA + " for this PR.", EvidenceID: []string{prID}})
	}
	facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("GitHub reports %d changed files and %d PR commits.", len(files), len(commits)), EvidenceID: []string{prID}})

	addDiscussionNodes(builder, prID, repo, pr, head, now)
	addReferencedIssues(ctx, builder, &facts, client, repo, prID, pr.PullRequest.Title+"\n"+pr.PullRequest.Body, head, now)

	prCommits := make(map[string]struct{}, len(commits)+1)
	for _, c := range commits {
		prCommits[c.SHA] = struct{}{}
	}
	if pr.PullRequest.MergeCommitSHA != "" {
		prCommits[pr.PullRequest.MergeCommitSHA] = struct{}{}
	}

	selected, skipped := selectPRHistoryFiles(files, maxPRHistoryFiles)
	archaeologist := &history.Archaeologist{Git: g}
	var timeline []evidence.Node
	for _, f := range selected {
		fileID := "file:" + filepath.ToSlash(f.Filename)
		fileNode := evidence.Node{ID: fileID, Kind: evidence.KindFile, Label: f.Filename, Attributes: map[string]any{"status": f.Status, "additions": f.Additions, "deletions": f.Deletions, "changes": f.Changes, "previous_filename": f.PreviousFilename}, Provenance: githubProvenance(repo, f.BlobURL, head, now)}
		builder.AddNode(fileNode)
		builder.AddEdge(evidence.Edge{From: prID, To: fileID, Kind: evidence.EdgeAssociatedWith, Attributes: map[string]any{"reason": "changed by pull request", "status": f.Status}, Provenance: githubProvenance(repo, pr.PullRequest.HTMLURL, head, now)})
		if f.PreviousFilename != "" {
			previousID := "file:" + filepath.ToSlash(f.PreviousFilename)
			builder.AddNode(evidence.Node{ID: previousID, Kind: evidence.KindFile, Label: f.PreviousFilename, Attributes: map[string]any{"historical_path": true}, Provenance: githubProvenance(repo, f.BlobURL, head, now)})
			builder.AddEdge(evidence.Edge{From: fileID, To: previousID, Kind: evidence.EdgeRenamedFrom, Provenance: githubProvenance(repo, pr.PullRequest.HTMLURL, head, now)})
		}

		path := f.Filename
		rep, e := archaeologist.File(ctx, path)
		if e != nil && f.PreviousFilename != "" {
			path = f.PreviousFilename
			rep, e = archaeologist.File(ctx, path)
		}
		if e != nil {
			continue
		}
		mergeHistoricalReport(builder, &facts, &timeline, rep, prCommits)
		// Historical reports may contain a less-specific node for this same path.
		// Re-apply the GitHub PR node so its changed-file metadata wins while all
		// other historical/supporting file nodes remain materialized.
		builder.AddNode(fileNode)
	}

	sort.Slice(timeline, func(i, j int) bool {
		return fmt.Sprint(timeline[i].Attributes["date"]) < fmt.Sprint(timeline[j].Attributes["date"])
	})
	unknowns := []evidence.Claim{{Class: evidence.Unknown, Text: "PR discussion can document rationale, but comments and review text are not promoted to FACT unless repository evidence corroborates them."}}
	if skipped > 0 {
		unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: fmt.Sprintf("%d changed files were omitted from per-file archaeology because the report is bounded or the paths are generated/vendor noise.", skipped)})
	}
	if len(timeline) == 0 {
		unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: "No pre-PR local Git history was recovered for the selected changed files; the local clone may be shallow or may not contain the relevant ancestry."})
	}
	return evidence.Report{Target: fmt.Sprintf("PR #%d", number), Generated: now, Repository: g.Dir, Revision: head, Graph: builder.Build(), Facts: dedupeAppClaims(facts), Unknowns: unknowns, Timeline: timeline}, nil
}

func addDiscussionNodes(builder *graph.Builder, prID string, repo githubx.Repo, pr githubx.PRContext, head string, now time.Time) {
	for _, c := range pr.IssueComments {
		id := fmt.Sprintf("issue-comment:%d", c.ID)
		builder.AddNode(evidence.Node{ID: id, Kind: evidence.KindIssueComment, Label: discussionLabel(c.Body), Attributes: map[string]any{"author": c.User.Login, "body": c.Body, "created_at": c.CreatedAt, "url": c.HTMLURL}, Provenance: githubProvenance(repo, c.HTMLURL, head, now)})
		builder.AddEdge(evidence.Edge{From: prID, To: id, Kind: evidence.EdgeDiscussedIn, Provenance: githubProvenance(repo, c.HTMLURL, head, now)})
	}
	for _, c := range pr.ReviewComments {
		id := fmt.Sprintf("review-comment:%d", c.ID)
		builder.AddNode(evidence.Node{ID: id, Kind: evidence.KindReviewComment, Label: discussionLabel(c.Body), Attributes: map[string]any{"author": c.User.Login, "body": c.Body, "created_at": c.CreatedAt, "url": c.HTMLURL}, Provenance: githubProvenance(repo, c.HTMLURL, head, now)})
		builder.AddEdge(evidence.Edge{From: prID, To: id, Kind: evidence.EdgeDiscussedIn, Provenance: githubProvenance(repo, c.HTMLURL, head, now)})
	}
	for _, r := range pr.Reviews {
		id := fmt.Sprintf("review:%d", r.ID)
		builder.AddNode(evidence.Node{ID: id, Kind: evidence.KindReviewComment, Label: strings.TrimSpace(r.State + " review by " + r.User.Login), Attributes: map[string]any{"author": r.User.Login, "body": r.Body, "state": r.State, "submitted_at": r.SubmittedAt, "url": r.HTMLURL}, Provenance: githubProvenance(repo, r.HTMLURL, head, now)})
		builder.AddEdge(evidence.Edge{From: prID, To: id, Kind: evidence.EdgeDiscussedIn, Provenance: githubProvenance(repo, r.HTMLURL, head, now)})
	}
}

func addReferencedIssues(ctx context.Context, builder *graph.Builder, facts *[]evidence.Claim, client *githubx.Client, repo githubx.Repo, prID, text, head string, now time.Time) {
	seen := map[string]struct{}{}
	for _, m := range prIssueReference.FindAllStringSubmatch(text, -1) {
		if len(m) < 2 || m[1] == strings.TrimPrefix(prID, "pr:") {
			continue
		}
		if _, ok := seen[m[1]]; ok || len(seen) >= 10 {
			continue
		}
		seen[m[1]] = struct{}{}
		var n int
		_, _ = fmt.Sscanf(m[1], "%d", &n)
		issue, comments, err := client.Issue(ctx, repo, n)
		if err != nil {
			continue
		}
		id := fmt.Sprintf("issue:%d", n)
		builder.AddNode(evidence.Node{ID: id, Kind: evidence.KindIssue, Label: issue.Title, Attributes: map[string]any{"number": n, "state": issue.State, "author": issue.User.Login, "body": issue.Body, "url": issue.HTMLURL}, Provenance: githubProvenance(repo, issue.HTMLURL, head, now)})
		builder.AddEdge(evidence.Edge{From: prID, To: id, Kind: evidence.EdgeReferences, Provenance: githubProvenance(repo, issue.HTMLURL, head, now)})
		*facts = append(*facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("PR text references GitHub issue/PR #%d: %s", n, issue.Title), EvidenceID: []string{prID, id}})
		for _, c := range comments {
			cid := fmt.Sprintf("issue-comment:%d", c.ID)
			builder.AddNode(evidence.Node{ID: cid, Kind: evidence.KindIssueComment, Label: discussionLabel(c.Body), Attributes: map[string]any{"author": c.User.Login, "body": c.Body, "created_at": c.CreatedAt, "url": c.HTMLURL}, Provenance: githubProvenance(repo, c.HTMLURL, head, now)})
			builder.AddEdge(evidence.Edge{From: id, To: cid, Kind: evidence.EdgeDiscussedIn, Provenance: githubProvenance(repo, c.HTMLURL, head, now)})
		}
	}
}

func mergeHistoricalReport(builder *graph.Builder, facts *[]evidence.Claim, timeline *[]evidence.Node, rep evidence.Report, excluded map[string]struct{}) {
	allowed := map[string]struct{}{}
	for _, n := range rep.Graph.Nodes {
		if n.Kind == evidence.KindCommit {
			sha, _ := n.Attributes["sha"].(string)
			if _, skip := excluded[sha]; skip {
				continue
			}
		}
		allowed[n.ID] = struct{}{}
		builder.AddNode(n)
		if n.Kind == evidence.KindCommit {
			*timeline = append(*timeline, n)
		}
	}
	for _, e := range rep.Graph.Edges {
		_, fromOK := allowed[e.From]
		_, toOK := allowed[e.To]
		if fromOK && toOK {
			builder.AddEdge(e)
		}
	}
	for _, c := range rep.Facts {
		include := len(c.EvidenceID) == 0
		for _, id := range c.EvidenceID {
			if _, ok := allowed[id]; ok {
				include = true
			}
		}
		if include {
			*facts = append(*facts, c)
		}
	}
}

func selectPRHistoryFiles(files []githubx.PRFile, limit int) ([]githubx.PRFile, int) {
	candidates := make([]githubx.PRFile, 0, len(files))
	skipped := 0
	for _, f := range files {
		if skipPRHistoryPath(f.Filename) {
			skipped++
			continue
		}
		candidates = append(candidates, f)
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].Changes > candidates[j].Changes })
	if len(candidates) > limit {
		skipped += len(candidates) - limit
		candidates = candidates[:limit]
	}
	return candidates, skipped
}

func skipPRHistoryPath(path string) bool {
	p := strings.ToLower(filepath.ToSlash(path))
	if strings.HasPrefix(p, "vendor/") || strings.Contains(p, "/vendor/") || strings.HasPrefix(p, "node_modules/") || strings.Contains(p, "/node_modules/") || strings.HasPrefix(p, "dist/") || strings.Contains(p, "/dist/") {
		return true
	}
	base := filepath.Base(p)
	return strings.HasSuffix(p, ".pb.go") || strings.HasSuffix(p, ".gen.go") || base == "go.sum" || base == "package-lock.json" || base == "pnpm-lock.yaml" || base == "yarn.lock"
}

func discussionLabel(body string) string {
	body = strings.Join(strings.Fields(body), " ")
	if len(body) > 96 {
		return body[:93] + "..."
	}
	if body == "" {
		return "discussion entry"
	}
	return body
}

func githubProvenance(repo githubx.Repo, locator, revision string, now time.Time) evidence.Provenance {
	return evidence.Provenance{Source: "github-rest", Locator: locator, Repository: repo.Owner + "/" + repo.Name, Revision: revision, CollectedAt: now}
}

func dedupeAppClaims(in []evidence.Claim) []evidence.Claim {
	seen := make(map[string]struct{}, len(in))
	out := make([]evidence.Claim, 0, len(in))
	for _, c := range in {
		key := string(c.Class) + "\x00" + c.Text
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, c)
	}
	return out
}
