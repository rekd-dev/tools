package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const toolVersion = "2.0.0"

const excludeMarker = "# repo-context-cli — analysis-only clone, never commit from here"

// ── Signal categories ────────────────────────────────────────────────────────

type signalCat struct {
	name     string
	patterns []string
}

var complexityCats = []signalCat{
	{"orchestration", []string{"Orchestrat", "Pipeline", "Workflow", "Coordinator"}},
	{"business-logic", []string{"Validator", "Resolver", "Calculator", "Processor", "StateMachine"}},
	{"handler", []string{"Handler", "Dispatcher", "Mediator"}},
	{"integration", []string{"Repository", "EventPublisher", "EventSubscriber", "MessageBus", "ServiceBus", "EventHub"}},
	{"resilience", []string{"retry", "backoff", "Polly", "CircuitBreaker", "exponential"}},
	{"async-csharp", []string{"async Task", "await ", "ContinueWith", "WhenAll", "WhenAny"}},
	{"async-js", []string{"async ", "await ", "Observable", "switchMap", "mergeMap", "combineLatest"}},
	{"logging", []string{
		"ILogger", "LoggingService", "ErrorHandler",
		"appInsights", "trackTrace", "trackEvent", "trackException", "trackPageView",
		"console.log", "console.error", "Console.WriteLine",
		"Serilog", "NLog", "Winston", "Bunyan", "Pino",
	}},
	{"react-hooks", []string{"useState", "useEffect", "useCallback", "useMemo", "useReducer", "useContext"}},
}

// patternToCategory maps lowercase pattern → category name for DB normalization.
var patternToCategory map[string]string

func init() {
	patternToCategory = make(map[string]string)
	for _, cat := range complexityCats {
		for _, pat := range cat.patterns {
			patternToCategory[strings.ToLower(pat)] = cat.name
		}
	}
}

var skipDirNames = map[string]bool{
	"bin": true, "obj": true, "node_modules": true, ".git": true,
	"packages": true, "dist": true, "_": true, "vendor": true,
	"coverage": true, "test-results": true,
}

var sourceExtSet = map[string]bool{
	".cs": true, ".go": true, ".ts": true, ".tsx": true,
	".js": true, ".jsx": true, ".py": true, ".java": true, ".rs": true,
}

var countExts = []string{".cs", ".ts", ".js", ".go", ".py"}

var dotnetRoutePatterns = []string{
	"[HttpGet", "[HttpPost", "[HttpPut", "[HttpDelete", "[Route(",
	"MapGet(", "MapPost(", "MapPut(", "MapDelete(",
}

var funcTriggerPatterns = []string{
	"[HttpTrigger", "[ServiceBusTrigger", "[EventHubTrigger",
	"[TimerTrigger", "[BlobTrigger", "[QueueTrigger",
}

var nodeRoutePatterns = []string{"app.get(", "app.post(", "router.get(", "router.post("}

var messagingPats = []string{
	"ServiceBusClient", "EventHubProducerClient", "EventHubConsumerClient",
	"ITopicClient", "IQueueClient", "BroadcastAsync", "SignalR", "IHubContext",
}

var entryPointNames = []string{
	"Program.cs", "Startup.cs", "main.go", "index.ts", "index.js", "app.py", "main.py",
	"main.tsx", "main.jsx", "App.tsx", "App.jsx",
}

// ── Regex patterns for symbol extraction ─────────────────────────────────────

// C# patterns
var csTypeRe = regexp.MustCompile(`(?m)(?:public|internal|private|protected)\s+(?:static\s+)?(?:abstract\s+)?(?:sealed\s+)?(?:partial\s+)?(class|record|struct|enum)\s+(\w+)(?:<[^>]+>)?(?:\s*:\s*([^{]+))?\s*\{`)
var csInterfaceRe = regexp.MustCompile(`(?m)(?:public|internal)\s+interface\s+(\w+)(?:<[^>]+>)?(?:\s*:\s*([^{]+))?\s*\{`)
var csNamespaceRe = regexp.MustCompile(`(?m)namespace\s+([\w.]+)`)
var csDIRe = regexp.MustCompile(`\.Add(Scoped|Transient|Singleton)<(\w+)(?:,\s*(\w+))?>`)
var csEndpointAttrRe = regexp.MustCompile(`(?m)\[Http(Get|Post|Put|Delete|Patch)(?:\("([^"]*)"\))?\]`)
var csMethodSigRe = regexp.MustCompile(`(?:public|private|protected|internal)\s+(?:static\s+)?(?:async\s+)?(\S+)\s+(\w+)\s*\(`)
var csAuthorizeRe = regexp.MustCompile(`\[Authorize`)
var csAllowAnonRe = regexp.MustCompile(`\[AllowAnonymous`)
var csExportedRe = regexp.MustCompile(`(?:public|internal)`)

// C# event flow patterns
var csMediatRHandlerRe = regexp.MustCompile(`I(?:Notification|Request)Handler<(\w+)`)
var csMediatRPublishRe = regexp.MustCompile(`(?:IMediator|ISender)\.(?:Publish|Send)(?:Async)?(?:<(\w+)>)?`)
var csSBSendRe = regexp.MustCompile(`(?i)SendMessageAsync|SendMessagesAsync`)
var csSBReceiveRe = regexp.MustCompile(`(?i)ProcessMessageAsync|RegisterMessageHandler`)
var csEHSendRe = regexp.MustCompile(`EventHubProducerClient.*SendAsync|SendAsync.*EventData`)
var csEHReceiveRe = regexp.MustCompile(`(?i)ProcessEventAsync|EventProcessorClient`)
var csDomainEventRe = regexp.MustCompile(`(?:AddDomainEvent|RaiseDomainEvent|IDomainEvent)(?:<(\w+)>)?`)
var csSignalRSendRe = regexp.MustCompile(`(?:Clients\.[^.]+\.SendAsync|SendAsync)\s*\(\s*"(\w+)"`)

// TypeScript/Angular patterns
var tsClassRe = regexp.MustCompile(`(?m)export\s+(?:abstract\s+)?class\s+(\w+)(?:\s+extends\s+(\w+))?(?:\s+implements\s+([^{]+))?`)
var tsInterfaceRe = regexp.MustCompile(`(?m)export\s+interface\s+(\w+)(?:\s+extends\s+([^{]+))?`)
var tsComponentRe = regexp.MustCompile(`@Component\(\{[^}]*selector:\s*'([^']+)'`)
var tsInjectableRe = regexp.MustCompile(`(?m)@Injectable\(`)
var tsExportFuncRe = regexp.MustCompile(`(?m)export\s+(?:async\s+)?function\s+(\w+)`)
var tsExportConstRe = regexp.MustCompile(`(?m)export\s+const\s+([A-Z]\w*)\s*[:=]`)
var tsOutputRe = regexp.MustCompile(`@Output\(\)\s*(\w+)\s*=\s*new\s+EventEmitter`)

// TS event flow patterns
var tsSignalROnRe = regexp.MustCompile(`\.on\(\s*["'](\w+)["']`)
var tsSignalRInvokeRe = regexp.MustCompile(`\.(?:invoke|send)\(\s*["'](\w+)["']`)
var tsWebSocketNewRe = regexp.MustCompile(`new\s+WebSocket\(`)
var tsNgrxDispatchRe = regexp.MustCompile(`(?:store\.dispatch|this\.store\.dispatch)\(`)
var tsNgrxOfTypeRe = regexp.MustCompile(`ofType\(\s*(\w+)`)
var tsNgrxEffectRe = regexp.MustCompile(`createEffect`)

// Go patterns
var goStructRe = regexp.MustCompile(`(?m)^type\s+(\w+)\s+struct\b`)
var goInterfaceRe = regexp.MustCompile(`(?m)^type\s+(\w+)\s+interface\b`)
var goFuncRe = regexp.MustCompile(`(?m)^func\s+([A-Z]\w*)\(`)
var goMethodRe = regexp.MustCompile(`(?m)^func\s+\(\w+\s+\*?(\w+)\)\s+([A-Z]\w*)\(`)

// ── Types ────────────────────────────────────────────────────────────────────

type Inventory struct {
	Repo              string                    `json:"repo"`
	Path              string                    `json:"path"`
	ScannedAt         string                    `json:"scannedAt"`
	GitSha            string                    `json:"gitSha"`
	Stack             string                    `json:"stack"`
	Stacks            []string                  `json:"stacks"`
	EntryPoints       []string                  `json:"entryPoints"`
	ProjectFiles      []ProjectFile             `json:"projectFiles"`
	ExternalDeps      []string                  `json:"externalDeps"`
	ComplexitySignals []ComplexitySignal        `json:"complexitySignals"`
	Routes            []RouteSignal             `json:"routes"`
	MessagingSignals  []MessagingSignal         `json:"messagingSignals"`
	TriggerSignals    []TriggerSignal           `json:"triggerSignals"`
	ProjectDeps       []ProjectDep              `json:"projectDeps"`
	FileCounts        map[string]map[string]int `json:"fileCounts"`

	// New: symbol extraction results
	TypeSymbols []TypeSymbol     `json:"typeSymbols,omitempty"`
	DIMappings  []DIMapping      `json:"diMappings,omitempty"`
	Endpoints   []EndpointDetail `json:"endpoints,omitempty"`
	EventFlows  []EventFlow      `json:"eventFlows,omitempty"`

	// Accumulators (not serialized)
	projRefEdges map[string]map[string]bool `json:"-"`
	tsPathMap    map[string]string          `json:"-"`
}

type ProjectDep struct {
	Project   string   `json:"project"`
	DependsOn []string `json:"dependsOn"`
}

type ComplexitySignal struct {
	Path    string   `json:"path"`
	LOC     int      `json:"loc"`
	Signals []string `json:"signals"`
	Score   int      `json:"score"`
}

type RouteSignal struct {
	File    string `json:"file"`
	Pattern string `json:"pattern"`
	Count   int    `json:"count"`
}

type MessagingSignal struct {
	File    string   `json:"file"`
	Signals []string `json:"signals"`
}

type TriggerSignal struct {
	File    string   `json:"file"`
	Signals []string `json:"signals"`
}

type ProjectFile struct {
	Path      string `json:"path"`
	Framework string `json:"framework"`
	Name      string `json:"name"`
}

type TypeSymbol struct {
	Name       string   `json:"name"`
	Kind       string   `json:"kind"`
	File       string   `json:"file"`
	Namespace  string   `json:"namespace,omitempty"`
	Extends    string   `json:"extends,omitempty"`
	Implements []string `json:"implements,omitempty"`
	Exported   bool     `json:"exported"`
	Source     string   `json:"source"`
}

type DIMapping struct {
	Interface      string `json:"interface"`
	Implementation string `json:"implementation"`
	Lifetime       string `json:"lifetime"`
	File           string `json:"file"`
	Source         string `json:"source"`
}

type EndpointDetail struct {
	Route      string `json:"route"`
	Method     string `json:"method"`
	Handler    string `json:"handler"`
	File       string `json:"file"`
	ReturnType string `json:"returnType,omitempty"`
	Auth       string `json:"auth,omitempty"`
	Source     string `json:"source"`
}

type EventFlow struct {
	File      string `json:"file"`
	Direction string `json:"direction"`
	Mechanism string `json:"mechanism"`
	EventType string `json:"eventType,omitempty"`
	Source    string `json:"source"`
}

// csproj XML structs
type csProjXML struct {
	XMLName    xml.Name      `xml:"Project"`
	PropGroups []csPropGroup `xml:"PropertyGroup"`
	ItemGroups []csItemGroup `xml:"ItemGroup"`
}

