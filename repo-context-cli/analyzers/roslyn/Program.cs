using System.Text.Json;
using Microsoft.CodeAnalysis;
using Microsoft.CodeAnalysis.CSharp;
using Microsoft.CodeAnalysis.CSharp.Syntax;

if (args.Length == 0)
{
    Emit(new { v = 1, kind = "coverage", status = "failed", message = "missing repo path" });
    return 1;
}

var repo = Path.GetFullPath(args[0]);
if (!Directory.Exists(repo))
{
    Emit(new { v = 1, kind = "coverage", status = "failed", message = "repo not found: " + repo });
    return 1;
}
var skip = new HashSet<string>(StringComparer.OrdinalIgnoreCase)
{
    "bin", "obj", ".git", "node_modules", "packages", "dist", "vendor", "testdata"
};

var files = new List<string>();
void Walk(string dir)
{
    foreach (var d in Directory.GetDirectories(dir))
    {
        if (skip.Contains(Path.GetFileName(d))) continue;
        Walk(d);
    }
    files.AddRange(Directory.GetFiles(dir, "*.cs"));
}
Walk(repo);

if (files.Count == 0)
{
    Emit(new { v = 1, kind = "coverage", analyzer = "roslyn", status = "ran", message = "no C# files" });
    return 0;
}

string Rel(string p) => Path.GetRelativePath(repo, p).Replace('\\', '/');

var trees = files.Select(f => CSharpSyntaxTree.ParseText(File.ReadAllText(f), path: f)).ToList();
var compilation = CSharpCompilation.Create(
    "analysis",
    trees,
    new[] { MetadataReference.CreateFromFile(typeof(object).Assembly.Location) },
    new CSharpCompilationOptions(OutputKind.DynamicallyLinkedLibrary));

string TypeId(string file, string name) => $"type:csharp:{file}:{name}";
string MethodId(string file, string recv, string name) =>
    string.IsNullOrEmpty(recv) ? $"method:csharp:{file}:{name}" : $"method:csharp:{file}:{recv}.{name}";
string FileId(string file) => $"file:csharp:{file}";

var declared = new Dictionary<string, string>(StringComparer.Ordinal);
foreach (var tree in trees)
{
    var file = Rel(tree.FilePath ?? "");
    var root = tree.GetRoot();
    foreach (var t in root.DescendantNodes().OfType<BaseTypeDeclarationSyntax>())
    {
        var name = t.Identifier.Text;
        declared[name] = TypeId(file, name);
    }
}

