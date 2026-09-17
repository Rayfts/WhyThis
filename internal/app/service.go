package app

import (
	"context"
	"fmt"

	"github.com/Rayfts/WhyThis/internal/githubx"
	"github.com/Rayfts/WhyThis/internal/gitx"
	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/history"
	"github.com/Rayfts/WhyThis/internal/risk"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type Service struct {
	Git             *gitx.Runner
	History         *history.Archaeologist
	HarnessRegistry *harness.Registry
	GitHubToken     string
	GitHubAPI       string
	GitHubCache     githubx.Cache
}

func NewService(g *gitx.Runner) *Service {
	return &Service{Git: g, History: &history.Archaeologist{Git: g}, HarnessRegistry: harness.NewRegistry(), GitHubAPI: "https://api.github.com"}
}
func (s *Service) ConfigureGitHub(token, api string, cache githubx.Cache) {
	s.GitHubToken = token
	if api != "" {
		s.GitHubAPI = api
	}
	s.GitHubCache = cache
}
func (s *Service) Line(ctx context.Context, target string) (evidence.Report, error) {
	t, err := history.ParseTarget(target)
	if err != nil {
		return evidence.Report{}, err
	}
	return s.History.Line(ctx, t)
}
func (s *Service) FileHistory(ctx context.Context, path string) (evidence.Report, error) {
	return s.History.File(ctx, path)
}
func (s *Service) Similar(ctx context.Context, sha string, limit int) (evidence.Report, error) {
	return s.History.Similar(ctx, sha, limit)
}
func (s *Service) Symbol(ctx context.Context, symbol string) (evidence.Report, error) {
	return s.History.Symbol(ctx, symbol)
}
func (s *Service) Commit(ctx context.Context, sha string) (evidence.Report, error) {
	return s.History.Commit(ctx, sha)
}
func (s *Service) Ask(ctx context.Context, q string) (evidence.Report, error) {
	return s.History.SearchQuestion(ctx, q)
}
func (s *Service) PR(ctx context.Context, number int) (evidence.Report, error) {
	if number <= 0 {
		return evidence.Report{}, fmt.Errorf("PR number must be positive")
	}
	return prReportEnhanced(ctx, s.Git, s.GitHubToken, s.GitHubAPI, number, s.GitHubCache)
}
func (s *Service) Risk(ctx context.Context, path string) (risk.Report, error) {
	if path == "" {
		return risk.Report{}, fmt.Errorf("path required")
	}
	return risk.Analyze(ctx, s.Git, path)
}
func (s *Service) Harnesses(ctx context.Context) []harness.Capabilities {
	return s.HarnessRegistry.List(ctx)
}
