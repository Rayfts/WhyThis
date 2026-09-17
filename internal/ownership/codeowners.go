package ownership

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/hmarr/codeowners"
)

const maxCODEOWNERSSize = 3 << 20

var standardLocations = []string{
	".github/CODEOWNERS",
	"CODEOWNERS",
	"docs/CODEOWNERS",
}

type Result struct {
	File       string   `json:"file,omitempty"`
	Pattern    string   `json:"pattern,omitempty"`
	Line       int      `json:"line,omitempty"`
	Owners     []string `json:"owners,omitempty"`
	Configured bool     `json:"configured"`
	Matched    bool     `json:"matched"`
}

// Resolve evaluates GitHub CODEOWNERS for path using GitHub's documented
// standard-location precedence. Matching semantics are delegated to the
// hmarr/codeowners parser, whose Ruleset.Match implements last-match-wins.
func Resolve(repoRoot, path string) (Result, error) {
	source, err := locate(repoRoot)
	if err != nil {
		return Result{}, err
	}
	if source == "" {
		return Result{}, nil
	}
	info, err := os.Stat(source)
	if err != nil {
		return Result{}, err
	}
	result := Result{File: relativeSlash(repoRoot, source), Configured: true}
	if info.Size() >= maxCODEOWNERSSize {
		return result, fmt.Errorf("CODEOWNERS %s is %d bytes; GitHub ignores files at or above 3 MiB", result.File, info.Size())
	}
	f, err := os.Open(source)
	if err != nil {
		return result, err
	}
	defer func() { _ = f.Close() }()
	rules, err := codeowners.ParseFile(f)
	if err != nil {
		return result, fmt.Errorf("parse %s: %w", result.File, err)
	}
	rel, err := filepath.Rel(repoRoot, filepath.Join(repoRoot, filepath.FromSlash(path)))
	if err != nil {
		return result, err
	}
	rel = filepath.ToSlash(filepath.Clean(rel))
	if rel == "." || strings.HasPrefix(rel, "../") {
		return result, fmt.Errorf("path %q is outside repository root", path)
	}
	rule, err := rules.Match(rel)
	if err != nil {
		return result, fmt.Errorf("match %s against %s: %w", rel, result.File, err)
	}
	if rule == nil {
		return result, nil
	}
	result.Matched = true
	result.Pattern = rule.RawPattern()
	result.Line = rule.LineNumber
	result.Owners = make([]string, 0, len(rule.Owners))
	for _, owner := range rule.Owners {
		result.Owners = append(result.Owners, owner.String())
	}
	return result, nil
}

func locate(repoRoot string) (string, error) {
	root, err := filepath.Abs(repoRoot)
	if err != nil {
		return "", err
	}
	for _, rel := range standardLocations {
		path := filepath.Join(root, filepath.FromSlash(rel))
		info, statErr := os.Stat(path)
		if statErr == nil {
			if info.IsDir() {
				continue
			}
			return path, nil
		}
		if !errors.Is(statErr, os.ErrNotExist) {
			return "", statErr
		}
	}
	return "", nil
}

func relativeSlash(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(rel)
}
