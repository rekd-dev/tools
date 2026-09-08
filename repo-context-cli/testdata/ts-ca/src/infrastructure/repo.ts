import type { Entity } from '../domain/entity'
import type { EntityStore } from '../application/ports'

export const memoryStore: EntityStore = {
  save(_e: Entity): void {},
}
