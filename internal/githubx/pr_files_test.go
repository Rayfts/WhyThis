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

func TestPRFilesPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		count := pageCount(page, 3)
		files := make([]PRFile, count)
		for i := range files {
			files[i].Filename = fmt.Sprintf("p%d/file-%d.go", page, i)
		}
		_ = json.NewEncoder(w).Encode(files)
	}))
	defer server.Close()

	client := New("", server.URL, nil)
	files, err := client.PRFiles(context.Background(), Repo{Owner: "o", Name: "r"}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(files), githubPageSize+3; got != want {
		t.Fatalf("got %d files, want %d", got, want)
	}
}

func TestPRCommitsPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "/commits") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		count := pageCount(page, 4)
		commits := make([]PRCommit, count)
		for i := range commits {
			commits[i].SHA = fmt.Sprintf("p%d-%d", page, i)
		}
		_ = json.NewEncoder(w).Encode(commits)
	}))
	defer server.Close()

	client := New("", server.URL, nil)
	commits, err := client.PRCommits(context.Background(), Repo{Owner: "o", Name: "r"}, 7)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := len(commits), githubPageSize+4; got != want {
		t.Fatalf("got %d commits, want %d", got, want)
	}
}

func pageCount(page, tail int) int {
	switch page {
	case 1:
		return githubPageSize
	case 2:
		return tail
	default:
		return 0
	}
}
