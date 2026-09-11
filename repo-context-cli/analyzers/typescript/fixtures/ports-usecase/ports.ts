export interface TimeSessionRepository {
  insertSession(id: string, startedAt: number): void;
  findById(id: string): string | undefined;
}
