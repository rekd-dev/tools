export interface Entity {
  id: string
  name: string
}

export function newEntity(id: string, name: string): Entity {
  return { id, name }
}
