package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/githubx"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

const maxTimelineRemoteCommits = 8

func (s *Service) enrichRemoteHistory(ctx context.Context, report evidence.Report) evidence.Report {
	// Automatic remote enrichment is opt-in via GITHUB_TOKEN so ordinary local
	// archaeology never depends on network access. Explicit `pr` queries may
	// still use GitHub anonymously for public repositories.
	if strings.TrimSpace(s.GitHubToken) == "" {
		return report
	}
	remote := s.Git.RemoteURL(ctx)
	repo, err := githubx.ParseRemote(remote)
	if err != nil || remote == "" {
		return report
	}
	client := githubx.New(s.GitHubToken, s.GitHubAPI, s.GitHubCache)
	nodes := make(map[string]evidence.Node, len(report.Graph.Nodes))
	edges := make(map[string]evidence.Edge, len(report.Graph.Edges))
	for _, n := range report.Graph.Nodes {
		nodes[n.ID] = n
	}
	for _, e := range report.Graph.Edges {
		edges[e.From+"|"+string(e.Kind)+"|"+e.To] = e
	}
	addNode := func(n evidence.Node) { nodes[n.ID] = n }
	addEdge := func(e evidence.Edge) { edges[e.From+"|"+string(e.Kind)+"|"+e.To] = e }

	commitIDs := timelineCommitSHAs(report)
	if len(commitIDs) > maxTimelineRemoteCommits {
		commitIDs = commitIDs[len(commitIDs)-maxTimelineRemoteCommits:]
	}
	now := time.Now().UTC()
	for _, sha := range commitIDs {
		checks, checkErr := client.CommitChecks(ctx, repo, sha)
		if checkErr == nil {
			for _, c := range checks {
				if !failedConclusion(c.Conclusion) {
					continue
				}
				id := fmt.Sprintf("ci-check:%d", c.ID)
				addNode(evidence.Node{ID: id, Kind: evidence.KindCIFailure, Label: c.Name, Attributes: map[string]any{"status": c.Status, "conclusion": c.Conclusion, "url": c.HTMLURL, "details_url": c.DetailsURL, "app": c.App.Name}, Provenance: evidence.Provenance{Source: "github-check-runs", Locator: c.HTMLURL, Repository: repo.Owner + "/" + repo.Name, Revision: sha, CollectedAt: now}})
				addEdge(evidence.Edge{From: "commit:" + sha, To: id, Kind: evidence.EdgeAssociatedWith, Attributes: map[string]any{"reason": "historical CI check failure"}, Provenance: evidence.Provenance{Source: "github-check-runs", Locator: c.HTMLURL, Repository: repo.Owner + "/" + repo.Name, Revision: sha, CollectedAt: now}})
				report.Facts = append(report.Facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("GitHub check %q concluded %s on commit %s.", c.Name, c.Conclusion, shortSHA(sha)), EvidenceID: []string{"commit:" + sha, id}})
			}
		} else {
			report.Unknowns = append(report.Unknowns, evidence.Claim{Class: evidence.Unknown, Text: fmt.Sprintf("GitHub check history for commit %s could not be retrieved: %v", shortSHA(sha), checkErr)})
		}
		statuses, statusErr := client.CommitStatuses(ctx, repo, sha)
		if statusErr == nil {
			for _, st := range statuses {
				if st.State == "success" || st.State == "pending" || st.State == "" {
					continue
				}
				id := fmt.Sprintf("ci-status:%d", st.ID)
				addNode(evidence.Node{ID: id, Kind: evidence.KindCIFailure, Label: st.Context, Attributes: map[string]any{"state": st.State, "description": st.Description, "url": st.TargetURL, "creator": st.Creator.Login}, Provenance: evidence.Provenance{Source: "github-statuses", Locator: st.TargetURL, Repository: repo.Owner + "/" + repo.Name, Revision: sha, CollectedAt: now}})
				addEdge(evidence.Edge{From: "commit:" + sha, To: id, Kind: evidence.EdgeAssociatedWith, Attributes: map[string]any{"reason": "historical commit status failure"}, Provenance: evidence.Provenance{Source: "github-statuses", Locator: st.TargetURL, Repository: repo.Owner + "/" + repo.Name, Revision: sha, CollectedAt: now}})
				report.Facts = append(report.Facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("GitHub status %q was %s on commit %s.", st.Context, st.State, shortSHA(sha)), EvidenceID: []string{"commit:" + sha, id}})
			}
		}
	}

	releases, releaseErr := client.Releases(ctx, repo)
	if releaseErr == nil {
		wanted := map[string]bool{}
		for _, sha := range commitIDs {
			wanted[sha] = true
		}
		for _, rel := range releases {
			if rel.TagName == "" {
				continue
			}
			sha, resolveErr := s.Git.ResolveRevision(ctx, rel.TagName)
			if resolveErr != nil || !wanted[sha] {
				continue
			}
			id := fmt.Sprintf("release:%d", rel.ID)
			label := rel.Name
			if label == "" {
				label = rel.TagName
			}
			attrs := map[string]any{"tag": rel.TagName, "target_commitish": rel.TargetCommitish, "url": rel.HTMLURL, "draft": rel.Draft, "prerelease": rel.Prerelease}
			if rel.PublishedAt != nil {
				attrs["published_at"] = rel.PublishedAt.Format(time.RFC3339)
			}
			addNode(evidence.Node{ID: id, Kind: evidence.KindRelease, Label: label, Attributes: attrs, Provenance: evidence.Provenance{Source: "github-releases", Locator: rel.HTMLURL, Repository: repo.Owner + "/" + repo.Name, Revision: sha, CollectedAt: now}})
			addEdge(evidence.Edge{From: "commit:" + sha, To: id, Kind: evidence.EdgeAssociatedWith, Attributes: map[string]any{"reason": "release tag resolves to commit"}, Provenance: evidence.Provenance{Source: "github-releases", Locator: rel.HTMLURL, Repository: repo.Owner + "/" + repo.Name, Revision: sha, CollectedAt: now}})
			report.Facts = append(report.Facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Release %s points to commit %s.", rel.TagName, shortSHA(sha)), EvidenceID: []string{"commit:" + sha, id}})
		}
	}

	report.Graph.Nodes = report.Graph.Nodes[:0]
	for _, n := range nodes {
		report.Graph.Nodes = append(report.Graph.Nodes, n)
	}
	sort.Slice(report.Graph.Nodes, func(i, j int) bool { return report.Graph.Nodes[i].ID < report.Graph.Nodes[j].ID })
	report.Graph.Edges = report.Graph.Edges[:0]
	for _, e := range edges {
		report.Graph.Edges = append(report.Graph.Edges, e)
	}
	sort.Slice(report.Graph.Edges, func(i, j int) bool {
		a, b := report.Graph.Edges[i], report.Graph.Edges[j]
		if a.From != b.From {
			return a.From < b.From
		}
		if a.Kind != b.Kind {
			return a.Kind < b.Kind
		}
		return a.To < b.To
	})
	report.Facts = dedupeAppClaims(report.Facts)
	report.Unknowns = dedupeAppClaims(report.Unknowns)
	return report
}

func timelineCommitSHAs(report evidence.Report) []string {
	seen := map[string]bool{}
	var out []string
	for _, n := range report.Timeline {
		if n.Kind != evidence.KindCommit {
			continue
		}
		sha := strings.TrimPrefix(n.ID, "commit:")
		if sha == "" || seen[sha] {
			continue
		}
		seen[sha] = true
		out = append(out, sha)
	}
	if len(out) == 0 {
		for _, n := range report.Graph.Nodes {
			if n.Kind != evidence.KindCommit {
				continue
			}
			sha := strings.TrimPrefix(n.ID, "commit:")
			if sha != "" && !seen[sha] {
				seen[sha] = true
				out = append(out, sha)
			}
		}
	}
	return out
}

func failedConclusion(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "failure", "cancelled", "timed_out", "action_required", "startup_failure", "stale":
		return true
	default:
		return false
	}
}

func shortSHA(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}
