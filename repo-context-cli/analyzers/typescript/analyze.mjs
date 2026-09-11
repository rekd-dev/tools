#!/usr/bin/env node
import fs from "node:fs";
import path from "node:path";
import ts from "typescript";

const repo = process.argv[2];
if (!repo) {
  emit({ kind: "coverage", status: "failed", message: "missing repo path" });
  process.exit(1);
}

const skipDirs = new Set([
  "node_modules",
  "dist",
  "build",
  ".git",
  "coverage",
  "bin",
  "obj",
  "vendor",
  "testdata",
  ".context",
]);

function emit(obj) {
  process.stdout.write(JSON.stringify({ v: 1, analyzer: "typescript", source: "typescript", ...obj }) + "\n");
}

function walk(dir, out = []) {
  let entries = [];
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return out;
  }
  for (const e of entries) {
    if (e.name.startsWith(".")) continue;
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (skipDirs.has(e.name)) continue;
      walk(full, out);
    } else if (/\.(ts|tsx|js|jsx)$/.test(e.name) && !e.name.endsWith(".d.ts") && !/\.(test|spec)\./.test(e.name)) {
      out.push(full);
    }
  }
  return out;
}

function rel(p) {
  return path.relative(repo, p).split(path.sep).join("/");
}

function typeId(file, name) {
  return `type:typescript:${file}:${name}`;
}
function methodId(file, recv, name) {
  return recv ? `method:typescript:${file}:${recv}.${name}` : `method:typescript:${file}:${name}`;
}
function fileId(file) {
  return `file:typescript:${file}`;
}

function loc(node, sf) {
  const pos = sf.getLineAndCharacterOfPosition(node.getStart(sf, false));
  return { line: pos.line + 1, column: pos.character + 1 };
}

function findTsconfigs(dir, out = []) {
  let entries = [];
  try {
    entries = fs.readdirSync(dir, { withFileTypes: true });
  } catch {
    return out;
  }
  for (const e of entries) {
    if (e.name.startsWith(".")) continue;
    const full = path.join(dir, e.name);
    if (e.isDirectory()) {
      if (skipDirs.has(e.name)) continue;
      findTsconfigs(full, out);
    } else if (e.name === "tsconfig.json") {
      out.push(full);
    }
  }
  return out;
}

function isSourceFile(p) {
  return /\.(ts|tsx|js|jsx)$/.test(p) && !p.endsWith(".d.ts") && !/\.(test|spec)\./.test(path.basename(p));
}

function projects(repo) {
  const fallbackOptions = {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.Bundler,
    jsx: ts.JsxEmit.ReactJSX,
    allowJs: true,
    skipLibCheck: true,
    noEmit: true,
    strict: false,
  };
  const seen = new Set();
  const out = [];
  for (const cfgPath of findTsconfigs(repo)) {
    const parsed = ts.getParsedCommandLineOfConfigFile(cfgPath, { skipLibCheck: true, noEmit: true }, {
      ...ts.sys,
      onUnRecoverableConfigFileDiagnostic() {},
    });
    if (!parsed) continue;
    const files = [];
    for (const f of parsed.fileNames) {
      if (!isSourceFile(f)) continue;
      const key = path.resolve(f);
      if (seen.has(key)) continue;
      seen.add(key);
      files.push(f);
    }
    if (files.length) out.push({ files, options: parsed.options });
  }
  if (!out.length) {
    const files = walk(repo);
    if (files.length) out.push({ files, options: fallbackOptions });
  }
  return out;
}

const allProjects = projects(repo);
const allFiles = allProjects.flatMap((p) => p.files);
if (!allFiles.length) {
  emit({ kind: "coverage", status: "ran", message: "no TypeScript files" });
  process.exit(0);
}

for (const proj of allProjects) {
  analyzeProject(proj.files, proj.options);
}

