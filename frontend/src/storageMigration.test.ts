import { describe, it, expect, beforeEach } from 'vitest'
import { migrateLegacyStorage } from './storageMigration'

describe('migrateLegacyStorage', () => {
  beforeEach(() => {
    localStorage.clear()
    sessionStorage.clear()
  })

  it('keeps every groflo_ key (regression: the rebrand version deleted them on each load)', () => {
    localStorage.setItem('groflo_theme', 'dark')
    localStorage.setItem('groflo_join_code', 'ABC123')
    sessionStorage.setItem('groflo_redirect', '/penny')
    migrateLegacyStorage()
    expect(localStorage.getItem('groflo_theme')).toBe('dark')
    expect(localStorage.getItem('groflo_join_code')).toBe('ABC123')
    expect(sessionStorage.getItem('groflo_redirect')).toBe('/penny')
  })

  it('renames thriftly_ keys to groflo_ in both stores', () => {
    localStorage.setItem('thriftly_install_dismissed', '1')
    sessionStorage.setItem('thriftly_redirect', '/saved-lists')
    migrateLegacyStorage()
    expect(localStorage.getItem('groflo_install_dismissed')).toBe('1')
    expect(localStorage.getItem('thriftly_install_dismissed')).toBeNull()
    expect(sessionStorage.getItem('groflo_redirect')).toBe('/saved-lists')
    expect(sessionStorage.getItem('thriftly_redirect')).toBeNull()
  })

  it('never overwrites a newer groflo_ value with a legacy one', () => {
    localStorage.setItem('groflo_theme', 'light')
    localStorage.setItem('thriftly_theme', 'dark')
    migrateLegacyStorage()
    expect(localStorage.getItem('groflo_theme')).toBe('light')
    expect(localStorage.getItem('thriftly_theme')).toBeNull()
  })

  it('leaves unrelated keys alone and is safe to run twice', () => {
    localStorage.setItem('penny_session_count', '3')
    localStorage.setItem('thriftly_trip_dismissed', 'x')
    migrateLegacyStorage()
    migrateLegacyStorage()
    expect(localStorage.getItem('penny_session_count')).toBe('3')
    expect(localStorage.getItem('groflo_trip_dismissed')).toBe('x')
  })

  it('does not throw when storage is unavailable', () => {
    const broken = { get length(): number { throw new Error('blocked') } } as unknown as Storage
    expect(() => migrateLegacyStorage([broken])).not.toThrow()
  })
})
