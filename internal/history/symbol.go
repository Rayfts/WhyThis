package history

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Rayfts/WhyThis/internal/graph"
	symbolx "github.com/Rayfts/WhyThis/internal/symbol"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func (a *Archaeologist) Symbol(ctx context.Context, query string) (evidence.Report, error) {
	head, err := a.Git.Head(ctx)
	if err != nil {
		return evidence.Report{}, err
	}
	now := time.Now().UTC()
	b := graph.New()
	rootID := "symbol:" + query
	b.AddNode(evidence.Node{ID: rootID, Kind: evidence.KindSymbol, Label: query, Attributes: map[string]any{"query": query}, Provenance: prov("symbol-query", query, head, now)})

	locations, astErr := symbolx.ResolveGo(a.Git.Dir, query)
	seenCommits := map[string]bool{}
	seenFiles := map[string]bool{}
	var timeline []evidence.Node
	var facts []evidence.Claim
	var unknowns []evidence.Claim

	for _, loc := range locations {
		fileID := "file:" + filepath.ToSlash(loc.Path)
		if !seenFiles[fileID] {
			seenFiles[fileID] = true
			b.AddNode(evidence.Node{ID: fileID, Kind: evidence.KindFile, Label: filepath.ToSlash(loc.Path), Provenance: prov("go-ast", loc.Path, head, now)})
			b.AddEdge(edge(rootID, fileID, evidence.EdgeAssociatedWith, head, now, "language-aware declaration location"))
			ownerFacts, ownerUnknowns := a.addCodeOwnership(b, fileID, loc.Path, head, now)
			facts = append(facts, ownerFacts...)
			facts = append(facts, a.pathSensitivityFacts(fileID, loc.Path)...)
			unknowns = append(unknowns, ownerUnknowns...)
		}
		locID := fmt.Sprintf("symbol:go:%s:%d-%d:%s", filepath.ToSlash(loc.Path), loc.Start, loc.End, loc.Name)
		attrs := map[string]any{"language": "go", "declaration_kind": loc.Kind, "path": loc.Path, "start": loc.Start, "end": loc.End}
		if loc.Receiver != "" {
			attrs["receiver"] = loc.Receiver
		}
		b.AddNode(evidence.Node{ID: locID, Kind: evidence.KindSymbol, Label: loc.Name, Attributes: attrs, Provenance: prov("go-ast", fmt.Sprintf("%s:%d-%d", loc.Path, loc.Start, loc.End), head, now)})
		b.AddEdge(edge(rootID, locID, evidence.EdgeAssociatedWith, head, now, "Go AST declaration match"))
		b.AddEdge(edge(fileID, locID, evidence.EdgeAssociatedWith, head, now, "declaration belongs to file"))
		facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Go AST anchors %s as a %s declaration at %s:%d-%d.", loc.Name, loc.Kind, loc.Path, loc.Start, loc.End), EvidenceID: []string{locID, fileID}})

		lineSHAs, lineErr := a.Git.LineLogSHAs(ctx, loc.Path, loc.Start, loc.End)
		if lineErr == nil {
			for _, sha := range lineSHAs {
				if seenCommits[sha] {
					continue
				}
				seenCommits[sha] = true
				bundle, bundleErr := a.commitEvidence(ctx, sha, now)
				if bundleErr != nil {
					continue
				}
				addCommitBundle(b, bundle)
				timeline = append(timeline, bundle.Commit)
				facts = append(facts, bundle.Claims...)
				b.AddEdge(edge(locID, bundle.Commit.ID, evidence.EdgeChangedBy, sha, now, "git log -L declaration evolution"))
			}
		}
		fileSHAs, _ := a.fileHistorySHAs(ctx, loc.Path)
		for _, sha := range fileSHAs {
			if seenCommits[sha] {
				continue
			}
			seenCommits[sha] = true
			bundle, bundleErr := a.commitEvidence(ctx, sha, now)
			if bundleErr != nil {
				continue
			}
			addCommitBundle(b, bundle)
			timeline = append(timeline, bundle.Commit)
			facts = append(facts, bundle.Claims...)
			b.AddEdge(edge(fileID, bundle.Commit.ID, evidence.EdgeChangedBy, sha, now, "surrounding file history and rename lineage"))
		}
	}

	// Textual pickaxe remains a fallback and supplements AST anchoring for
	// deleted symbols, non-Go languages and historical names that no longer
	// exist at HEAD.
	pickaxe := query
	if i := strings.LastIndex(pickaxe, "::"); i >= 0 {
		pickaxe = pickaxe[i+2:]
	}
	if i := strings.LastIndex(pickaxe, "."); i >= 0 {
		pickaxe = pickaxe[i+1:]
	}
	pickaxeSHAs, pickaxeErr := a.Git.SymbolLogSHAs(ctx, pickaxe)
	if pickaxeErr == nil {
		for _, sha := range pickaxeSHAs {
			if sha == "" || seenCommits[sha] {
				continue
			}
			seenCommits[sha] = true
			bundle, bundleErr := a.commitEvidence(ctx, sha, now)
			if bundleErr != nil {
				continue
			}
			addCommitBundle(b, bundle)
			timeline = append(timeline, bundle.Commit)
			facts = append(facts, bundle.Claims...)
			b.AddEdge(edge(rootID, bundle.Commit.ID, evidence.EdgeChangedBy, sha, now, "Git pickaxe matched symbol text"))
		}
	}

	sort.Slice(timeline, func(i, j int) bool {
		return fmt.Sprint(timeline[i].Attributes["date"]) < fmt.Sprint(timeline[j].Attributes["date"])
	})
	if len(locations) == 0 {
		if astErr != nil {
			unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: "Language-aware Go symbol resolution was unavailable: " + astErr.Error()})
		} else {
			unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: "No Go declaration with this symbol identity exists at HEAD; textual history is used as fallback evidence."})
		}
	} else {
		facts = append(facts, evidence.Claim{Class: evidence.Fact, Text: fmt.Sprintf("Language-aware resolution found %d Go declaration(s) for %q at HEAD.", len(locations), query), EvidenceID: []string{rootID}})
	}
	unknowns = append(unknowns, a.historyDepthUnknown(ctx)...)
	unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: "A semantic rename that changes both identifier and surrounding declaration can only be established when Git lineage or recorded discussion connects the old and new identities."})
	if len(timeline) == 0 {
		unknowns = append(unknowns, evidence.Claim{Class: evidence.Unknown, Text: "No attributable Git history was found for this symbol query."})
	}
	return evidence.Report{Target: query, Generated: now, Repository: a.Git.Dir, Revision: head, Graph: b.Build(), Facts: dedupeClaims(facts), Unknowns: dedupeClaims(unknowns), Timeline: timeline}, nil
}
