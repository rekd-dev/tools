import type { Entity } from '../domain/entity'
// Planted cycle edge: infrastructure -> application
import { createEntity } from '../application/create'

export function saveEntity(e: Entity): void {
  void createEntity(e.name)
}