foreach (var tree in trees)
{
    var file = Rel(tree.FilePath ?? "");
    var model = compilation.GetSemanticModel(tree);
    var root = tree.GetRoot();
    Emit(new { v = 1, kind = "node", id = FileId(file), nodeKind = "file", name = Path.GetFileName(file), language = "csharp", file });

    foreach (var t in root.DescendantNodes().OfType<BaseTypeDeclarationSyntax>())
    {
        var name = t.Identifier.Text;
        var id = TypeId(file, name);
        var line = t.GetLocation().GetLineSpan().StartLinePosition.Line + 1;
        var isIface = t is InterfaceDeclarationSyntax;
        Emit(new
        {
            v = 1,
            kind = "node",
            id,
            nodeKind = "type",
            name,
            qualifiedName = name,
            language = "csharp",
            file,
            line,
            abstract_ = isIface,
            extra = new Dictionary<string, string> { ["typeKind"] = isIface ? "interface" : "class" }
        });
        Emit(new { v = 1, kind = "edge", from = FileId(file), to = id, edgeKind = "contains", source = "roslyn", confidence = 0.92, file, line });

        if (t.BaseList != null)
        {
            foreach (var b in t.BaseList.Types)
            {
                var bname = b.Type.ToString().Split('<')[0];
                declared.TryGetValue(bname, out var to);
                var unresolved = to == null;
                to ??= TypeId(file, bname);
                var kind = bname.StartsWith("I") && bname.Length > 1 && char.IsUpper(bname[1]) ? "implements" : "extends";
                if (t is InterfaceDeclarationSyntax) kind = "extends";
                if (isIface == false && b.Type is IdentifierNameSyntax idn && idn.Identifier.Text.StartsWith("I"))
                    kind = "implements";
                Emit(new
                {
                    v = 1,
                    kind = "edge",
                    from = id,
                    to,
                    edgeKind = kind,
                    source = "roslyn",
                    confidence = unresolved ? 0.25 : 0.92,
                    unresolved,
                    file,
                    line,
                    detail = kind
                });
            }
        }

        foreach (var ctor in t.DescendantNodes().OfType<ConstructorDeclarationSyntax>())
        {
            foreach (var p in ctor.ParameterList.Parameters)
            {
                var ptype = p.Type?.ToString().Split('<')[0] ?? "";
                if (!declared.TryGetValue(ptype, out var to)) continue;
                Emit(new
                {
                    v = 1,
                    kind = "edge",
                    from = id,
                    to,
                    edgeKind = "injects",
                    source = "roslyn",
                    confidence = 0.92,
                    file,
                    line = ctor.GetLocation().GetLineSpan().StartLinePosition.Line + 1,
                    detail = "ctor " + p.Identifier.Text
                });
            }
        }
    }

    foreach (var method in root.DescendantNodes().OfType<MethodDeclarationSyntax>())
    {
        var typeName = method.Ancestors().OfType<BaseTypeDeclarationSyntax>().FirstOrDefault()?.Identifier.Text ?? "";
        var from = string.IsNullOrEmpty(typeName) ? FileId(file) : TypeId(file, typeName);
        var mid = MethodId(file, typeName, method.Identifier.Text);
        var line = method.GetLocation().GetLineSpan().StartLinePosition.Line + 1;
        Emit(new { v = 1, kind = "node", id = mid, nodeKind = "method", name = typeName + "." + method.Identifier.Text, language = "csharp", file, line });
        Emit(new { v = 1, kind = "edge", from, to = mid, edgeKind = "contains", source = "roslyn", confidence = 0.92, file, line });

        foreach (var inv in method.DescendantNodes().OfType<InvocationExpressionSyntax>())
        {
            var iline = inv.GetLocation().GetLineSpan().StartLinePosition.Line + 1;
            var text = inv.Expression.ToString();
            if (text.Contains("Task.WhenAll"))
                Emit(new { v = 1, kind = "edge", from = mid, to = mid, edgeKind = "forks", source = "roslyn", confidence = 0.92, file, line = iline, detail = "Task.WhenAll" });
            else if (text.Contains("Task.WhenAny"))
                Emit(new { v = 1, kind = "edge", from = mid, to = mid, edgeKind = "races", source = "roslyn", confidence = 0.92, file, line = iline, detail = "Task.WhenAny" });
            else
            {
                var name = text.Contains('.') ? text.Split('.').Last() : text;
                if (name is "Save" or "Insert" or "Update" or "Execute")
                {
                    var sink = $"sink:csharp:{file}:{name}";
                    Emit(new { v = 1, kind = "node", id = sink, nodeKind = "data_sink", name, language = "csharp", file, line = iline });
                    Emit(new { v = 1, kind = "edge", from = mid, to = sink, edgeKind = "writes", source = "roslyn", confidence = 0.7, file, line = iline });
                    Emit(new { v = 1, kind = "edge", from = mid, to = sink, edgeKind = "carries_data", source = "roslyn", confidence = 0.7, file, line = iline, detail = "symbol-level lineage" });
                }
                Emit(new { v = 1, kind = "edge", from = mid, to = mid, edgeKind = "calls", source = "roslyn", confidence = 0.7, file, line = iline, detail = text });
            }
        }
        foreach (var aw in method.DescendantNodes().OfType<AwaitExpressionSyntax>())
        {
            var iline = aw.GetLocation().GetLineSpan().StartLinePosition.Line + 1;
            Emit(new { v = 1, kind = "edge", from = mid, to = mid, edgeKind = "awaits", source = "roslyn", confidence = 0.92, file, line = iline });
        }
        if (method.ToFullString().Contains(".Result") || method.ToFullString().Contains(".Wait(") || method.ToFullString().Contains("GetAwaiter().GetResult()"))
        {
            Emit(new { v = 1, kind = "edge", from = mid, to = mid, edgeKind = "awaits", source = "roslyn", confidence = 0.8, file, line, detail = "sync-over-async" });
        }
        var hasCancel = method.ParameterList.Parameters.Any(p => (p.Type?.ToString() ?? "").Contains("CancellationToken"));
        if (method.ReturnType.ToString().Contains("Task") && !hasCancel)
        {
            Emit(new { v = 1, kind = "edge", from = mid, to = mid, edgeKind = "awaits", source = "roslyn", confidence = 0.4, unresolved = true, file, line, detail = "async method without CancellationToken" });
        }
    }

    foreach (var inv in root.DescendantNodes().OfType<InvocationExpressionSyntax>())
    {
        var text = inv.ToString();
        var m = System.Text.RegularExpressions.Regex.Match(text, @"Add(Scoped|Transient|Singleton)<(\w+)(?:,\s*(\w+))?>");
        if (!m.Success) continue;
        var lifetime = m.Groups[1].Value.ToLowerInvariant();
        var iface = m.Groups[2].Value;
        var impl = m.Groups[3].Success ? m.Groups[3].Value : iface;
        if (!declared.TryGetValue(iface, out var ifaceId) || !declared.TryGetValue(impl, out var implId)) continue;
        var line = inv.GetLocation().GetLineSpan().StartLinePosition.Line + 1;
        Emit(new { v = 1, kind = "edge", from = implId, to = ifaceId, edgeKind = "binds", source = "roslyn", confidence = 0.92, file, line, detail = "lifetime=" + lifetime });
    }
}

Emit(new { v = 1, kind = "coverage", analyzer = "roslyn", status = "ran", message = files.Count + " files" });
return 0;

static void Emit(object obj)
{
    var json = JsonSerializer.Serialize(obj, new JsonSerializerOptions { PropertyNamingPolicy = JsonNamingPolicy.CamelCase, DefaultIgnoreCondition = System.Text.Json.Serialization.JsonIgnoreCondition.WhenWritingNull });
    json = json.Replace("\"abstract_\"", "\"abstract\"");
    Console.WriteLine(json);
}
