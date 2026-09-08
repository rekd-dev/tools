package store

const SchemaVersion = "1"

// GraphDDL is the evidence-aware graph schema. Applied after the inventory tables.
const GraphDDL = `
CREATE TABLE IF NOT EXISTS graph_nodes (
    id              TEXT PRIMARY KEY,
    kind            TEXT NOT NULL,
    name            TEXT NOT NULL,
    qualified_name  TEXT,
    language        TEXT,
    file            TEXT,
    line            INTEGER,
    col             INTEGER,
    layer           TEXT,
    abstract        INTEGER NOT NULL DEFAULT 0,
    extra           TEXT
);

CREATE TABLE IF NOT EXISTS graph_edges (
    id          TEXT PRIMARY KEY,
    from_id     TEXT NOT NULL,
    to_id       TEXT NOT NULL,
    kind        TEXT NOT NULL,
    source      TEXT NOT NULL,
    confidence  REAL NOT NULL,
    analyzer    TEXT NOT NULL,
    file        TEXT,
    line        INTEGER,
    col         INTEGER,
    detail      TEXT,
    unresolved  INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS graph_evidence (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    edge_id     TEXT NOT NULL,
    source      TEXT NOT NULL,
    confidence  REAL NOT NULL,
    analyzer    TEXT NOT NULL,
    file        TEXT,
    line        INTEGER,
    col         INTEGER,
    snippet     TEXT,
    detail      TEXT
);

CREATE TABLE IF NOT EXISTS analysis_coverage (
    analyzer TEXT PRIMARY KEY,
    status   TEXT NOT NULL,
    message  TEXT
);

CREATE INDEX IF NOT EXISTS idx_graph_nodes_kind ON graph_nodes(kind);
CREATE INDEX IF NOT EXISTS idx_graph_nodes_name ON graph_nodes(name);
CREATE INDEX IF NOT EXISTS idx_graph_nodes_file ON graph_nodes(file);
CREATE INDEX IF NOT EXISTS idx_graph_edges_from ON graph_edges(from_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_to ON graph_edges(to_id);
CREATE INDEX IF NOT EXISTS idx_graph_edges_kind ON graph_edges(kind);
CREATE INDEX IF NOT EXISTS idx_graph_evidence_edge ON graph_evidence(edge_id);
`
