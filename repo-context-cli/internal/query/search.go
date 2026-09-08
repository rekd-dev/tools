package query

import (
	"sort"
	"strings"

	"repo-context-cli/internal/model"
)

type Hit struct {
	ID       string  `json:"id"`
	Kind     string  `json:"kind"`
	Name     string  `json:"name"`
	File     string  `json:"file,omitempty"`
	Language string  `json:"language,omitempty"`
	Score    float64 `json:"score"`
}

func Search(g model.Graph, q string, limit int) []Hit {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return []Hit{}
	}
	if limit <= 0 {
		limit = 40
	}
	var hits []Hit
	for _, n := range g.Nodes {
		score := scoreNode(n, q)
		if score <= 0 {
			continue
		}
		hits = append(hits, Hit{ID: n.ID, Kind: n.Kind, Name: n.Name, File: n.File, Language: n.Language, Score: score})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Name < hits[j].Name
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

func scoreNode(n model.Node, q string) float64 {
	name := strings.ToLower(n.Name)
	qual := strings.ToLower(n.QualifiedName)
	id := strings.ToLower(n.ID)
	file := strings.ToLower(n.File)
	switch {
	case name == q:
		return 10
	case strings.HasPrefix(name, q):
		return 7
	case strings.Contains(name, q):
		return 5
	case strings.Contains(qual, q):
		return 4
	case strings.Contains(id, q):
		return 3
	case strings.Contains(file, q):
		return 2
	default:
		return 0
	}
}
