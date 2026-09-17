package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Rayfts/WhyThis/pkg/evidence"
)

type Store struct {
	db *sql.DB
}

type CommitRow struct {
	SHA     string
	Parents string
	Author  string
	Email   string
	Date    time.Time
	Subject string
	Body    string
}

type FileChangeRow struct {
	CommitSHA string
	Status    string
	OldPath   string
	Path      string
	Additions int
	Deletions int
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error { return s.db.Close() }

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
CREATE TABLE IF NOT EXISTS meta (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS commits (
  sha TEXT PRIMARY KEY,
  parents TEXT NOT NULL,
  author TEXT NOT NULL,
  email TEXT NOT NULL,
  authored_at TEXT NOT NULL,
  subject TEXT NOT NULL,
  body TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS file_changes (
  commit_sha TEXT NOT NULL,
  status TEXT NOT NULL,
  old_path TEXT NOT NULL DEFAULT '',
  path TEXT NOT NULL,
  additions INTEGER NOT NULL DEFAULT 0,
  deletions INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY(commit_sha, path, status),
  FOREIGN KEY(commit_sha) REFERENCES commits(sha) ON DELETE CASCADE
);
CREATE INDEX IF NOT EXISTS idx_file_changes_path ON file_changes(path);
CREATE TABLE IF NOT EXISTS evidence_nodes (
  id TEXT PRIMARY KEY,
  kind TEXT NOT NULL,
  label TEXT NOT NULL,
  json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS evidence_edges (
  edge_key TEXT PRIMARY KEY,
  from_id TEXT NOT NULL,
  to_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  json TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS github_cache (
  cache_key TEXT PRIMARY KEY,
  etag TEXT NOT NULL DEFAULT '',
  body BLOB NOT NULL,
  fetched_at TEXT NOT NULL,
  expires_at TEXT NOT NULL
);
`
	_, err := s.db.ExecContext(ctx, schema)
	return err
}

func (s *Store) GetMeta(ctx context.Context, key string) (string, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key=?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *Store) SetMeta(ctx context.Context, key, value string) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

func (s *Store) ResetIndex(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM file_changes; DELETE FROM commits; DELETE FROM evidence_edges; DELETE FROM evidence_nodes; DELETE FROM meta WHERE key LIKE 'index.%';`)
	return err
}

func (s *Store) UpsertCommit(ctx context.Context, c CommitRow, changes []FileChangeRow) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO commits(sha,parents,author,email,authored_at,subject,body) VALUES(?,?,?,?,?,?,?) ON CONFLICT(sha) DO UPDATE SET parents=excluded.parents,author=excluded.author,email=excluded.email,authored_at=excluded.authored_at,subject=excluded.subject,body=excluded.body`, c.SHA, c.Parents, c.Author, c.Email, c.Date.UTC().Format(time.RFC3339Nano), c.Subject, c.Body)
	if err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM file_changes WHERE commit_sha=?`, c.SHA); err != nil {
		return err
	}
	for _, fc := range changes {
		if _, err = tx.ExecContext(ctx, `INSERT INTO file_changes(commit_sha,status,old_path,path,additions,deletions) VALUES(?,?,?,?,?,?)`, fc.CommitSHA, fc.Status, fc.OldPath, fc.Path, fc.Additions, fc.Deletions); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) PutGraph(ctx context.Context, g evidence.Graph) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	for _, n := range g.Nodes {
		b, _ := json.Marshal(n)
		if _, err := tx.ExecContext(ctx, `INSERT INTO evidence_nodes(id,kind,label,json) VALUES(?,?,?,?) ON CONFLICT(id) DO UPDATE SET kind=excluded.kind,label=excluded.label,json=excluded.json`, n.ID, n.Kind, n.Label, string(b)); err != nil {
			return err
		}
	}
	for _, e := range g.Edges {
		b, _ := json.Marshal(e)
		key := fmt.Sprintf("%s|%s|%s", e.From, e.Kind, e.To)
		if _, err := tx.ExecContext(ctx, `INSERT INTO evidence_edges(edge_key,from_id,to_id,kind,json) VALUES(?,?,?,?,?) ON CONFLICT(edge_key) DO UPDATE SET json=excluded.json`, key, e.From, e.To, e.Kind, string(b)); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) Cached(ctx context.Context, key string) (body []byte, etag string, ok bool, err error) {
	var expires string
	err = s.db.QueryRowContext(ctx, `SELECT body,etag,expires_at FROM github_cache WHERE cache_key=?`, key).Scan(&body, &etag, &expires)
	if err == sql.ErrNoRows {
		return nil, "", false, nil
	}
	if err != nil {
		return nil, "", false, err
	}
	t, _ := time.Parse(time.RFC3339Nano, expires)
	return body, etag, time.Now().Before(t), nil
}

func (s *Store) PutCache(ctx context.Context, key, etag string, body []byte, ttl time.Duration) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx, `INSERT INTO github_cache(cache_key,etag,body,fetched_at,expires_at) VALUES(?,?,?,?,?) ON CONFLICT(cache_key) DO UPDATE SET etag=excluded.etag,body=excluded.body,fetched_at=excluded.fetched_at,expires_at=excluded.expires_at`, key, etag, body, now.Format(time.RFC3339Nano), now.Add(ttl).Format(time.RFC3339Nano))
	return err
}
