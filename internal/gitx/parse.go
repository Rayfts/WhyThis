package gitx

import (
	"bufio"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BlameSegment struct {
	Commit       string
	OriginalLine int
	FinalLine    int
	LineCount    int
	Author       string
	AuthorMail   string
	AuthorTime   time.Time
	Summary      string
	Filename     string
	Previous     string
	Content      []string
}

var blameHeader = regexp.MustCompile(`^([0-9a-f]{40,64}) (\d+) (\d+)(?: (\d+))?$`)

func ParseBlamePorcelain(raw string) []BlameSegment {
	s := bufio.NewScanner(strings.NewReader(raw))
	var result []BlameSegment
	var cur *BlameSegment
	for s.Scan() {
		line := s.Text()
		if m := blameHeader.FindStringSubmatch(line); m != nil {
			if cur != nil {
				result = append(result, *cur)
			}
			orig, _ := strconv.Atoi(m[2])
			final, _ := strconv.Atoi(m[3])
			count := 1
			if m[4] != "" {
				count, _ = strconv.Atoi(m[4])
			}
			cur = &BlameSegment{Commit: m[1], OriginalLine: orig, FinalLine: final, LineCount: count}
			continue
		}
		if cur == nil {
			continue
		}
		switch {
		case strings.HasPrefix(line, "author "):
			cur.Author = strings.TrimPrefix(line, "author ")
		case strings.HasPrefix(line, "author-mail "):
			cur.AuthorMail = strings.Trim(strings.TrimPrefix(line, "author-mail "), "<>")
		case strings.HasPrefix(line, "author-time "):
			if sec, err := strconv.ParseInt(strings.TrimPrefix(line, "author-time "), 10, 64); err == nil {
				cur.AuthorTime = time.Unix(sec, 0).UTC()
			}
		case strings.HasPrefix(line, "summary "):
			cur.Summary = strings.TrimPrefix(line, "summary ")
		case strings.HasPrefix(line, "filename "):
			cur.Filename = strings.TrimPrefix(line, "filename ")
		case strings.HasPrefix(line, "previous "):
			cur.Previous = strings.TrimPrefix(line, "previous ")
		case strings.HasPrefix(line, "\t"):
			cur.Content = append(cur.Content, strings.TrimPrefix(line, "\t"))
		}
	}
	if cur != nil {
		result = append(result, *cur)
	}
	return result
}

type CommitRecord struct {
	SHA     string
	Parents []string
	Author  string
	Email   string
	Date    time.Time
	Subject string
	Body    string
	Changes []FileChange
}

type FileChange struct {
	Status string
	Old    string
	Path   string
}

func ParseCommitShow(raw string) CommitRecord {
	parts := strings.SplitN(raw, "\x1e", 2)
	header := strings.TrimSpace(parts[0])
	fields := strings.Split(header, "\x1f")
	var c CommitRecord
	if len(fields) >= 7 {
		c.SHA = fields[0]
		c.Parents = strings.Fields(fields[1])
		c.Author = fields[2]
		c.Email = fields[3]
		c.Date, _ = time.Parse(time.RFC3339, fields[4])
		c.Subject = fields[5]
		c.Body = fields[6]
	}
	if len(parts) == 2 {
		for _, line := range strings.Split(strings.TrimSpace(parts[1]), "\n") {
			cols := strings.Split(line, "\t")
			if len(cols) < 2 {
				continue
			}
			fc := FileChange{Status: cols[0], Path: cols[len(cols)-1]}
			if len(cols) == 3 {
				fc.Old = cols[1]
			}
			c.Changes = append(c.Changes, fc)
		}
	}
	return c
}
