import { newEntity, type Entity } from '../domain/entity'
import type { EntityStore } from './ports'

export function createEntity(store: EntityStore, name: string): Entity {
  const e = newEntity('1', name)
  store.save(e)
  return e
}
