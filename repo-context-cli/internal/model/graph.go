package model

import "strings"

// Node is a stable graph entity.
type Node struct {
	ID             string            `json:"id"`
	Kind           string            `json:"kind"`
	Name           string            `json:"name"`
	QualifiedName  string            `json:"qualifiedName,omitempty"`
	Language       string            `json:"language,omitempty"`
	File           string            `json:"file,omitempty"`
	Line           int               `json:"line,omitempty"`
	Column         int               `json:"column,omitempty"`
	Layer          string            `json:"layer,omitempty"`
	Abstract       bool              `json:"abstract,omitempty"`
	Extra          map[string]string `json:"extra,omitempty"`
}

// Evidence is one observation supporting an edge.
type Evidence struct {
	Source     string  `json:"source"`
	Confidence float64 `json:"confidence"`
	Analyzer   string  `json:"analyzer"`
	File       string  `json:"file,omitempty"`
	Line       int     `json:"line,omitempty"`
	Column     int     `json:"column,omitempty"`
	Snippet    string  `json:"snippet,omitempty"`
	Detail     string  `json:"detail,omitempty"`
}

// Edge is a typed relation between nodes, with the highest-confidence evidence selected.
type Edge struct {
	ID         string     `json:"id"`
	FromID     string      `json:"fromId"`
	ToID       string      `json:"toId"`
	Kind       string     `json:"kind"`
	Source     string     `json:"source"`
	Confidence float64    `json:"confidence"`
	Analyzer   string     `json:"analyzer"`
	File       string     `json:"file,omitempty"`
	Line       int        `json:"line,omitempty"`
	Column     int        `json:"column,omitempty"`
	Detail     string     `json:"detail,omitempty"`
	Unresolved bool       `json:"unresolved,omitempty"`
	Evidence   []Evidence `json:"evidence,omitempty"`
}

// Coverage records whether an analyzer ran.
type Coverage struct {
	Analyzer string `json:"analyzer"`
	Status   string `json:"status"` // ran | missing | failed | skipped
	Message  string `json:"message,omitempty"`
}

// Graph is the full evidence-aware architecture graph.
type Graph struct {
	Nodes    []Node     `json:"nodes"`
	Edges    []Edge     `json:"edges"`
	Coverage []Coverage `json:"coverage"`
}

func (g Graph) NodeByID() map[string]Node {
	out := make(map[string]Node, len(g.Nodes))
	for _, n := range g.Nodes {
		out[n.ID] = n
	}
	return out
}

func LanguageOfPath(path string) string {
	p := strings.ToLower(Slash(path))
	switch {
	case strings.HasSuffix(p, ".cs"):
		return "csharp"
	case strings.HasSuffix(p, ".ts"), strings.HasSuffix(p, ".tsx"), strings.HasSuffix(p, ".js"), strings.HasSuffix(p, ".jsx"):
		return "typescript"
	default:
		return ""
	}
}
