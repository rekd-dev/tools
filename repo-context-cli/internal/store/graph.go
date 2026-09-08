package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"repo-context-cli/internal/model"

	_ "modernc.org/sqlite"
)

func Open(dbPath string) (*sql.DB, error) {
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn = dbPath + "?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL; PRAGMA busy_timeout=5000;"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func ApplyGraphSchema(db *sql.DB) error {
	if _, err := db.Exec(GraphDDL); err != nil {
		return fmt.Errorf("graph schema: %w", err)
	}
	return nil
}

func ReplaceGraph(db *sql.DB, g model.Graph) error {
	if err := ApplyGraphSchema(db); err != nil {
		return err
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, stmt := range []string{
		"DELETE FROM graph_evidence",
		"DELETE FROM graph_edges",
		"DELETE FROM graph_nodes",
		"DELETE FROM analysis_coverage",
	} {
		if _, err := tx.Exec(stmt); err != nil {
			return err
		}
	}

	nStmt, err := tx.Prepare(`INSERT INTO graph_nodes(id, kind, name, qualified_name, language, file, line, col, layer, abstract, extra) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer nStmt.Close()
	for _, n := range g.Nodes {
		abs := 0
		if n.Abstract {
			abs = 1
		}
		extra := ""
		if len(n.Extra) > 0 {
			b, _ := json.Marshal(n.Extra)
			extra = string(b)
		}
		if _, err := nStmt.Exec(n.ID, n.Kind, n.Name, n.QualifiedName, n.Language, n.File, n.Line, n.Column, n.Layer, abs, extra); err != nil {
			return err
		}
	}

	eStmt, err := tx.Prepare(`INSERT INTO graph_edges(id, from_id, to_id, kind, source, confidence, analyzer, file, line, col, detail, unresolved) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer eStmt.Close()
	evStmt, err := tx.Prepare(`INSERT INTO graph_evidence(edge_id, source, confidence, analyzer, file, line, col, snippet, detail) VALUES(?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer evStmt.Close()
	for _, e := range g.Edges {
		un := 0
		if e.Unresolved {
			un = 1
		}
		if _, err := eStmt.Exec(e.ID, e.FromID, e.ToID, e.Kind, e.Source, e.Confidence, e.Analyzer, e.File, e.Line, e.Column, e.Detail, un); err != nil {
			return err
		}
		evs := e.Evidence
		if len(evs) == 0 {
			evs = []model.Evidence{{
				Source: e.Source, Confidence: e.Confidence, Analyzer: e.Analyzer,
				File: e.File, Line: e.Line, Column: e.Column, Detail: e.Detail,
			}}
		}
		for _, ev := range evs {
			if _, err := evStmt.Exec(e.ID, ev.Source, ev.Confidence, ev.Analyzer, ev.File, ev.Line, ev.Column, ev.Snippet, ev.Detail); err != nil {
				return err
			}
		}
	}

	cStmt, err := tx.Prepare(`INSERT INTO analysis_coverage(analyzer, status, message) VALUES(?,?,?)`)
	if err != nil {
		return err
	}
	defer cStmt.Close()
	for _, c := range g.Coverage {
		if _, err := cStmt.Exec(c.Analyzer, c.Status, c.Message); err != nil {
			return err
		}
	}

	if _, err := tx.Exec(`INSERT OR REPLACE INTO metadata(key, value) VALUES('graphSchema', ?)`, SchemaVersion); err != nil {
		return err
	}
	return tx.Commit()
}

func LoadGraph(db *sql.DB) (model.Graph, error) {
	g := model.Graph{}
	rows, err := db.Query(`SELECT id, kind, name, qualified_name, language, file, line, col, layer, abstract, extra FROM graph_nodes`)
	if err != nil {
		return g, err
	}
	defer rows.Close()
	for rows.Next() {
		var n model.Node
		var abs int
		var qn, lang, file, layer, extra sql.NullString
		var line, col sql.NullInt64
		if err := rows.Scan(&n.ID, &n.Kind, &n.Name, &qn, &lang, &file, &line, &col, &layer, &abs, &extra); err != nil {
			return g, err
		}
		n.QualifiedName = qn.String
		n.Language = lang.String
		n.File = file.String
		n.Line = int(line.Int64)
		n.Column = int(col.Int64)
		n.Layer = layer.String
		n.Abstract = abs == 1
		if extra.String != "" {
			_ = json.Unmarshal([]byte(extra.String), &n.Extra)
		}
		g.Nodes = append(g.Nodes, n)
	}

	erows, err := db.Query(`SELECT id, from_id, to_id, kind, source, confidence, analyzer, file, line, col, detail, unresolved FROM graph_edges`)
	if err != nil {
		return g, err
	}
	defer erows.Close()
	for erows.Next() {
		var e model.Edge
		var file, detail sql.NullString
		var line, col, un sql.NullInt64
		if err := erows.Scan(&e.ID, &e.FromID, &e.ToID, &e.Kind, &e.Source, &e.Confidence, &e.Analyzer, &file, &line, &col, &detail, &un); err != nil {
			return g, err
		}
		e.File = file.String
		e.Line = int(line.Int64)
		e.Column = int(col.Int64)
		e.Detail = detail.String
		e.Unresolved = un.Int64 == 1
		g.Edges = append(g.Edges, e)
	}

	evMap := map[string][]model.Evidence{}
	evrows, err := db.Query(`SELECT edge_id, source, confidence, analyzer, file, line, col, snippet, detail FROM graph_evidence`)
	if err == nil {
		defer evrows.Close()
		for evrows.Next() {
			var edgeID string
			var ev model.Evidence
			var file, snippet, detail sql.NullString
			var line, col sql.NullInt64
			if err := evrows.Scan(&edgeID, &ev.Source, &ev.Confidence, &ev.Analyzer, &file, &line, &col, &snippet, &detail); err != nil {
				return g, err
			}
			ev.File = file.String
			ev.Line = int(line.Int64)
			ev.Column = int(col.Int64)
			ev.Snippet = snippet.String
			ev.Detail = detail.String
			evMap[edgeID] = append(evMap[edgeID], ev)
		}
	}
	for i := range g.Edges {
		g.Edges[i].Evidence = evMap[g.Edges[i].ID]
	}

	crows, err := db.Query(`SELECT analyzer, status, message FROM analysis_coverage`)
	if err == nil {
		defer crows.Close()
		for crows.Next() {
			var c model.Coverage
			var msg sql.NullString
			crows.Scan(&c.Analyzer, &c.Status, &msg)
			c.Message = msg.String
			g.Coverage = append(g.Coverage, c)
		}
	}
	return g, nil
}