function analyzeProject(files, compilerOptions) {
  const host = ts.createCompilerHost(compilerOptions, true);
  const program = ts.createProgram({ rootNames: files, options: compilerOptions, host });
  const checker = program.getTypeChecker();
  const allowed = new Set(files.map((f) => path.resolve(f)));

  function resolveName(name, sf) {
    name = name.replace(/[<>].*$/, "");
    for (const source of program.getSourceFiles()) {
      if (source.isDeclarationFile) continue;
      const f = rel(source.fileName);
      const match = source.statements.find(
        (s) =>
          (ts.isClassDeclaration(s) || ts.isInterfaceDeclaration(s) || ts.isFunctionDeclaration(s)) &&
          s.name &&
          s.name.text === name,
      );
      if (match) return typeId(f, name);
    }
    const sym = checker.resolveName(name, sf.endOfFileToken, ts.SymbolFlags.Type, false);
    if (sym && sym.declarations && sym.declarations[0]) {
      const d = sym.declarations[0];
      const dsf = d.getSourceFile();
      if (!dsf.isDeclarationFile) return typeId(rel(dsf.fileName), name);
    }
    return null;
  }

  function resolveSymbolDecl(symbol) {
    if (!symbol) return null;
    if (symbol.flags & ts.SymbolFlags.Alias) {
      try {
        symbol = checker.getAliasedSymbol(symbol);
      } catch {
        return null;
      }
    }
    const decls = symbol.getDeclarations();
    return decls?.length ? decls[0] : null;
  }

  function enclosingTypeName(node) {
    let cur = node;
    while (cur) {
      if ((ts.isClassDeclaration(cur) || ts.isInterfaceDeclaration(cur)) && cur.name) {
        return cur.name.text;
      }
      cur = cur.parent;
    }
    return "";
  }

  function idFromDeclaration(decl) {
    const dsf = decl.getSourceFile();
    if (dsf.isDeclarationFile) return null;
    const declFile = rel(dsf.fileName);

    if (ts.isMethodDeclaration(decl) || ts.isMethodSignature(decl)) {
      const recv = enclosingTypeName(decl);
      const name = decl.name?.text;
      if (!name) return null;
      return { id: methodId(declFile, recv, name), file: declFile, name, recv };
    }
    if (ts.isFunctionDeclaration(decl) && decl.name) {
      return { id: methodId(declFile, "", decl.name.text), file: declFile, name: decl.name.text, recv: "" };
    }
    if (ts.isVariableDeclaration(decl) && ts.isIdentifier(decl.name)) {
      const init = decl.initializer;
      if (init && (ts.isArrowFunction(init) || ts.isFunctionExpression(init))) {
        return { id: methodId(declFile, "", decl.name.text), file: declFile, name: decl.name.text, recv: "" };
      }
    }
    if (ts.isPropertyAssignment(decl) && ts.isIdentifier(decl.name)) {
      const init = decl.initializer;
      if (init && (ts.isArrowFunction(init) || ts.isFunctionExpression(init))) {
        const recv = enclosingTypeName(decl.parent?.parent) || "";
        return { id: methodId(declFile, recv, decl.name.text), file: declFile, name: decl.name.text, recv };
      }
    }
    if ((ts.isPropertyDeclaration(decl) || ts.isPropertySignature(decl)) && ts.isIdentifier(decl.name)) {
      const init = decl.initializer;
      const fnInit = init && (ts.isArrowFunction(init) || ts.isFunctionExpression(init));
      const fnType = decl.type && ts.isFunctionTypeNode(decl.type);
      if (fnInit || fnType) {
        const recv = enclosingTypeName(decl);
        return { id: methodId(declFile, recv, decl.name.text), file: declFile, name: decl.name.text, recv };
      }
    }
    return null;
  }

  function memberNameText(member) {
    const n = member.name;
    if (!n) return null;
    if (ts.isIdentifier(n) || ts.isPrivateIdentifier(n)) return n.text;
    if (ts.isStringLiteral(n) || ts.isNumericLiteral(n)) return n.text;
    return null;
  }

  function isCallableTypeMember(member) {
    if (ts.isMethodDeclaration(member) || ts.isMethodSignature(member)) return true;
    if (ts.isPropertyDeclaration(member) || ts.isPropertySignature(member)) {
      const init = member.initializer;
      if (init && (ts.isArrowFunction(init) || ts.isFunctionExpression(init))) return true;
      if (member.type && ts.isFunctionTypeNode(member.type)) return true;
    }
    return false;
  }

  function callableWalkRoot(member) {
    if (ts.isMethodDeclaration(member) || ts.isFunctionDeclaration(member)) return member;
    if (ts.isPropertyDeclaration(member)) {
      const init = member.initializer;
      if (init && (ts.isArrowFunction(init) || ts.isFunctionExpression(init))) return init;
    }
    return null;
  }

  function emitMethodDeclarationNode(ownerId, ownerName, member, file, sf) {
    if (!isCallableTypeMember(member)) return null;
    const name = memberNameText(member);
    if (!name) return null;
    const id = methodId(file, ownerName, name);
    const { line, column } = loc(member, sf);
    emit({
      kind: "node",
      id,
      nodeKind: "method",
      name,
      language: "typescript",
      file,
      line,
      column,
    });
    emit({ kind: "edge", from: ownerId, to: id, edgeKind: "contains", confidence: 0.92, file, line });
    const body = callableWalkRoot(member);
    if (body) walkCalls(body, id, file, sf);
    return { id, name, line };
  }

  function collectCallableMembers(typeDecl) {
    const map = new Map();
    if (!typeDecl.name) return map;
    const tfile = rel(typeDecl.getSourceFile().fileName);
    const tname = typeDecl.name.text;
    for (const member of typeDecl.members ?? []) {
      if (!isCallableTypeMember(member)) continue;
      const name = memberNameText(member);
      if (!name) continue;
      map.set(name, { id: methodId(tfile, tname, name), name });
    }
    return map;
  }

  function declarationFromHeritageType(typeNode, sf) {
    const type = checker.getTypeAtLocation(typeNode.expression);
    const decl = resolveSymbolDecl(type.getSymbol() ?? type.aliasSymbol);
    if (decl && (ts.isInterfaceDeclaration(decl) || ts.isClassDeclaration(decl))) {
      const dsf = decl.getSourceFile();
      if (!dsf.isDeclarationFile) return decl;
    }
    const tname = typeNode.expression.getText(sf).replace(/[<>].*$/, "");
    for (const source of program.getSourceFiles()) {
      if (source.isDeclarationFile) continue;
      const match = source.statements.find(
        (s) =>
          (ts.isClassDeclaration(s) || ts.isInterfaceDeclaration(s)) &&
          s.name &&
          s.name.text === tname,
      );
      if (match) return match;
    }
    return null;
  }

  function resolveCallTarget(node, sf, callerFile) {
    const detail = node.expression.getText(sf);

    const sig = checker.getResolvedSignature(node);
    if (sig) {
      const decl = sig.getDeclaration();
      if (decl) {
        const resolved = idFromDeclaration(decl);
        if (resolved) return { ...resolved, confidence: 0.92, unresolved: false, detail };
      }
    }

    const expr = node.expression;
    const symbol = ts.isPropertyAccessExpression(expr)
      ? checker.getSymbolAtLocation(expr.name)
      : checker.getSymbolAtLocation(expr);
    const symDecl = resolveSymbolDecl(symbol);
    if (symDecl) {
      const resolved = idFromDeclaration(symDecl);
      if (resolved) return { ...resolved, confidence: 0.9, unresolved: false, detail };
    }

    const name = detail.split(".").pop();
    const recv = detail.includes(".") ? detail.split(".")[0] : "";
    const fallbackRecv = recv && /^[A-Z]/.test(recv) ? recv : "";
    return {
      id: methodId(callerFile, fallbackRecv, name),
      file: callerFile,
      name,
      recv: fallbackRecv,
      confidence: 0.25,
      unresolved: true,
      detail,
    };
  }

  function persistVerb(name) {
    const n = String(name || "").split(".").pop();
    if (/prepare|transaction/i.test(n)) return "";
    if (/^(insert|update|upsert|delete|save|set|run|exec)/i.test(n)) return "writes";
    if (/^(find|get|list|count|select|all|query)/i.test(n)) return "reads";
    return "";
  }

  function methodClass(fromId) {
    const tail = String(fromId || "").split(":").pop() || "";
    const dot = tail.lastIndexOf(".");
    if (dot <= 0) return "";
    return tail.slice(0, dot);
  }

  function posixFile(file) {
    return String(file || "").replaceAll("\\", "/");
  }

  function isPortFile(file) {
    const f = posixFile(file);
    return /\/ports\//i.test(f);
  }

  function isInfrastructureFile(file) {
    const f = posixFile(file);
    return /\/infrastructure\//i.test(f) || /\/persistence\//i.test(f);
  }

  function isSqliteReceiver(node) {
    if (!ts.isPropertyAccessExpression(node.expression)) return false;
    const type = checker.getTypeAtLocation(node.expression.expression);
    const typeName = type.getSymbol()?.getName() ?? checker.typeToString(type);
    return /Database|Statement|BetterSqlite3/i.test(typeName);
  }

  function shouldEmitSink(from, callerFile, node) {
    if (isPortFile(callerFile)) return false;
    if (isInfrastructureFile(callerFile) || isSqliteReceiver(node)) return true;
    const cls = methodClass(from);
    return /Repository|Store|Repo|Database/i.test(cls);
  }

  function adapterSink(callerFile, from) {
    const cls = methodClass(from) || "sqlite";
    return { id: `sink:typescript:${callerFile}:${cls}`, name: cls };
  }

  const emittedSinks = new Set();

  function walkCalls(fn, from, file, sf) {
    const visitCalls = (node) => {
      if (ts.isCallExpression(node)) {
        const { line } = loc(node, sf);
        const text = node.expression.getText(sf);
        if (text === "Promise.all" || text === "Promise.allSettled") {
          emit({ kind: "edge", from, to: from, edgeKind: "forks", confidence: 0.92, file, line, detail: text });
        } else if (text === "Promise.race") {
          emit({ kind: "edge", from, to: from, edgeKind: "races", confidence: 0.92, file, line, detail: text });
        } else {
          const target = resolveCallTarget(node, sf, file);
          emit({
            kind: "edge",
            from,
            to: target.id,
            edgeKind: "calls",
            confidence: target.confidence,
            unresolved: target.unresolved,
            file,
            line,
            detail: target.detail,
          });
          const verb = persistVerb(target.name);
          if (verb && shouldEmitSink(from, file, node)) {
            const sink = adapterSink(file, from);
            if (!emittedSinks.has(sink.id)) {
              emittedSinks.add(sink.id);
              emit({ kind: "node", id: sink.id, nodeKind: "data_sink", name: sink.name, language: "typescript", file, line });
            }
            emit({
              kind: "edge",
              from,
              to: sink.id,
              edgeKind: verb,
              confidence: target.confidence,
              unresolved: false,
              file,
              line,
              detail: "symbol-level lineage",
            });
            if (verb === "writes") {
              emit({
                kind: "edge",
                from,
                to: sink.id,
                edgeKind: "carries_data",
                confidence: target.confidence,
                unresolved: false,
                file,
                line,
              });
            }
          }
        }
        if (ts.isPropertyAccessExpression(node.expression) && node.expression.name.text === "then" && !ts.isAwaitExpression(node.parent)) {
          emit({ kind: "edge", from, to: from, edgeKind: "calls", confidence: 0.4, unresolved: true, file, line, detail: "likely fire-and-forget" });
        }
      }
      if (ts.isAwaitExpression(node)) {
        const { line } = loc(node, sf);
        emit({ kind: "edge", from, to: from, edgeKind: "awaits", confidence: 0.92, file, line, detail: node.getText(sf).slice(0, 80) });
      }
      ts.forEachChild(node, visitCalls);
    };
    visitCalls(fn);
  }

  for (const sf of program.getSourceFiles()) {
    if (sf.isDeclarationFile) continue;
    if (!allowed.has(path.resolve(sf.fileName))) continue;
    const file = rel(sf.fileName);
    emit({ kind: "node", id: fileId(file), nodeKind: "file", name: path.basename(file), language: "typescript", file });

    const visit = (node) => {
      if (ts.isClassDeclaration(node) && node.name) {
        const name = node.name.text;
        const id = typeId(file, name);
        const { line, column } = loc(node, sf);
        emit({
          kind: "node",
          id,
          nodeKind: "type",
          name,
          qualifiedName: name,
          language: "typescript",
          file,
          line,
          column,
          extra: { typeKind: "class" },
        });
        emit({ kind: "edge", from: fileId(file), to: id, edgeKind: "contains", confidence: 0.92, file, line });
        for (const h of node.heritageClauses ?? []) {
          const kind = h.token === ts.SyntaxKind.ImplementsKeyword ? "implements" : "extends";
          for (const t of h.types) {
            const tname = t.expression.getText(sf);
            const to = resolveName(tname, sf) ?? typeId(file, tname);
            emit({
              kind: "edge",
              from: id,
              to,
              edgeKind: kind,
              confidence: resolveName(tname, sf) ? 0.92 : 0.25,
              unresolved: !resolveName(tname, sf),
              file,
              line,
              detail: kind,
            });
          }
        }
        const classMethods = [];
        for (const member of node.members) {
          if (ts.isConstructorDeclaration(member)) {
            for (const p of member.parameters) {
              const pname = p.name.getText(sf);
              if (!p.type) continue;
              const tname = p.type.getText(sf).replace(/[<>].*$/, "");
              const to = resolveName(tname, sf);
              if (!to) continue;
              emit({
                kind: "edge",
                from: id,
                to,
                edgeKind: "injects",
                confidence: 0.9,
                file,
                line: loc(member, sf).line,
                detail: "ctor " + pname,
              });
            }
          }
          const declared = emitMethodDeclarationNode(id, name, member, file, sf);
          if (declared) classMethods.push(declared);
        }
        for (const h of node.heritageClauses ?? []) {
          if (h.token !== ts.SyntaxKind.ImplementsKeyword) continue;
          for (const t of h.types) {
            const ifaceDecl = declarationFromHeritageType(t, sf);
            if (!ifaceDecl || !ts.isInterfaceDeclaration(ifaceDecl)) continue;
            const ifaceMethods = collectCallableMembers(ifaceDecl);
            for (const cm of classMethods) {
              const target = ifaceMethods.get(cm.name);
              if (!target) continue;
              emit({
                kind: "edge",
                from: cm.id,
                to: target.id,
                edgeKind: "implements",
                confidence: 0.92,
                file,
                line: cm.line,
                detail: "implements",
              });
            }
          }
        }
      }
      if (ts.isInterfaceDeclaration(node) && node.name) {
        const name = node.name.text;
        const id = typeId(file, name);
        const { line, column } = loc(node, sf);
        emit({
          kind: "node",
          id,
          nodeKind: "type",
          name,
          qualifiedName: name,
          language: "typescript",
          file,
          line,
          column,
          abstract: true,
          extra: { typeKind: "interface" },
        });
        emit({ kind: "edge", from: fileId(file), to: id, edgeKind: "contains", confidence: 0.92, file, line });
        for (const member of node.members) {
          emitMethodDeclarationNode(id, name, member, file, sf);
        }
      }
      if (ts.isFunctionDeclaration(node) && node.name) {
        const name = node.name.text;
        const id = methodId(file, "", name);
        const { line, column } = loc(node, sf);
        emit({
          kind: "node",
          id,
          nodeKind: "method",
          name,
          language: "typescript",
          file,
          line,
          column,
        });
        emit({ kind: "edge", from: fileId(file), to: id, edgeKind: "contains", confidence: 0.92, file, line });
        walkCalls(node, id, file, sf);
      }
      ts.forEachChild(node, visit);
    };
    visit(sf);
  }
}

emit({ kind: "coverage", status: "ran", message: `${allFiles.length} files` });
