package model

import "strings"

const (
	KindModule   = "module"
	KindFile     = "file"
	KindType     = "type"
	KindMethod   = "method"
	KindEndpoint = "endpoint"
	KindEvent    = "event"
	KindSink     = "data_sink"
)

const (
	RelImports     = "imports"
	RelCalls       = "calls"
	RelImplements  = "implements"
	RelExtends     = "extends"
	RelBinds       = "binds"
	RelInjects     = "injects"
	RelPublishes   = "publishes"
	RelSubscribes  = "subscribes"
	RelAwaits      = "awaits"
	RelForks       = "forks"
	RelJoins       = "joins"
	RelRaces       = "races"
	RelReads       = "reads"
	RelWrites      = "writes"
	RelCarriesData = "carries_data"
	RelContains    = "contains"
	RelHandles     = "handles"
)

const (
	SrcHeuristic  = "heuristic"
	SrcTypeScript = "typescript"
	SrcRoslyn     = "roslyn"
)

const (
	ConfidenceHeuristic     = 0.55
	ConfidenceHeuristicCall = 0.40
	ConfidenceCompiler      = 0.92
	ConfidenceUnresolved    = 0.25
)

func Slash(p string) string {
	return strings.ReplaceAll(p, "\\", "/")
}

func FileID(lang, path string) string {
	return "file:" + lang + ":" + Slash(path)
}

func TypeID(lang, file, name string) string {
	return "type:" + lang + ":" + Slash(file) + ":" + name
}

func MethodID(lang, file, recv, name string) string {
	if recv == "" {
		return "method:" + lang + ":" + Slash(file) + ":" + name
	}
	return "method:" + lang + ":" + Slash(file) + ":" + recv + "." + name
}

func EndpointID(lang, file, method, route string) string {
	return "endpoint:" + lang + ":" + Slash(file) + ":" + strings.ToUpper(method) + ":" + route
}

func EventID(lang, file, mechanism, name string) string {
	return "event:" + lang + ":" + Slash(file) + ":" + mechanism + ":" + name
}

func SinkID(lang, file, name string) string {
	return "sink:" + lang + ":" + Slash(file) + ":" + name
}

func EdgeGroupKey(from, to, kind string) string {
	return from + "\x00" + to + "\x00" + kind
}