type csPropGroup struct {
	TargetFramework string `xml:"TargetFramework"`
}

type csItemGroup struct {
	PkgRefs     []csPkgRef     `xml:"PackageReference"`
	ProjectRefs []csProjectRef `xml:"ProjectReference"`
}

type csPkgRef struct {
	Include string `xml:"Include,attr"`
}

type csProjectRef struct {
	Include string `xml:"Include,attr"`
}

const defaultMinScore = 3

// IndexEntry is one row in the index table.
type IndexEntry struct {
	Repo      string `json:"repo"`
	Stack     string `json:"stack"`
	Inventory string `json:"inventory"`
	Maps      int    `json:"maps"`
	GitSha    string `json:"gitSha"`
	Status    string `json:"status"`
	Path      string `json:"path"`
}

// InventoryTOC is the lightweight summary written alongside the DB.
type InventoryTOC struct {
	Repo      string     `json:"repo"`
	Path      string     `json:"path"`
	ScannedAt string     `json:"scannedAt"`
	GitSha    string     `json:"gitSha"`
	Stack     string     `json:"stack"`
	Stacks    []string   `json:"stacks"`
	Summary   TOCSummary `json:"summary"`
}

type TOCSummary struct {
	Projects          int `json:"projects"`
	ExternalDeps      int `json:"externalDeps"`
	EntryPoints       int `json:"entryPoints"`
	ComplexitySignals int `json:"complexitySignals"`
	HighScoreSignals  int `json:"highScoreSignals"`
	MinScore          int `json:"minScore"`
	Routes            int `json:"routes"`
	MessagingSignals  int `json:"messagingSignals"`
	TriggerSignals    int `json:"triggerSignals"`
	Types             int `json:"types"`
	Interfaces        int `json:"interfaces"`
	DIMappings        int `json:"diMappings"`
	Endpoints         int `json:"endpoints"`
	EventFlows        int `json:"eventFlows"`
}

// ── SQLite schema ────────────────────────────────────────────────────────────

const schemaDDL = `
CREATE TABLE IF NOT EXISTS metadata (
    key   TEXT PRIMARY KEY,
    value TEXT
);

CREATE TABLE IF NOT EXISTS projects (
    name      TEXT PRIMARY KEY,
    path      TEXT NOT NULL,
    framework TEXT
);

CREATE TABLE IF NOT EXISTS project_deps (
    project    TEXT NOT NULL,
    depends_on TEXT NOT NULL,
    PRIMARY KEY (project, depends_on)
);

CREATE TABLE IF NOT EXISTS external_deps (
    name TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS entry_points (
    path TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS file_counts (
    directory TEXT NOT NULL,
    extension TEXT NOT NULL,
    count     INTEGER NOT NULL,
    PRIMARY KEY (directory, extension)
);

CREATE TABLE IF NOT EXISTS complexity_signals (
    file  TEXT PRIMARY KEY,
    loc   INTEGER NOT NULL,
    score INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS signal_matches (
    file     TEXT NOT NULL,
    category TEXT NOT NULL,
    pattern  TEXT NOT NULL,
    PRIMARY KEY (file, pattern)
);

CREATE TABLE IF NOT EXISTS routes (
    file    TEXT NOT NULL,
    pattern TEXT NOT NULL,
    count   INTEGER NOT NULL,
    PRIMARY KEY (file, pattern)
);

CREATE TABLE IF NOT EXISTS messaging_signals (
    file   TEXT NOT NULL,
    signal TEXT NOT NULL,
    PRIMARY KEY (file, signal)
);

CREATE TABLE IF NOT EXISTS trigger_signals (
    file   TEXT NOT NULL,
    signal TEXT NOT NULL,
    PRIMARY KEY (file, signal)
);

CREATE TABLE IF NOT EXISTS types (
    name      TEXT NOT NULL,
    kind      TEXT NOT NULL,
    file      TEXT NOT NULL,
    namespace TEXT,
    extends   TEXT,
    exported  INTEGER NOT NULL DEFAULT 1,
    source    TEXT NOT NULL DEFAULT 'heuristic',
    PRIMARY KEY (name, file)
);

CREATE TABLE IF NOT EXISTS type_implements (
    type_name TEXT NOT NULL,
    type_file TEXT NOT NULL,
    interface TEXT NOT NULL,
    PRIMARY KEY (type_name, type_file, interface)
);

CREATE TABLE IF NOT EXISTS di_mappings (
    interface      TEXT NOT NULL,
    implementation TEXT NOT NULL,
    lifetime       TEXT NOT NULL,
    file           TEXT NOT NULL,
    source         TEXT NOT NULL DEFAULT 'heuristic',
    PRIMARY KEY (interface, implementation, file)
);

CREATE TABLE IF NOT EXISTS endpoints (
    route       TEXT NOT NULL,
    method      TEXT NOT NULL,
    handler     TEXT NOT NULL,
    file        TEXT NOT NULL,
    return_type TEXT,
    auth        TEXT,
    source      TEXT NOT NULL DEFAULT 'heuristic',
    PRIMARY KEY (route, method, file)
);

CREATE TABLE IF NOT EXISTS event_flows (
    file       TEXT NOT NULL,
    direction  TEXT NOT NULL,
    mechanism  TEXT NOT NULL,
    event_type TEXT NOT NULL DEFAULT '',
    source     TEXT NOT NULL DEFAULT 'heuristic',
    PRIMARY KEY (file, direction, mechanism, event_type)
);

CREATE INDEX IF NOT EXISTS idx_types_kind ON types(kind);
CREATE INDEX IF NOT EXISTS idx_types_namespace ON types(namespace);
CREATE INDEX IF NOT EXISTS idx_type_implements_interface ON type_implements(interface);
CREATE INDEX IF NOT EXISTS idx_di_interface ON di_mappings(interface);
CREATE INDEX IF NOT EXISTS idx_endpoints_file ON endpoints(file);
CREATE INDEX IF NOT EXISTS idx_complexity_score ON complexity_signals(score);
CREATE INDEX IF NOT EXISTS idx_event_flows_type ON event_flows(event_type);
CREATE INDEX IF NOT EXISTS idx_event_flows_mechanism ON event_flows(mechanism);
CREATE INDEX IF NOT EXISTS idx_event_flows_direction ON event_flows(direction);
`

// ── main ─────────────────────────────────────────────────────────────────────

func main() {
	if len(os.Args) < 2 {
		printHelp()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		runInit(os.Args[2:])
	case "inventory":
		runInventory(os.Args[2:])
	case "index":
		runIndex(os.Args[2:])
	case "query":
		runQuery(os.Args[2:])
	case "version":
		fmt.Printf("repo-context v%s\n", toolVersion)
	case "help", "--help", "-help", "-h":
		printHelp()
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n", os.Args[1])
		printHelp()
		os.Exit(1)
	}
}

// ── init ─────────────────────────────────────────────────────────────────────

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	force := fs.Bool("force", false, "Re-initialize even if already done")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: repo-context init <path> [--force]")
		fmt.Fprintln(os.Stderr, "  Initialize analysis protection for git repos in <path>.")
		fs.PrintDefaults()
	}

	pathArg, _ := parseSubArgs(fs, args)
	if pathArg == "" {
		fs.Usage()
		os.Exit(1)
	}

	basePath, err := validatePath(pathArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	repos := findGitRepos(basePath)
	if len(repos) == 0 {
		fmt.Println("No git repositories found.")
		return
	}

	for _, repo := range repos {
		repoName := filepath.Base(repo)
		contextDir := filepath.Join(repo, ".context")
		initialized := dirExists(contextDir) && isExcludeSet(repo)

		if initialized && !*force {
			fmt.Printf("→ %s — already initialized\n", repoName)
			continue
		}

		if mkErr := os.MkdirAll(contextDir, 0755); mkErr != nil {
			fmt.Fprintf(os.Stderr, "✗ %s — cannot create .context/: %v\n", repoName, mkErr)
			continue
		}

		infoDir := filepath.Join(repo, ".git", "info")
		if mkErr := os.MkdirAll(infoDir, 0755); mkErr != nil {
			fmt.Fprintf(os.Stderr, "✗ %s — cannot create .git/info/: %v\n", repoName, mkErr)
			continue
		}

		excludeFile := filepath.Join(infoDir, "exclude")
		if wErr := appendExclude(excludeFile); wErr != nil {
			fmt.Fprintf(os.Stderr, "✗ %s — cannot update .git/info/exclude: %v\n", repoName, wErr)
			continue
		}

		statusOut, gitErr := runGit(repo, "status", "--porcelain")
		if gitErr != nil {
			fmt.Printf("✓ %s — initialized (git status unavailable: %v)\n", repoName, gitErr)
			continue
		}
		if strings.TrimSpace(statusOut) != "" {
			fmt.Printf("✗ %s — initialized but git status not clean:\n%s\n", repoName, strings.TrimSpace(statusOut))
			continue
		}

		fmt.Printf("✓ %s — .git/info/exclude updated, .context/ created\n", repoName)
	}
}

func isExcludeSet(repoPath string) bool {
	data, err := os.ReadFile(filepath.Join(repoPath, ".git", "info", "exclude"))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), excludeMarker)
}

func appendExclude(path string) error {
	data, _ := os.ReadFile(path)
	existing := string(data)
	if strings.Contains(existing, excludeMarker) {
		return nil
	}
	if len(existing) > 0 && !strings.HasSuffix(existing, "\n") {
		existing += "\n"
	}
	return os.WriteFile(path, []byte(existing+excludeMarker+"\n*\n"), 0644)
}

// ── inventory ─────────────────────────────────────────────────────────────────

func runInventory(args []string) {
	fs := flag.NewFlagSet("inventory", flag.ExitOnError)
	format := fs.String("format", "json", "Output format: json (default), table, agent")
	refresh := fs.Bool("refresh", false, "Only re-scan files changed since last inventory")
	force := fs.Bool("force", false, "Full rescan even if inventory is current")
	pull := fs.Bool("pull", false, "Run git pull before scanning")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: repo-context inventory <path> [flags]")
		fmt.Fprintln(os.Stderr, "  Deep signal scan; writes inventory.db to each repo's .context/.")
		fs.PrintDefaults()
	}

	pathArg, _ := parseSubArgs(fs, args)
	if pathArg == "" {
		fs.Usage()
		os.Exit(1)
	}

	basePath, err := validatePath(pathArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var repos []string
	if isGitRepo(basePath) {
		repos = []string{basePath}
	} else {
		repos = findInitializedRepos(basePath)
	}

	if len(repos) == 0 {
		fmt.Fprintln(os.Stderr, "No initialized repos found. Run 'repo-context init <path>' first.")
		return
	}

	for _, repo := range repos {
		if err := processRepo(repo, *format, *refresh, *force, *pull); err != nil {
			fmt.Fprintf(os.Stderr, "error processing %s: %v\n", filepath.Base(repo), err)
		}
	}
}

