import { describe, expect, it } from 'vitest'
import { findingInView, findingTouchesNode, nodesTouchedByFinding } from './findings'
import type { Finding, ViewNode } from './types'

const cookie: ViewNode = {
  id: 'cookie',
  label: 'cookie',
  layer: 0,
  leaf: true,
  abstract: false,
  cycle: false,
  path: 'packages/auth/src/cookie.ts',
  fileRoot: 'packages/auth/src/cookie.ts',
}

const jwt: ViewNode = {
  ...cookie,
  id: 'jwt',
  label: 'jwt',
  path: 'packages/auth/src/jwt.ts',
  fileRoot: 'packages/auth/src/jwt.ts',
}

describe('finding scope', () => {
  it('keeps package files and drops substring false positives', () => {
    const real: Finding = {
      id: '1',
      kind: 'unlayered',
      severity: 'warning',
      message: 'unlayered',
      from: 'packages/auth/src/cookie.ts',
    }
    const other: Finding = {
      id: '2',
      kind: 'layer_violation',
      severity: 'error',
      message: 'depends',
      from: 'apps/schedule-api/src/http/routes/auth.ts',
      to: 'apps/schedule-api/src/composition/container.ts',
    }
    expect(findingInView(real, 'packages/auth/src', [cookie, jwt])).toBe(true)
    expect(findingInView(other, 'packages/auth/src', [cookie, jwt])).toBe(false)
  })

  it('selecting a file only keeps that file', () => {
    const cookieHit: Finding = {
      id: '1',
      kind: 'unlayered',
      severity: 'warning',
      message: 'unlayered',
      from: 'packages/auth/src/cookie.ts',
    }
    const jwtHit: Finding = {
      id: '2',
      kind: 'unlayered',
      severity: 'warning',
      message: 'unlayered',
      from: 'packages/auth/src/jwt.ts',
    }
    expect(findingTouchesNode(cookieHit, cookie)).toBe(true)
    expect(findingTouchesNode(jwtHit, cookie)).toBe(false)
  })

  it('lists the boxes a finding touches', () => {
    const hit: Finding = {
      id: '1',
      kind: 'unlayered',
      severity: 'warning',
      message: 'unlayered',
      from: 'packages/auth/src/cookie.ts',
    }
    expect(nodesTouchedByFinding(hit, [cookie, jwt]).map((n) => n.id)).toEqual(['cookie'])
  })
})
