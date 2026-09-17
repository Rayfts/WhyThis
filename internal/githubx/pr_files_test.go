package githubx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

func TestPRFilesPaginates(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page, _ := strconv.Atoi(r.URL.Query().Get("page"))
		var count int
		switch page {
		case 1:
			count = githubPageSize
		case 2:
			count = 3
		default:
			count = 0
		}
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
