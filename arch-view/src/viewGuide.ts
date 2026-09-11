import type { Lens } from './types'
import type { ViewerState } from './state'

export type GuideContext = {
  lens: Lens
  view: ViewerState['view']
  path: string
  sel: string
  overlays: ViewerState['overlays']
  empty: boolean
  truncated?: boolean
  selectedKind?: string
  selectedLabel?: string
  picked?: boolean
}

export type ViewGuide = {
  lookingAt: string
  meaning: string
  lookFor: string
}

export function viewGuide(ctx: GuideContext): ViewGuide {
  const place = placeLabel(ctx)
  const lookingAt = ctx.selectedLabel && !ctx.empty ? `${place} · ${ctx.selectedLabel}` : place
  const meaning = meaningFor(ctx)
  const lookFor = [lookForFor(ctx), ...overlayNotes(ctx)].filter(Boolean).join(' ')
  return { lookingAt, meaning, lookFor }
}

function placeLabel(ctx: GuideContext): string {
  if (ctx.lens === 'architecture') {
    if (ctx.view === 'matrix') return ctx.path ? `Architecture · matrix · ${ctx.path}` : 'Architecture · matrix'
    return ctx.path ? `Architecture · ${ctx.path}` : 'Architecture · all modules'
  }
  if (ctx.lens === 'focus') return 'Focus'
  return 'Flow'
}

function meaningFor(ctx: GuideContext): string {
  if (ctx.lens === 'architecture') {
    if (ctx.view === 'matrix') return 'A dependency matrix: each row depends on the columns it marks.'
    if (ctx.path) return 'You are inside this folder. Outer rows depend on the rows below.'
    return 'How the repo is packaged: apps that share a name sit together, outer layers depending on inner ones.'
  }
  if (ctx.lens === 'focus') {
    if (ctx.empty) return 'Neighborhood around one type: who implements it, who injects it, who calls it.'
    if (ctx.selectedKind === 'endpoint') return 'Direct neighbors of this route — not the whole call path.'
    return 'Direct neighbors of this type — not the whole call path.'
  }
  if (ctx.empty) return 'A bounded static path from one endpoint or function.'
  return 'Left to right is the call and persistence spine. Clock and other constructor wiring stay hidden unless Deps is on.'
}

function lookForFor(ctx: GuideContext): string {
  if (ctx.lens === 'architecture') {
    if (ctx.empty) return 'Nothing under this path. Run inventory and serve, then refresh.'
    if (ctx.view === 'matrix') {
      return 'Marks below the diagonal are expected. Marks above it, and red, are cycles. Click a mark for the import.'
    }
    if (ctx.path) {
      return 'Who this folder talks to, red borders (cycles or layer errors), and whether interfaces live here or one level in. Double-click a folder to go deeper.'
    }
    return 'Shared app names, red borders, and arrows that jump the wrong way. Double-click a folder to go in. Right-click for Focus or Flow.'
  }

  if (ctx.lens === 'focus') {
    if (ctx.empty) return 'Search for an interface or class, or right-click a box in Architecture and choose Focus.'
    if (ctx.selectedKind === 'interface') {
      return 'Implementers and DI bindings first, then who injects or calls this port. Click a neighbor to move the center.'
    }
    if (ctx.selectedKind === 'endpoint') {
      return 'Who handles the route, and the file that contains it. Use Flow when you want the path through use cases and adapters.'
    }
    return 'Implementers, injects, and callers. Click a neighbor to move the center. Use Flow when you want the path, not the neighborhood.'
  }

  if (ctx.empty) return 'Search for an HTTP route or use case, or right-click a box and choose Flow.'
  if (ctx.truncated) return 'This path hit the node cap. Pick a more specific symbol, or turn overlays off.'
  if (ctx.selectedKind === 'endpoint') {
    return 'Does the route reach a use case, then ports, then adapters? A gap is missing resolution, not missing code.'
  }
  return 'Follow calls into ports, then adapters on the right. A gap is missing resolution, not missing code.'
}

function overlayNotes(ctx: GuideContext): string[] {
  if (ctx.empty) return []
  const notes: string[] = []
  if (ctx.lens === 'flow') {
    if (ctx.overlays.data) notes.push('Data adds the adapter write sink on the right.')
    if (ctx.overlays.async) notes.push('Async shows awaits and forks off the spine — background work, not the return path.')
    if (ctx.overlays.deps) notes.push('Deps brings back Clock, notifications, and other constructor wiring.')
    if (ctx.overlays.unresolved) notes.push('Unresolved shows dashed guesses and library calls the default path hides.')
  }
  if (ctx.lens === 'architecture' && ctx.view !== 'matrix' && ctx.overlays.churn) {
    notes.push('Orange heat is 90-day git churn — activity, not a verdict.')
  }
  if (ctx.picked) notes.push('The inspector has the selected line — that is the relation, not another box.')
  return notes
}

export function guideKind(sel: string, focus?: { kind?: string; abstract?: boolean; leaf?: boolean } | null): string {
  if (sel.startsWith('endpoint:')) return 'endpoint'
  if (sel.startsWith('method:')) return 'method'
  if (sel.startsWith('type:')) return focus?.abstract ? 'interface' : 'type'
  if (sel.startsWith('file:')) return 'file'
  if (focus?.kind === 'type') return focus.abstract ? 'interface' : 'type'
  if (focus?.kind === 'method') return 'method'
  if (focus?.kind === 'endpoint') return 'endpoint'
  if (focus?.leaf) return 'file'
  if (focus) return 'folder'
  return ''
}
