import type { EntityStore } from './ports'

export class CreateHandler {
  constructor(private store: EntityStore) {}
}
