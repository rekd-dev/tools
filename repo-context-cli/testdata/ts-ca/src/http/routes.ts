import { createEntity } from '../application/create'
import type { EntityStore } from '../application/ports'

export function handleCreate(store: EntityStore, name: string) {
  return createEntity(store, name)
}