func processRepo(repoPath, format string, refresh, force, doPull bool) error {
	name := filepath.Base(repoPath)

	if doPull {
		fmt.Fprintf(os.Stderr, "Pulling %s...\n", name)
		if _, err := runGit(repoPath, "pull"); err != nil {
			fmt.Fprintf(os.Stderr, "warning: git pull failed: %v\n", err)
		}
	}

	currentSha := strings.TrimSpace(runGitSilent(repoPath, "rev-parse", "--short", "HEAD"))

	dbPath := filepath.Join(repoPath, ".context", "inventory.db")
	existingSha := readMetadataFromDB(dbPath, "gitSha")

	if !force && !refresh && existingSha == currentSha && currentSha != "" {
		fmt.Printf("%s: inventory is current (SHA %s), use --force to rescan\n", name, currentSha)
		return nil
	}

	var inv *Inventory
	if refresh && existingSha != "" && existingSha != currentSha {
		inv = refreshInventory(repoPath, dbPath, existingSha, currentSha)
	} else {
		inv = fullScan(repoPath, currentSha)
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	if err := writeInventoryDB(inv, dbPath); err != nil {
		return fmt.Errorf("write db: %w", err)
	}

	tocPath := filepath.Join(filepath.Dir(dbPath), "inventory-toc.json")
	toc := buildTOC(inv)
	if tocData, tocErr := json.MarshalIndent(toc, "", "  "); tocErr == nil {
		_ = os.WriteFile(tocPath, tocData, 0644)
	}

	switch format {
	case "table":
		printInventoryTable(inv)
	case "agent":
		printInventoryAgent(inv)
	default:
		fmt.Printf("%s: written — stack=%s sha=%s signals=%d types=%d di=%d endpoints=%d events=%d\n",
			name, inv.Stack, inv.GitSha, len(inv.ComplexitySignals),
			len(inv.TypeSymbols), len(inv.DIMappings), len(inv.Endpoints), len(inv.EventFlows))
	}
	return nil
}

func fullScan(repoPath, gitSha string) *Inventory {
	name := filepath.Base(repoPath)
	fmt.Fprintf(os.Stderr, "Scanning %s...\n", name)

	inv := &Inventory{
		Repo:              name,
		Path:              repoPath,
		ScannedAt:         time.Now().UTC().Format(time.RFC3339),
		GitSha:            gitSha,
		EntryPoints:       []string{},
		ProjectFiles:      []ProjectFile{},
		ExternalDeps:      []string{},
		ComplexitySignals: []ComplexitySignal{},
		Routes:            []RouteSignal{},
		MessagingSignals:  []MessagingSignal{},
		TriggerSignals:    []TriggerSignal{},
		ProjectDeps:       []ProjectDep{},
		FileCounts:        make(map[string]map[string]int),
		TypeSymbols:       []TypeSymbol{},
		DIMappings:        []DIMapping{},
		Endpoints:         []EndpointDetail{},
		EventFlows:        []EventFlow{},
		projRefEdges:      make(map[string]map[string]bool),
		tsPathMap:         make(map[string]string),
	}

	stacks := detectStacks(repoPath)
	inv.Stacks = stacks
	inv.Stack = stacks[0]
	ajDirs := parseAngularJsonFiles(repoPath, inv)
	parseTsConfigPathsInDirs(repoPath, ajDirs, inv)
	walkAndCollect(repoPath, inv, nil)
	finalizeInventory(inv)
	return inv
}

func refreshInventory(repoPath, dbPath, existingSha, currentSha string) *Inventory {
	diffOut, err := runGit(repoPath, "diff", "--name-only", existingSha, "HEAD")
	if err != nil {
		return fullScan(repoPath, currentSha)
	}

	changed := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSpace(diffOut), "\n") {
		if f := strings.TrimSpace(line); f != "" {
			changed[filepath.ToSlash(f)] = true
		}
	}

	if len(changed) == 0 {
		existing := loadInventoryFromDB(dbPath)
		if existing == nil {
			return fullScan(repoPath, currentSha)
		}
		existing.GitSha = currentSha
		existing.ScannedAt = time.Now().UTC().Format(time.RFC3339)
		return existing
	}

	existing := loadInventoryFromDB(dbPath)
	if existing == nil {
		return fullScan(repoPath, currentSha)
	}

	stacks := detectStacks(repoPath)

	inv := &Inventory{
		Repo:         existing.Repo,
		Path:         existing.Path,
		ScannedAt:    time.Now().UTC().Format(time.RFC3339),
		GitSha:       currentSha,
		Stack:        stacks[0],
		Stacks:       stacks,
		EntryPoints:  existing.EntryPoints,
		ProjectFiles: existing.ProjectFiles,
		ExternalDeps: existing.ExternalDeps,
		ProjectDeps:  []ProjectDep{},
		FileCounts:   existing.FileCounts,
		projRefEdges: make(map[string]map[string]bool),
		tsPathMap:    make(map[string]string),
	}

	for _, s := range existing.ComplexitySignals {
		if !changed[s.Path] {
			inv.ComplexitySignals = append(inv.ComplexitySignals, s)
		}
	}
	for _, r := range existing.Routes {
		if !changed[r.File] {
			inv.Routes = append(inv.Routes, r)
		}
	}
	for _, m := range existing.MessagingSignals {
		if !changed[m.File] {
			inv.MessagingSignals = append(inv.MessagingSignals, m)
		}
	}
	for _, t := range existing.TriggerSignals {
		if !changed[t.File] {
			inv.TriggerSignals = append(inv.TriggerSignals, t)
		}
	}
	for _, ts := range existing.TypeSymbols {
		if !changed[ts.File] {
			inv.TypeSymbols = append(inv.TypeSymbols, ts)
		}
	}
	for _, di := range existing.DIMappings {
		if !changed[di.File] {
			inv.DIMappings = append(inv.DIMappings, di)
		}
	}
	for _, ep := range existing.Endpoints {
		if !changed[ep.File] {
			inv.Endpoints = append(inv.Endpoints, ep)
		}
	}
	for _, ef := range existing.EventFlows {
		if !changed[ef.File] {
			inv.EventFlows = append(inv.EventFlows, ef)
		}
	}

	ajDirs := parseAngularJsonFiles(repoPath, inv)
	parseTsConfigPathsInDirs(repoPath, ajDirs, inv)
	walkAndCollect(repoPath, inv, changed)
	finalizeInventory(inv)
	return inv
}

func walkAndCollect(repoPath string, inv *Inventory, onlyChanged map[string]bool) {
	stackSet := make(map[string]bool)
	for _, s := range inv.Stacks {
		stackSet[s] = true
	}
	isRefresh := onlyChanged != nil

	_ = filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() {
			if skipDirNames[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}

		rel, relErr := filepath.Rel(repoPath, path)
		if relErr != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		ext := strings.ToLower(filepath.Ext(path))

		if !isRefresh {
			for _, cx := range countExts {
				if ext == cx {
					parts := strings.SplitN(rel, "/", 2)
					if len(parts) == 2 {
						dir := parts[0]
						if inv.FileCounts[dir] == nil {
							inv.FileCounts[dir] = make(map[string]int)
						}
						inv.FileCounts[dir][strings.TrimPrefix(cx, ".")]++
					}
					break
				}
			}
		}

		if isRefresh && !onlyChanged[rel] {
			return nil
		}

		base := filepath.Base(path)

		for _, ep := range entryPointNames {
			if strings.EqualFold(base, ep) {
				if !sliceContains(inv.EntryPoints, rel) {
					inv.EntryPoints = append(inv.EntryPoints, rel)
				}
				break
			}
		}

		if ext == ".csproj" {
			pf, deps, projRefs := parseCsProj(path, repoPath)
			if pf != nil {
				inv.ProjectFiles = append(inv.ProjectFiles, *pf)
				for _, refPath := range projRefs {
					if inv.projRefEdges[pf.Name] == nil {
						inv.projRefEdges[pf.Name] = make(map[string]bool)
					}
					inv.projRefEdges[pf.Name][refPath] = true
				}
			}
			inv.ExternalDeps = append(inv.ExternalDeps, deps...)
		}

		if !sourceExtSet[ext] {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}

		text := string(content)
		lc := strings.ToLower(text)
		loc := bytes.Count(content, []byte("\n")) + 1

		// Complexity signals
		matchedPats := map[string]bool{}
		matchedCats := map[string]bool{}
		for _, cat := range complexityCats {
			for _, pat := range cat.patterns {
				if strings.Contains(lc, strings.ToLower(pat)) {
					matchedPats[pat] = true
					matchedCats[cat.name] = true
				}
			}
		}
		if len(matchedPats) > 0 {
			score := len(matchedCats)
			if score > 5 {
				score = 5
			}
			inv.ComplexitySignals = append(inv.ComplexitySignals, ComplexitySignal{
				Path:    rel,
				LOC:     loc,
				Signals: sortedMapKeys(matchedPats),
				Score:   score,
			})
		}

		// Route scanning
		if stackSet["dotnet-webapi"] || stackSet["dotnet"] || stackSet["dotnet-functions"] {
			for _, pat := range dotnetRoutePatterns {
				cnt := strings.Count(lc, strings.ToLower(pat))
				if cnt > 0 {
					inv.Routes = append(inv.Routes, RouteSignal{File: rel, Pattern: pat, Count: cnt})
				}
			}
		}
		if stackSet["angular"] {
			if strings.Contains(strings.ToLower(base), "routing") ||
				strings.Contains(strings.ToLower(base), "routes") {
				cnt := strings.Count(lc, "path:")
				if cnt > 0 {
					inv.Routes = append(inv.Routes, RouteSignal{File: rel, Pattern: "path:", Count: cnt})
				}
			}
		}
		if stackSet["node"] {
			for _, pat := range nodeRoutePatterns {
				cnt := strings.Count(lc, pat)
				if cnt > 0 {
					inv.Routes = append(inv.Routes, RouteSignal{File: rel, Pattern: pat, Count: cnt})
				}
			}
		}
		if stackSet["nextjs"] || stackSet["react"] {
			relFwd := filepath.ToSlash(rel)
			// Pages router API routes: pages/api/**
			if strings.HasPrefix(relFwd, "pages/api/") && (ext == ".ts" || ext == ".js") {
				inv.Routes = append(inv.Routes, RouteSignal{File: rel, Pattern: "pages/api", Count: 1})
			}
			// App router API routes: app/**/route.ts|js
			if (base == "route.ts" || base == "route.js") && strings.Contains(relFwd, "/app/") {
				inv.Routes = append(inv.Routes, RouteSignal{File: rel, Pattern: "app/route", Count: 1})
			}
		}

		// Trigger signals
		trigMatched := map[string]bool{}
		for _, pat := range funcTriggerPatterns {
			if strings.Contains(lc, strings.ToLower(pat)) {
				trigMatched[pat] = true
			}
		}
		if len(trigMatched) > 0 {
			inv.TriggerSignals = append(inv.TriggerSignals, TriggerSignal{
				File:    rel,
				Signals: sortedMapKeys(trigMatched),
			})
		}

		// Messaging signals
		msgMatched := map[string]bool{}
		for _, pat := range messagingPats {
			if strings.Contains(lc, strings.ToLower(pat)) {
				msgMatched[pat] = true
			}
		}
		if len(msgMatched) > 0 {
			inv.MessagingSignals = append(inv.MessagingSignals, MessagingSignal{
				File:    rel,
				Signals: sortedMapKeys(msgMatched),
			})
		}

		// TS path-alias dependency scanning
		if (ext == ".ts" || ext == ".tsx") && len(inv.tsPathMap) > 0 {
			fileDir := filepath.ToSlash(filepath.Dir(rel))
			scanTsImports(text, fileDir, inv)
		}

		// Symbol extraction
		switch ext {
		case ".cs":
			extractCSharpSymbols(text, rel, inv)
			extractCSharpEventFlows(text, rel, inv)
		case ".ts", ".tsx", ".jsx":
			extractTypeScriptSymbols(text, rel, ext, inv)
			extractTypeScriptEventFlows(text, rel, inv)
		case ".go":
			extractGoSymbols(text, rel, inv)
		}

		return nil
	})
}

