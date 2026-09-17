package app

import (
	"context"
	"fmt"

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
}

func NewService(g *gitx.Runner) *Service {
	return &Service{Git: g, History: &history.Archaeologist{Git: g}, HarnessRegistry: harness.NewRegistry()}
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
func (s *Service) Risk(ctx context.Context, path string) (risk.Report, error) {
	if path == "" {
		return risk.Report{}, fmt.Errorf("path required")
	}
	return risk.Analyze(ctx, s.Git, path)
}
func (s *Service) Harnesses(ctx context.Context) []harness.Capabilities {
	return s.HarnessRegistry.List(ctx)
}
