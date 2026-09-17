package storage

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLiteRoundTrip(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "whythis.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	if err := s.SetMeta(ctx, "index.head", "abc"); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetMeta(ctx, "index.head")
	if err != nil || got != "abc" {
		t.Fatalf("got %q err=%v", got, err)
	}
	if err := s.PutCache(ctx, "k", "etag", []byte("{}"), time.Minute); err != nil {
		t.Fatal(err)
	}
	body, etag, ok, err := s.Cached(ctx, "k")
	if err != nil || !ok || etag != "etag" || string(body) != "{}" {
		t.Fatalf("cache mismatch: %q %q %v %v", body, etag, ok, err)
	}
}
func BenchmarkSQLiteMetaRead(b *testing.B) {
	s, err := Open(filepath.Join(b.TempDir(), "whythis.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	ctx := context.Background()
	_ = s.SetMeta(ctx, "index.head", "abc")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := s.GetMeta(ctx, "index.head"); err != nil {
			b.Fatal(err)
		}
	}
}
