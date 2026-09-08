export interface Entity {
  id: string
  name: string
}

// Planted purity violation: domain must not import better-sqlite3
import Database from 'better-sqlite3'

export function loadEntity(id: string): Entity {
  const db = new Database(':memory:')
  return { id, name: String(db) }
}
