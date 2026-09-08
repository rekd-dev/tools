import { createEntity } from '../application/create'

export function handleCreate(name: string) {
  return createEntity(name)
}
