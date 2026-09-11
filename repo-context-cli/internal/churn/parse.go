package churn

import (
	"sort"
	"strings"
)

// Stat is git activity for one architecture box.
type Stat struct {
	Commits int      `json:"commits"`
	Authors int      `json:"authors"`
	Last    string   `json:"last,omitempty"`
	Author  []string `json:"-"`
}

// Report maps view node ids to churn.
type Report struct {
	Since   string          `json:"since"`
	Message string          `json:"message,omitempty"`
	Nodes   map[string]Stat `json:"nodes"`
}

// Root is a view node and the source tree it covers.
type Root struct {
	ID       string
	FileRoot string
}

// ParseLog aggregates `git log --pretty=format:COMMIT\t%an\t%ad --date=short --name-only`.
func ParseLog(raw string, roots []Root) map[string]Stat {
	type acc struct {
		commits int
		authors map[string]bool
		last    string
	}
	byID := map[string]*acc{}
	for _, r := range roots {
		if r.ID == "" || r.FileRoot == "" {
			continue
		}
		byID[r.ID] = &acc{authors: map[string]bool{}}
	}
	var author, date string
	seenThisCommit := map[string]bool{}
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "COMMIT\t") {
			parts := strings.SplitN(strings.TrimPrefix(line, "COMMIT\t"), "\t", 2)
			author = ""
			date = ""
			if len(parts) > 0 {
				author = parts[0]
			}
			if len(parts) > 1 {
				date = parts[1]
			}
			seenThisCommit = map[string]bool{}
			continue
		}
		file := strings.ReplaceAll(line, "\\", "/")
		for _, r := range roots {
			root := strings.ReplaceAll(r.FileRoot, "\\", "/")
			if !underRoot(file, root) {
				continue
			}
			a := byID[r.ID]
			if a == nil || seenThisCommit[r.ID] {
				continue
			}
			seenThisCommit[r.ID] = true
			a.commits++
			if author != "" {
				a.authors[author] = true
			}
			if date > a.last {
				a.last = date
			}
		}
	}
	out := map[string]Stat{}
	for id, a := range byID {
		if a.commits == 0 {
			continue
		}
		names := make([]string, 0, len(a.authors))
		for n := range a.authors {
			names = append(names, n)
		}
		sort.Strings(names)
		out[id] = Stat{Commits: a.commits, Authors: len(a.authors), Last: a.last, Author: names}
	}
	return out
}

func underRoot(file, root string) bool {
	f := strings.ToLower(file)
	r := strings.ToLower(strings.TrimSuffix(root, "/"))
	if r == "" {
		return false
	}
	return f == r || strings.HasPrefix(f, r+"/")
}
