package server

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Rayfts/WhyThis/internal/harness"
	"github.com/Rayfts/WhyThis/internal/risk"
	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type fakeService struct {
	called string
	arg    string
	n      int
}

func (f *fakeService) report(name, arg string) (evidence.Report, error) {
	f.called, f.arg = name, arg
	if arg == "boom" {
		return evidence.Report{}, errors.New("boom")
	}
	return evidence.Report{Target: arg}, nil
}
func (f *fakeService) Line(_ context.Context, v string) (evidence.Report, error) {
	return f.report("line", v)
}
func (f *fakeService) FileHistory(_ context.Context, v string) (evidence.Report, error) {
	return f.report("history", v)
}
func (f *fakeService) Symbol(_ context.Context, v string) (evidence.Report, error) {
	return f.report("symbol", v)
}
func (f *fakeService) Commit(_ context.Context, v string) (evidence.Report, error) {
	return f.report("commit", v)
}
func (f *fakeService) Similar(_ context.Context, v string, n int) (evidence.Report, error) {
	f.n = n
	return f.report("similar", v)
}
func (f *fakeService) Ask(_ context.Context, v string) (evidence.Report, error) {
	return f.report("ask", v)
}
func (f *fakeService) PR(_ context.Context, n int) (evidence.Report, error) {
	f.called, f.n = "pr", n
	return evidence.Report{Target: "pr"}, nil
}
func (f *fakeService) Risk(_ context.Context, v string) (risk.Report, error) {
	f.called, f.arg = "risk", v
	return risk.Report{Path: v}, nil
}
func (f *fakeService) Harnesses(context.Context) []harness.Capabilities { return nil }
func (f *fakeService) Capability(_ context.Context, id string) (harness.Capabilities, error) {
	return harness.Capabilities{ID: id, Available: true}, nil
}

func TestAPIRoutes(t *testing.T) {
	tests := []struct {
		url, called, arg string
		n                int
	}{
		{"/v1/archaeology?target=a.go:3", "line", "a.go:3", 0},
		{"/v1/history?path=a.go", "history", "a.go", 0},
		{"/v1/symbol?name=Retry", "symbol", "Retry", 0},
		{"/v1/commit?sha=abc1234", "commit", "abc1234", 0},
		{"/v1/similar?sha=abc1234&limit=7", "similar", "abc1234", 7},
		{"/v1/ask?q=why+retry", "ask", "why retry", 0},
		{"/v1/pr?number=42", "pr", "", 42},
		{"/v1/risk?path=a.go", "risk", "a.go", 0},
	}
	for _, tt := range tests {
		t.Run(tt.called, func(t *testing.T) {
			fake := &fakeService{}
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tt.url, nil)
			(&Server{Service: fake}).Handler().ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
			}
			if fake.called != tt.called || fake.arg != tt.arg || fake.n != tt.n {
				t.Fatalf("call = %q %q %d, want %q %q %d", fake.called, fake.arg, fake.n, tt.called, tt.arg, tt.n)
			}
		})
	}
}

func TestAPIValidation(t *testing.T) {
	fake := &fakeService{}
	h := (&Server{Service: fake}).Handler()
	for _, url := range []string{"/v1/archaeology", "/v1/similar?sha=x&limit=nope", "/v1/pr?number=0"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s status = %d, want 400", url, rec.Code)
		}
	}
}

func TestAPIServiceErrorIsUnprocessable(t *testing.T) {
	fake := &fakeService{}
	rec := httptest.NewRecorder()
	(&Server{Service: fake}).Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/commit?sha=boom", nil))
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want 422", rec.Code)
	}
}
