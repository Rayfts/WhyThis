package app

import (
	"fmt"
	"io"
)

func usage(w io.Writer) {
	_, _ = fmt.Fprint(w, `WhyThis reconstructs why code exists from deterministic Git/GitHub evidence.

Usage:
  whythis [global flags] path/to/file.go:100-140
  whythis file path/to/file.go:100-140
  whythis symbol Foo.Bar
  whythis blame path/to/file.go[:line-range]
  whythis history path/to/file.go
  whythis risk path/to/file.go
  whythis commit <sha>
  whythis similar <sha> [limit]
  whythis pr <number>
  whythis ask "Why is this retry loop here?"
  whythis index
  whythis tui
  whythis serve [127.0.0.1:7788]
  whythis doctor
  whythis harnesses
  whythis capabilities <id>

Global flags:
  --repo <path>       repository root (default .)
  --db <path>         SQLite index path (default .whyth/whythis.db)
  --json              machine-readable output
  --no-ai             deterministic evidence only
  --harness <id>      optional semantic synthesis adapter

Harness IDs: codex, claude-code, opencode, pi, gemini, aider, goose, cline, roo-code, continue
`)
}
