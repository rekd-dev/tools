import { TimeSessionRepository } from "./ports";

export class SqliteTimeSessionRepository implements TimeSessionRepository {
  db = { run(_sql: string) {}, get(_sql: string) { return undefined; } };

  insertSession(id: string, startedAt: number): void {
    this.db.run("INSERT INTO time_sessions VALUES (?, ?)");
  }

  findById(id: string): string | undefined {
    return this.db.get("SELECT id FROM time_sessions WHERE id = ?");
  }
}
