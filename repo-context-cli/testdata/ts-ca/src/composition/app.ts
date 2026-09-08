import { handleCreate } from '../http/routes'
import { memoryStore } from '../infrastructure/repo'

export function appCreate(name: string) {
  return handleCreate(memoryStore, name)
}
