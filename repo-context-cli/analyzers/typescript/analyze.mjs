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

const files = walk(repo);
if (files.length === 0) {
  emit({ kind: "coverage", status: "ran", message: "no TypeScript files" });
  process.exit(0);
}

const compilerOptions = {
  target: ts.ScriptTarget.ES2022,
  module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.Bundler,
  jsx: ts.JsxEmit.ReactJSX,
  allowJs: true,
  skipLibCheck: true,
  noEmit: true,
  strict: false,
};

const host = ts.createCompilerHost(compilerOptions, true);
const program = ts.createProgram({ rootNames: files, options: compilerOptions, host });
const checker = program.getTypeChecker();

function loc(node, sf) {
  const pos = sf.getLineAndCharacterOfPosition(node.getStart(sf, false));
  return { line: pos.line + 1, column: pos.character + 1 };
}

for (const sf of program.getSourceFiles()) {
  if (sf.isDeclarationFile) continue;
  if (!files.includes(sf.fileName) && !files.some((f) => path.resolve(f) === path.resolve(sf.fileName))) continue;
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
      for (const member of node.members) {
        if (ts.isConstructorDeclaration(node)) {
          /* skip */
        }
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
        if ((ts.isMethodDeclaration(member) || ts.isFunctionDeclaration(member)) && member.name) {
          walkCalls(member, id, file, sf);
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

function walkCalls(fn, from, file, sf) {
  const visit = (node) => {
    if (ts.isCallExpression(node)) {
      const { line } = loc(node, sf);
      const text = node.expression.getText(sf);
      if (text === "Promise.all" || text === "Promise.allSettled") {
        emit({ kind: "edge", from, to: from, edgeKind: "forks", confidence: 0.92, file, line, detail: text });
      } else if (text === "Promise.race") {
        emit({ kind: "edge", from, to: from, edgeKind: "races", confidence: 0.92, file, line, detail: text });
      } else {
        const name = text.split(".").pop();
        const recv = text.includes(".") ? text.split(".")[0] : "";
        const to = methodId(file, recv && /^[A-Z]/.test(recv) ? recv : "", name);
        emit({
          kind: "node",
          id: to,
          nodeKind: "method",
          name: text,
          language: "typescript",
          file,
          line,
        });
        emit({ kind: "edge", from, to, edgeKind: "calls", confidence: 0.7, file, line, detail: text });
        if (/^(save|insert|update|execute|query|fetch)$/i.test(name)) {
          const sink = `sink:typescript:${file}:${name}`;
          emit({ kind: "node", id: sink, nodeKind: "data_sink", name, language: "typescript", file, line });
          emit({ kind: "edge", from, to: sink, edgeKind: "writes", confidence: 0.7, file, line, detail: "symbol-level lineage" });
          emit({ kind: "edge", from, to: sink, edgeKind: "carries_data", confidence: 0.7, file, line });
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
    ts.forEachChild(node, visit);
  };
  visit(fn);
}

emit({ kind: "coverage", status: "ran", message: `${files.length} files` });