func finalizeInventory(inv *Inventory) {
	sort.Slice(inv.ComplexitySignals, func(i, j int) bool {
		if inv.ComplexitySignals[i].Score != inv.ComplexitySignals[j].Score {
			return inv.ComplexitySignals[i].Score > inv.ComplexitySignals[j].Score
		}
		return inv.ComplexitySignals[i].LOC > inv.ComplexitySignals[j].LOC
	})
	inv.ExternalDeps = uniqueSorted(inv.ExternalDeps)
	sort.Strings(inv.EntryPoints)
	inv.ProjectDeps = buildProjectDeps(inv)
}

// ── Symbol extractors ────────────────────────────────────────────────────────

func extractCSharpSymbols(text, rel string, inv *Inventory) {
	ns := ""
	if m := csNamespaceRe.FindStringSubmatch(text); len(m) > 1 {
		ns = m[1]
	}

	// Determine current class name for endpoint handler attribution
	var currentClass string

	for _, m := range csTypeRe.FindAllStringSubmatch(text, -1) {
		kind := m[1]
		name := m[2]
		inheritance := strings.TrimSpace(m[3])
		exported := csExportedRe.MatchString(m[0])
		currentClass = name

		var extends string
		var implements []string
		if inheritance != "" {
			parts := splitInheritance(inheritance)
			for _, p := range parts {
				p = strings.TrimSpace(p)
				p = stripGeneric(p)
				if p == "" {
					continue
				}
				if strings.HasPrefix(p, "I") && len(p) > 1 && p[1] >= 'A' && p[1] <= 'Z' {
					implements = append(implements, p)
				} else if extends == "" {
					extends = p
				} else {
					implements = append(implements, p)
				}
			}
		}

		inv.TypeSymbols = append(inv.TypeSymbols, TypeSymbol{
			Name: name, Kind: kind, File: rel, Namespace: ns,
			Extends: extends, Implements: implements, Exported: exported,
			Source: "heuristic",
		})
	}

	for _, m := range csInterfaceRe.FindAllStringSubmatch(text, -1) {
		name := m[1]
		var extends []string
		if m[2] != "" {
			for _, p := range splitInheritance(m[2]) {
				p = strings.TrimSpace(stripGeneric(p))
				if p != "" {
					extends = append(extends, p)
				}
			}
		}
		extendsStr := ""
		if len(extends) > 0 {
			extendsStr = extends[0]
		}
		inv.TypeSymbols = append(inv.TypeSymbols, TypeSymbol{
			Name: name, Kind: "interface", File: rel, Namespace: ns,
			Extends: extendsStr, Implements: extends, Exported: true,
			Source: "heuristic",
		})
	}

	// DI registrations
	for _, m := range csDIRe.FindAllStringSubmatch(text, -1) {
		lifetime := strings.ToLower(m[1])
		iface := m[2]
		impl := m[3]
		if impl == "" {
			impl = iface
			iface = ""
		}
		inv.DIMappings = append(inv.DIMappings, DIMapping{
			Interface: iface, Implementation: impl, Lifetime: lifetime,
			File: rel, Source: "heuristic",
		})
	}

	// Endpoint extraction
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if m := csEndpointAttrRe.FindStringSubmatch(line); m != nil {
			method := strings.ToUpper(m[1])
			route := m[2]

			auth := ""
			for j := maxInt(0, i-5); j < i; j++ {
				if csAuthorizeRe.MatchString(lines[j]) {
					auth = "Authorize"
				}
				if csAllowAnonRe.MatchString(lines[j]) {
					auth = "AllowAnonymous"
				}
			}

			handler := currentClass
			returnType := ""
			for j := i + 1; j < minInt(len(lines), i+5); j++ {
				if sm := csMethodSigRe.FindStringSubmatch(lines[j]); sm != nil {
					returnType = sm[1]
					handler = currentClass + "." + sm[2]
					break
				}
			}

			inv.Endpoints = append(inv.Endpoints, EndpointDetail{
				Route: route, Method: method, Handler: handler,
				File: rel, ReturnType: returnType, Auth: auth,
				Source: "heuristic",
			})
		}
	}
}

func extractTypeScriptSymbols(text, rel, ext string, inv *Inventory) {
	// Check for Angular component decorator
	componentSelector := ""
	if m := tsComponentRe.FindStringSubmatch(text); len(m) > 1 {
		componentSelector = m[1]
	}

	isInjectable := tsInjectableRe.MatchString(text)

	for _, m := range tsClassRe.FindAllStringSubmatch(text, -1) {
		name := m[1]
		extends := strings.TrimSpace(m[2])
		implementsStr := strings.TrimSpace(m[3])

		kind := "class"
		if componentSelector != "" {
			kind = "component"
		}

		var implements []string
		if implementsStr != "" {
			for _, p := range strings.Split(implementsStr, ",") {
				p = strings.TrimSpace(stripGeneric(p))
				if p != "" {
					implements = append(implements, p)
				}
			}
		}

		sym := TypeSymbol{
			Name: name, Kind: kind, File: rel,
			Extends: stripGeneric(extends), Implements: implements,
			Exported: true, Source: "heuristic",
		}
		if isInjectable {
			sym.Kind = "service"
		}
		inv.TypeSymbols = append(inv.TypeSymbols, sym)
	}

	for _, m := range tsInterfaceRe.FindAllStringSubmatch(text, -1) {
		name := m[1]
		extends := ""
		if m[2] != "" {
			extends = strings.TrimSpace(stripGeneric(strings.Split(m[2], ",")[0]))
		}
		inv.TypeSymbols = append(inv.TypeSymbols, TypeSymbol{
			Name: name, Kind: "interface", File: rel,
			Extends: extends, Exported: true, Source: "heuristic",
		})
	}

	isReactFile := ext == ".tsx" || ext == ".jsx"
	for _, m := range tsExportFuncRe.FindAllStringSubmatch(text, -1) {
		name := m[1]
		kind := "function"
		if isReactFile && len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
			kind = "component"
		}
		inv.TypeSymbols = append(inv.TypeSymbols, TypeSymbol{
			Name: name, Kind: kind, File: rel,
			Exported: true, Source: "heuristic",
		})
	}
	if isReactFile {
		for _, m := range tsExportConstRe.FindAllStringSubmatch(text, -1) {
			inv.TypeSymbols = append(inv.TypeSymbols, TypeSymbol{
				Name: m[1], Kind: "component", File: rel,
				Exported: true, Source: "heuristic",
			})
		}
	}
}

func extractGoSymbols(text, rel string, inv *Inventory) {
	pkg := ""
	for _, line := range strings.SplitN(text, "\n", 20) {
		if strings.HasPrefix(strings.TrimSpace(line), "package ") {
			pkg = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "package "))
			break
		}
	}

	for _, m := range goStructRe.FindAllStringSubmatch(text, -1) {
		exported := m[1][0] >= 'A' && m[1][0] <= 'Z'
		inv.TypeSymbols = append(inv.TypeSymbols, TypeSymbol{
			Name: m[1], Kind: "struct", File: rel, Namespace: pkg,
			Exported: exported, Source: "heuristic",
		})
	}

	for _, m := range goInterfaceRe.FindAllStringSubmatch(text, -1) {
		exported := m[1][0] >= 'A' && m[1][0] <= 'Z'
		inv.TypeSymbols = append(inv.TypeSymbols, TypeSymbol{
			Name: m[1], Kind: "interface", File: rel, Namespace: pkg,
			Exported: exported, Source: "heuristic",
		})
	}

	for _, m := range goFuncRe.FindAllStringSubmatch(text, -1) {
		inv.TypeSymbols = append(inv.TypeSymbols, TypeSymbol{
			Name: m[1], Kind: "function", File: rel, Namespace: pkg,
			Exported: true, Source: "heuristic",
		})
	}
}

// ── Event flow extractors ────────────────────────────────────────────────────

func extractCSharpEventFlows(text, rel string, inv *Inventory) {
	// MediatR handlers
	for _, m := range csMediatRHandlerRe.FindAllStringSubmatch(text, -1) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "handle", Mechanism: "MediatR",
			EventType: m[1], Source: "heuristic",
		})
	}

	// MediatR publish/send
	for _, m := range csMediatRPublishRe.FindAllStringSubmatch(text, -1) {
		eventType := m[1]
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "publish", Mechanism: "MediatR",
			EventType: eventType, Source: "heuristic",
		})
	}

	// ServiceBus send
	if csSBSendRe.MatchString(text) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "publish", Mechanism: "ServiceBus",
			Source: "heuristic",
		})
	}

	// ServiceBus receive
	if csSBReceiveRe.MatchString(text) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "subscribe", Mechanism: "ServiceBus",
			Source: "heuristic",
		})
	}

	// EventHub send
	if csEHSendRe.MatchString(text) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "publish", Mechanism: "EventHub",
			Source: "heuristic",
		})
	}

	// EventHub receive
	if csEHReceiveRe.MatchString(text) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "subscribe", Mechanism: "EventHub",
			Source: "heuristic",
		})
	}

	// Domain events
	for _, m := range csDomainEventRe.FindAllStringSubmatch(text, -1) {
		eventType := m[1]
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "publish", Mechanism: "DomainEvent",
			EventType: eventType, Source: "heuristic",
		})
	}

	// SignalR server-side sends
	for _, m := range csSignalRSendRe.FindAllStringSubmatch(text, -1) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "publish", Mechanism: "SignalR",
			EventType: m[1], Source: "heuristic",
		})
	}
}

func extractTypeScriptEventFlows(text, rel string, inv *Inventory) {
	// SignalR client subscribe
	for _, m := range tsSignalROnRe.FindAllStringSubmatch(text, -1) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "subscribe", Mechanism: "SignalR",
			EventType: m[1], Source: "heuristic",
		})
	}

	// SignalR client invoke/send
	for _, m := range tsSignalRInvokeRe.FindAllStringSubmatch(text, -1) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "publish", Mechanism: "SignalR",
			EventType: m[1], Source: "heuristic",
		})
	}

	// WebSocket
	if tsWebSocketNewRe.MatchString(text) {
		if strings.Contains(text, ".onmessage") || strings.Contains(text, "addEventListener('message'") || strings.Contains(text, `addEventListener("message"`) {
			inv.EventFlows = append(inv.EventFlows, EventFlow{
				File: rel, Direction: "subscribe", Mechanism: "WebSocket",
				Source: "heuristic",
			})
		}
		if strings.Contains(text, ".send(") {
			inv.EventFlows = append(inv.EventFlows, EventFlow{
				File: rel, Direction: "publish", Mechanism: "WebSocket",
				Source: "heuristic",
			})
		}
	}

	// EventEmitter @Output()
	for _, m := range tsOutputRe.FindAllStringSubmatch(text, -1) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "publish", Mechanism: "EventEmitter",
			EventType: m[1], Source: "heuristic",
		})
	}

	// NgRx dispatch
	if tsNgrxDispatchRe.MatchString(text) {
		inv.EventFlows = append(inv.EventFlows, EventFlow{
			File: rel, Direction: "publish", Mechanism: "NgRx",
			Source: "heuristic",
		})
	}

	// NgRx effects
	if tsNgrxEffectRe.MatchString(text) {
		for _, m := range tsNgrxOfTypeRe.FindAllStringSubmatch(text, -1) {
			inv.EventFlows = append(inv.EventFlows, EventFlow{
				File: rel, Direction: "handle", Mechanism: "NgRx",
				EventType: m[1], Source: "heuristic",
			})
		}
	}
}

// ── SQLite DB operations ─────────────────────────────────────────────────────

