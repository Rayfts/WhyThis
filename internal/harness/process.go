package harness

import (
	"bufio"
	"encoding/json"
	"strings"
)

func extractText(raw string) string {
	var chunks []string
	s := bufio.NewScanner(strings.NewReader(raw))
	buf := make([]byte, 0, 64*1024)
	s.Buffer(buf, 4<<20)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" {
			continue
		}
		var v any
		if json.Unmarshal([]byte(line), &v) == nil {
			collectStrings(v, &chunks, 0)
			continue
		}
		chunks = append(chunks, line)
	}
	if len(chunks) == 0 {
		return strings.TrimSpace(raw)
	}
	// Prefer later textual payloads because event streams generally end with the final answer.
	seen := map[string]bool{}
	var out []string
	for _, c := range chunks {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] {
			continue
		}
		seen[c] = true
		out = append(out, c)
	}
	return strings.Join(out, "\n")
}

func collectStrings(v any, out *[]string, depth int) {
	if depth > 6 {
		return
	}
	switch x := v.(type) {
	case map[string]any:
		for _, key := range []string{"text", "content", "message", "output", "result", "final", "response"} {
			if val, ok := x[key]; ok {
				switch s := val.(type) {
				case string:
					*out = append(*out, s)
				default:
					collectStrings(s, out, depth+1)
				}
			}
		}
	case []any:
		for _, e := range x {
			collectStrings(e, out, depth+1)
		}
	}
}
