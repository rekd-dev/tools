import type { Entity } from '../domain/entity'
import { saveEntity } from '../infrastructure/repo'

export function createEntity(name: string): Entity {
  const e = { id: '1', name }
  saveEntity(e)
  return e
}
