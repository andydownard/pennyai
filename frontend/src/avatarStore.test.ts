import { describe, it, expect } from 'vitest'
import { initialsOf } from './avatarStore'

describe('initialsOf', () => {
  it('uses at most two initials', () => {
    expect(initialsOf('Andy', 'Downard')).toBe('AD')
    expect(initialsOf('Andy Downard', '')).toBe('AD')
    expect(initialsOf('Mary Ann', 'Smith')).toBe('MS')
    expect(initialsOf('cher', '')).toBe('C')
    expect(initialsOf('', '')).toBe('')
  })
})
