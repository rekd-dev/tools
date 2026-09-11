import { TimeSessionRepository } from "./ports";

export class ClockInUseCase {
  constructor(private readonly sessions: TimeSessionRepository) {}

  execute(): void {
    this.sessions.insertSession("s1", 0);
  }
}
