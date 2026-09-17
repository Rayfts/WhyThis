package githubx

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCommitChecksPaginates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("page") == "1" {
			_, _ = fmt.Fprint(w, `{"check_runs":[`)
			for i := 0; i < 100; i++ {
				if i > 0 {
					_, _ = fmt.Fprint(w, ",")
				}
				_, _ = fmt.Fprintf(w, `{"id":%d,"name":"check-%d","conclusion":"success"}`, i+1, i+1)
			}
			_, _ = fmt.Fprint(w, `]}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"check_runs":[{"id":101,"name":"last","conclusion":"failure"}]}`)
	}))
	defer srv.Close()
	c := New("", srv.URL, nil)
	got, err := c.CommitChecks(context.Background(), Repo{Owner: "o", Name: "r"}, "abc")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 101 || got[100].Conclusion != "failure" {
		t.Fatalf("unexpected: len=%d last=%+v", len(got), got[len(got)-1])
	}
}
