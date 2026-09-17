package tui

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/risk"
)

func renderRisk(w io.Writer, r risk.Report) {
	_, _ = fmt.Fprintf(w, "\nHISTORICAL RISK\n  path        %s\n  score       %d/100\n  confidence  %s\n  commits     %d\n  fix-like    %d\n  reverts     %d\n  authors     %d\n  churn       %d lines\n", r.Path, r.Score, r.Confidence, r.Commits, r.FixLike, r.Reverts, r.Authors, r.Churn)
	if len(r.CodeOwners) > 0 {
		_, _ = fmt.Fprintf(w, "  codeowners  %s (%s:%d)\n", strings.Join(r.CodeOwners, ", "), r.CodeOwnerSource, r.CodeOwnerLine)
	}
	if r.Sensitive {
		categories := make([]string, 0, len(r.Sensitivity))
		seen := map[string]bool{}
		for _, finding := range r.Sensitivity {
			if !seen[finding.Category] {
				seen[finding.Category] = true
				categories = append(categories, finding.Category)
			}
		}
		sort.Strings(categories)
		_, _ = fmt.Fprintf(w, "  sensitive   %s\n", strings.Join(categories, ", "))
	}
	if r.Generated || r.Vendor || r.Submodule {
		classes := make([]string, 0, 3)
		if r.Generated {
			classes = append(classes, "generated")
		}
		if r.Vendor {
			classes = append(classes, "vendor")
		}
		if r.Submodule {
			classes = append(classes, "submodule")
		}
		_, _ = fmt.Fprintf(w, "  class       %s\n", strings.Join(classes, ", "))
	}
	_, _ = fmt.Fprintln(w, "\nSIGNALS")
	for _, s := range r.Indicators {
		_, _ = fmt.Fprintf(w, "  %-9s %-18s %s\n", s.Severity, s.ID, s.Explanation)
	}
}

func renderHarnesses(w io.Writer, caps []harness.Capabilities) {
	_, _ = fmt.Fprintln(w, "\nHARNESS CAPABILITIES")
	for _, c := range caps {
		state := "missing"
		if c.Available {
			state = "available"
		}
		_, _ = fmt.Fprintf(w, "  %-13s %-10s %s\n", c.ID, state, c.Integration)
	}
}