func openDB(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA synchronous=NORMAL;"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

func readMetadataFromDB(dbPath, key string) string {
	db, err := openDB(dbPath)
	if err != nil {
		return ""
	}
	defer db.Close()

	var val string
	err = db.QueryRow("SELECT value FROM metadata WHERE key = ?", key).Scan(&val)
	if err != nil {
		return ""
	}
	return val
}

func writeInventoryDB(inv *Inventory, dbPath string) error {
	_ = os.Remove(dbPath)
	_ = os.Remove(dbPath + "-wal")
	_ = os.Remove(dbPath + "-shm")

	db, err := openDB(dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	if _, err := db.Exec(schemaDDL); err != nil {
		return fmt.Errorf("schema: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Metadata
	metaStmt, _ := tx.Prepare("INSERT INTO metadata(key, value) VALUES(?, ?)")
	defer metaStmt.Close()
	metaStmt.Exec("repo", inv.Repo)
	metaStmt.Exec("path", inv.Path)
	metaStmt.Exec("scannedAt", inv.ScannedAt)
	metaStmt.Exec("gitSha", inv.GitSha)
	metaStmt.Exec("stack", inv.Stack)
	metaStmt.Exec("stacks", strings.Join(inv.Stacks, ","))

	// Projects
	projStmt, _ := tx.Prepare("INSERT OR IGNORE INTO projects(name, path, framework) VALUES(?, ?, ?)")
	defer projStmt.Close()
	for _, pf := range inv.ProjectFiles {
		projStmt.Exec(pf.Name, pf.Path, pf.Framework)
	}

	// Project deps
	pdStmt, _ := tx.Prepare("INSERT OR IGNORE INTO project_deps(project, depends_on) VALUES(?, ?)")
	defer pdStmt.Close()
	for _, pd := range inv.ProjectDeps {
		for _, dep := range pd.DependsOn {
			pdStmt.Exec(pd.Project, dep)
		}
	}

	// External deps
	edStmt, _ := tx.Prepare("INSERT OR IGNORE INTO external_deps(name) VALUES(?)")
	defer edStmt.Close()
	for _, d := range inv.ExternalDeps {
		edStmt.Exec(d)
	}

	// Entry points
	epStmt, _ := tx.Prepare("INSERT OR IGNORE INTO entry_points(path) VALUES(?)")
	defer epStmt.Close()
	for _, ep := range inv.EntryPoints {
		epStmt.Exec(ep)
	}

	// File counts
	fcStmt, _ := tx.Prepare("INSERT OR IGNORE INTO file_counts(directory, extension, count) VALUES(?, ?, ?)")
	defer fcStmt.Close()
	for dir, exts := range inv.FileCounts {
		for ext, count := range exts {
			fcStmt.Exec(dir, ext, count)
		}
	}

	// Complexity signals + signal matches
	csStmt, _ := tx.Prepare("INSERT OR IGNORE INTO complexity_signals(file, loc, score) VALUES(?, ?, ?)")
	defer csStmt.Close()
	smStmt, _ := tx.Prepare("INSERT OR IGNORE INTO signal_matches(file, category, pattern) VALUES(?, ?, ?)")
	defer smStmt.Close()
	for _, s := range inv.ComplexitySignals {
		csStmt.Exec(s.Path, s.LOC, s.Score)
		for _, pat := range s.Signals {
			cat := patternToCategory[strings.ToLower(pat)]
			if cat == "" {
				cat = "unknown"
			}
			smStmt.Exec(s.Path, cat, pat)
		}
	}

	// Routes
	rtStmt, _ := tx.Prepare("INSERT OR IGNORE INTO routes(file, pattern, count) VALUES(?, ?, ?)")
	defer rtStmt.Close()
	for _, r := range inv.Routes {
		rtStmt.Exec(r.File, r.Pattern, r.Count)
	}

	// Messaging signals
	msStmt, _ := tx.Prepare("INSERT OR IGNORE INTO messaging_signals(file, signal) VALUES(?, ?)")
	defer msStmt.Close()
	for _, m := range inv.MessagingSignals {
		for _, sig := range m.Signals {
			msStmt.Exec(m.File, sig)
		}
	}

	// Trigger signals
	tsStmt, _ := tx.Prepare("INSERT OR IGNORE INTO trigger_signals(file, signal) VALUES(?, ?)")
	defer tsStmt.Close()
	for _, t := range inv.TriggerSignals {
		for _, sig := range t.Signals {
			tsStmt.Exec(t.File, sig)
		}
	}

	// Types + type_implements
	tyStmt, _ := tx.Prepare("INSERT OR IGNORE INTO types(name, kind, file, namespace, extends, exported, source) VALUES(?, ?, ?, ?, ?, ?, ?)")
	defer tyStmt.Close()
	tiStmt, _ := tx.Prepare("INSERT OR IGNORE INTO type_implements(type_name, type_file, interface) VALUES(?, ?, ?)")
	defer tiStmt.Close()
	for _, ts := range inv.TypeSymbols {
		exp := 0
		if ts.Exported {
			exp = 1
		}
		tyStmt.Exec(ts.Name, ts.Kind, ts.File, ts.Namespace, ts.Extends, exp, ts.Source)
		for _, iface := range ts.Implements {
			tiStmt.Exec(ts.Name, ts.File, iface)
		}
	}

	// DI mappings
	diStmt, _ := tx.Prepare("INSERT OR IGNORE INTO di_mappings(interface, implementation, lifetime, file, source) VALUES(?, ?, ?, ?, ?)")
	defer diStmt.Close()
	for _, di := range inv.DIMappings {
		diStmt.Exec(di.Interface, di.Implementation, di.Lifetime, di.File, di.Source)
	}

	// Endpoints
	enStmt, _ := tx.Prepare("INSERT OR IGNORE INTO endpoints(route, method, handler, file, return_type, auth, source) VALUES(?, ?, ?, ?, ?, ?, ?)")
	defer enStmt.Close()
	for _, ep := range inv.Endpoints {
		enStmt.Exec(ep.Route, ep.Method, ep.Handler, ep.File, ep.ReturnType, ep.Auth, ep.Source)
	}

	// Event flows
	efStmt, _ := tx.Prepare("INSERT OR IGNORE INTO event_flows(file, direction, mechanism, event_type, source) VALUES(?, ?, ?, ?, ?)")
	defer efStmt.Close()
	for _, ef := range inv.EventFlows {
		efStmt.Exec(ef.File, ef.Direction, ef.Mechanism, ef.EventType, ef.Source)
	}

	return tx.Commit()
}

func loadInventoryFromDB(dbPath string) *Inventory {
	db, err := openDB(dbPath)
	if err != nil {
		return nil
	}
	defer db.Close()

	inv := &Inventory{
		FileCounts:   make(map[string]map[string]int),
		projRefEdges: make(map[string]map[string]bool),
		tsPathMap:    make(map[string]string),
	}

	// Metadata
	rows, err := db.Query("SELECT key, value FROM metadata")
	if err != nil {
		return nil
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		rows.Scan(&k, &v)
		switch k {
		case "repo":
			inv.Repo = v
		case "path":
			inv.Path = v
		case "scannedAt":
			inv.ScannedAt = v
		case "gitSha":
			inv.GitSha = v
		case "stack":
			inv.Stack = v
		case "stacks":
			inv.Stacks = strings.Split(v, ",")
		}
	}

	// Projects
	rows, err = db.Query("SELECT name, path, framework FROM projects")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var pf ProjectFile
			rows.Scan(&pf.Name, &pf.Path, &pf.Framework)
			inv.ProjectFiles = append(inv.ProjectFiles, pf)
		}
	}

	// Project deps
	rows, err = db.Query("SELECT project, depends_on FROM project_deps")
	if err == nil {
		defer rows.Close()
		depMap := map[string][]string{}
		for rows.Next() {
			var proj, dep string
			rows.Scan(&proj, &dep)
			depMap[proj] = append(depMap[proj], dep)
		}
		for proj, deps := range depMap {
			inv.ProjectDeps = append(inv.ProjectDeps, ProjectDep{Project: proj, DependsOn: deps})
		}
	}

	// External deps
	rows, err = db.Query("SELECT name FROM external_deps")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var name string
			rows.Scan(&name)
			inv.ExternalDeps = append(inv.ExternalDeps, name)
		}
	}

	// Entry points
	rows, err = db.Query("SELECT path FROM entry_points")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var p string
			rows.Scan(&p)
			inv.EntryPoints = append(inv.EntryPoints, p)
		}
	}

	// File counts
	rows, err = db.Query("SELECT directory, extension, count FROM file_counts")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var dir, ext string
			var cnt int
			rows.Scan(&dir, &ext, &cnt)
			if inv.FileCounts[dir] == nil {
				inv.FileCounts[dir] = make(map[string]int)
			}
			inv.FileCounts[dir][ext] = cnt
		}
	}

	// Complexity signals
	rows, err = db.Query("SELECT cs.file, cs.loc, cs.score FROM complexity_signals cs ORDER BY cs.score DESC, cs.loc DESC")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var cs ComplexitySignal
			rows.Scan(&cs.Path, &cs.LOC, &cs.Score)
			inv.ComplexitySignals = append(inv.ComplexitySignals, cs)
		}
	}
	// Load signal patterns for each complexity signal
	for i := range inv.ComplexitySignals {
		rows, err = db.Query("SELECT pattern FROM signal_matches WHERE file = ?", inv.ComplexitySignals[i].Path)
		if err == nil {
			for rows.Next() {
				var pat string
				rows.Scan(&pat)
				inv.ComplexitySignals[i].Signals = append(inv.ComplexitySignals[i].Signals, pat)
			}
			rows.Close()
		}
	}

	// Routes
	rows, err = db.Query("SELECT file, pattern, count FROM routes")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var r RouteSignal
			rows.Scan(&r.File, &r.Pattern, &r.Count)
			inv.Routes = append(inv.Routes, r)
		}
	}

	// Messaging signals
	rows, err = db.Query("SELECT file, signal FROM messaging_signals")
	if err == nil {
		defer rows.Close()
		msgMap := map[string][]string{}
		for rows.Next() {
			var f, s string
			rows.Scan(&f, &s)
			msgMap[f] = append(msgMap[f], s)
		}
		for f, sigs := range msgMap {
			inv.MessagingSignals = append(inv.MessagingSignals, MessagingSignal{File: f, Signals: sigs})
		}
	}

	// Trigger signals
	rows, err = db.Query("SELECT file, signal FROM trigger_signals")
	if err == nil {
		defer rows.Close()
		trigMap := map[string][]string{}
		for rows.Next() {
			var f, s string
			rows.Scan(&f, &s)
			trigMap[f] = append(trigMap[f], s)
		}
		for f, sigs := range trigMap {
			inv.TriggerSignals = append(inv.TriggerSignals, TriggerSignal{File: f, Signals: sigs})
		}
	}

	// Types
	rows, err = db.Query("SELECT name, kind, file, namespace, extends, exported, source FROM types")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ts TypeSymbol
			var exp int
			var ns, ext sql.NullString
			rows.Scan(&ts.Name, &ts.Kind, &ts.File, &ns, &ext, &exp, &ts.Source)
			ts.Namespace = ns.String
			ts.Extends = ext.String
			ts.Exported = exp == 1
			inv.TypeSymbols = append(inv.TypeSymbols, ts)
		}
	}
	// Load implements for each type
	for i := range inv.TypeSymbols {
		rows, err = db.Query("SELECT interface FROM type_implements WHERE type_name = ? AND type_file = ?",
			inv.TypeSymbols[i].Name, inv.TypeSymbols[i].File)
		if err == nil {
			for rows.Next() {
				var iface string
				rows.Scan(&iface)
				inv.TypeSymbols[i].Implements = append(inv.TypeSymbols[i].Implements, iface)
			}
			rows.Close()
		}
	}

	// DI mappings
	rows, err = db.Query("SELECT interface, implementation, lifetime, file, source FROM di_mappings")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var di DIMapping
			rows.Scan(&di.Interface, &di.Implementation, &di.Lifetime, &di.File, &di.Source)
			inv.DIMappings = append(inv.DIMappings, di)
		}
	}

	// Endpoints
	rows, err = db.Query("SELECT route, method, handler, file, return_type, auth, source FROM endpoints")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ep EndpointDetail
			var rt, au sql.NullString
			rows.Scan(&ep.Route, &ep.Method, &ep.Handler, &ep.File, &rt, &au, &ep.Source)
			ep.ReturnType = rt.String
			ep.Auth = au.String
			inv.Endpoints = append(inv.Endpoints, ep)
		}
	}

	// Event flows
	rows, err = db.Query("SELECT file, direction, mechanism, event_type, source FROM event_flows")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var ef EventFlow
			rows.Scan(&ef.File, &ef.Direction, &ef.Mechanism, &ef.EventType, &ef.Source)
			inv.EventFlows = append(inv.EventFlows, ef)
		}
	}

	return inv
}

