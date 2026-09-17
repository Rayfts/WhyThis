package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	RepoPath    string
	DBPath      string
	GitHubToken string
	GitHubAPI   string
	Harness     string
	NoAI        bool
}

func Load(repoPath string) Config {
	if repoPath == "" {
		repoPath = "."
	}
	abs, err := filepath.Abs(repoPath)
	if err == nil {
		repoPath = abs
	}
	return Config{
		RepoPath:    repoPath,
		DBPath:      filepath.Join(repoPath, ".whyth", "whythis.db"),
		GitHubToken: firstNonEmpty(os.Getenv("WHYTHIS_GITHUB_TOKEN"), os.Getenv("GITHUB_TOKEN"), os.Getenv("GH_TOKEN")),
		GitHubAPI:   firstNonEmpty(os.Getenv("WHYTHIS_GITHUB_API"), "https://api.github.com"),
		Harness:     os.Getenv("WHYTHIS_HARNESS"),
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
