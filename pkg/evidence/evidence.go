package evidence

import "time"

type Kind string

const (
	KindRepository    Kind = "repository"
	KindFile          Kind = "file"
	KindLineRange     Kind = "line_range"
	KindSymbol        Kind = "symbol"
	KindCommit        Kind = "commit"
	KindDiff          Kind = "diff"
	KindPatch         Kind = "patch"
	KindPullRequest   Kind = "pull_request"
	KindIssue         Kind = "issue"
	KindReviewComment Kind = "review_comment"
	KindIssueComment  Kind = "issue_comment"
	KindTest          Kind = "test"
	KindDependency    Kind = "dependency"
	KindRelease       Kind = "release"
	KindBlameSegment  Kind = "blame_segment"
	KindAuthor        Kind = "author"
	KindDocumentation Kind = "documentation"
	KindCIFailure     Kind = "ci_failure"
)

type EdgeKind string

const (
	EdgeIntroducedBy   EdgeKind = "introduced-by"
	EdgeChangedBy      EdgeKind = "changed-by"
	EdgeRemovedBy      EdgeKind = "removed-by"
	EdgeRestoredBy     EdgeKind = "restored-by"
	EdgeRevertedBy     EdgeKind = "reverted-by"
	EdgeMovedFrom      EdgeKind = "moved-from"
	EdgeRenamedFrom    EdgeKind = "renamed-from"
	EdgeFixes          EdgeKind = "fixes"
	EdgeReferences     EdgeKind = "references"
	EdgeDiscussedIn    EdgeKind = "discussed-in"
	EdgeTestedBy       EdgeKind = "tested-by"
	EdgeFollowedBy     EdgeKind = "followed-by"
	EdgeDependsOn      EdgeKind = "depends-on"
	EdgeSupersedes     EdgeKind = "supersedes"
	EdgeAssociatedWith EdgeKind = "associated-with"
)

type Provenance struct {
	Source      string    `json:"source"`
	Locator     string    `json:"locator"`
	Repository  string    `json:"repository,omitempty"`
	Revision    string    `json:"revision,omitempty"`
	CollectedAt time.Time `json:"collected_at"`
}

type Node struct {
	ID         string         `json:"id"`
	Kind       Kind           `json:"kind"`
	Label      string         `json:"label"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Provenance Provenance     `json:"provenance"`
}

type Edge struct {
	From       string         `json:"from"`
	To         string         `json:"to"`
	Kind       EdgeKind       `json:"kind"`
	Attributes map[string]any `json:"attributes,omitempty"`
	Provenance Provenance     `json:"provenance"`
}

type Graph struct {
	Nodes []Node `json:"nodes"`
	Edges []Edge `json:"edges"`
}

type ClaimClass string

const (
	Fact      ClaimClass = "fact"
	Inference ClaimClass = "inference"
	Unknown   ClaimClass = "unknown"
)

type Claim struct {
	Class      ClaimClass `json:"class"`
	Text       string     `json:"text"`
	EvidenceID []string   `json:"evidence_ids,omitempty"`
}

type Report struct {
	Target     string    `json:"target"`
	Generated  time.Time `json:"generated_at"`
	Repository string    `json:"repository"`
	Revision   string    `json:"revision"`
	Graph      Graph     `json:"graph"`
	Facts      []Claim   `json:"facts"`
	Inferences []Claim   `json:"inferences"`
	Unknowns   []Claim   `json:"unknowns"`
	Timeline   []Node    `json:"timeline,omitempty"`
}