func buildTOC(inv *Inventory) *InventoryTOC {
	highScore := 0
	for _, s := range inv.ComplexitySignals {
		if s.Score >= defaultMinScore {
			highScore++
		}
	}
	ifaceCount := 0
	for _, ts := range inv.TypeSymbols {
		if ts.Kind == "interface" {
			ifaceCount++
		}
	}
	return &InventoryTOC{
		Repo:      inv.Repo,
		Path:      inv.Path,
		ScannedAt: inv.ScannedAt,
		GitSha:    inv.GitSha,
		Stack:     inv.Stack,
		Stacks:    inv.Stacks,
		Summary: TOCSummary{
			Projects:          len(inv.ProjectFiles),
			ExternalDeps:      len(inv.ExternalDeps),
			EntryPoints:       len(inv.EntryPoints),
			ComplexitySignals: len(inv.ComplexitySignals),
			HighScoreSignals:  highScore,
			MinScore:          defaultMinScore,
			Routes:            len(inv.Routes),
			MessagingSignals:  len(inv.MessagingSignals),
			TriggerSignals:    len(inv.TriggerSignals),
			Types:             len(inv.TypeSymbols),
			Interfaces:        ifaceCount,
			DIMappings:        len(inv.DIMappings),
			Endpoints:         len(inv.Endpoints),
			EventFlows:        len(inv.EventFlows),
		},
	}
}

// ── index ─────────────────────────────────────────────────────────────────────

func runIndex(args []string) {
	fs := flag.NewFlagSet("index", flag.ExitOnError)
	format := fs.String("format", "table", "Output format: table (default), json, agent")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: repo-context index <path> [flags]")
		fmt.Fprintln(os.Stderr, "  Show status of all git repos in <path>.")
		fs.PrintDefaults()
	}

	pathArg, _ := parseSubArgs(fs, args)
	if pathArg == "" {
		fs.Usage()
		os.Exit(1)
	}

	basePath, err := validatePath(pathArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	repos := findAllGitRepos(basePath)
	if len(repos) == 0 {
		fmt.Println("No git repositories found.")
		return
	}

	entries := make([]IndexEntry, 0, len(repos))
	for _, repo := range repos {
		entries = append(entries, buildIndexEntry(repo))
	}

	switch *format {
	case "json":
		data, _ := json.MarshalIndent(entries, "", "  ")
		fmt.Println(string(data))
	case "agent":
		for _, e := range entries {
			inv := e.Inventory
			if inv == "" {
				inv = "-"
			}
			stack := e.Stack
			if stack == "" {
				stack = "-"
			}
			sha := e.GitSha
			if sha == "" {
				sha = "-"
			}
			fmt.Printf("%s|%s|%s|%s|maps=%d\n", e.Repo, stack, e.Status, sha, e.Maps)
		}
	default:
		printIndexTable(entries)
	}
}

func buildIndexEntry(repoPath string) IndexEntry {
	name := filepath.Base(repoPath)
	contextDir := filepath.Join(repoPath, ".context")
	dbPath := filepath.Join(contextDir, "inventory.db")

	entry := IndexEntry{
		Repo: name,
		Path: repoPath,
	}

	currentSha := strings.TrimSpace(runGitSilent(repoPath, "rev-parse", "--short", "HEAD"))
	entry.GitSha = currentSha

	if !dirExists(contextDir) {
		entry.Status = "NOT INITIALIZED"
		return entry
	}

	if mdFiles, err := filepath.Glob(filepath.Join(contextDir, "*.md")); err == nil {
		entry.Maps = len(mdFiles)
	}

	if _, err := os.Stat(dbPath); err == nil {
		invSha := readMetadataFromDB(dbPath, "gitSha")
		entry.Stack = readMetadataFromDB(dbPath, "stack")
		scannedAt := readMetadataFromDB(dbPath, "scannedAt")
		if t, parseErr := time.Parse(time.RFC3339, scannedAt); parseErr == nil {
			entry.Inventory = t.Format("2006-01-02")
		} else {
			entry.Inventory = scannedAt
		}
		if invSha != "" && invSha == currentSha {
			entry.Status = "current"
		} else {
			entry.Status = "STALE"
		}
		return entry
	}

	// Fallback: check for legacy inventory.json
	invPath := filepath.Join(contextDir, "inventory.json")
	if raw, err := os.ReadFile(invPath); err == nil {
		var inv struct {
			Stack     string `json:"stack"`
			GitSha    string `json:"gitSha"`
			ScannedAt string `json:"scannedAt"`
		}
		if json.Unmarshal(raw, &inv) == nil {
			entry.Stack = inv.Stack
			if t, parseErr := time.Parse(time.RFC3339, inv.ScannedAt); parseErr == nil {
				entry.Inventory = t.Format("2006-01-02")
			}
			entry.Status = "STALE (v1 json)"
			return entry
		}
	}

	entry.Status = "STALE"
	return entry
}

// ── query ─────────────────────────────────────────────────────────────────────

var sectionAliases = map[string]string{
	"structure": `SELECT key, value FROM metadata
UNION ALL
SELECT 'entry_point', path FROM entry_points
UNION ALL
SELECT 'external_dep', name FROM external_deps
UNION ALL
SELECT 'project', name || '|' || path || '|' || COALESCE(framework, '') FROM projects
UNION ALL
SELECT 'project_dep', project || ' -> ' || depends_on FROM project_deps
UNION ALL
SELECT 'file_count', directory || '/' || extension || ': ' || count FROM file_counts`,

	"signals": `SELECT cs.file, cs.loc, cs.score,
       GROUP_CONCAT(sm.pattern, ', ') AS patterns
FROM complexity_signals cs
LEFT JOIN signal_matches sm ON cs.file = sm.file
WHERE cs.score >= %d
GROUP BY cs.file
ORDER BY cs.score DESC, cs.loc DESC`,

	"routes":    "SELECT file, pattern, count FROM routes ORDER BY file",
	"messaging": "SELECT file, signal FROM messaging_signals ORDER BY file",
	"triggers":  "SELECT file, signal FROM trigger_signals ORDER BY file",

	"types": `SELECT t.name, t.kind, t.file, t.namespace, t.extends, t.exported,
       GROUP_CONCAT(ti.interface, ', ') AS implements
FROM types t
LEFT JOIN type_implements ti ON t.name = ti.type_name AND t.file = ti.type_file
GROUP BY t.name, t.file
ORDER BY t.kind, t.name`,

	"di":        "SELECT interface, implementation, lifetime, file, source FROM di_mappings ORDER BY interface",
	"endpoints": "SELECT route, method, handler, file, return_type, auth, source FROM endpoints ORDER BY file, route",
	"events":    "SELECT file, direction, mechanism, event_type, source FROM event_flows ORDER BY mechanism, event_type, direction",
}

