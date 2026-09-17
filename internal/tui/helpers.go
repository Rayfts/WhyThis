package tui

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/Rayfts/WhyThis/pkg/evidence"
)

func prompt(r *bufio.Reader, w io.Writer, label string) (string, error) {
	_, _ = fmt.Fprintf(w, "%s: ", label)
	return readLine(r)
}
func readLine(r *bufio.Reader) (string, error) {
	s, err := r.ReadString('\n')
	return strings.TrimSpace(s), err
}
func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 2 {
		return s[:n]
	}
	return s[:n-1] + "…"
}
func short(s string) string { return prefix(s, 12) }
func prefix(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}
func attr(n evidence.Node, key string) string {
	if n.Attributes == nil {
		return ""
	}
	return fmt.Sprint(n.Attributes[key])
}
func tailNodes(v []evidence.Node, n int) []evidence.Node {
	if len(v) <= n {
		return v
	}
	return v[len(v)-n:]
}
func tailEdges(v []evidence.Edge, n int) []evidence.Edge {
	if len(v) <= n {
		return v
	}
	return v[len(v)-n:]
}
