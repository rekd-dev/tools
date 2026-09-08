package modules

// Module is one source-file node in the architecture graph.
type Module struct {
	ID         string   `json:"id"`
	Kind       string   `json:"kind"` // file
	Path       string   `json:"path"`
	Language   string   `json:"language"` // typescript | csharp
	Layer      string   `json:"layer,omitempty"`
	Abstract   bool     `json:"abstract"`
	Namespace  string   `json:"namespace,omitempty"`
	DrillPath  string   `json:"drillPath,omitempty"`
	ExtImports []string `json:"extImports,omitempty"`
	Source     string   `json:"source"`
}

// Dep is a directed dependency between modules (or to an external package).
type Dep struct {
	FromID  string `json:"fromId"`
	ToID    string `json:"toId"`
	Kind    string `json:"kind"` // direct | abstract | external
	ViaFile string `json:"viaFile,omitempty"`
}

// Graph is the full module dependency graph for a repo.
type Graph struct {
	Modules []Module `json:"modules"`
	Deps    []Dep    `json:"deps"`
}