func runQuery(args []string) {
	fs := flag.NewFlagSet("query", flag.ExitOnError)
	sqlFlag := fs.String("sql", "", "Raw SQL query to execute against inventory.db")
	section := fs.String("section", "", "Predefined query: structure, signals, routes, messaging, triggers, types, di, endpoints, events, all")
	minScore := fs.Int("min-score", defaultMinScore, "Minimum complexity score (signals section only)")
	format := fs.String("format", "json", "Output format: json (default), table, csv")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: repo-context query <path> [flags]")
		fmt.Fprintln(os.Stderr, "  Query inventory.db with SQL or predefined sections.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  Use --sql for raw SQL queries or --section for predefined aliases.")
		fmt.Fprintln(os.Stderr, "  If neither is provided, defaults to --section structure.")
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "  Sections: structure, signals, routes, messaging, triggers,")
		fmt.Fprintln(os.Stderr, "            types, di, endpoints, events, all")
		fmt.Fprintln(os.Stderr)
		fs.PrintDefaults()
	}

	pathArg, _ := parseSubArgs(fs, args)
	if pathArg == "" {
		fs.Usage()
		os.Exit(1)
	}

	repoPath, err := resolveRepoPath(pathArg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	dbPath := filepath.Join(repoPath, ".context", "inventory.db")
	if _, err := os.Stat(dbPath); err != nil {
		fmt.Fprintf(os.Stderr, "error: no inventory.db found at %s\n  Run 'repo-context inventory %s' first.\n", dbPath, pathArg)
		os.Exit(1)
	}

	db, err := openDB(dbPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: cannot open db: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	if *sqlFlag != "" {
		if err := execAndFormat(db, *sqlFlag, *format); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	sec := *section
	if sec == "" {
		sec = "structure"
	}

	if sec == "all" {
		for _, s := range []string{"structure", "signals", "routes", "messaging", "triggers", "types", "di", "endpoints", "events"} {
			if *format == "table" || *format == "csv" {
				fmt.Printf("── %s ──\n", s)
			}
			query := sectionAliases[s]
			if s == "signals" {
				query = fmt.Sprintf(query, *minScore)
			}
			if err := execAndFormat(db, query, *format); err != nil {
				fmt.Fprintf(os.Stderr, "  error querying %s: %v\n", s, err)
			}
			if *format == "table" || *format == "csv" {
				fmt.Println()
			}
		}
		return
	}

	query, ok := sectionAliases[sec]
	if !ok {
		fmt.Fprintf(os.Stderr, "error: unknown section %q\n", sec)
		os.Exit(1)
	}
	if sec == "signals" {
		query = fmt.Sprintf(query, *minScore)
	}
	if err := execAndFormat(db, query, *format); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func execAndFormat(db *sql.DB, query, format string) error {
	rows, err := db.Query(query)
	if err != nil {
		return err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return err
	}

	var results []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return err
		}
		row := make(map[string]interface{})
		for i, col := range cols {
			val := values[i]
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			row[col] = val
		}
		results = append(results, row)
	}

	switch format {
	case "json":
		if results == nil {
			results = []map[string]interface{}{}
		}
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	case "csv":
		fmt.Println(strings.Join(cols, ","))
		for _, row := range results {
			vals := make([]string, len(cols))
			for i, col := range cols {
				vals[i] = fmt.Sprintf("%v", row[col])
			}
			fmt.Println(strings.Join(vals, ","))
		}
	default: // table
		if len(results) == 0 {
			fmt.Println("(no rows)")
			return nil
		}
		widths := make([]int, len(cols))
		for i, col := range cols {
			widths[i] = len(col)
		}
		for _, row := range results {
			for i, col := range cols {
				s := fmt.Sprintf("%v", row[col])
				if len(s) > widths[i] {
					widths[i] = len(s)
				}
			}
		}
		for i, w := range widths {
			if w > 60 {
				widths[i] = 60
			}
		}
		// Header
		for i, col := range cols {
			fmt.Printf("%-*s  ", widths[i], truncate(col, widths[i]))
		}
		fmt.Println()
		for i := range cols {
			fmt.Printf("%-*s  ", widths[i], strings.Repeat("-", widths[i]))
		}
		fmt.Println()
		// Rows
		for _, row := range results {
			for i, col := range cols {
				s := fmt.Sprintf("%v", row[col])
				fmt.Printf("%-*s  ", widths[i], truncate(s, widths[i]))
			}
			fmt.Println()
		}
	}
	return nil
}

func resolveRepoPath(pathArg string) (string, error) {
	abs, err := validatePath(pathArg)
	if err != nil {
		return "", err
	}
	if isGitRepo(abs) {
		return abs, nil
	}
	repos := findInitializedRepos(abs)
	if len(repos) == 1 {
		return repos[0], nil
	}
	if len(repos) > 1 {
		return "", fmt.Errorf("multiple repos under %s, specify a single repo path", pathArg)
	}
	return "", fmt.Errorf("no git repo found at %s", pathArg)
}

// ── Stack detection ───────────────────────────────────────────────────────────

func detectStacks(repoPath string) []string {
	var stacks []string

	slnFiles := findFilesExt(repoPath, ".sln")
	if len(slnFiles) > 0 {
		csprojFiles := findFilesExt(repoPath, ".csproj")

		dotnetStack := "dotnet"
		for _, f := range csprojFiles {
			if fileContainsCI(f, "Microsoft.AspNetCore") {
				dotnetStack = "dotnet-webapi"
				break
			}
		}
		if dotnetStack == "dotnet" {
			for _, f := range csprojFiles {
				if fileContainsCI(f, "Microsoft.Azure.Functions") || fileContainsCI(f, "AzureFunctionsVersion") {
					dotnetStack = "dotnet-functions"
					break
				}
			}
		}
		if dotnetStack == "dotnet" {
			if len(findFilesExt(repoPath, ".btproj")) > 0 || len(findFilesExt(repoPath, ".odx")) > 0 {
				dotnetStack = "biztalk"
			}
		}
		if dotnetStack == "dotnet" {
			if len(findFilesExt(repoPath, ".dtsx")) > 0 {
				dotnetStack = "ssis"
			}
		}
		stacks = append(stacks, dotnetStack)
	}

	ajFiles := findFilesNamed(repoPath, "angular.json")
	if len(ajFiles) > 0 {
		stacks = append(stacks, "angular")
	}

	if fileExists(filepath.Join(repoPath, "go.mod")) {
		stacks = append(stacks, "go")
	}

	pkgJson := filepath.Join(repoPath, "package.json")
	if fileExists(pkgJson) {
		if !sliceContains(stacks, "angular") {
			data, err := os.ReadFile(pkgJson)
			if err == nil {
				pkg := string(data)
				if strings.Contains(pkg, `"@angular/core"`) {
					stacks = append(stacks, "angular")
				} else if strings.Contains(pkg, `"next"`) {
					stacks = append(stacks, "react")
					stacks = append(stacks, "nextjs")
				} else if strings.Contains(pkg, `"react"`) {
					stacks = append(stacks, "react")
				} else {
					stacks = append(stacks, "node")
				}
			} else {
				stacks = append(stacks, "node")
			}
		}
	}

	if fileExists(filepath.Join(repoPath, "pyproject.toml")) || fileExists(filepath.Join(repoPath, "requirements.txt")) {
		stacks = append(stacks, "python")
	}

	if fileExists(filepath.Join(repoPath, "Cargo.toml")) {
		stacks = append(stacks, "rust")
	}

	if len(stacks) == 0 {
		return []string{"unknown"}
	}
	return stacks
}

// ── .csproj parsing ───────────────────────────────────────────────────────────

func parseCsProj(path, repoPath string) (*ProjectFile, []string, []string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil
	}

	var proj csProjXML
	if xmlErr := xml.Unmarshal(data, &proj); xmlErr != nil {
		return nil, nil, nil
	}

	pf := &ProjectFile{
		Name: strings.TrimSuffix(filepath.Base(path), ".csproj"),
	}
	if rel, relErr := filepath.Rel(repoPath, path); relErr == nil {
		pf.Path = filepath.ToSlash(rel)
	}
	for _, pg := range proj.PropGroups {
		if pg.TargetFramework != "" {
			pf.Framework = pg.TargetFramework
			break
		}
	}

	var deps []string
	var projRefs []string
	csprojDir := filepath.Dir(path)
	for _, ig := range proj.ItemGroups {
		for _, ref := range ig.PkgRefs {
			if ref.Include != "" {
				deps = append(deps, ref.Include)
			}
		}
		for _, ref := range ig.ProjectRefs {
			if ref.Include == "" {
				continue
			}
			refPath := filepath.Join(csprojDir, filepath.FromSlash(ref.Include))
			refPath = filepath.Clean(refPath)
			if rel, relErr := filepath.Rel(repoPath, refPath); relErr == nil {
				projRefs = append(projRefs, filepath.ToSlash(rel))
			}
		}
	}
	return pf, deps, projRefs
}

// ── angular.json / tsconfig parsing ──────────────────────────────────────────

func parseAngularJsonFiles(repoPath string, inv *Inventory) []string {
	ajPaths := findFilesNamed(repoPath, "angular.json")
	if len(ajPaths) == 0 {
		return nil
	}

	knownPaths := make(map[string]bool)
	for _, pf := range inv.ProjectFiles {
		knownPaths[pf.Path] = true
	}

	var ajDirs []string
	for _, ajPath := range ajPaths {
		ajDir := filepath.Dir(ajPath)
		ajDirs = append(ajDirs, ajDir)

		data, err := os.ReadFile(ajPath)
		if err != nil {
			continue
		}

		var raw map[string]json.RawMessage
		if json.Unmarshal(data, &raw) != nil {
			continue
		}
		projsRaw, ok := raw["projects"]
		if !ok {
			continue
		}

		var projects map[string]struct {
			Root       string `json:"root"`
			SourceRoot string `json:"sourceRoot"`
		}
		if json.Unmarshal(projsRaw, &projects) != nil {
			continue
		}

		for name, proj := range projects {
			projRoot := proj.Root
			if projRoot == "" && proj.SourceRoot != "" {
				projRoot = strings.TrimSuffix(proj.SourceRoot, "/src")
			}
			absRoot := filepath.Join(ajDir, filepath.FromSlash(projRoot))
			rel, relErr := filepath.Rel(repoPath, absRoot)
			if relErr != nil {
				continue
			}
			root := filepath.ToSlash(rel)
			pkgPath := root + "/package.json"
			if knownPaths[pkgPath] || knownPaths[root] {
				continue
			}
			inv.ProjectFiles = append(inv.ProjectFiles, ProjectFile{
				Name: name,
				Path: root,
			})
			knownPaths[root] = true
		}
	}
	return ajDirs
}

func parseTsConfigPathsInDirs(repoPath string, dirs []string, inv *Inventory) {
	searchDirs := []string{repoPath}
	seen := map[string]bool{repoPath: true}
	for _, d := range dirs {
		if !seen[d] {
			seen[d] = true
			searchDirs = append(searchDirs, d)
		}
	}

	for _, dir := range searchDirs {
		var data []byte
		var err error
		for _, name := range []string{"tsconfig.base.json", "tsconfig.json"} {
			data, err = os.ReadFile(filepath.Join(dir, name))
			if err == nil {
				break
			}
		}
		if err != nil {
			continue
		}

		cleaned := stripJSONComments(data)

		var tsconfig struct {
			CompilerOptions struct {
				Paths map[string][]string `json:"paths"`
			} `json:"compilerOptions"`
		}
		if json.Unmarshal(cleaned, &tsconfig) != nil {
			continue
		}

		if len(tsconfig.CompilerOptions.Paths) == 0 {
			continue
		}

		for alias, targets := range tsconfig.CompilerOptions.Paths {
			if len(targets) == 0 {
				continue
			}
			absTarget := filepath.Join(dir, filepath.FromSlash(targets[0]))
			rel, relErr := filepath.Rel(repoPath, absTarget)
			if relErr != nil {
				continue
			}
			target := filepath.ToSlash(rel)
			target = strings.TrimSuffix(target, "/*")
			target = strings.TrimSuffix(target, "/index.ts")
			target = strings.TrimSuffix(target, "/public-api.ts")
			target = strings.TrimSuffix(target, "/public_api.ts")
			target = strings.TrimSuffix(target, "/src")

			cleanAlias := strings.TrimSuffix(alias, "/*")
			inv.tsPathMap[cleanAlias] = target
		}
	}
}

func stripJSONComments(data []byte) []byte {
	lines := bytes.Split(data, []byte("\n"))
	var out [][]byte
	for _, line := range lines {
		trimmed := bytes.TrimSpace(line)
		if bytes.HasPrefix(trimmed, []byte("//")) {
			continue
		}
		if idx := findInlineComment(line); idx >= 0 {
			line = line[:idx]
		}
		out = append(out, line)
	}
	result := bytes.Join(out, []byte("\n"))

	var cleaned []byte
	for i := 0; i < len(result); i++ {
		if result[i] == ',' {
			j := i + 1
			for j < len(result) && (result[j] == ' ' || result[j] == '\t' || result[j] == '\n' || result[j] == '\r') {
				j++
			}
			if j < len(result) && (result[j] == '}' || result[j] == ']') {
				continue
			}
		}
		cleaned = append(cleaned, result[i])
	}
	return cleaned
}

func findInlineComment(line []byte) int {
	inString := false
	for i := 0; i < len(line)-1; i++ {
		if line[i] == '\\' && inString {
			i++
			continue
		}
		if line[i] == '"' {
			inString = !inString
		}
		if !inString && line[i] == '/' && line[i+1] == '/' {
			return i
		}
	}
	return -1
}

func scanTsImports(content string, fileProjectDir string, inv *Inventory) {
	if len(inv.tsPathMap) == 0 {
		return
	}

	srcProject := resolveProjectForDir(fileProjectDir, inv)
	if srcProject == "" {
		return
	}

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.Contains(trimmed, "import") || !strings.Contains(trimmed, "from") {
			continue
		}
		for alias, targetDir := range inv.tsPathMap {
			if !strings.Contains(trimmed, alias) {
				continue
			}
			targetProject := resolveProjectForDir(targetDir, inv)
			if targetProject == "" || targetProject == srcProject {
				continue
			}
			if inv.projRefEdges[srcProject] == nil {
				inv.projRefEdges[srcProject] = make(map[string]bool)
			}
			inv.projRefEdges[srcProject][targetProject] = true
		}
	}
}

func resolveProjectForDir(dir string, inv *Inventory) string {
	dir = filepath.ToSlash(dir)
	bestName := ""
	bestLen := -1

	for _, pf := range inv.ProjectFiles {
		pfPath := filepath.ToSlash(pf.Path)
		pfDir := filepath.ToSlash(filepath.Dir(pf.Path))

		matchLen := -1
		if pfPath == dir || pfDir == dir {
			matchLen = len(pfDir)
		} else if strings.HasPrefix(dir, pfPath+"/") {
			matchLen = len(pfPath)
		} else if strings.HasPrefix(dir, pfDir+"/") {
			matchLen = len(pfDir)
		}

		if matchLen > bestLen {
			bestLen = matchLen
			bestName = pf.Name
		}
	}
	return bestName
}

