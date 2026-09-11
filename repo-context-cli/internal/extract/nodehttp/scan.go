// Package nodehttp extracts Fastify-style routes and composition-root binds
// from TypeScript (Nest is not required). Heuristic: prefixes from
// app.register(plugin, { prefix }), handlers from container.field.execute,
// binds from AppContainer field types + `new Impl`.
package nodehttp

import (
	"regexp"
	"strings"
)

type Acc struct {
	registers []reg
	plugins   []plugin
	routes    []route
	fields    []field
	news      []news
}

type reg struct{ plugin, prefix, file string }
type plugin struct{ name, file string }
type route struct{ method, path, file, handler, auth string }
type field struct{ ident, typ, file string }
type news struct{ ident, impl, file string }

type Endpoint struct {
	Method, Route, Handler, File, Auth string
}

type DI struct {
	Interface, Implementation, File, Lifetime string
}

var (
	registerRe          = regexp.MustCompile("\\.register\\(\\s*(\\w+)\\s*,\\s*\\{[^}]*prefix:\\s*['\"`]([^'\"`]+)")
	routeRe             = regexp.MustCompile("\\b(?:app|fastify|server|instance)\\.(get|post|put|patch|delete|head|options)\\(\\s*['\"`]([^'\"`]*)['\"`]")
	executeRe  = regexp.MustCompile(`container\.(\w+)\.execute`)
	exportFnRe = regexp.MustCompile(`(?m)export\s+(?:async\s+)?function\s+(\w+)`)
	exportConstPluginRe = regexp.MustCompile(`(?m)export\s+const\s+(\w+)\s*:\s*FastifyPluginAsync`)
	containerIfaceRe    = regexp.MustCompile(`(?m)export\s+interface\s+(\w*Container)\s*\{`)
	fieldRe             = regexp.MustCompile(`(?m)^\s*(\w+)\s*:\s*([A-Z]\w*)\s*[;,]`)
	newRe               = regexp.MustCompile(`(?:const|let)\s+(\w+)\s*=\s*new\s+(\w+)\s*\(`)
	preHandlerRe        = regexp.MustCompile(`preHandler`)
)

func New() *Acc { return &Acc{} }

func (a *Acc) Scan(file, text string) {
	if a == nil {
		return
	}
	file = strings.ReplaceAll(file, "\\", "/")
	for _, m := range registerRe.FindAllStringSubmatch(text, -1) {
		a.registers = append(a.registers, reg{plugin: m[1], prefix: m[2], file: file})
	}
	routeHits := routeRe.FindAllStringSubmatchIndex(text, -1)
	if len(routeHits) > 0 {
		for _, m := range exportFnRe.FindAllStringSubmatch(text, -1) {
			a.plugins = append(a.plugins, plugin{name: m[1], file: file})
		}
		for _, m := range exportConstPluginRe.FindAllStringSubmatch(text, -1) {
			a.plugins = append(a.plugins, plugin{name: m[1], file: file})
		}
	}
	for i, loc := range routeHits {
		method := strings.ToUpper(text[loc[2]:loc[3]])
		path := text[loc[4]:loc[5]]
		end := len(text)
		if i+1 < len(routeHits) {
			end = routeHits[i+1][0]
		}
		if end-loc[1] > 1200 {
			end = loc[1] + 1200
		}
		body := text[loc[1]:end]
		handler := ""
		if em := executeRe.FindStringSubmatch(body); em != nil {
			handler = em[1]
		}
		auth := ""
		if preHandlerRe.MatchString(body) {
			auth = "preHandler"
		}
		a.routes = append(a.routes, route{method: method, path: path, file: file, handler: handler, auth: auth})
	}
	if idx := containerIfaceRe.FindStringIndex(text); idx != nil {
		chunk := text[idx[1]:]
		if close := strings.Index(chunk, "\n}"); close >= 0 {
			chunk = chunk[:close]
		} else if len(chunk) > 8000 {
			chunk = chunk[:8000]
		}
		for _, m := range fieldRe.FindAllStringSubmatch(chunk, -1) {
			a.fields = append(a.fields, field{ident: m[1], typ: m[2], file: file})
		}
	}
	if strings.Contains(file, "/composition/") || strings.HasSuffix(file, "/container.ts") {
		for _, m := range newRe.FindAllStringSubmatch(text, -1) {
			a.news = append(a.news, news{ident: m[1], impl: m[2], file: file})
		}
	}
}

func Flush(a *Acc) (eps []Endpoint, dis []DI) {
	if a == nil {
		return nil, nil
	}
	pluginFile := map[string]string{}
	for _, p := range a.plugins {
		pluginFile[p.name] = p.file
	}
	prefixes := map[string][]string{}
	for _, r := range a.registers {
		file := pluginFile[r.plugin]
		if file == "" {
			continue
		}
		prefixes[file] = appendUnique(prefixes[file], r.prefix)
	}
	fieldType := map[string]map[string]string{} // appRoot -> ident -> type
	implOf := map[string]map[string]string{}    // appRoot -> ident -> impl
	for _, f := range a.fields {
		root := appRoot(f.file)
		if fieldType[root] == nil {
			fieldType[root] = map[string]string{}
		}
		fieldType[root][f.ident] = f.typ
	}
	for _, n := range a.news {
		root := appRoot(n.file)
		if implOf[root] == nil {
			implOf[root] = map[string]string{}
		}
		implOf[root][n.ident] = n.impl
		typ := ""
		if fieldType[root] != nil {
			typ = fieldType[root][n.ident]
		}
		if typ == "" || typ == n.impl {
			continue
		}
		dis = append(dis, DI{
			Interface: typ, Implementation: n.impl, File: n.file, Lifetime: "singleton",
		})
	}
	for _, rt := range a.routes {
		prefs := prefixes[rt.file]
		if len(prefs) == 0 {
			prefs = []string{""}
		}
		handler := rt.handler
		root := appRoot(rt.file)
		if handler != "" && fieldType[root] != nil {
			if typ := fieldType[root][handler]; typ != "" {
				handler = typ
			} else if implOf[root] != nil {
				if impl := implOf[root][handler]; impl != "" {
					handler = impl
				}
			}
		}
		for _, p := range prefs {
			eps = append(eps, Endpoint{
				Method: rt.method, Route: joinRoute(p, rt.path), Handler: handler, File: rt.file, Auth: rt.auth,
			})
		}
	}
	return eps, dis
}

func joinRoute(prefix, path string) string {
	prefix = strings.TrimRight(prefix, "/")
	if path == "" || path == "/" {
		if prefix == "" {
			return "/"
		}
		return prefix
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return prefix + path
}

func appRoot(file string) string {
	file = strings.ReplaceAll(file, "\\", "/")
	parts := strings.Split(file, "/")
	if len(parts) >= 2 && (parts[0] == "apps" || parts[0] == "packages") {
		return parts[0] + "/" + parts[1]
	}
	return strings.Join(parts[:max(1, len(parts)-1)], "/")
}

func appendUnique(in []string, v string) []string {
	for _, x := range in {
		if x == v {
			return in
		}
	}
	return append(in, v)
}
