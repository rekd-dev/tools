import type { Entity } from '../domain/entity'

export interface EntityStore {
  save(e: Entity): void
}