func buildProjectDeps(inv *Inventory) []ProjectDep {
	pathToName := make(map[string]string)
	for _, pf := range inv.ProjectFiles {
		pathToName[pf.Path] = pf.Name
	}

	depMap := make(map[string]map[string]bool)
	allProjects := make(map[string]bool)

	for _, pf := range inv.ProjectFiles {
		allProjects[pf.Name] = true
	}

	if len(inv.projRefEdges) == 0 {
		return []ProjectDep{}
	}

	for project, refs := range inv.projRefEdges {
		allProjects[project] = true
		for ref := range refs {
			targetName := ref
			if name, ok := pathToName[ref]; ok {
				targetName = name
			}
			if depMap[project] == nil {
				depMap[project] = make(map[string]bool)
			}
			depMap[project][targetName] = true
			allProjects[targetName] = true
		}
	}

	return topoSort(allProjects, depMap)
}

func topoSort(projects map[string]bool, depMap map[string]map[string]bool) []ProjectDep {
	inDegree := make(map[string]int)
	for p := range projects {
		inDegree[p] = len(depMap[p])
	}

	var queue []string
	for p := range projects {
		if inDegree[p] == 0 {
			queue = append(queue, p)
		}
	}
	sort.Strings(queue)

	reverseDeps := make(map[string][]string)
	for p, deps := range depMap {
		for d := range deps {
			reverseDeps[d] = append(reverseDeps[d], p)
		}
	}

	var result []ProjectDep
	visited := make(map[string]bool)

	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		if visited[node] {
			continue
		}
		visited[node] = true

		deps := sortedMapKeys(depMap[node])
		result = append(result, ProjectDep{Project: node, DependsOn: deps})

		dependents := reverseDeps[node]
		sort.Strings(dependents)
		for _, dep := range dependents {
			inDegree[dep]--
			if inDegree[dep] == 0 {
				queue = append(queue, dep)
			}
		}
	}

	if len(visited) < len(projects) {
		fmt.Fprintf(os.Stderr, "warning: dependency cycle detected among projects\n")
		var remaining []string
		for p := range projects {
			if !visited[p] {
				remaining = append(remaining, p)
			}
		}
		sort.Strings(remaining)
		for _, p := range remaining {
			deps := sortedMapKeys(depMap[p])
			result = append(result, ProjectDep{Project: p, DependsOn: deps})
		}
	}

	return result
}

// ── git helpers ───────────────────────────────────────────────────────────────

func runGit(repoPath string, args ...string) (string, error) {
	cmdArgs := append([]string{"-C", repoPath}, args...)
	cmd := exec.Command("git", cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func runGitSilent(repoPath string, args ...string) string {
	out, _ := runGit(repoPath, args...)
	return out
}

// ── repo discovery ────────────────────────────────────────────────────────────

func findGitRepos(basePath string) []string {
	if isGitRepo(basePath) {
		return []string{basePath}
	}
	return shallowGitScan(basePath)
}

func findAllGitRepos(basePath string) []string {
	if isGitRepo(basePath) {
		return []string{basePath}
	}
	return shallowGitScan(basePath)
}

func findInitializedRepos(basePath string) []string {
	var repos []string
	for _, r := range findGitRepos(basePath) {
		if dirExists(filepath.Join(r, ".context")) {
			repos = append(repos, r)
		}
	}
	return repos
}

func shallowGitScan(basePath string) []string {
	var repos []string
	entries, err := os.ReadDir(basePath)
	if err != nil {
		return repos
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(basePath, e.Name())
		if isGitRepo(sub) {
			repos = append(repos, sub)
		}
	}
	return repos
}

func isGitRepo(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

// ── output formatters ─────────────────────────────────────────────────────────

func printInventoryTable(inv *Inventory) {
	fmt.Printf("Inventory: %s\n", inv.Repo)
	fmt.Printf("  Stack:     %s\n", strings.Join(inv.Stacks, ", "))
	fmt.Printf("  SHA:       %s\n", inv.GitSha)
	fmt.Printf("  Scanned:   %s\n", inv.ScannedAt)
	fmt.Printf("  Signals:   %d complexity  %d routes  %d messaging  %d triggers\n",
		len(inv.ComplexitySignals), len(inv.Routes), len(inv.MessagingSignals), len(inv.TriggerSignals))
	fmt.Printf("  Symbols:   %d types  %d DI mappings  %d endpoints  %d event flows\n",
		len(inv.TypeSymbols), len(inv.DIMappings), len(inv.Endpoints), len(inv.EventFlows))
	fmt.Printf("  Deps:      %d external\n", len(inv.ExternalDeps))
	fmt.Printf("  Entry pts: %s\n", strings.Join(inv.EntryPoints, ", "))

	if len(inv.ComplexitySignals) > 0 {
		fmt.Println("  Top complexity files:")
		limit := 10
		if len(inv.ComplexitySignals) < limit {
			limit = len(inv.ComplexitySignals)
		}
		for _, s := range inv.ComplexitySignals[:limit] {
			fmt.Printf("    [%d] %-60s %5d LOC  %s\n",
				s.Score, truncate(s.Path, 60), s.LOC, strings.Join(s.Signals, ", "))
		}
	}

	if len(inv.ProjectDeps) > 0 {
		fmt.Println("  Project dependencies:")
		for _, pd := range inv.ProjectDeps {
			if len(pd.DependsOn) == 0 {
				fmt.Printf("    %-40s (no dependencies)\n", pd.Project)
			} else {
				fmt.Printf("    %-40s → %s\n", pd.Project, strings.Join(pd.DependsOn, ", "))
			}
		}
	}
}

func printInventoryAgent(inv *Inventory) {
	fmt.Printf("repo=%s stacks=%s sha=%s signals=%d routes=%d messaging=%d types=%d di=%d endpoints=%d events=%d\n",
		inv.Repo, strings.Join(inv.Stacks, "+"), inv.GitSha,
		len(inv.ComplexitySignals), len(inv.Routes), len(inv.MessagingSignals),
		len(inv.TypeSymbols), len(inv.DIMappings), len(inv.Endpoints), len(inv.EventFlows))
}

func printIndexTable(entries []IndexEntry) {
	repoW, stackW := 9, 13
	for _, e := range entries {
		if n := len(e.Repo); n > repoW {
			repoW = n
		}
		if n := len(e.Stack); n > stackW {
			stackW = n
		}
	}

	fmt.Printf("%-*s  %-*s  %-11s  %-4s  %-8s  %s\n",
		repoW, "Repo", stackW, "Stack", "Inventory", "Maps", "Git SHA", "Status")
	fmt.Printf("%-*s  %-*s  %-11s  %-4s  %-8s  %s\n",
		repoW, strings.Repeat("-", repoW),
		stackW, strings.Repeat("-", stackW),
		"-----------", "----", "--------", "-------")

	for _, e := range entries {
		inv := dash(e.Inventory)
		stack := dash(e.Stack)
		sha := dash(e.GitSha)
		maps := fmt.Sprintf("%d", e.Maps)
		if e.Status == "NOT INITIALIZED" {
			maps = "—"
		}
		fmt.Printf("%-*s  %-*s  %-11s  %-4s  %-8s  %s\n",
			repoW, e.Repo, stackW, stack, inv, maps, sha, e.Status)
	}
}

// ── utility helpers ───────────────────────────────────────────────────────────

func parseSubArgs(fs *flag.FlagSet, args []string) (path string, err error) {
	if len(args) == 0 {
		return "", nil
	}
	if !strings.HasPrefix(args[0], "-") {
		path = args[0]
		err = fs.Parse(args[1:])
		return
	}
	err = fs.Parse(args)
	if fs.NArg() > 0 {
		path = fs.Arg(0)
	}
	return
}

func validatePath(p string) (string, error) {
	abs, err := filepath.Abs(filepath.Clean(p))
	if err != nil {
		return "", fmt.Errorf("invalid path %q: %w", p, err)
	}
	if _, statErr := os.Stat(abs); statErr != nil {
		return "", fmt.Errorf("path does not exist: %s", abs)
	}
	return abs, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileContainsCI(path, sub string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), strings.ToLower(sub))
}

func sliceContains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func sortedMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func uniqueSorted(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	sort.Strings(result)
	return result
}

func findFilesExt(root, ext string) []string {
	var result []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirNames[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == ext {
			result = append(result, path)
		}
		return nil
	})
	return result
}

func findFilesNamed(root, name string) []string {
	lowerName := strings.ToLower(name)
	var result []string
	_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			if skipDirNames[info.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(info.Name()) == lowerName {
			result = append(result, path)
		}
		return nil
	})
	return result
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return "..." + s[len(s)-maxLen+3:]
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func splitInheritance(s string) []string {
	var parts []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			depth++
		case '>':
			depth--
		case ',':
			if depth == 0 {
				parts = append(parts, strings.TrimSpace(s[start:i]))
				start = i + 1
			}
		}
	}
	parts = append(parts, strings.TrimSpace(s[start:]))
	return parts
}

func stripGeneric(s string) string {
	if idx := strings.Index(s, "<"); idx >= 0 {
		return strings.TrimSpace(s[:idx])
	}
	return strings.TrimSpace(s)
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ── help ─────────────────────────────────────────────────────────────────────

func printHelp() {
	fmt.Print(`repo-context — pre-compute repository analysis signals for AI agents

USAGE:
    repo-context <subcommand> [flags]

SUBCOMMANDS:
    init <path>           Initialize analysis protection for git repos in <path>
    inventory <path>      Deep signal scan; writes inventory.db + inventory-toc.json
    query <path>          Query inventory.db with SQL or predefined sections
    index <path>          Show status table of all repos in <path>
    version               Print version string
    help                  Show this help

FLAGS (init):
    --force               Re-initialize even if already done

FLAGS (inventory):
    --format string       Output format: json (default), table, agent
    --refresh             Re-scan only files changed since last inventory
    --force               Full rescan even if inventory is current
    --pull                Run git pull before scanning (explicit opt-in)

FLAGS (query):
    --sql string          Raw SQL query against inventory.db
    --section string      Predefined query alias (see below)
    --min-score int       Min complexity score for signals section (default 3)
    --format string       Output format: json (default), table, csv

QUERY SECTIONS:
    structure             Metadata, projects, deps, entry points, file counts
    signals               Complexity signals (filtered by --min-score)
    routes                Route patterns (HTTP endpoints, function triggers)
    messaging             Messaging signals (ServiceBus, EventHub, SignalR)
    triggers              Function trigger signals
    types                 Type symbols (classes, interfaces, structs, enums)
    di                    DI mappings (interface → implementation)
    endpoints             Detailed endpoint signatures
    events                Event flow topology (pub/sub across stacks)
    all                   All sections

FLAGS (index):
    --format string       Output format: table (default), json, agent

EXAMPLES:
    repo-context init ./repos
    repo-context inventory ./repos --format table
    repo-context inventory ./repos --refresh

    repo-context query ./repos/MyRepo --section signals --min-score 2
    repo-context query ./repos/MyRepo --section types
    repo-context query ./repos/MyRepo --section events
    repo-context query ./repos/MyRepo --sql "SELECT * FROM types WHERE kind = 'interface'"
    repo-context query ./repos/MyRepo --sql "SELECT t.name, t.file FROM types t JOIN type_implements ti ON t.name = ti.type_name WHERE ti.interface = 'IOrderService'"

    repo-context index ./repos

STORAGE:
    inventory.db          SQLite database (primary store, queryable)
    inventory-toc.json    Lightweight summary (~1-3KB, for quick agent reads)
`)
}
