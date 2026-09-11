#!/usr/bin/env node
/**
 * Graph quality P0-1 / P0-2.
 * Run from analyzers/typescript: npm test
 * (or: node test/p0-graph-quality.mjs)
 */
import { spawnSync } from "node:child_process";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const analyzer = path.join(here, "..", "analyze.mjs");
const fixture = path.join(here, "..", "fixtures", "ports-usecase");

const result = spawnSync(process.execPath, [analyzer, fixture], {
  encoding: "utf8",
  cwd: path.join(here, ".."),
});

if (result.status !== 0) {
  process.stderr.write(result.stderr || result.stdout || "analyzer failed\n");
  process.exit(result.status || 1);
}

const recs = result.stdout
  .split(/\r?\n/)
  .filter(Boolean)
  .map((line, i) => {
    try {
      return JSON.parse(line);
    } catch (err) {
      throw new Error(`NDJSON parse error on line ${i + 1}: ${err.message}\n${line}`);
    }
  });

const nodes = recs.filter((r) => r.kind === "node");
const edges = recs.filter((r) => r.kind === "edge");
const failures = [];

function idEnds(id, suffix) {
  return typeof id === "string" && id.replaceAll("\\", "/").endsWith(suffix);
}

function findMethodNode(suffix) {
  return nodes.find((n) => n.nodeKind === "method" && idEnds(n.id, suffix));
}

function findEdge(kind, fromSuffix, toSuffix) {
  return edges.find(
    (e) =>
      e.edgeKind === kind &&
      idEnds(e.from, fromSuffix) &&
      idEnds(e.to, toSuffix),
  );
}

const executeNode = findMethodNode("ClockInUseCase.execute");
if (!executeNode) failures.push("missing method node ClockInUseCase.execute");
else if (executeNode.name !== "execute") {
  failures.push(`ClockInUseCase.execute name should be short 'execute', got ${JSON.stringify(executeNode.name)}`);
}

const sqliteInsertNode = findMethodNode("SqliteTimeSessionRepository.insertSession");
if (!sqliteInsertNode) failures.push("missing declaration node SqliteTimeSessionRepository.insertSession");
else if (sqliteInsertNode.name !== "insertSession") {
  failures.push(`sqlite insertSession name should be short, got ${JSON.stringify(sqliteInsertNode.name)}`);
}

const portInsertNode = findMethodNode("TimeSessionRepository.insertSession");
if (!portInsertNode) failures.push("missing interface method node TimeSessionRepository.insertSession");

const callsToPort = findEdge("calls", "ClockInUseCase.execute", "TimeSessionRepository.insertSession");
const callsToSqlite = findEdge("calls", "ClockInUseCase.execute", "SqliteTimeSessionRepository.insertSession");
if (!callsToPort && !callsToSqlite) {
  failures.push("missing calls from ClockInUseCase.execute to insertSession (port or sqlite impl)");
}

const implEdge = findEdge(
  "implements",
  "SqliteTimeSessionRepository.insertSession",
  "TimeSessionRepository.insertSession",
);
if (!implEdge) {
  failures.push("missing implements from SqliteTimeSessionRepository.insertSession to TimeSessionRepository.insertSession");
} else if (implEdge.confidence !== 0.92) {
  failures.push(`implements confidence expected 0.92, got ${implEdge.confidence}`);
}

const clockTypeId = nodes.find(
  (n) => n.nodeKind === "type" && n.name === "ClockInUseCase",
)?.id;
if (clockTypeId) {
  const classCalls = edges.filter((e) => e.edgeKind === "calls" && e.from === clockTypeId);
  if (classCalls.length) {
    failures.push(`class-level calls from ClockInUseCase type id: ${JSON.stringify(classCalls)}`);
  }
} else {
  failures.push("missing ClockInUseCase type node");
}

const phantom = nodes.filter(
  (n) =>
    n.nodeKind === "method" &&
    (n.name?.includes(".") || /INSERT\s+INTO/i.test(n.name ?? "")),
);
if (phantom.length) {
  failures.push(`phantom call-site method nodes: ${phantom.map((n) => `${n.id} name=${n.name}`).join("; ")}`);
}

const adapterSink = nodes.find(
  (n) => n.nodeKind === "data_sink" && idEnds(n.id, "SqliteTimeSessionRepository"),
);
if (!adapterSink) {
  failures.push("missing one data_sink for SqliteTimeSessionRepository");
} else if (adapterSink.name !== "SqliteTimeSessionRepository") {
  failures.push(`adapter sink name should be the class, got ${JSON.stringify(adapterSink.name)}`);
}

const writeFromInsert = findEdge("writes", "SqliteTimeSessionRepository.insertSession", "SqliteTimeSessionRepository");
if (!writeFromInsert) failures.push("insertSession should write to the adapter sink");

const readFromFind = findEdge("reads", "SqliteTimeSessionRepository.findById", "SqliteTimeSessionRepository");
if (!readFromFind) failures.push("findById should read the adapter sink, not write");

const writeFromFind = findEdge("writes", "SqliteTimeSessionRepository.findById", "SqliteTimeSessionRepository");
if (writeFromFind) failures.push("findById must not emit writes");

const perMethodSinks = nodes.filter(
  (n) => n.nodeKind === "data_sink" && (idEnds(n.id, ":run") || idEnds(n.id, ":get") || idEnds(n.id, ":findById") || idEnds(n.id, ":insertSession")),
);
if (perMethodSinks.length) {
  failures.push(`per-method sinks still present: ${perMethodSinks.map((n) => n.id).join("; ")}`);
}

const ucWrite = edges.find(
  (e) => e.edgeKind === "writes" && idEnds(e.from, "ClockInUseCase.execute"),
);
if (ucWrite) failures.push("use case execute should not emit writes; adapters own the sink");

const sinkCount = nodes.filter((n) => n.nodeKind === "data_sink").length;
if (sinkCount !== 1) {
  failures.push(`expected one adapter sink, got ${sinkCount}: ${nodes.filter((n) => n.nodeKind === "data_sink").map((n) => n.id).join("; ")}`);
}

if (failures.length) {
  for (const f of failures) console.error("FAIL:", f);
  console.error("\nSample method nodes:");
  for (const n of nodes.filter((x) => x.nodeKind === "method")) {
    console.error(" ", n.id, "name=", n.name);
  }
  console.error("\nSample calls/implements:");
  for (const e of edges.filter((x) => x.edgeKind === "calls" || x.edgeKind === "implements")) {
    console.error(" ", e.edgeKind, e.from, "->", e.to, e.unresolved ? "unresolved" : "");
  }
  process.exit(1);
}

console.log("ok p0-graph-quality");
console.log("  node", executeNode.id);
console.log("  calls", (callsToPort ?? callsToSqlite).from, "->", (callsToPort ?? callsToSqlite).to);
console.log("  node", sqliteInsertNode.id);
console.log("  implements", implEdge.from, "->", implEdge.to);
if (callsToPort) console.log("  (calls target is port, preferred)");
else console.log("  (calls target is sqlite impl; checker did not resolve to port)");
