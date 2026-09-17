package githubx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestParseRemote(t *testing.T) {
	cases := map[string]Repo{"git@github.com:Rayfts/WhyThis.git": {Owner: "Rayfts", Name: "WhyThis"}, "https://github.com/Rayfts/WhyThis.git": {Owner: "Rayfts", Name: "WhyThis"}}
	for in, want := range cases {
		got, err := ParseRemote(in)
		if err != nil || got != want {
			t.Fatalf("%s => %#v %v", in, got, err)
		}
	}
}

func TestPRPaginatesAllDiscussionKinds(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r/pulls/7" {
			_ = json.NewEncoder(w).Encode(PullRequest{Number: 7, Title: "PR"})
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		count := pageCount(page, 2)
		switch {
		case strings.HasSuffix(r.URL.Path, "/reviews"):
			items := make([]Review, count)
			for i := range items {
				items[i].ID = int64(page*1000 + i)
			}
			_ = json.NewEncoder(w).Encode(items)
		case strings.HasSuffix(r.URL.Path, "/comments"):
			items := make([]Comment, count)
			for i := range items {
				items[i].ID = int64(page*1000 + i)
			}
			_ = json.NewEncoder(w).Encode(items)
		default:
			http.Error(w, "unexpected path", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := New("", server.URL, nil)
	ctx, err := client.PR(context.Background(), Repo{Owner: "o", Name: "r"}, 7)
	if err != nil {
		t.Fatal(err)
	}
	want := githubPageSize + 2
	if len(ctx.IssueComments) != want || len(ctx.ReviewComments) != want || len(ctx.Reviews) != want {
		t.Fatalf("counts issue=%d review-comments=%d reviews=%d want=%d each", len(ctx.IssueComments), len(ctx.ReviewComments), len(ctx.Reviews), want)
	}
}

func TestIssuePaginatesComments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/repos/o/r/issues/9" {
			_ = json.NewEncoder(w).Encode(Issue{Number: 9, Title: "Issue"})
			return
		}
		if r.URL.Path != "/repos/o/r/issues/9/comments" {
			http.Error(w, fmt.Sprintf("unexpected path %s", r.URL.Path), http.StatusNotFound)
			return
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		items := make([]Comment, pageCount(page, 5))
		_ = json.NewEncoder(w).Encode(items)
	}))
	defer server.Close()

	client := New("", server.URL, nil)
	_, comments, err := client.Issue(context.Background(), Repo{Owner: "o", Name: "r"}, 9)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(comments), githubPageSize+5; got != want {
		t.Fatalf("got %d comments, want %d", got, want)
	}
}
